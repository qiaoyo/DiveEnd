package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreprocessDeepStartResultsWritesMarkdownCacheAtomically(t *testing.T) {
	pdfService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/parse/upload" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v", err)
		}
		if _, _, err := r.FormFile("file"); err != nil {
			t.Fatalf("expected uploaded pdf file: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":  true,
			"markdown": "# Parsed Paper\n\nBody",
			"metadata": map[string]any{"title": "Parsed Paper"},
		})
	}))
	defer pdfService.Close()

	previousDownload := deepStartDownloadWithCandidates
	deepStartDownloadWithCandidates = func(ctx context.Context, candidates []string, targetPath string) error {
		if len(candidates) == 0 {
			return fmt.Errorf("expected at least one pdf candidate")
		}
		return writeFileAtomic(targetPath, []byte("%PDF-1.4\n%DiveEnd DeepStart cache test\n"), 0600)
	}
	t.Cleanup(func() {
		deepStartDownloadWithCandidates = previousDownload
	})

	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	app.pdfService = NewPDFServiceClient(pdfService.URL)
	app.llm = nil
	app.strongLLM = nil
	app.weakLLM = nil
	t.Cleanup(func() {
		app.stopDownloadWorkers()
		if app.db != nil {
			_ = app.db.Close()
		}
	})

	papers, batch, err := app.preprocessDeepStartResults(
		context.Background(),
		"session-atomic",
		time.Now(),
		[]SearchPaper{{
			ID:     "deepstart-paper-1",
			Title:  "Original Title",
			URL:    "https://example.test/paper.pdf",
			Source: "test",
		}},
		SearchRetrievalStats{Query: "cache atomic"},
	)
	if err != nil {
		t.Fatalf("preprocessDeepStartResults() error = %v", err)
	}
	if batch.Parsed != 1 || batch.Failed != 0 {
		t.Fatalf("unexpected batch stats: %+v", batch)
	}
	if len(papers) != 1 || strings.TrimSpace(papers[0].MarkdownPath) == "" {
		t.Fatalf("expected one paper with markdown cache path, got papers=%+v", papers)
	}

	markdownPath := papers[0].MarkdownPath
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("ReadFile(markdown cache) error = %v", err)
	}
	if string(content) != "# Parsed Paper\n\nBody" {
		t.Fatalf("unexpected markdown cache content: %q", string(content))
	}
	info, err := os.Stat(markdownPath)
	if err != nil {
		t.Fatalf("Stat(markdown cache) error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("expected markdown cache permission 0600, got %o", got)
	}
	tempMatches, err := filepath.Glob(filepath.Join(filepath.Dir(markdownPath), "."+filepath.Base(markdownPath)+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob markdown temp files error = %v", err)
	}
	if len(tempMatches) > 0 {
		t.Fatalf("expected no leftover markdown temp files, got %v", tempMatches)
	}
}

func TestPreprocessDeepStartResultsSanitizesSessionCacheDirectory(t *testing.T) {
	pdfService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/parse/upload" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":  true,
			"markdown": "# Sanitized Session\n\nBody",
			"metadata": map[string]any{"title": "Sanitized Session"},
		})
	}))
	defer pdfService.Close()

	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	app.pdfService = NewPDFServiceClient(pdfService.URL)
	app.llm = nil
	app.strongLLM = nil
	app.weakLLM = nil
	t.Cleanup(func() {
		app.stopDownloadWorkers()
		if app.db != nil {
			_ = app.db.Close()
		}
	})

	cacheRoot := filepath.Join(config.DataPath, "deepstart_cache")
	previousDownload := deepStartDownloadWithCandidates
	deepStartDownloadWithCandidates = func(ctx context.Context, candidates []string, targetPath string) error {
		if _, ok := managedPathInsideRoot(cacheRoot, targetPath); !ok {
			t.Fatalf("download target escaped cache root: path=%q root=%q", targetPath, cacheRoot)
		}
		if strings.Contains(filepath.ToSlash(targetPath), "../") {
			t.Fatalf("download target retained traversal segment: %q", targetPath)
		}
		return writeFileAtomic(targetPath, []byte("%PDF-1.4\n%DiveEnd DeepStart sanitized session test\n"), 0600)
	}
	t.Cleanup(func() {
		deepStartDownloadWithCandidates = previousDownload
	})

	papers, _, err := app.preprocessDeepStartResults(
		context.Background(),
		"../escaped/session",
		time.Now(),
		[]SearchPaper{{
			ID:     "deepstart-paper-session-sanitize",
			Title:  "Original Title",
			URL:    "https://example.test/paper.pdf",
			Source: "test",
		}},
		SearchRetrievalStats{Query: "cache sanitize"},
	)
	if err != nil {
		t.Fatalf("preprocessDeepStartResults() error = %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected one processed paper, got %d", len(papers))
	}
	if _, ok := managedPathInsideRoot(cacheRoot, papers[0].MarkdownPath); !ok {
		t.Fatalf("markdown path escaped cache root: path=%q root=%q", papers[0].MarkdownPath, cacheRoot)
	}
	if strings.Contains(filepath.ToSlash(papers[0].MarkdownPath), "../") {
		t.Fatalf("markdown path retained traversal segment: %q", papers[0].MarkdownPath)
	}
	if !strings.Contains(filepath.ToSlash(papers[0].MarkdownPath), safeSyncSegment("../escaped/session")) {
		t.Fatalf("expected sanitized session segment in markdown path, got %q", papers[0].MarkdownPath)
	}
}

func TestPreprocessDeepStartResultsRejectsSymlinkedCacheRoot(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		app.stopDownloadWorkers()
		if app.db != nil {
			_ = app.db.Close()
		}
	})

	outsideDir := t.TempDir()
	cacheRoot := filepath.Join(config.DataPath, "deepstart_cache")
	if err := os.Symlink(outsideDir, cacheRoot); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	previousDownload := deepStartDownloadWithCandidates
	downloadCalled := false
	deepStartDownloadWithCandidates = func(ctx context.Context, candidates []string, targetPath string) error {
		downloadCalled = true
		return fmt.Errorf("download should not be called")
	}
	t.Cleanup(func() {
		deepStartDownloadWithCandidates = previousDownload
	})

	_, _, err := app.preprocessDeepStartResults(
		context.Background(),
		"session-symlink-cache",
		time.Now(),
		[]SearchPaper{{
			ID:     "deepstart-paper-symlink-cache",
			Title:  "Symlink Cache Root",
			URL:    "https://example.test/paper.pdf",
			Source: "test",
		}},
		SearchRetrievalStats{Query: "cache symlink"},
	)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked cache root error, got %v", err)
	}
	if downloadCalled {
		t.Fatal("download should not be attempted when deepstart cache root is a symlink")
	}
	entries, readErr := os.ReadDir(outsideDir)
	if readErr != nil {
		t.Fatalf("ReadDir outside cache target error = %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no cache files outside DataPath, got %v", entries)
	}
}
