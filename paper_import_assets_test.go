package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testAppForImportAssets(t *testing.T) *App {
	t.Helper()
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		app.stopDownloadWorkers()
		_ = app.db.Close()
	})
	return app
}

func TestBuildPDFCandidateURLsIncludesSemanticAndArxivFallbacks(t *testing.T) {
	candidates := buildPDFCandidateURLs(
		"https://arxiv.org/abs/2501.12345",
		"",
		map[string]string{
			"ArXiv": "2501.12345v2",
			"DOI":   "10.1145/1234567.1234568",
		},
		[]string{
			"https://example.org/open-access.pdf",
			"https://example.org/open-access.pdf",
		},
	)

	expected := []string{
		"https://example.org/open-access.pdf",
		"https://arxiv.org/pdf/2501.12345.pdf",
		"https://arxiv.org/abs/2501.12345",
		"https://arxiv.org/pdf/2501.12345v2.pdf",
		"https://doi.org/10.1145/1234567.1234568",
	}
	for _, want := range expected {
		found := false
		for _, candidate := range candidates {
			if candidate == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected candidate %q, got %+v", want, candidates)
		}
	}
}

func TestRetryPaperDownloadResetsFailedState(t *testing.T) {
	app := testAppForImportAssets(t)
	app.stopDownloadWorkers()

	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected at least one default folder")
	}

	paper := &Paper{
		ID:             "paper-retry-1",
		SourcePaperID:  "2501.12345",
		Title:          "Retry Download Test",
		Authors:        "Alice",
		Abstract:       "test",
		Year:           2025,
		Journal:        "ArXiv",
		URL:            "https://arxiv.org/abs/2501.12345",
		FolderID:       folders[0].ID,
		DownloadStatus: "failed",
		DownloadError:  "no downloadable pdf url",
		AddedAt:        time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	if err := app.RetryPaperDownload(paper.ID); err != nil {
		t.Fatalf("RetryPaperDownload() error = %v", err)
	}

	latest, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if latest.DownloadStatus != "queued" {
		t.Fatalf("expected download status queued, got %q", latest.DownloadStatus)
	}
	if latest.DownloadError != "" {
		t.Fatalf("expected cleared download error, got %q", latest.DownloadError)
	}
}

func TestRetryPaperDownloadWithURLPersistsManualURLAndQueues(t *testing.T) {
	app := testAppForImportAssets(t)
	app.stopDownloadWorkers()

	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected at least one default folder")
	}

	paper := &Paper{
		ID:             "paper-manual-url-1",
		SourcePaperID:  "paper-manual-url-1",
		Title:          "Manual URL Retry",
		Authors:        "Alice",
		Abstract:       "test",
		Year:           2025,
		Journal:        "ArXiv",
		URL:            "",
		FolderID:       folders[0].ID,
		DownloadStatus: "failed",
		DownloadError:  "no downloadable pdf url",
		AddedAt:        time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	if err := app.RetryPaperDownloadWithURL(paper.ID, "https://example.org/manual-paper.pdf"); err != nil {
		t.Fatalf("RetryPaperDownloadWithURL() error = %v", err)
	}

	latest, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if latest.URL != "https://example.org/manual-paper.pdf" {
		t.Fatalf("expected manual URL to persist, got %q", latest.URL)
	}
	if latest.DownloadStatus != "queued" {
		t.Fatalf("expected queued status after manual retry, got %q", latest.DownloadStatus)
	}
	if latest.DownloadError != "" {
		t.Fatalf("expected cleared download error, got %q", latest.DownloadError)
	}
}

func TestRetryPaperDownloadWithURLRejectsInvalidURL(t *testing.T) {
	app := testAppForImportAssets(t)
	app.stopDownloadWorkers()

	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected at least one default folder")
	}

	paper := &Paper{
		ID:             "paper-manual-url-invalid",
		SourcePaperID:  "paper-manual-url-invalid",
		Title:          "Manual URL Retry Invalid",
		Authors:        "Alice",
		Abstract:       "test",
		Year:           2025,
		Journal:        "ArXiv",
		URL:            "",
		FolderID:       folders[0].ID,
		DownloadStatus: "failed",
		DownloadError:  "no downloadable pdf url",
		AddedAt:        time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	if err := app.RetryPaperDownloadWithURL(paper.ID, "ftp://example.org/file.pdf"); err == nil {
		t.Fatal("expected invalid manual URL scheme to be rejected")
	}
}

func TestGetDeepReadStateRepairsStaleMissingPDFCache(t *testing.T) {
	app := testAppForImportAssets(t)

	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected at least one default folder")
	}

	pdfPath := filepath.Join(t.TempDir(), "ready.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	paper := &Paper{
		ID:             "paper-deepread-cache-1",
		SourcePaperID:  "paper-deepread-cache-1",
		Title:          "DeepRead Cache Repair",
		Authors:        "Bob",
		Abstract:       "test",
		Year:           2025,
		Journal:        "ICRA",
		URL:            "https://example.org/paper",
		PDFPath:        pdfPath,
		FolderID:       folders[0].ID,
		DownloadStatus: "downloaded",
		DownloadError:  "",
		AddedAt:        time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	if err := app.db.UpsertDeepReadParseCache(&DeepReadParseCache{
		PaperID:        paper.ID,
		PDFPath:        pdfPath,
		Status:         "missing_pdf",
		ErrorMessage:   "本地 PDF 不可用，请先等待下载完成或手动导入 PDF。",
		Sections:       []DeepReadSection{},
		LastPreparedAt: time.Now(),
	}); err != nil {
		t.Fatalf("UpsertDeepReadParseCache() error = %v", err)
	}

	state, err := app.GetDeepReadState(paper.ID)
	if err != nil {
		t.Fatalf("GetDeepReadState() error = %v", err)
	}
	if !state.HasPDF {
		t.Fatal("expected HasPDF=true when local file exists")
	}
	if state.ParseStatus != "idle" {
		t.Fatalf("expected parse status idle, got %q", state.ParseStatus)
	}
	if state.ParseError != "" {
		t.Fatalf("expected parse error to be cleared, got %q", state.ParseError)
	}

	cache, err := app.db.GetDeepReadParseCache(paper.ID)
	if err != nil {
		t.Fatalf("GetDeepReadParseCache() error = %v", err)
	}
	if cache.Status != "idle" {
		t.Fatalf("expected parse cache status idle, got %q", cache.Status)
	}
}
