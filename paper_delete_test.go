package main

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeletePaperRemovesUnsharedManagedPDF(t *testing.T) {
	app := testAppForImportAssets(t)
	folder, err := app.CreateFolder("Delete Managed")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	pdfPath := writeManagedDeleteTestPDF(t, app, folder.Path, "delete-managed.pdf")
	paper := upsertDeleteTestPaper(t, app, "delete-managed-paper", folder.ID, pdfPath)

	if err := app.DeletePaper(paper.ID); err != nil {
		t.Fatalf("DeletePaper() error = %v", err)
	}
	if _, err := os.Stat(pdfPath); !os.IsNotExist(err) {
		t.Fatalf("expected managed PDF to be removed, stat err=%v", err)
	}
	if _, err := app.db.GetPaperByID(paper.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected paper row to be deleted, got %v", err)
	}
}

func TestDeletePaperPreservesSharedManagedPDF(t *testing.T) {
	app := testAppForImportAssets(t)
	folder, err := app.CreateFolder("Shared Managed")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	pdfPath := writeManagedDeleteTestPDF(t, app, folder.Path, "shared-managed.pdf")
	first := upsertDeleteTestPaper(t, app, "shared-managed-paper-a", folder.ID, pdfPath)
	second := upsertDeleteTestPaper(t, app, "shared-managed-paper-b", folder.ID, pdfPath)

	if err := app.DeletePaper(first.ID); err != nil {
		t.Fatalf("DeletePaper() error = %v", err)
	}
	if _, err := os.Stat(pdfPath); err != nil {
		t.Fatalf("expected shared managed PDF to remain, stat err=%v", err)
	}
	if _, err := app.db.GetPaperByID(first.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected first paper row to be deleted, got %v", err)
	}
	if _, err := app.db.GetPaperByID(second.ID); err != nil {
		t.Fatalf("expected second paper row to remain, got %v", err)
	}
}

func TestDeletePaperPreservesExternalPDF(t *testing.T) {
	app := testAppForImportAssets(t)
	folder, err := app.CreateFolder("External Delete")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	externalPDF := filepath.Join(t.TempDir(), "external.pdf")
	if err := os.WriteFile(externalPDF, []byte("%PDF-1.4\nexternal\n"), 0600); err != nil {
		t.Fatalf("WriteFile external PDF error = %v", err)
	}
	paper := upsertDeleteTestPaper(t, app, "external-delete-paper", folder.ID, externalPDF)

	if err := app.DeletePaper(paper.ID); err != nil {
		t.Fatalf("DeletePaper() error = %v", err)
	}
	if _, err := os.Stat(externalPDF); err != nil {
		t.Fatalf("expected external PDF to remain, stat err=%v", err)
	}
	if _, err := app.db.GetPaperByID(paper.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected paper row to be deleted, got %v", err)
	}
}

func TestDeletePaperRejectsSymlinkedManagedRootAndPreservesDB(t *testing.T) {
	app := testAppForImportAssets(t)
	folder, err := app.CreateFolder("Delete Symlink Guard")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	papersRoot := filepath.Join(app.config.DataPath, "papers")
	if err := os.RemoveAll(papersRoot); err != nil {
		t.Fatalf("RemoveAll papers root error = %v", err)
	}
	if err := os.Symlink(t.TempDir(), papersRoot); err != nil {
		t.Fatalf("Symlink papers root error = %v", err)
	}

	pdfPath := filepath.Join(papersRoot, folder.Path, "symlink-delete.pdf")
	paper := upsertDeleteTestPaper(t, app, "symlink-delete-paper", folder.ID, pdfPath)

	err = app.DeletePaper(paper.ID)
	if err == nil || !strings.Contains(err.Error(), "managed root directory is a symbolic link") {
		t.Fatalf("expected symlinked managed root error, got %v", err)
	}
	if _, err := app.db.GetPaperByID(paper.ID); err != nil {
		t.Fatalf("expected paper row to remain after failed delete, got %v", err)
	}
}

func writeManagedDeleteTestPDF(t *testing.T, app *App, folderPath, fileName string) string {
	t.Helper()
	pdfPath := filepath.Join(app.config.DataPath, "papers", filepath.FromSlash(normalizeFolderPath(folderPath)), fileName)
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0700); err != nil {
		t.Fatalf("MkdirAll managed PDF dir error = %v", err)
	}
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\ndelete test\n"), 0600); err != nil {
		t.Fatalf("WriteFile managed PDF error = %v", err)
	}
	return pdfPath
}

func upsertDeleteTestPaper(t *testing.T, app *App, id, folderID, pdfPath string) *Paper {
	t.Helper()
	now := time.Now()
	paper := &Paper{
		ID:             id,
		SourcePaperID:  id,
		Title:          "Delete Test Paper",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		Journal:        "Local",
		URL:            "https://example.org/" + id,
		PDFPath:        pdfPath,
		DownloadStatus: "downloaded",
		FolderID:       folderID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper(%s) error = %v", id, err)
	}
	return paper
}
