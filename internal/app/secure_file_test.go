package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomicRejectsSymlinkedTargetDirectory(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target-dir")
	if err := os.Symlink(outsideDir, targetDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := writeFileAtomic(filepath.Join(targetDir, "secret.json"), []byte(`{"token":"secret"}`), 0600)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked target directory error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "secret.json")); !os.IsNotExist(statErr) {
		t.Fatalf("expected atomic write not to write through symlinked target directory, stat err=%v", statErr)
	}
}

func TestWriteFileAtomicRejectsIntermediateSymlinkedTargetDirectory(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	linkDir := filepath.Join(tempDir, "link")
	if err := os.Symlink(outsideDir, linkDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := writeFileAtomic(filepath.Join(linkDir, "nested", "secret.json"), []byte(`{"token":"secret"}`), 0600)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected intermediate symlinked target directory error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "nested", "secret.json")); !os.IsNotExist(statErr) {
		t.Fatalf("expected atomic write not to write through intermediate symlink, stat err=%v", statErr)
	}
}

func TestWriteFileAtomicRejectsDirectorySwapDuringWrite(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target-dir")
	movedDir := filepath.Join(tempDir, "target-dir-original")
	if err := os.Mkdir(targetDir, 0700); err != nil {
		t.Fatalf("Mkdir target dir error = %v", err)
	}

	err := writeFileAtomicWithWriter(filepath.Join(targetDir, "secret.json"), 0600, func(tmp *os.File) error {
		if err := os.Rename(targetDir, movedDir); err != nil {
			t.Fatalf("Rename target dir during write error = %v", err)
		}
		if err := os.Symlink(outsideDir, targetDir); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		_, err := tmp.Write([]byte(`{"token":"secret"}`))
		return err
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "target directory changed") {
		t.Fatalf("expected directory swap error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "secret.json")); !os.IsNotExist(statErr) {
		t.Fatalf("expected atomic write not to write through swapped symlink directory, stat err=%v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(movedDir, "secret.json")); !os.IsNotExist(statErr) {
		t.Fatalf("expected failed atomic write to clean original directory temp/final file, stat err=%v", statErr)
	}
}

func TestRenameFileAtomicWithinDirRejectsSymlinkedDirectory(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target-dir")
	if err := os.WriteFile(filepath.Join(outsideDir, "source.pdf"), []byte("%PDF-1.4\noutside\n"), 0600); err != nil {
		t.Fatalf("WriteFile outside source error = %v", err)
	}
	if err := os.Symlink(outsideDir, targetDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := renameFileAtomicWithinDir(filepath.Join(targetDir, "source.pdf"), filepath.Join(targetDir, "target.pdf"))
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked directory rename error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "target.pdf")); !os.IsNotExist(statErr) {
		t.Fatalf("expected rename not to operate through symlinked directory, stat err=%v", statErr)
	}
}

func TestRemoveFileAndSyncDirRejectsSymlinkedDirectory(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target-dir")
	outsideFile := filepath.Join(outsideDir, "secret.json")
	if err := os.WriteFile(outsideFile, []byte(`{"token":"secret"}`), 0600); err != nil {
		t.Fatalf("WriteFile outside file error = %v", err)
	}
	if err := os.Symlink(outsideDir, targetDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := removeFileAndSyncDir(filepath.Join(targetDir, "secret.json"))
	if err == nil || (!strings.Contains(err.Error(), "symbolic link") && !strings.Contains(err.Error(), "not a directory")) {
		t.Fatalf("expected symlinked directory remove error, got %v", err)
	}
	if _, statErr := os.Stat(outsideFile); statErr != nil {
		t.Fatalf("expected external file to be preserved, stat err=%v", statErr)
	}
}

func TestRemoveTreeAndSyncParentRemovesNestedTree(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target-dir")
	nestedFile := filepath.Join(targetDir, "a", "b", "file.txt")
	if err := os.MkdirAll(filepath.Dir(nestedFile), 0700); err != nil {
		t.Fatalf("MkdirAll nested dir error = %v", err)
	}
	if err := os.WriteFile(nestedFile, []byte("content"), 0600); err != nil {
		t.Fatalf("WriteFile nested file error = %v", err)
	}

	if err := removeTreeAndSyncParent(targetDir); err != nil {
		t.Fatalf("removeTreeAndSyncParent() error = %v", err)
	}
	if _, statErr := os.Stat(targetDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected target tree to be removed, stat err=%v", statErr)
	}
}

func TestRemoveTreeAndSyncParentDoesNotFollowInternalSymlink(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "keep.txt")
	if err := os.WriteFile(outsideFile, []byte("keep"), 0600); err != nil {
		t.Fatalf("WriteFile outside file error = %v", err)
	}
	targetDir := filepath.Join(tempDir, "target-dir")
	if err := os.MkdirAll(filepath.Join(targetDir, "nested"), 0700); err != nil {
		t.Fatalf("MkdirAll target dir error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "nested", "owned.txt"), []byte("owned"), 0600); err != nil {
		t.Fatalf("WriteFile owned file error = %v", err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(targetDir, "nested", "outside-link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if err := removeTreeAndSyncParent(targetDir); err != nil {
		t.Fatalf("removeTreeAndSyncParent() error = %v", err)
	}
	if _, statErr := os.Stat(targetDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected target tree to be removed, stat err=%v", statErr)
	}
	if _, statErr := os.Stat(outsideFile); statErr != nil {
		t.Fatalf("expected symlink target file to be preserved, stat err=%v", statErr)
	}
}
