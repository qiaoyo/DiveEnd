package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMovePaperToFolderMovesManagedPDFAndUpdatesCache(t *testing.T) {
	app := testAppForImportAssets(t)
	source, err := app.CreateFolder("Move Source")
	if err != nil {
		t.Fatalf("CreateFolder(source) error = %v", err)
	}
	target, err := app.CreateFolder("Move Target")
	if err != nil {
		t.Fatalf("CreateFolder(target) error = %v", err)
	}

	oldPDFPath := writeManagedDeleteTestPDF(t, app, source.Path, "move-managed.pdf")
	paper := upsertMoveTestPaper(t, app, "move-managed-paper", "move-managed-source", source.ID, oldPDFPath)
	if err := app.db.UpsertDeepReadParseCache(&DeepReadParseCache{
		PaperID:  paper.ID,
		PDFPath:  oldPDFPath,
		Status:   "ready",
		Markdown: "# cached",
		Sections: []DeepReadSection{{
			ID:      "intro",
			Title:   "Intro",
			Content: "cached",
			Index:   0,
		}},
		LastPreparedAt: time.Now(),
	}); err != nil {
		t.Fatalf("UpsertDeepReadParseCache() error = %v", err)
	}

	moved, err := app.MovePaperToFolder(paper.ID, target.ID)
	if err != nil {
		t.Fatalf("MovePaperToFolder() error = %v", err)
	}

	targetPrefix := filepath.Join(app.config.DataPath, "papers", filepath.FromSlash(target.Path)) + string(os.PathSeparator)
	if moved.FolderID != target.ID {
		t.Fatalf("expected folder %q, got %q", target.ID, moved.FolderID)
	}
	if !strings.HasPrefix(moved.PDFPath, targetPrefix) {
		t.Fatalf("expected PDF under target folder %q, got %q", targetPrefix, moved.PDFPath)
	}
	if filepath.Base(moved.PDFPath) != filepath.Base(oldPDFPath) {
		t.Fatalf("expected moved PDF filename to be preserved, got %q", moved.PDFPath)
	}
	if _, err := os.Stat(moved.PDFPath); err != nil {
		t.Fatalf("expected moved PDF to exist, stat err=%v", err)
	}
	if _, err := os.Stat(oldPDFPath); !os.IsNotExist(err) {
		t.Fatalf("expected old managed PDF to be removed, stat err=%v", err)
	}

	dbPaper, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if dbPaper.PDFPath != moved.PDFPath {
		t.Fatalf("expected database paper PDF path %q, got %q", moved.PDFPath, dbPaper.PDFPath)
	}
	cache, err := app.db.GetDeepReadParseCache(paper.ID)
	if err != nil {
		t.Fatalf("GetDeepReadParseCache() error = %v", err)
	}
	if cache.PDFPath != moved.PDFPath {
		t.Fatalf("expected cache PDF path %q, got %q", moved.PDFPath, cache.PDFPath)
	}
}

func TestMovePaperToFolderPreservesSharedOldManagedPDF(t *testing.T) {
	app := testAppForImportAssets(t)
	source, err := app.CreateFolder("Move Shared Source")
	if err != nil {
		t.Fatalf("CreateFolder(source) error = %v", err)
	}
	target, err := app.CreateFolder("Move Shared Target")
	if err != nil {
		t.Fatalf("CreateFolder(target) error = %v", err)
	}

	oldPDFPath := writeManagedDeleteTestPDF(t, app, source.Path, "shared-move.pdf")
	moving := upsertMoveTestPaper(t, app, "shared-move-paper-a", "shared-move-source-a", source.ID, oldPDFPath)
	sibling := upsertMoveTestPaper(t, app, "shared-move-paper-b", "shared-move-source-b", source.ID, oldPDFPath)

	moved, err := app.MovePaperToFolder(moving.ID, target.ID)
	if err != nil {
		t.Fatalf("MovePaperToFolder() error = %v", err)
	}
	if moved.PDFPath == oldPDFPath {
		t.Fatalf("expected moving paper to receive a copied target PDF path")
	}
	if _, err := os.Stat(oldPDFPath); err != nil {
		t.Fatalf("expected shared old PDF to remain for sibling, stat err=%v", err)
	}
	siblingAfter, err := app.db.GetPaperByID(sibling.ID)
	if err != nil {
		t.Fatalf("GetPaperByID(sibling) error = %v", err)
	}
	if siblingAfter.PDFPath != oldPDFPath {
		t.Fatalf("expected sibling to keep old PDF path %q, got %q", oldPDFPath, siblingAfter.PDFPath)
	}
}

func TestMovePaperToFolderPreservesExternalPDF(t *testing.T) {
	app := testAppForImportAssets(t)
	source, err := app.CreateFolder("Move External Source")
	if err != nil {
		t.Fatalf("CreateFolder(source) error = %v", err)
	}
	target, err := app.CreateFolder("Move External Target")
	if err != nil {
		t.Fatalf("CreateFolder(target) error = %v", err)
	}

	externalPDF := filepath.Join(t.TempDir(), "external-move.pdf")
	if err := os.WriteFile(externalPDF, []byte("%PDF-1.4\nexternal move\n"), 0600); err != nil {
		t.Fatalf("WriteFile external PDF error = %v", err)
	}
	paper := upsertMoveTestPaper(t, app, "external-move-paper", "external-move-source", source.ID, externalPDF)

	moved, err := app.MovePaperToFolder(paper.ID, target.ID)
	if err != nil {
		t.Fatalf("MovePaperToFolder() error = %v", err)
	}
	if moved.FolderID != target.ID {
		t.Fatalf("expected folder %q, got %q", target.ID, moved.FolderID)
	}
	if moved.PDFPath != externalPDF {
		t.Fatalf("expected external PDF path to remain %q, got %q", externalPDF, moved.PDFPath)
	}
	if _, err := os.Stat(externalPDF); err != nil {
		t.Fatalf("expected external PDF to remain, stat err=%v", err)
	}
}

func TestMovePaperToFolderRejectsDuplicateSourceInTarget(t *testing.T) {
	app := testAppForImportAssets(t)
	source, err := app.CreateFolder("Move Duplicate Source")
	if err != nil {
		t.Fatalf("CreateFolder(source) error = %v", err)
	}
	target, err := app.CreateFolder("Move Duplicate Target")
	if err != nil {
		t.Fatalf("CreateFolder(target) error = %v", err)
	}

	oldPDFPath := writeManagedDeleteTestPDF(t, app, source.Path, "duplicate-move.pdf")
	moving := upsertMoveTestPaper(t, app, "duplicate-move-paper-a", "duplicate-source", source.ID, oldPDFPath)
	_ = upsertMoveTestPaper(t, app, "duplicate-move-paper-b", "duplicate-source", target.ID, "")

	err = nil
	if _, err = app.MovePaperToFolder(moving.ID, target.ID); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate source error, got %v", err)
	}
	unchanged, err := app.db.GetPaperByID(moving.ID)
	if err != nil {
		t.Fatalf("GetPaperByID(moving) error = %v", err)
	}
	if unchanged.FolderID != source.ID {
		t.Fatalf("expected paper to remain in source folder %q, got %q", source.ID, unchanged.FolderID)
	}
	if unchanged.PDFPath != oldPDFPath {
		t.Fatalf("expected PDF path to remain %q, got %q", oldPDFPath, unchanged.PDFPath)
	}
}

func TestMovePapersToFolderMovesSharedManagedPDFOnceAndUpdatesCaches(t *testing.T) {
	app := testAppForImportAssets(t)
	source, err := app.CreateFolder("Batch Move Shared Source")
	if err != nil {
		t.Fatalf("CreateFolder(source) error = %v", err)
	}
	target, err := app.CreateFolder("Batch Move Shared Target")
	if err != nil {
		t.Fatalf("CreateFolder(target) error = %v", err)
	}

	oldPDFPath := writeManagedDeleteTestPDF(t, app, source.Path, "batch-shared.pdf")
	first := upsertMoveTestPaper(t, app, "batch-shared-paper-a", "batch-shared-source-a", source.ID, oldPDFPath)
	second := upsertMoveTestPaper(t, app, "batch-shared-paper-b", "batch-shared-source-b", source.ID, oldPDFPath)
	for _, paper := range []*Paper{first, second} {
		if err := app.db.UpsertDeepReadParseCache(&DeepReadParseCache{
			PaperID:        paper.ID,
			PDFPath:        oldPDFPath,
			Status:         "ready",
			Markdown:       "# cached",
			Sections:       []DeepReadSection{{ID: "intro", Title: "Intro", Content: "cached", Index: 0}},
			LastPreparedAt: time.Now(),
		}); err != nil {
			t.Fatalf("UpsertDeepReadParseCache(%s) error = %v", paper.ID, err)
		}
	}

	moved, err := app.MovePapersToFolder([]string{first.ID, second.ID, first.ID, ""}, target.ID)
	if err != nil {
		t.Fatalf("MovePapersToFolder() error = %v", err)
	}
	if len(moved) != 2 {
		t.Fatalf("expected 2 moved papers, got %d", len(moved))
	}
	if moved[0].PDFPath == "" || moved[0].PDFPath != moved[1].PDFPath {
		t.Fatalf("expected shared moved PDF path, got %+v", moved)
	}
	if moved[0].PDFPath == oldPDFPath {
		t.Fatalf("expected shared PDF to move away from old path")
	}
	if _, err := os.Stat(moved[0].PDFPath); err != nil {
		t.Fatalf("expected moved shared PDF to exist, stat err=%v", err)
	}
	if _, err := os.Stat(oldPDFPath); !os.IsNotExist(err) {
		t.Fatalf("expected old shared PDF to be removed after all references moved, stat err=%v", err)
	}

	for _, paper := range []*Paper{first, second} {
		updated, err := app.db.GetPaperByID(paper.ID)
		if err != nil {
			t.Fatalf("GetPaperByID(%s) error = %v", paper.ID, err)
		}
		if updated.FolderID != target.ID {
			t.Fatalf("expected %s folder %q, got %q", paper.ID, target.ID, updated.FolderID)
		}
		if updated.PDFPath != moved[0].PDFPath {
			t.Fatalf("expected %s PDF path %q, got %q", paper.ID, moved[0].PDFPath, updated.PDFPath)
		}
		cache, err := app.db.GetDeepReadParseCache(paper.ID)
		if err != nil {
			t.Fatalf("GetDeepReadParseCache(%s) error = %v", paper.ID, err)
		}
		if cache.PDFPath != moved[0].PDFPath {
			t.Fatalf("expected %s cache path %q, got %q", paper.ID, moved[0].PDFPath, cache.PDFPath)
		}
	}
}

func TestMovePapersToFolderRejectsDuplicateSourceBeforePartialMove(t *testing.T) {
	app := testAppForImportAssets(t)
	source, err := app.CreateFolder("Batch Duplicate Source")
	if err != nil {
		t.Fatalf("CreateFolder(source) error = %v", err)
	}
	target, err := app.CreateFolder("Batch Duplicate Target")
	if err != nil {
		t.Fatalf("CreateFolder(target) error = %v", err)
	}

	firstPDFPath := writeManagedDeleteTestPDF(t, app, source.Path, "batch-first.pdf")
	first := upsertMoveTestPaper(t, app, "batch-duplicate-paper-a", "batch-source-a", source.ID, firstPDFPath)
	second := upsertMoveTestPaper(t, app, "batch-duplicate-paper-b", "batch-source-duplicate", source.ID, "")
	_ = upsertMoveTestPaper(t, app, "batch-duplicate-paper-c", "batch-source-duplicate", target.ID, "")

	if _, err := app.MovePapersToFolder([]string{first.ID, second.ID}, target.ID); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate target source error, got %v", err)
	}
	firstAfter, err := app.db.GetPaperByID(first.ID)
	if err != nil {
		t.Fatalf("GetPaperByID(first) error = %v", err)
	}
	secondAfter, err := app.db.GetPaperByID(second.ID)
	if err != nil {
		t.Fatalf("GetPaperByID(second) error = %v", err)
	}
	if firstAfter.FolderID != source.ID || secondAfter.FolderID != source.ID {
		t.Fatalf("expected both selected papers to remain in source, got %q and %q", firstAfter.FolderID, secondAfter.FolderID)
	}
	targetPDFPath := filepath.Join(app.config.DataPath, "papers", filepath.FromSlash(target.Path), filepath.Base(firstPDFPath))
	if _, err := os.Stat(targetPDFPath); !os.IsNotExist(err) {
		t.Fatalf("expected no partial copied PDF at target, stat err=%v", err)
	}
}

func TestMovePapersToFolderRejectsTargetPDFNameCollision(t *testing.T) {
	app := testAppForImportAssets(t)
	sourceA, err := app.CreateFolder("Batch Collision A")
	if err != nil {
		t.Fatalf("CreateFolder(sourceA) error = %v", err)
	}
	sourceB, err := app.CreateFolder("Batch Collision B")
	if err != nil {
		t.Fatalf("CreateFolder(sourceB) error = %v", err)
	}
	target, err := app.CreateFolder("Batch Collision Target")
	if err != nil {
		t.Fatalf("CreateFolder(target) error = %v", err)
	}

	firstPDFPath := writeManagedDeleteTestPDF(t, app, sourceA.Path, "same-name.pdf")
	secondPDFPath := writeManagedDeleteTestPDF(t, app, sourceB.Path, "same-name.pdf")
	first := upsertMoveTestPaper(t, app, "batch-collision-paper-a", "batch-collision-source-a", sourceA.ID, firstPDFPath)
	second := upsertMoveTestPaper(t, app, "batch-collision-paper-b", "batch-collision-source-b", sourceB.ID, secondPDFPath)

	if _, err := app.MovePapersToFolder([]string{first.ID, second.ID}, target.ID); err == nil || !strings.Contains(err.Error(), "target paper PDF") {
		t.Fatalf("expected target PDF collision error, got %v", err)
	}
	firstAfter, err := app.db.GetPaperByID(first.ID)
	if err != nil {
		t.Fatalf("GetPaperByID(first) error = %v", err)
	}
	secondAfter, err := app.db.GetPaperByID(second.ID)
	if err != nil {
		t.Fatalf("GetPaperByID(second) error = %v", err)
	}
	if firstAfter.FolderID != sourceA.ID || secondAfter.FolderID != sourceB.ID {
		t.Fatalf("expected both selected papers to remain in source folders, got %q and %q", firstAfter.FolderID, secondAfter.FolderID)
	}
	targetPDFPath := filepath.Join(app.config.DataPath, "papers", filepath.FromSlash(target.Path), "same-name.pdf")
	if _, err := os.Stat(targetPDFPath); !os.IsNotExist(err) {
		t.Fatalf("expected no target PDF after collision preflight, stat err=%v", err)
	}
}

func upsertMoveTestPaper(t *testing.T, app *App, id, sourcePaperID, folderID, pdfPath string) *Paper {
	t.Helper()
	now := time.Now()
	paper := &Paper{
		ID:             id,
		SourcePaperID:  sourcePaperID,
		Title:          "Move Test Paper",
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
