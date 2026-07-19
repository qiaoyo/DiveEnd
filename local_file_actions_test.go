package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttachLocalPDFToPaperCopiesValidPDFSecurely(t *testing.T) {
	app := testAppForImportAssets(t)
	paper := createDeepReadPaperWithPath(t, app, "attach-valid-paper", "")

	sourcePath := filepath.Join(t.TempDir(), "source.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nlocal attach\n"), 0644); err != nil {
		t.Fatalf("WriteFile source pdf error = %v", err)
	}

	updated, err := app.AttachLocalPDFToPaper(paper.ID, sourcePath)
	if err != nil {
		t.Fatalf("AttachLocalPDFToPaper() error = %v", err)
	}
	if strings.TrimSpace(updated.PDFPath) == "" {
		t.Fatal("expected attached paper to have managed pdf path")
	}
	expectedRoot := filepath.Join(app.config.DataPath, "papers")
	if !strings.HasPrefix(filepath.Clean(updated.PDFPath), filepath.Clean(expectedRoot)+string(os.PathSeparator)) {
		t.Fatalf("expected managed pdf path under %q, got %q", expectedRoot, updated.PDFPath)
	}
	content, err := os.ReadFile(updated.PDFPath)
	if err != nil {
		t.Fatalf("ReadFile managed pdf error = %v", err)
	}
	if !strings.HasPrefix(string(content), "%PDF-") {
		t.Fatalf("expected managed pdf content, got %q", string(content))
	}
	info, err := os.Stat(updated.PDFPath)
	if err != nil {
		t.Fatalf("Stat managed pdf error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected managed pdf permissions 0600, got %o", info.Mode().Perm())
	}
	tempMatches, err := filepath.Glob(filepath.Join(filepath.Dir(updated.PDFPath), "."+filepath.Base(updated.PDFPath)+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob temp pdf files error = %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected no leftover temp pdf files, got %v", tempMatches)
	}
}

func TestAttachLocalPDFToPaperSanitizesPaperIDFileName(t *testing.T) {
	app := testAppForImportAssets(t)
	paper := createDeepReadPaperWithPath(t, app, "../attach/escaped-paper", "")

	sourcePath := filepath.Join(t.TempDir(), "source.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nlocal attach escaped id\n"), 0600); err != nil {
		t.Fatalf("WriteFile source pdf error = %v", err)
	}

	updated, err := app.AttachLocalPDFToPaper(paper.ID, sourcePath)
	if err != nil {
		t.Fatalf("AttachLocalPDFToPaper() error = %v", err)
	}

	papersRoot := filepath.Join(app.config.DataPath, "papers")
	if _, ok := managedPathInsideRoot(papersRoot, updated.PDFPath); !ok {
		t.Fatalf("expected managed pdf path under %q, got %q", papersRoot, updated.PDFPath)
	}
	if filepath.Base(updated.PDFPath) != safePaperPDFFileName(paper.ID) {
		t.Fatalf("expected sanitized file name %q, got %q", safePaperPDFFileName(paper.ID), filepath.Base(updated.PDFPath))
	}
	unsafePath := filepath.Clean(filepath.Join(papersRoot, filepath.FromSlash(paper.ID)+".pdf"))
	if _, err := os.Stat(unsafePath); err == nil {
		t.Fatalf("unexpected unsafe pdf path was created at %q", unsafePath)
	}
}

func TestAttachLocalPDFToPaperRejectsFakePDFContent(t *testing.T) {
	app := testAppForImportAssets(t)
	paper := createDeepReadPaperWithPath(t, app, "attach-fake-paper", "")

	sourcePath := filepath.Join(t.TempDir(), "fake.pdf")
	if err := os.WriteFile(sourcePath, []byte("not actually a pdf"), 0600); err != nil {
		t.Fatalf("WriteFile fake pdf error = %v", err)
	}

	if _, err := app.AttachLocalPDFToPaper(paper.ID, sourcePath); err == nil {
		t.Fatal("expected AttachLocalPDFToPaper() to reject fake pdf content")
	}
	latest, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if strings.TrimSpace(latest.PDFPath) != "" {
		t.Fatalf("expected failed attach to preserve empty PDFPath, got %q", latest.PDFPath)
	}
}

func TestAttachLocalPDFToPaperRejectsSymlinkSource(t *testing.T) {
	app := testAppForImportAssets(t)
	paper := createDeepReadPaperWithPath(t, app, "attach-symlink-paper", "")

	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.pdf")
	if err := os.WriteFile(targetPath, []byte("%PDF-1.4\nsymlink target\n"), 0600); err != nil {
		t.Fatalf("WriteFile symlink target error = %v", err)
	}
	linkPath := filepath.Join(tempDir, "linked.pdf")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if _, err := app.AttachLocalPDFToPaper(paper.ID, linkPath); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlink source error, got %v", err)
	}
	latest, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if strings.TrimSpace(latest.PDFPath) != "" {
		t.Fatalf("expected failed attach to preserve empty PDFPath, got %q", latest.PDFPath)
	}
}

func TestAttachLocalPDFToPaperRejectsSymlinkedManagedFolder(t *testing.T) {
	app := testAppForImportAssets(t)

	folder, err := app.CreateFolder("Attach Symlink Target")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	paper := createDeepReadPaperWithPath(t, app, "attach-target-symlink-paper", "")
	paper.FolderID = folder.ID
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() with target folder error = %v", err)
	}

	sourcePath := filepath.Join(t.TempDir(), "source.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nlocal attach target symlink\n"), 0600); err != nil {
		t.Fatalf("WriteFile source pdf error = %v", err)
	}

	outsideDir := t.TempDir()
	papersRoot := filepath.Join(app.config.DataPath, "papers")
	targetDir := filepath.Join(papersRoot, filepath.FromSlash(normalizeFolderPath(folder.Path)))
	if err := os.MkdirAll(filepath.Dir(targetDir), 0700); err != nil {
		t.Fatalf("MkdirAll managed root error = %v", err)
	}
	if err := os.Symlink(outsideDir, targetDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if _, err := app.AttachLocalPDFToPaper(paper.ID, sourcePath); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked target folder error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, safePaperPDFFileName(paper.ID))); !os.IsNotExist(statErr) {
		t.Fatalf("expected no pdf to be written through symlinked target folder, stat err=%v", statErr)
	}
	latest, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if strings.TrimSpace(latest.PDFPath) != "" {
		t.Fatalf("expected failed attach to preserve empty PDFPath, got %q", latest.PDFPath)
	}
}

func TestAttachLocalPDFToPaperRejectsIntermediateSymlinkedManagedFolder(t *testing.T) {
	app := testAppForImportAssets(t)

	folder, err := app.CreateFolderNode(CreateFolderNodeRequest{Path: "Attach Parent/Attach Child"})
	if err != nil {
		t.Fatalf("CreateFolderNode() error = %v", err)
	}
	paper := createDeepReadPaperWithPath(t, app, "attach-intermediate-symlink-paper", "")
	paper.FolderID = folder.ID
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() with nested folder error = %v", err)
	}

	sourcePath := filepath.Join(t.TempDir(), "source.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nlocal attach intermediate symlink\n"), 0600); err != nil {
		t.Fatalf("WriteFile source pdf error = %v", err)
	}

	outsideDir := t.TempDir()
	papersRoot := filepath.Join(app.config.DataPath, "papers")
	intermediateDir := filepath.Join(papersRoot, "Attach Parent")
	if err := os.MkdirAll(filepath.Dir(intermediateDir), 0700); err != nil {
		t.Fatalf("MkdirAll managed root error = %v", err)
	}
	if err := os.Symlink(outsideDir, intermediateDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if _, err := app.AttachLocalPDFToPaper(paper.ID, sourcePath); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected intermediate symlinked managed folder error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "Attach Child", safePaperPDFFileName(paper.ID))); !os.IsNotExist(statErr) {
		t.Fatalf("expected no pdf to be written through intermediate symlinked folder, stat err=%v", statErr)
	}
	latest, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if strings.TrimSpace(latest.PDFPath) != "" {
		t.Fatalf("expected failed attach to preserve empty PDFPath, got %q", latest.PDFPath)
	}
}

func TestCopyLocalPDFRejectsSourceSwapDuringOpen(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\noriginal\n"), 0600); err != nil {
		t.Fatalf("WriteFile original source error = %v", err)
	}
	targetPath := filepath.Join(tempDir, "target.pdf")

	swapped := false
	err := copyLocalPDFWithOpen(sourcePath, targetPath, func(openPath string) (*os.File, error) {
		if !swapped {
			swapped = true
			if err := os.Remove(openPath); err != nil {
				t.Fatalf("Remove original source error = %v", err)
			}
			if err := os.WriteFile(openPath, []byte("not a pdf replacement"), 0600); err != nil {
				t.Fatalf("WriteFile replacement source error = %v", err)
			}
		}
		return os.Open(openPath)
	})
	if err == nil || !strings.Contains(err.Error(), "changed while opening") {
		t.Fatalf("expected source swap error, got %v", err)
	}
	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no target after rejected source swap, stat err=%v", statErr)
	}
	tempMatches, err := filepath.Glob(filepath.Join(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob temp pdf files error = %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected no leftover temp pdf files, got %v", tempMatches)
	}
}

func TestCopyLocalPDFRejectsFinalValidationFailure(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\noriginal\n"), 0600); err != nil {
		t.Fatalf("WriteFile source pdf error = %v", err)
	}
	targetPath := filepath.Join(tempDir, "target.pdf")

	err := copyLocalPDFWithOpenAndValidator(sourcePath, targetPath, os.Open, func(copiedPath string) error {
		if _, statErr := os.Stat(copiedPath); statErr != nil {
			t.Fatalf("expected copied temp file to exist before final validation: %v", statErr)
		}
		return errors.New("forced final validation failure")
	})
	if err == nil || !strings.Contains(err.Error(), "forced final validation failure") {
		t.Fatalf("expected final validation error, got %v", err)
	}
	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no target after failed final validation, stat err=%v", statErr)
	}
	tempMatches, err := filepath.Glob(filepath.Join(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob temp pdf files error = %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected no leftover temp pdf files, got %v", tempMatches)
	}
}

func TestCopyLocalPDFRejectsSymlinkedTargetDirectory(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nlocal source\n"), 0600); err != nil {
		t.Fatalf("WriteFile source pdf error = %v", err)
	}

	outsideDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target-dir")
	if err := os.Symlink(outsideDir, targetDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := copyLocalPDF(sourcePath, filepath.Join(targetDir, "target.pdf"))
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked target directory error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "target.pdf")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no pdf to be written through symlinked target directory, stat err=%v", statErr)
	}
}

func TestAttachLocalPDFToPaperRejectsOversizedPDF(t *testing.T) {
	app := testAppForImportAssets(t)
	paper := createDeepReadPaperWithPath(t, app, "attach-oversized-paper", "")

	oldMaxBytes := pdfDownloadMaxBytes
	pdfDownloadMaxBytes = int64(len("%PDF-1.4\n"))
	t.Cleanup(func() { pdfDownloadMaxBytes = oldMaxBytes })

	sourcePath := filepath.Join(t.TempDir(), "oversized.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\noversized local attach\n"), 0600); err != nil {
		t.Fatalf("WriteFile oversized pdf error = %v", err)
	}

	if _, err := app.AttachLocalPDFToPaper(paper.ID, sourcePath); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected oversized pdf error, got %v", err)
	}
	latest, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if strings.TrimSpace(latest.PDFPath) != "" {
		t.Fatalf("expected failed attach to preserve empty PDFPath, got %q", latest.PDFPath)
	}
}
