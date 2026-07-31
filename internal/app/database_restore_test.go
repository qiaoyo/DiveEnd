package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabaseRestoreScheduleAndApply(t *testing.T) {
	localDir := t.TempDir()
	localDB, err := NewDB(localDir)
	if err != nil {
		t.Fatalf("NewDB(localDir) error = %v", err)
	}
	if _, err := localDB.conn.Exec(`CREATE TABLE local_marker (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create local marker error = %v", err)
	}
	if err := localDB.Close(); err != nil {
		t.Fatalf("Close(localDB) error = %v", err)
	}

	stagedDir := filepath.Join(databaseRestoreDir(localDir), "staged-fixture")
	stagedDB, err := NewDB(stagedDir)
	if err != nil {
		t.Fatalf("NewDB(stagedDir) error = %v", err)
	}
	if _, err := stagedDB.conn.Exec(`CREATE TABLE cloud_marker (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create cloud marker error = %v", err)
	}
	if err := stagedDB.Close(); err != nil {
		t.Fatalf("Close(stagedDB) error = %v", err)
	}
	stagedPath := filepath.Join(stagedDir, "diveend.db")

	scheduled, err := scheduleDatabaseRestore(localDir, stagedPath, "/apps/pcstest_oauth/diveend-v1/data/diveend.db")
	if err != nil {
		t.Fatalf("scheduleDatabaseRestore() error = %v", err)
	}
	if !scheduled.Pending {
		t.Fatalf("expected scheduled restore to be pending, got %+v", scheduled)
	}
	if _, err := os.Stat(databaseRestoreMarkerPath(localDir)); err != nil {
		t.Fatalf("expected restore marker to exist: %v", err)
	}
	assertFilePerm(t, databaseRestoreMarkerPath(localDir), 0600)
	assertNoGlobMatches(t, filepath.Join(filepath.Dir(databaseRestoreMarkerPath(localDir)), ".pending.json.tmp-*"))

	pending, err := getPendingDatabaseRestore(localDir)
	if err != nil {
		t.Fatalf("getPendingDatabaseRestore() error = %v", err)
	}
	if !pending.Pending || pending.StagedPath != stagedPath {
		t.Fatalf("unexpected pending restore status: %+v", pending)
	}

	applied, err := applyPendingDatabaseRestore(localDir)
	if err != nil {
		t.Fatalf("applyPendingDatabaseRestore() error = %v", err)
	}
	if !applied.Applied || applied.Pending {
		t.Fatalf("unexpected applied restore status: %+v", applied)
	}
	if _, err := os.Stat(applied.BackupPath); err != nil {
		t.Fatalf("expected local database backup to exist: %v", err)
	}
	assertFilePerm(t, applied.BackupPath, 0600)
	if _, err := os.Stat(databaseRestoreMarkerPath(localDir)); !os.IsNotExist(err) {
		t.Fatalf("expected restore marker to be removed, stat err=%v", err)
	}
	restoredPath := filepath.Join(localDir, "diveend.db")
	assertFilePerm(t, restoredPath, 0600)
	assertNoGlobMatches(t, filepath.Join(localDir, ".diveend.db.tmp-*"))
	assertNoGlobMatches(t, filepath.Join(localDir, "diveend.db.restore-tmp"))

	restoredDB, err := NewDB(localDir)
	if err != nil {
		t.Fatalf("NewDB(restored localDir) error = %v", err)
	}
	defer restoredDB.Close()
	var tableName string
	if err := restoredDB.conn.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'cloud_marker'`).Scan(&tableName); err != nil {
		t.Fatalf("expected restored database to contain cloud marker table: %v", err)
	}
}

func TestDatabaseRestoreNoPending(t *testing.T) {
	status, err := getPendingDatabaseRestore(t.TempDir())
	if err != nil {
		t.Fatalf("getPendingDatabaseRestore() error = %v", err)
	}
	if status.Pending {
		t.Fatalf("expected no pending restore, got %+v", status)
	}
}

func TestNewDBRejectsSymlinkedDataPath(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	dataPath := filepath.Join(tempDir, "data-link")
	if err := os.Symlink(outsideDir, dataPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	db, err := NewDB(dataPath)
	if db != nil {
		_ = db.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked data path error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "diveend.db")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no database to be created through symlinked data path, stat err=%v", statErr)
	}
}

func TestScheduleDatabaseRestoreRejectsSymlinkedDataPath(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	dataPath := filepath.Join(tempDir, "data-link")
	if err := os.Symlink(outsideDir, dataPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := scheduleDatabaseRestore(
		dataPath,
		filepath.Join(dataPath, ".sync-restore", "staged.db"),
		"/apps/pcstest_oauth/diveend-v1/data/diveend.db",
	)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked data path error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, ".sync-restore")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no restore directory to be created through symlinked data path, stat err=%v", statErr)
	}
}

func TestCancelPendingDatabaseRestoreRemovesMarker(t *testing.T) {
	localDir := t.TempDir()
	stagedDir := filepath.Join(databaseRestoreDir(localDir), "staged-fixture")
	stagedDB, err := NewDB(stagedDir)
	if err != nil {
		t.Fatalf("NewDB(stagedDir) error = %v", err)
	}
	if err := stagedDB.Close(); err != nil {
		t.Fatalf("Close(stagedDB) error = %v", err)
	}

	if _, err := scheduleDatabaseRestore(localDir, filepath.Join(stagedDir, "diveend.db"), "/apps/pcstest_oauth/diveend-v1/data/diveend.db"); err != nil {
		t.Fatalf("scheduleDatabaseRestore() error = %v", err)
	}
	if err := cancelPendingDatabaseRestore(localDir); err != nil {
		t.Fatalf("cancelPendingDatabaseRestore() error = %v", err)
	}
	if _, err := os.Stat(databaseRestoreMarkerPath(localDir)); !os.IsNotExist(err) {
		t.Fatalf("expected restore marker to be removed, stat err=%v", err)
	}
}

func TestScheduleDatabaseRestoreRejectsExternalStagedPath(t *testing.T) {
	localDir := t.TempDir()
	stagedDir := t.TempDir()
	stagedDB, err := NewDB(stagedDir)
	if err != nil {
		t.Fatalf("NewDB(stagedDir) error = %v", err)
	}
	if err := stagedDB.Close(); err != nil {
		t.Fatalf("Close(stagedDB) error = %v", err)
	}

	_, err = scheduleDatabaseRestore(localDir, filepath.Join(stagedDir, "diveend.db"), "/apps/pcstest_oauth/diveend-v1/data/diveend.db")
	if err == nil {
		t.Fatal("expected external staged database path to be rejected")
	}
	if _, statErr := os.Stat(databaseRestoreMarkerPath(localDir)); !os.IsNotExist(statErr) {
		t.Fatalf("expected no restore marker after rejected schedule, stat err=%v", statErr)
	}
}

func TestPendingDatabaseRestoreRejectsTamperedPathEscape(t *testing.T) {
	localDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideDB, err := NewDB(outsideDir)
	if err != nil {
		t.Fatalf("NewDB(outsideDir) error = %v", err)
	}
	if err := outsideDB.Close(); err != nil {
		t.Fatalf("Close(outsideDB) error = %v", err)
	}

	writePendingRestoreMarker(t, localDir, pendingDatabaseRestore{
		StagedPath: filepath.Join(outsideDir, "diveend.db"),
		BackupPath: filepath.Join(databaseRestoreDir(localDir), "backup.db"),
	})

	if _, err := getPendingDatabaseRestore(localDir); err == nil {
		t.Fatal("expected tampered staged path outside restore directory to be rejected")
	}
	if _, err := applyPendingDatabaseRestore(localDir); err == nil {
		t.Fatal("expected apply to reject tampered staged path outside restore directory")
	}
}

func TestPendingDatabaseRestoreRejectsSymlinkBackupEscape(t *testing.T) {
	localDir := t.TempDir()
	stagedDir := filepath.Join(databaseRestoreDir(localDir), "staged-fixture")
	stagedDB, err := NewDB(stagedDir)
	if err != nil {
		t.Fatalf("NewDB(stagedDir) error = %v", err)
	}
	if err := stagedDB.Close(); err != nil {
		t.Fatalf("Close(stagedDB) error = %v", err)
	}

	outsideDir := t.TempDir()
	symlinkDir := filepath.Join(databaseRestoreDir(localDir), "escape")
	if err := os.Symlink(outsideDir, symlinkDir); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	writePendingRestoreMarker(t, localDir, pendingDatabaseRestore{
		StagedPath: filepath.Join(stagedDir, "diveend.db"),
		BackupPath: filepath.Join(symlinkDir, "backup.db"),
	})

	if _, err := getPendingDatabaseRestore(localDir); err == nil {
		t.Fatal("expected symlinked backup parent outside restore directory to be rejected")
	}
	if _, err := applyPendingDatabaseRestore(localDir); err == nil {
		t.Fatal("expected apply to reject symlinked backup parent outside restore directory")
	}
	if _, err := os.Stat(filepath.Join(outsideDir, "backup.db")); !os.IsNotExist(err) {
		t.Fatalf("expected no backup file outside restore directory, stat err=%v", err)
	}
}

func TestPendingDatabaseRestoreRejectsOversizedMarker(t *testing.T) {
	localDir := t.TempDir()
	markerPath := databaseRestoreMarkerPath(localDir)
	if err := os.MkdirAll(filepath.Dir(markerPath), 0700); err != nil {
		t.Fatalf("MkdirAll restore marker dir error = %v", err)
	}
	if err := os.WriteFile(markerPath, []byte(strings.Repeat(" ", int(databaseRestoreMarkerLimitBytes)+1)), 0600); err != nil {
		t.Fatalf("WriteFile oversized marker error = %v", err)
	}

	if _, err := getPendingDatabaseRestore(localDir); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized pending marker to be rejected, got %v", err)
	}
	if _, err := applyPendingDatabaseRestore(localDir); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected apply to reject oversized pending marker, got %v", err)
	}
}

func TestPendingDatabaseRestoreRejectsSymlinkedMarker(t *testing.T) {
	localDir := t.TempDir()
	markerPath := databaseRestoreMarkerPath(localDir)
	if err := os.MkdirAll(filepath.Dir(markerPath), 0700); err != nil {
		t.Fatalf("MkdirAll restore marker dir error = %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "pending.json")
	if err := os.WriteFile(targetPath, []byte(`{}`), 0600); err != nil {
		t.Fatalf("WriteFile symlink target marker error = %v", err)
	}
	if err := os.Symlink(targetPath, markerPath); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	if _, err := getPendingDatabaseRestore(localDir); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked pending marker to be rejected, got %v", err)
	}
	if _, err := applyPendingDatabaseRestore(localDir); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected apply to reject symlinked pending marker, got %v", err)
	}
}

func TestDatabaseRestoreRejectsSymlinkedRestoreDirectory(t *testing.T) {
	localDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.Symlink(outsideDir, databaseRestoreDir(localDir)); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	if _, err := getPendingDatabaseRestore(localDir); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked restore directory to be rejected, got %v", err)
	}
	if err := cancelPendingDatabaseRestore(localDir); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected cancel to reject symlinked restore directory, got %v", err)
	}

	stagedDir := filepath.Join(databaseRestoreDir(localDir), "staged-fixture")
	_, err := scheduleDatabaseRestore(localDir, filepath.Join(stagedDir, "diveend.db"), "/apps/pcstest_oauth/diveend-v1/data/diveend.db")
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected schedule to reject symlinked restore directory, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outsideDir, "pending.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no pending marker to be written outside restore directory, stat err=%v", err)
	}
}

func TestCopyFileAtomicFailurePreservesTarget(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.db")
	original := []byte("original-content")
	if err := os.WriteFile(targetPath, original, 0600); err != nil {
		t.Fatalf("write target fixture error = %v", err)
	}

	err := copyFileAtomic(filepath.Join(dir, "missing-source.db"), targetPath, 0600)
	if err == nil {
		t.Fatal("expected copyFileAtomic to fail for missing source")
	}

	data, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read target after failed copy error = %v", readErr)
	}
	if string(data) != string(original) {
		t.Fatalf("target content changed after failed copy: got %q want %q", string(data), string(original))
	}
	assertNoGlobMatches(t, filepath.Join(dir, ".target.db.tmp-*"))
}

func writePendingRestoreMarker(t *testing.T, dataPath string, pending pendingDatabaseRestore) {
	t.Helper()

	payload, err := json.Marshal(pending)
	if err != nil {
		t.Fatalf("marshal pending marker error = %v", err)
	}
	if err := writeFileAtomic(databaseRestoreMarkerPath(dataPath), payload, 0600); err != nil {
		t.Fatalf("write pending marker error = %v", err)
	}
}

func assertFilePerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("unexpected mode for %s: got %v want %v", path, got, want)
	}
}

func assertNoGlobMatches(t *testing.T, pattern string) {
	t.Helper()

	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s error = %v", pattern, err)
	}
	if len(matches) > 0 {
		t.Fatalf("expected no matches for %s, got %v", pattern, matches)
	}
}
