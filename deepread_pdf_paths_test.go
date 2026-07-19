package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeepReadManagedPDFURLAndBytes(t *testing.T) {
	app := testAppForImportAssets(t)
	paper := createDeepReadTestPaper(t, app, "managed-paper", "papers/managed-paper.pdf")

	encoded, err := app.GetDeepReadPDFBytes(paper.ID)
	if err != nil {
		t.Fatalf("GetDeepReadPDFBytes() error = %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	if !strings.HasPrefix(string(decoded), "%PDF-") {
		t.Fatalf("expected pdf bytes, got %q", string(decoded))
	}

	urlPath, err := app.GetDeepReadPDFURL(paper.ID)
	if err != nil {
		t.Fatalf("GetDeepReadPDFURL() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, urlPath, nil)
	rec := httptest.NewRecorder()
	app.assetServerHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected managed pdf to be served, got status %d body %q", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(rec.Body.String(), "%PDF-") {
		t.Fatalf("expected served pdf bytes, got %q", rec.Body.String())
	}
}

func TestDeepReadPDFBytesRejectsLargeFallbackButAssetURLStillWorks(t *testing.T) {
	previousLimit := deepReadBase64FallbackMaxBytes
	deepReadBase64FallbackMaxBytes = 8
	t.Cleanup(func() { deepReadBase64FallbackMaxBytes = previousLimit })

	app := testAppForImportAssets(t)
	paper := createDeepReadTestPaperWithContent(t, app, "large-managed-paper", "papers/large-managed-paper.pdf", []byte("%PDF-1.4\nlarge fallback\n"))

	if _, err := app.GetDeepReadPDFBytes(paper.ID); err == nil || !strings.Contains(err.Error(), "too large for base64 fallback") {
		t.Fatalf("expected large base64 fallback rejection, got %v", err)
	}

	urlPath, err := app.GetDeepReadPDFURL(paper.ID)
	if err != nil {
		t.Fatalf("GetDeepReadPDFURL() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, urlPath, nil)
	rec := httptest.NewRecorder()
	app.assetServerHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected asset URL to still serve large pdf, got status %d body %q", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(rec.Body.String(), "%PDF-") {
		t.Fatalf("expected served pdf bytes, got %q", rec.Body.String())
	}
}

func TestDeepReadRejectsPDFOutsideManagedPapersDirectory(t *testing.T) {
	app := testAppForImportAssets(t)
	outsidePath := filepath.Join(t.TempDir(), "outside.pdf")
	if err := os.WriteFile(outsidePath, []byte("%PDF-1.4\noutside\n"), 0600); err != nil {
		t.Fatalf("WriteFile outside pdf error = %v", err)
	}
	paper := createDeepReadPaperWithPath(t, app, "outside-paper", outsidePath)

	state, err := app.GetDeepReadState(paper.ID)
	if err != nil {
		t.Fatalf("GetDeepReadState() error = %v", err)
	}
	if state.HasPDF {
		t.Fatal("expected outside pdf path to be unavailable to DeepRead")
	}
	if _, err := app.GetDeepReadPDFBytes(paper.ID); err == nil {
		t.Fatal("expected GetDeepReadPDFBytes() to reject outside pdf")
	}
	if _, err := app.GetDeepReadPDFURL(paper.ID); err == nil {
		t.Fatal("expected GetDeepReadPDFURL() to reject outside pdf")
	}

	token := "unsafe-token"
	app.storeDeepReadPDFResource(token, deepReadPDFResource{
		PaperID:   paper.ID,
		Path:      outsidePath,
		ExpiresAt: time.Now().Add(time.Minute),
	})
	req := httptest.NewRequest(http.MethodGet, "/deepread/pdf/"+token, nil)
	rec := httptest.NewRecorder()
	app.assetServerHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected asset server to reject outside pdf, got status %d", rec.Code)
	}
}

func TestDeepReadRejectsManagedSymlinkEscape(t *testing.T) {
	app := testAppForImportAssets(t)
	outsidePath := filepath.Join(t.TempDir(), "outside.pdf")
	if err := os.WriteFile(outsidePath, []byte("%PDF-1.4\noutside\n"), 0600); err != nil {
		t.Fatalf("WriteFile outside pdf error = %v", err)
	}

	linkPath := filepath.Join(app.config.DataPath, "papers", "links", "escape.pdf")
	if err := os.MkdirAll(filepath.Dir(linkPath), 0700); err != nil {
		t.Fatalf("MkdirAll link dir error = %v", err)
	}
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	paper := createDeepReadPaperWithPath(t, app, "symlink-paper", linkPath)

	if _, err := app.GetDeepReadPDFURL(paper.ID); err == nil {
		t.Fatal("expected GetDeepReadPDFURL() to reject symlink escape")
	}
	if _, err := app.GetDeepReadPDFBytes(paper.ID); err == nil {
		t.Fatal("expected GetDeepReadPDFBytes() to reject symlink escape")
	}
}

func createDeepReadTestPaper(t *testing.T, app *App, paperID string, relativePDFPath string) *Paper {
	t.Helper()
	return createDeepReadTestPaperWithContent(t, app, paperID, relativePDFPath, []byte("%PDF-1.4\nmanaged\n"))
}

func createDeepReadTestPaperWithContent(t *testing.T, app *App, paperID string, relativePDFPath string, content []byte) *Paper {
	t.Helper()
	pdfPath := filepath.Join(app.config.DataPath, filepath.FromSlash(relativePDFPath))
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0700); err != nil {
		t.Fatalf("MkdirAll pdf dir error = %v", err)
	}
	if err := os.WriteFile(pdfPath, content, 0600); err != nil {
		t.Fatalf("WriteFile pdf error = %v", err)
	}
	return createDeepReadPaperWithPath(t, app, paperID, pdfPath)
}

func createDeepReadPaperWithPath(t *testing.T, app *App, paperID string, pdfPath string) *Paper {
	t.Helper()
	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected default folder")
	}
	now := time.Now()
	paper := &Paper{
		ID:             paperID,
		SourcePaperID:  paperID,
		Title:          "DeepRead Security Test",
		Authors:        "Tester",
		Abstract:       "Test",
		Year:           2026,
		Journal:        "Local",
		URL:            "https://example.org/paper",
		PDFPath:        pdfPath,
		DownloadStatus: "downloaded",
		FolderID:       folders[0].ID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}
	return paper
}
