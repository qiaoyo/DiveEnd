package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestStorageOverviewCountsOnlyRegularValidPDFs(t *testing.T) {
	app := testAppForImportAssets(t)
	folder, err := app.CreateFolder("Storage Count")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	folderPath := filepath.Join(app.config.DataPath, "papers", filepath.FromSlash(folder.Path))
	if err := os.MkdirAll(folderPath, 0700); err != nil {
		t.Fatalf("MkdirAll folder path error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(folderPath, "valid.pdf"), []byte("%PDF-1.4\nvalid\n"), 0600); err != nil {
		t.Fatalf("WriteFile valid pdf error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(folderPath, "fake.pdf"), []byte("not actually a pdf"), 0600); err != nil {
		t.Fatalf("WriteFile fake pdf error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(folderPath, "notes.txt"), []byte("%PDF-1.4\nnot counted\n"), 0600); err != nil {
		t.Fatalf("WriteFile notes error = %v", err)
	}
	externalPDF := filepath.Join(t.TempDir(), "external.pdf")
	if err := os.WriteFile(externalPDF, []byte("%PDF-1.4\nexternal\n"), 0600); err != nil {
		t.Fatalf("WriteFile external pdf error = %v", err)
	}
	if err := os.Symlink(externalPDF, filepath.Join(folderPath, "linked.pdf")); err != nil {
		t.Fatalf("Symlink external pdf error = %v", err)
	}

	overview, err := app.GetLocalStorageOverview()
	if err != nil {
		t.Fatalf("GetLocalStorageOverview() error = %v", err)
	}
	var folderOverview *LocalStorageFolderOverview
	for i := range overview.Folders {
		if overview.Folders[i].FolderID == folder.ID {
			folderOverview = &overview.Folders[i]
			break
		}
	}
	if folderOverview == nil {
		t.Fatal("expected storage overview for created folder")
	}
	if folderOverview.StoredFileCount != 1 {
		t.Fatalf("expected only one regular valid pdf to be counted, got %+v", folderOverview)
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

func TestDownloadJobUsesQueuedDataPathSnapshot(t *testing.T) {
	allowPrivatePDFDownloadHostsForTest = true
	t.Cleanup(func() {
		allowPrivatePDFDownloadHostsForTest = false
	})

	pdfPayload := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdfPayload)
	}))
	defer server.Close()

	app := testAppForImportAssets(t)
	app.stopDownloadWorkers()

	oldDataPath := app.config.DataPath
	newDataPath := t.TempDir()

	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected at least one default folder")
	}

	paper := &Paper{
		ID:             "paper-data-path-snapshot",
		SourcePaperID:  "paper-data-path-snapshot",
		Title:          "DataPath Snapshot",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		URL:            server.URL + "/paper.pdf",
		FolderID:       folders[0].ID,
		DownloadStatus: "queued",
		AddedAt:        time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	app.config.DataPath = newDataPath
	app.processDownloadJob(context.Background(), paperDownloadJob{
		PaperID:       paper.ID,
		FolderID:      paper.FolderID,
		FolderPath:    folders[0].Path,
		DataPath:      oldDataPath,
		SourceURL:     paper.URL,
		SourcePaperID: paper.SourcePaperID,
	})

	latest, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if latest.DownloadStatus != "downloaded" {
		t.Fatalf("expected downloaded status, got %+v", latest)
	}
	if !strings.HasPrefix(filepath.Clean(latest.PDFPath), filepath.Clean(oldDataPath)+string(os.PathSeparator)) {
		t.Fatalf("expected pdf path to use old queued data path %q, got %q", oldDataPath, latest.PDFPath)
	}
	if strings.HasPrefix(filepath.Clean(latest.PDFPath), filepath.Clean(newDataPath)+string(os.PathSeparator)) {
		t.Fatalf("pdf path unexpectedly used current app data path %q: %q", newDataPath, latest.PDFPath)
	}
	if content, err := os.ReadFile(latest.PDFPath); err != nil {
		t.Fatalf("ReadFile(downloaded pdf) error = %v", err)
	} else if string(content) != string(pdfPayload) {
		t.Fatalf("unexpected downloaded content: %q", string(content))
	}
	info, err := os.Stat(latest.PDFPath)
	if err != nil {
		t.Fatalf("Stat(downloaded pdf) error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected downloaded pdf permissions 0600, got %o", info.Mode().Perm())
	}
	tempMatches, err := filepath.Glob(filepath.Join(filepath.Dir(latest.PDFPath), "."+filepath.Base(latest.PDFPath)+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob temp pdf files error = %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected no leftover temp pdf files, got %v", tempMatches)
	}
}

func TestProcessDownloadJobRejectsSymlinkedManagedFolder(t *testing.T) {
	app := testAppForImportAssets(t)
	app.stopDownloadWorkers()

	folder, err := app.CreateFolder("Download Symlink Target")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	paper := &Paper{
		ID:             "download-target-symlink-paper",
		SourcePaperID:  "download-target-symlink-paper",
		Title:          "Download Target Symlink",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		URL:            "http://93.184.216.34/paper.pdf",
		FolderID:       folder.ID,
		DownloadStatus: "queued",
		AddedAt:        time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
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

	originalTransport := pdfDownloadHTTPTransport
	networkCalled := false
	pdfDownloadHTTPTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		networkCalled = true
		return nil, fmt.Errorf("network should not be called")
	})
	t.Cleanup(func() {
		pdfDownloadHTTPTransport = originalTransport
	})

	app.processDownloadJob(context.Background(), paperDownloadJob{
		PaperID:       paper.ID,
		FolderID:      paper.FolderID,
		FolderPath:    folder.Path,
		DataPath:      app.config.DataPath,
		SourceURL:     paper.URL,
		SourcePaperID: paper.SourcePaperID,
	})

	if networkCalled {
		t.Fatal("PDF download should not be attempted when the target folder is a symlink")
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, safePaperPDFFileName(paper.ID))); !os.IsNotExist(statErr) {
		t.Fatalf("expected no pdf to be written through symlinked target folder, stat err=%v", statErr)
	}
	latest, err := app.db.GetPaperByID(paper.ID)
	if err != nil {
		t.Fatalf("GetPaperByID() error = %v", err)
	}
	if latest.DownloadStatus != "failed" {
		t.Fatalf("expected failed download status, got %+v", latest)
	}
	if !strings.Contains(latest.DownloadError, "symbolic link") {
		t.Fatalf("expected symbolic link download error, got %q", latest.DownloadError)
	}
	if strings.TrimSpace(latest.PDFPath) != "" {
		t.Fatalf("expected failed download to preserve empty PDFPath, got %q", latest.PDFPath)
	}
}

func TestDownloadPDFToFileFailurePreservesExistingTarget(t *testing.T) {
	allowPrivatePDFDownloadHostsForTest = true
	t.Cleanup(func() {
		allowPrivatePDFDownloadHostsForTest = false
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", "128")
		_, _ = w.Write([]byte("%PDF-1.4\npartial"))
	}))
	defer server.Close()

	targetPath := filepath.Join(t.TempDir(), "paper.pdf")
	existing := []byte("%PDF-1.4\nexisting complete file\n")
	if err := os.WriteFile(targetPath, existing, 0600); err != nil {
		t.Fatalf("WriteFile existing target error = %v", err)
	}

	if err := downloadPDFToFile(context.Background(), server.URL+"/paper.pdf", targetPath); err == nil {
		t.Fatal("expected downloadPDFToFile() to fail on truncated response")
	}
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile target after failed download error = %v", err)
	}
	if string(content) != string(existing) {
		t.Fatalf("expected existing target to be preserved, got %q", string(content))
	}
	tempMatches, err := filepath.Glob(filepath.Join(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob temp pdf files error = %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected failed download temp files to be cleaned, got %v", tempMatches)
	}
}

func TestDownloadPDFToFileRedactsSignedURLFromNetworkError(t *testing.T) {
	originalTransport := pdfDownloadHTTPTransport
	pdfDownloadHTTPTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network failed for %s", req.URL.String())
	})
	t.Cleanup(func() {
		pdfDownloadHTTPTransport = originalTransport
	})

	targetPath := filepath.Join(t.TempDir(), "paper.pdf")
	err := downloadPDFToFile(
		context.Background(),
		"http://93.184.216.34/paper.pdf?X-Amz-Signature=signed-url-secret&token=download-token-secret",
		targetPath,
	)
	if err == nil {
		t.Fatal("expected network error")
	}
	message := err.Error()
	for _, leaked := range []string{"signed-url-secret", "download-token-secret"} {
		if strings.Contains(message, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, message)
		}
	}
	if !strings.Contains(message, redactedValue) && !strings.Contains(message, "%5Bredacted%5D") {
		t.Fatalf("expected redacted marker in %q", message)
	}
}

func TestDownloadPDFToFileRejectsSymlinkedTargetDirectoryBeforeRequest(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target-dir")
	if err := os.Symlink(outsideDir, targetDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	originalTransport := pdfDownloadHTTPTransport
	networkCalled := false
	pdfDownloadHTTPTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		networkCalled = true
		return nil, fmt.Errorf("network should not be called")
	})
	t.Cleanup(func() {
		pdfDownloadHTTPTransport = originalTransport
	})

	err := downloadPDFToFile(context.Background(), "http://93.184.216.34/paper.pdf", filepath.Join(targetDir, "paper.pdf"))
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked target directory error, got %v", err)
	}
	if networkCalled {
		t.Fatal("PDF download should not be attempted when the target directory is a symlink")
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "paper.pdf")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no pdf to be written through symlinked target directory, stat err=%v", statErr)
	}
}

func TestDownloadPDFToFileRejectsRedirectToPrivateHost(t *testing.T) {
	originalTransport := pdfDownloadHTTPTransport
	privateRequested := false
	pdfDownloadHTTPTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "93.184.216.34":
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://127.0.0.1/private.pdf"}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		case "127.0.0.1":
			privateRequested = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/pdf"}},
				Body:       io.NopCloser(strings.NewReader("%PDF-1.4\nprivate\n")),
				Request:    req,
			}, nil
		default:
			t.Fatalf("unexpected request host %q", req.URL.Host)
			return nil, nil
		}
	})
	t.Cleanup(func() {
		pdfDownloadHTTPTransport = originalTransport
	})

	targetPath := filepath.Join(t.TempDir(), "paper.pdf")
	err := downloadPDFToFile(context.Background(), "http://93.184.216.34/start.pdf", targetPath)
	if err == nil {
		t.Fatal("expected downloadPDFToFile() to reject redirect to private host")
	}
	if privateRequested {
		t.Fatal("redirected private host should not be requested")
	}
	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no target file after rejected redirect, stat err=%v", statErr)
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

	pdfPath := filepath.Join(app.config.DataPath, "papers", "cache-repair", "ready.pdf")
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0700); err != nil {
		t.Fatalf("MkdirAll pdf dir error = %v", err)
	}
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
