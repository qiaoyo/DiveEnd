package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type pendingDatabaseRestore struct {
	StagedPath  string    `json:"stagedPath"`
	BackupPath  string    `json:"backupPath"`
	RemotePath  string    `json:"remotePath"`
	ScheduledAt time.Time `json:"scheduledAt"`
}

const databaseRestoreMarkerLimitBytes int64 = 64 * 1024

func scheduleDatabaseRestore(dataPath, stagedPath, remotePath string) (*DatabaseRestoreStatus, error) {
	dataPath = normalizeDataPath(dataPath, defaultAppConfig().DataPath)
	if err := ensureDatabaseRestoreDir(dataPath); err != nil {
		return nil, err
	}
	stagedPath = strings.TrimSpace(stagedPath)
	if stagedPath == "" {
		return nil, fmt.Errorf("staged database path is empty")
	}
	managedStagedPath, err := validateExistingDatabaseRestorePath(dataPath, stagedPath, "staged database")
	if err != nil {
		return nil, err
	}
	stagedPath = managedStagedPath
	if err := validateSQLiteDatabase(stagedPath); err != nil {
		return nil, fmt.Errorf("staged database validation failed: %w", err)
	}

	backupPath := filepath.Join(databaseRestoreDir(dataPath), fmt.Sprintf("diveend-local-backup-%s.db", time.Now().Format("20060102-150405")))
	pending := pendingDatabaseRestore{
		StagedPath:  stagedPath,
		BackupPath:  backupPath,
		RemotePath:  strings.TrimSpace(remotePath),
		ScheduledAt: time.Now(),
	}

	markerPath := databaseRestoreMarkerPath(dataPath)
	payload, err := json.MarshalIndent(pending, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomic(markerPath, payload, 0600); err != nil {
		return nil, err
	}

	return databaseRestoreStatusFromPending(pending, "云端数据库已下载并等待恢复。可以立即应用，或重启应用后自动应用。"), nil
}

func getPendingDatabaseRestore(dataPath string) (*DatabaseRestoreStatus, error) {
	pending, err := readPendingDatabaseRestore(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &DatabaseRestoreStatus{Pending: false}, nil
		}
		return nil, err
	}
	return databaseRestoreStatusFromPending(pending, "云端数据库恢复正在等待应用。"), nil
}

func applyPendingDatabaseRestore(dataPath string) (*DatabaseRestoreStatus, error) {
	dataPath = normalizeDataPath(dataPath, defaultAppConfig().DataPath)
	pending, err := readPendingDatabaseRestore(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &DatabaseRestoreStatus{Pending: false, Message: "没有待应用的数据库恢复。"}, nil
		}
		return nil, err
	}
	if err := validateSQLiteDatabase(pending.StagedPath); err != nil {
		return nil, fmt.Errorf("staged database validation failed: %w", err)
	}

	dbPath := filepath.Join(dataPath, "diveend.db")
	if err := ensurePlainDirectory(filepath.Dir(dbPath)); err != nil {
		return nil, err
	}
	if err := ensurePlainDirectory(filepath.Dir(pending.BackupPath)); err != nil {
		return nil, err
	}

	if info, err := os.Stat(dbPath); err == nil && !info.IsDir() {
		if err := copyFileAtomic(dbPath, pending.BackupPath, 0600); err != nil {
			return nil, fmt.Errorf("failed to backup current database: %w", err)
		}
	}

	if err := copyFileAtomic(pending.StagedPath, dbPath, 0600); err != nil {
		return nil, fmt.Errorf("failed to replace database: %w", err)
	}
	if err := validateSQLiteDatabase(dbPath); err != nil {
		return nil, fmt.Errorf("restored database validation failed: %w", err)
	}
	if err := removeFileAndSyncDir(databaseRestoreMarkerPath(dataPath)); err != nil {
		return nil, fmt.Errorf("failed to clear pending database restore marker: %w", err)
	}

	return &DatabaseRestoreStatus{
		Pending:     false,
		Applied:     true,
		StagedPath:  pending.StagedPath,
		BackupPath:  pending.BackupPath,
		RemotePath:  pending.RemotePath,
		ScheduledAt: pending.ScheduledAt,
		AppliedAt:   time.Now(),
		Message:     "云端数据库已应用，本地旧数据库已备份。",
	}, nil
}

func cancelPendingDatabaseRestore(dataPath string) error {
	dataPath = normalizeDataPath(dataPath, defaultAppConfig().DataPath)
	if err := ensureDatabaseRestoreDirNotSymlink(dataPath); err != nil {
		return err
	}
	err := removeFileAndSyncDir(databaseRestoreMarkerPath(dataPath))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func readPendingDatabaseRestore(dataPath string) (pendingDatabaseRestore, error) {
	dataPath = normalizeDataPath(dataPath, defaultAppConfig().DataPath)
	if err := ensureDatabaseRestoreDirNotSymlink(dataPath); err != nil {
		return pendingDatabaseRestore{}, err
	}
	data, err := readLimitedFile(databaseRestoreMarkerPath(dataPath), databaseRestoreMarkerLimitBytes)
	if err != nil {
		return pendingDatabaseRestore{}, err
	}
	var pending pendingDatabaseRestore
	if err := json.Unmarshal(data, &pending); err != nil {
		return pendingDatabaseRestore{}, err
	}
	pending.StagedPath = strings.TrimSpace(pending.StagedPath)
	if pending.StagedPath == "" {
		return pendingDatabaseRestore{}, fmt.Errorf("pending database restore marker is missing stagedPath")
	}
	pending.StagedPath, err = validateExistingDatabaseRestorePath(dataPath, pending.StagedPath, "pending staged database")
	if err != nil {
		return pendingDatabaseRestore{}, err
	}
	if pending.BackupPath == "" {
		pending.BackupPath = filepath.Join(databaseRestoreDir(dataPath), fmt.Sprintf("diveend-local-backup-%s.db", time.Now().Format("20060102-150405")))
	}
	pending.BackupPath, err = validateDatabaseRestoreTargetPath(dataPath, pending.BackupPath, "pending backup database")
	if err != nil {
		return pendingDatabaseRestore{}, err
	}
	return pending, nil
}

func databaseRestoreStatusFromPending(pending pendingDatabaseRestore, message string) *DatabaseRestoreStatus {
	return &DatabaseRestoreStatus{
		Pending:     true,
		Applied:     false,
		StagedPath:  pending.StagedPath,
		BackupPath:  pending.BackupPath,
		RemotePath:  pending.RemotePath,
		ScheduledAt: pending.ScheduledAt,
		Message:     message,
	}
}

func databaseRestoreMarkerPath(dataPath string) string {
	return filepath.Join(databaseRestoreDir(dataPath), "pending.json")
}

func databaseRestoreDir(dataPath string) string {
	dataPath = normalizeDataPath(dataPath, defaultAppConfig().DataPath)
	return filepath.Join(dataPath, ".sync-restore")
}

func ensureDatabaseRestoreDirNotSymlink(dataPath string) error {
	restoreDir := databaseRestoreDir(dataPath)
	if err := ensurePathHasNoSymlinkComponents(restoreDir, "database restore directory"); err != nil {
		return err
	}
	info, err := os.Lstat(restoreDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("database restore directory is a symbolic link")
	}
	if !info.IsDir() {
		return fmt.Errorf("database restore directory is not a directory")
	}
	return nil
}

func ensureDatabaseRestoreDir(dataPath string) error {
	dataPath = normalizeDataPath(dataPath, defaultAppConfig().DataPath)
	if err := ensureDatabaseRestoreDirNotSymlink(dataPath); err != nil {
		return err
	}
	if err := ensurePlainDirectory(databaseRestoreDir(dataPath)); err != nil {
		return err
	}
	return ensureDatabaseRestoreDirNotSymlink(dataPath)
}

func validateExistingDatabaseRestorePath(dataPath, candidatePath, label string) (string, error) {
	cleanPath, err := validateDatabaseRestorePathLexical(dataPath, candidatePath, label)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		return "", fmt.Errorf("%s is not available: %w", label, err)
	}
	if err := ensurePathWithinResolvedDatabaseRestoreDir(dataPath, resolvedPath, label); err != nil {
		return "", err
	}
	return cleanPath, nil
}

func validateDatabaseRestoreTargetPath(dataPath, candidatePath, label string) (string, error) {
	cleanPath, err := validateDatabaseRestorePathLexical(dataPath, candidatePath, label)
	if err != nil {
		return "", err
	}
	parentPath, err := filepath.EvalSymlinks(filepath.Dir(cleanPath))
	if err != nil {
		return "", fmt.Errorf("%s parent is not available: %w", label, err)
	}
	if err := ensurePathWithinResolvedDatabaseRestoreDir(dataPath, parentPath, label+" parent"); err != nil {
		return "", err
	}
	return cleanPath, nil
}

func validateDatabaseRestorePathLexical(dataPath, candidatePath, label string) (string, error) {
	candidatePath = strings.TrimSpace(candidatePath)
	if candidatePath == "" {
		return "", fmt.Errorf("%s path is empty", label)
	}
	if !filepath.IsAbs(candidatePath) {
		return "", fmt.Errorf("%s path must be absolute", label)
	}
	cleanPath := filepath.Clean(candidatePath)
	if err := ensurePathLexicallyWithinDatabaseRestoreDir(dataPath, cleanPath, label); err != nil {
		return "", err
	}
	if filepath.Clean(cleanPath) == filepath.Clean(databaseRestoreDir(dataPath)) {
		return "", fmt.Errorf("%s path must be a file inside the restore directory", label)
	}
	return cleanPath, nil
}

func ensurePathLexicallyWithinDatabaseRestoreDir(dataPath, candidatePath, label string) error {
	restoreDir := filepath.Clean(databaseRestoreDir(dataPath))
	return ensurePathWithinDirectory(restoreDir, candidatePath, label)
}

func ensurePathWithinResolvedDatabaseRestoreDir(dataPath, candidatePath, label string) error {
	restoreDir := filepath.Clean(databaseRestoreDir(dataPath))
	resolvedRestoreDir, err := filepath.EvalSymlinks(restoreDir)
	if err != nil {
		return fmt.Errorf("database restore directory is not available: %w", err)
	}
	return ensurePathWithinDirectory(resolvedRestoreDir, candidatePath, label)
}

func ensurePathWithinDirectory(rootPath, candidatePath, label string) error {
	restoreDir := filepath.Clean(rootPath)
	rel, err := filepath.Rel(restoreDir, filepath.Clean(candidatePath))
	if err != nil {
		return fmt.Errorf("failed to validate %s path: %w", label, err)
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("%s path escapes database restore directory", label)
	}
	return nil
}

func validateSQLiteDatabase(dbPath string) error {
	info, err := os.Stat(dbPath)
	if err != nil {
		return err
	}
	if info.IsDir() || info.Size() == 0 {
		return fmt.Errorf("database file is not a non-empty file")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	var result string
	if err := db.QueryRow("PRAGMA quick_check").Scan(&result); err != nil {
		return err
	}
	if strings.TrimSpace(strings.ToLower(result)) != "ok" {
		return fmt.Errorf("quick_check returned %q", result)
	}
	return nil
}

func copyFileAtomic(sourcePath, targetPath string, mode os.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	return writeFileAtomicWithWriter(targetPath, mode, func(tmp *os.File) error {
		_, err := io.Copy(tmp, source)
		return err
	}, nil)
}
