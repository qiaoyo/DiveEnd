package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeleteFolderNodeDoesNotRemoveExternalPDFPath(t *testing.T) {
	app := testAppForImportAssets(t)
	folder, err := app.CreateFolder("Unsafe Delete")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	externalPDF := filepath.Join(t.TempDir(), "external.pdf")
	if err := os.WriteFile(externalPDF, []byte("%PDF-1.4\nexternal\n"), 0600); err != nil {
		t.Fatalf("WriteFile external pdf error = %v", err)
	}
	now := time.Now()
	paper := &Paper{
		ID:             "external-pdf-paper",
		SourcePaperID:  "external-pdf-paper",
		Title:          "External PDF",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		Journal:        "Local",
		URL:            "https://example.org/external",
		PDFPath:        externalPDF,
		DownloadStatus: "downloaded",
		FolderID:       folder.ID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	if err := app.DeleteFolderNode(folder.ID); err != nil {
		t.Fatalf("DeleteFolderNode() error = %v", err)
	}
	if _, err := os.Stat(externalPDF); err != nil {
		t.Fatalf("expected external pdf to remain untouched, stat err=%v", err)
	}
}

func TestDeleteFolderNodeDoesNotRemovePathEscapedByCorruptFolderID(t *testing.T) {
	app := testAppForImportAssets(t)
	escapedDir := filepath.Join(app.config.DataPath, "outside-folder-id")
	sentinelPath := filepath.Join(escapedDir, "sentinel.txt")
	if err := os.MkdirAll(escapedDir, 0700); err != nil {
		t.Fatalf("MkdirAll escaped dir error = %v", err)
	}
	if err := os.WriteFile(sentinelPath, []byte("keep"), 0600); err != nil {
		t.Fatalf("WriteFile sentinel error = %v", err)
	}

	corruptID := "../outside-folder-id"
	_, err := app.db.conn.Exec(
		`INSERT INTO folders (id, name, parent_id, path, is_system, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		corruptID,
		"Corrupt",
		sql.NullString{},
		"Corrupt",
		0,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("insert corrupt folder error = %v", err)
	}

	if err := app.DeleteFolderNode(corruptID); err != nil {
		t.Fatalf("DeleteFolderNode() error = %v", err)
	}
	if _, err := os.Stat(sentinelPath); err != nil {
		t.Fatalf("expected escaped sentinel to remain untouched, stat err=%v", err)
	}
}

func TestDeleteFolderNodeRejectsSymlinkedManagedRootAndPreservesDB(t *testing.T) {
	app := testAppForImportAssets(t)
	folder, err := app.CreateFolder("Symlink Guard")
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

	now := time.Now()
	paper := &Paper{
		ID:             "symlink-guard-paper",
		SourcePaperID:  "symlink-guard-paper",
		Title:          "Symlink Guard Paper",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		Journal:        "Local",
		URL:            "https://example.org/symlink-guard",
		PDFPath:        filepath.Join(papersRoot, folder.Path, "symlink-guard-paper.pdf"),
		DownloadStatus: "downloaded",
		FolderID:       folder.ID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	err = app.DeleteFolderNode(folder.ID)
	if err == nil || !strings.Contains(err.Error(), "managed root directory is a symbolic link") {
		t.Fatalf("expected symlinked managed root error, got %v", err)
	}

	if _, err := app.db.GetPaperByID(paper.ID); err != nil {
		t.Fatalf("expected paper row to remain after failed folder delete, got %v", err)
	}
	if _, err := app.db.GetFolderByID(folder.ID); err != nil {
		t.Fatalf("expected folder row to remain after failed folder delete, got %v", err)
	}
}

func TestDeleteFolderNodeRejectsSymlinkedManagedIntermediateDirectoryAndPreservesDB(t *testing.T) {
	app := testAppForImportAssets(t)
	folder, err := app.CreateFolderNode(CreateFolderNodeRequest{Path: "Parent Folder/Child Folder"})
	if err != nil {
		t.Fatalf("CreateFolderNode() error = %v", err)
	}

	papersRoot := filepath.Join(app.config.DataPath, "papers")
	if err := os.MkdirAll(papersRoot, 0700); err != nil {
		t.Fatalf("MkdirAll papers root error = %v", err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(papersRoot, "Parent Folder")); err != nil {
		t.Fatalf("Symlink intermediate folder error = %v", err)
	}

	err = app.DeleteFolderNode(folder.ID)
	if err == nil || !strings.Contains(err.Error(), "managed path parent contains symbolic link") {
		t.Fatalf("expected symlinked managed parent error, got %v", err)
	}

	if _, err := app.db.GetFolderByID(folder.ID); err != nil {
		t.Fatalf("expected child folder row to remain after failed folder delete, got %v", err)
	}
}

func TestRenameFolderNodeMovesManagedDirectoryAndUpdatesDescendants(t *testing.T) {
	app := testAppForImportAssets(t)
	nested, err := app.CreateFolderNode(CreateFolderNodeRequest{Path: "Rename Parent/Rename Child/Nested"})
	if err != nil {
		t.Fatalf("CreateFolderNode(nested) error = %v", err)
	}
	child, err := app.db.getFolderByPath("Rename Parent/Rename Child")
	if err != nil {
		t.Fatalf("getFolderByPath(child) error = %v", err)
	}

	oldPDFPath := filepath.Join(app.config.DataPath, "papers", "Rename Parent", "Rename Child", "Nested", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(oldPDFPath), 0700); err != nil {
		t.Fatalf("MkdirAll old pdf dir error = %v", err)
	}
	if err := os.WriteFile(oldPDFPath, []byte("%PDF-1.4\nrename\n"), 0600); err != nil {
		t.Fatalf("WriteFile old pdf error = %v", err)
	}

	now := time.Now()
	paper := &Paper{
		ID:             "rename-folder-paper",
		SourcePaperID:  "rename-folder-paper",
		Title:          "Rename Folder Paper",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		Journal:        "Local",
		URL:            "https://example.org/rename-folder-paper",
		PDFPath:        oldPDFPath,
		DownloadStatus: "downloaded",
		FolderID:       nested.ID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}
	if err := app.db.UpsertDeepReadParseCache(&DeepReadParseCache{
		PaperID:        paper.ID,
		PDFPath:        oldPDFPath,
		Status:         "idle",
		Sections:       []DeepReadSection{},
		LastPreparedAt: now,
	}); err != nil {
		t.Fatalf("UpsertDeepReadParseCache() error = %v", err)
	}

	renamed, err := app.RenameFolderNode(RenameFolderNodeRequest{FolderID: child.ID, Name: "Vision"})
	if err != nil {
		t.Fatalf("RenameFolderNode() error = %v", err)
	}
	if renamed.Path != "Rename Parent/Vision" {
		t.Fatalf("expected renamed child path Rename Parent/Vision, got %q", renamed.Path)
	}

	updatedNested, err := app.db.GetFolderByID(nested.ID)
	if err != nil {
		t.Fatalf("GetFolderByID(nested) error = %v", err)
	}
	if updatedNested.Path != "Rename Parent/Vision/Nested" {
		t.Fatalf("expected nested path Rename Parent/Vision/Nested, got %q", updatedNested.Path)
	}

	newPDFPath := filepath.Join(app.config.DataPath, "papers", "Rename Parent", "Vision", "Nested", "paper.pdf")
	if _, err := os.Stat(newPDFPath); err != nil {
		t.Fatalf("expected renamed PDF at new path, stat err=%v", err)
	}
	if _, err := os.Stat(oldPDFPath); !os.IsNotExist(err) {
		t.Fatalf("expected old PDF path to be gone, stat err=%v", err)
	}

	updatedPaper, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if updatedPaper.PDFPath != newPDFPath {
		t.Fatalf("expected updated paper pdf path %q, got %q", newPDFPath, updatedPaper.PDFPath)
	}
	cache, err := app.db.GetDeepReadParseCache(paper.ID)
	if err != nil {
		t.Fatalf("GetDeepReadParseCache() error = %v", err)
	}
	if cache.PDFPath != newPDFPath {
		t.Fatalf("expected updated parse cache pdf path %q, got %q", newPDFPath, cache.PDFPath)
	}
}

func TestRenameFolderNodeRejectsSystemFolder(t *testing.T) {
	app := testAppForImportAssets(t)
	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected default system folder")
	}

	_, err = app.RenameFolderNode(RenameFolderNodeRequest{FolderID: folders[0].ID, Name: "Renamed"})
	if err == nil || !strings.Contains(err.Error(), "system folder cannot be renamed") {
		t.Fatalf("expected system folder rename rejection, got %v", err)
	}
}

func TestMoveFolderNodeMovesManagedDirectoryAndUpdatesDescendants(t *testing.T) {
	app := testAppForImportAssets(t)
	nested, err := app.CreateFolderNode(CreateFolderNodeRequest{Path: "Move Parent/Move Child/Nested"})
	if err != nil {
		t.Fatalf("CreateFolderNode(nested) error = %v", err)
	}
	child, err := app.db.getFolderByPath("Move Parent/Move Child")
	if err != nil {
		t.Fatalf("getFolderByPath(child) error = %v", err)
	}
	targetParent, err := app.CreateFolderNode(CreateFolderNodeRequest{Path: "Move Target"})
	if err != nil {
		t.Fatalf("CreateFolderNode(target) error = %v", err)
	}

	oldPDFPath := filepath.Join(app.config.DataPath, "papers", "Move Parent", "Move Child", "Nested", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(oldPDFPath), 0700); err != nil {
		t.Fatalf("MkdirAll old pdf dir error = %v", err)
	}
	if err := os.WriteFile(oldPDFPath, []byte("%PDF-1.4\nmove folder\n"), 0600); err != nil {
		t.Fatalf("WriteFile old pdf error = %v", err)
	}

	now := time.Now()
	paper := &Paper{
		ID:             "move-folder-paper",
		SourcePaperID:  "move-folder-paper",
		Title:          "Move Folder Paper",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		Journal:        "Local",
		URL:            "https://example.org/move-folder-paper",
		PDFPath:        oldPDFPath,
		DownloadStatus: "downloaded",
		FolderID:       nested.ID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}
	if err := app.db.UpsertDeepReadParseCache(&DeepReadParseCache{
		PaperID:        paper.ID,
		PDFPath:        oldPDFPath,
		Status:         "idle",
		Sections:       []DeepReadSection{},
		LastPreparedAt: now,
	}); err != nil {
		t.Fatalf("UpsertDeepReadParseCache() error = %v", err)
	}

	moved, err := app.MoveFolderNode(MoveFolderNodeRequest{FolderID: child.ID, ParentID: targetParent.ID})
	if err != nil {
		t.Fatalf("MoveFolderNode() error = %v", err)
	}
	if moved.ParentID != targetParent.ID {
		t.Fatalf("expected parent %q, got %q", targetParent.ID, moved.ParentID)
	}
	if moved.Path != "Move Target/Move Child" {
		t.Fatalf("expected moved path Move Target/Move Child, got %q", moved.Path)
	}

	updatedNested, err := app.db.GetFolderByID(nested.ID)
	if err != nil {
		t.Fatalf("GetFolderByID(nested) error = %v", err)
	}
	if updatedNested.Path != "Move Target/Move Child/Nested" {
		t.Fatalf("expected nested path Move Target/Move Child/Nested, got %q", updatedNested.Path)
	}

	newPDFPath := filepath.Join(app.config.DataPath, "papers", "Move Target", "Move Child", "Nested", "paper.pdf")
	if _, err := os.Stat(newPDFPath); err != nil {
		t.Fatalf("expected moved PDF at new path, stat err=%v", err)
	}
	if _, err := os.Stat(oldPDFPath); !os.IsNotExist(err) {
		t.Fatalf("expected old PDF path to be gone, stat err=%v", err)
	}

	updatedPaper, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if updatedPaper.PDFPath != newPDFPath {
		t.Fatalf("expected paper PDF path %q, got %q", newPDFPath, updatedPaper.PDFPath)
	}
	cache, err := app.db.GetDeepReadParseCache(paper.ID)
	if err != nil {
		t.Fatalf("GetDeepReadParseCache() error = %v", err)
	}
	if cache.PDFPath != newPDFPath {
		t.Fatalf("expected cache PDF path %q, got %q", newPDFPath, cache.PDFPath)
	}
}

func TestMoveFolderNodeRejectsDescendantParent(t *testing.T) {
	app := testAppForImportAssets(t)
	nested, err := app.CreateFolderNode(CreateFolderNodeRequest{Path: "Move Cycle Parent/Move Cycle Child/Nested"})
	if err != nil {
		t.Fatalf("CreateFolderNode(nested) error = %v", err)
	}
	parent, err := app.db.getFolderByPath("Move Cycle Parent")
	if err != nil {
		t.Fatalf("getFolderByPath(parent) error = %v", err)
	}

	if _, err := app.MoveFolderNode(MoveFolderNodeRequest{FolderID: parent.ID, ParentID: nested.ID}); err == nil || !strings.Contains(err.Error(), "descendant") {
		t.Fatalf("expected descendant move rejection, got %v", err)
	}
	parentAfter, err := app.db.GetFolderByID(parent.ID)
	if err != nil {
		t.Fatalf("GetFolderByID(parent) error = %v", err)
	}
	if parentAfter.ParentID != "" || parentAfter.Path != "Move Cycle Parent" {
		t.Fatalf("expected parent folder to remain at root, got parent=%q path=%q", parentAfter.ParentID, parentAfter.Path)
	}
}

func TestMoveFolderNodeRejectsTargetPathConflict(t *testing.T) {
	app := testAppForImportAssets(t)
	source, err := app.CreateFolderNode(CreateFolderNodeRequest{Path: "Move Conflict Source/Duplicate"})
	if err != nil {
		t.Fatalf("CreateFolderNode(source) error = %v", err)
	}
	targetParent, err := app.CreateFolderNode(CreateFolderNodeRequest{Path: "Move Conflict Target"})
	if err != nil {
		t.Fatalf("CreateFolderNode(targetParent) error = %v", err)
	}
	if _, err := app.CreateFolderNode(CreateFolderNodeRequest{ParentID: targetParent.ID, Name: "Duplicate"}); err != nil {
		t.Fatalf("CreateFolderNode(duplicate) error = %v", err)
	}

	if _, err := app.MoveFolderNode(MoveFolderNodeRequest{FolderID: source.ID, ParentID: targetParent.ID}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected target path conflict, got %v", err)
	}
	unchanged, err := app.db.GetFolderByID(source.ID)
	if err != nil {
		t.Fatalf("GetFolderByID(source) error = %v", err)
	}
	if unchanged.Path != "Move Conflict Source/Duplicate" {
		t.Fatalf("expected source folder path to remain, got %q", unchanged.Path)
	}
}

func TestMoveFolderNodeRejectsSystemFolder(t *testing.T) {
	app := testAppForImportAssets(t)
	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected default system folder")
	}

	_, err = app.MoveFolderNode(MoveFolderNodeRequest{FolderID: folders[0].ID})
	if err == nil || !strings.Contains(err.Error(), "system folder cannot be moved") {
		t.Fatalf("expected system folder move rejection, got %v", err)
	}
}
