package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeLLM struct {
	translated string
	summary    string
	analysis   DeepStartAIResponse
	screening  *ScreeningDecisionNode
}

func (f fakeLLM) TranslateSection(section, originalText string) (string, string, error) {
	return f.translated, f.summary, nil
}

func (f fakeLLM) AnalyzeDeepStart(request DeepStartAIRequest) (*DeepStartAIResponse, error) {
	response := f.analysis
	if response.Title == "" {
		response.Title = normalizeDeepStartTitle("", request.RootPrompt, request.CurrentQuery)
	}
	if response.Analysis.Overview == "" {
		response.Analysis = DeepStartAnalysis{
			Overview:            "测试分析",
			RecommendedPaperIDs: []string{},
		}
	}
	return &response, nil
}

func (f fakeLLM) AnalyzeScreening(request ScreeningAIRequest) (*ScreeningDecisionNode, error) {
	if f.screening != nil {
		cloned := *f.screening
		return &cloned, nil
	}
	return &ScreeningDecisionNode{
		ID:        "screen-node-test",
		NodeType:  "branch",
		Message:   "按主题继续筛选",
		Dimension: "主题",
		Options: []ScreeningDecisionOption{
			{Key: "core", Label: "核心方向", PaperIDs: []string{request.Papers[0].ID}, Count: 1},
		},
		AllowMultiSelect:  true,
		AllowSkip:         false,
		RemainingPaperIDs: []string{request.Papers[0].ID},
	}, nil
}

type fakeSearch struct {
	calls     []string
	limits    []int
	results   map[string][]SearchPaper
	lastStats SearchRetrievalStats
}

func (f *fakeSearch) Search(query string, limit int) ([]SearchPaper, error) {
	f.calls = append(f.calls, query)
	f.limits = append(f.limits, limit)
	if results, ok := f.results[query]; ok {
		f.lastStats = SearchRetrievalStats{
			Query:      query,
			RawCount:   len(results),
			DedupCount: len(results),
			FinalCount: len(results),
		}
		return results, nil
	}
	f.lastStats = SearchRetrievalStats{Query: query}
	return nil, fmt.Errorf("query not found: %s", query)
}

func (f *fakeSearch) SearchWithContext(ctx context.Context, query string, limit int) ([]SearchPaper, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return f.Search(query, limit)
}

func (f *fakeSearch) EnhancedSearch(query string, limit int, offset int, yearStart int, yearEnd int, sortBy string) (*EnhancedSearchResult, error) {
	f.calls = append(f.calls, query)
	if results, ok := f.results[query]; ok {
		return &EnhancedSearchResult{
			Query:   query,
			Limit:   limit,
			Offset:  offset,
			Total:   len(results),
			HasMore: offset+limit < len(results),
			Papers:  results,
			Sources: []SearchSourceStatus{
				{Name: "test", Success: true, Count: len(results)},
			},
		}, nil
	}
	return nil, fmt.Errorf("query not found: %s", query)
}

func (f *fakeSearch) LastSearchStats() SearchRetrievalStats {
	return f.lastStats
}

type panicDeepStartLLM struct{}

func (panicDeepStartLLM) TranslateSection(section, originalText string) (string, string, error) {
	return "", "", nil
}

func (panicDeepStartLLM) AnalyzeDeepStart(request DeepStartAIRequest) (*DeepStartAIResponse, error) {
	panic("AnalyzeDeepStart should not be called when search results are empty")
}

func (panicDeepStartLLM) AnalyzeScreening(request ScreeningAIRequest) (*ScreeningDecisionNode, error) {
	return &ScreeningDecisionNode{
		ID:                "panic-llm-screen",
		NodeType:          "complete",
		Message:           "complete",
		Dimension:         "dimension",
		Options:           []ScreeningDecisionOption{},
		AllowMultiSelect:  false,
		AllowSkip:         false,
		RemainingPaperIDs: []string{},
	}, nil
}

type blockingSearch struct {
	started chan struct{}
	calls   int
}

func (b *blockingSearch) Search(query string, limit int) ([]SearchPaper, error) {
	return nil, context.Canceled
}

func (b *blockingSearch) SearchWithContext(ctx context.Context, query string, limit int) ([]SearchPaper, error) {
	b.calls++
	if b.started != nil {
		select {
		case b.started <- struct{}{}:
		default:
		}
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (b *blockingSearch) EnhancedSearch(query string, limit int, offset int, yearStart int, yearEnd int, sortBy string) (*EnhancedSearchResult, error) {
	return nil, context.Canceled
}

func (b *blockingSearch) LastSearchStats() SearchRetrievalStats {
	return SearchRetrievalStats{
		Query:      "blocking",
		RawCount:   0,
		DedupCount: 0,
		FinalCount: 0,
	}
}

func waitForRunningDeepStartTaskID(app *App, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		app.deepStartTaskMu.Lock()
		for sessionID := range app.deepStartTasks {
			app.deepStartTaskMu.Unlock()
			return sessionID, nil
		}
		app.deepStartTaskMu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	return "", fmt.Errorf("timeout waiting for running deepstart task")
}

func TestAppSaveConfigPersistsAndSignalsRestart(t *testing.T) {
	useTestConfigPath(t)

	app := NewApp()
	initialConfig := defaultAppConfig()
	initialConfig.DataPath = t.TempDir()
	initialConfig.LLM = defaultOpenAICompatibleLLMConfig()
	initialConfig.LLM.ProviderID = "duckcoding"
	initialConfig.LLM.ProviderName = "DuckCoding"
	initialConfig.LLM.BaseURL = "https://api.duckcoding.ai/v1"
	initialConfig.LLM.APIKey = "initial-key"
	initialConfig.LLM.Model = "gpt-5.3-codex"
	initialConfig.WeakLLM = defaultOpenAICompatibleLLMConfig()
	initialConfig.WeakLLM.ProviderID = "duckcoding-lite"
	initialConfig.WeakLLM.ProviderName = "DuckCoding Lite"
	initialConfig.WeakLLM.BaseURL = "https://api.duckcoding.ai/v1"
	initialConfig.WeakLLM.APIKey = "weak-initial-key"
	initialConfig.WeakLLM.Model = "gpt-4o-mini"
	if err := app.applyConfig(initialConfig, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	nextConfig := app.config
	nextConfig.Theme = "dark"
	nextConfig.DataPath = t.TempDir()
	nextConfig.LLM.APIKey = ""
	nextConfig.WeakLLM.APIKey = ""
	nextConfig.LLM.ReasoningEffort = "xhigh"

	result, err := app.SaveConfig(nextConfig)
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	if !result.RestartRequired {
		t.Fatal("expected changing data path to require restart")
	}
	if result.Config.Theme != "dark" {
		t.Fatalf("expected saved theme to be dark, got %q", result.Config.Theme)
	}
	if result.Config.LLM.APIKey != "" {
		t.Fatal("expected returned config to redact llm api key")
	}
	if !result.Config.LLM.HasAPIKey {
		t.Fatal("expected returned config to report api key is configured")
	}

	loaded, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}
	if loaded.DataPath != nextConfig.DataPath {
		t.Fatalf("expected config file to persist data path %q, got %q", nextConfig.DataPath, loaded.DataPath)
	}
	if loaded.LLM.APIKey != "initial-key" {
		t.Fatalf("expected empty save payload to preserve existing api key, got %q", loaded.LLM.APIKey)
	}
	if loaded.WeakLLM.APIKey != "weak-initial-key" {
		t.Fatalf("expected empty weak llm save payload to preserve existing api key, got %q", loaded.WeakLLM.APIKey)
	}
	if loaded.LLM.ReasoningEffort != "xhigh" {
		t.Fatalf("expected reasoning effort to persist, got %q", loaded.LLM.ReasoningEffort)
	}
}

func TestAppImportAndTranslateFlow(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	folder, err := app.CreateFolder("Agents")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	imported, err := app.ImportPapers(folder.ID, []SearchPaper{
		{
			ID:       "paper-1",
			Title:    "Paper 1",
			Authors:  "Author",
			Abstract: "Abstract",
			Year:     2025,
			Journal:  "arXiv",
			URL:      "https://example.com/paper-1",
			Category: "agents",
			Tags:     []string{"llm"},
		},
	})
	if err != nil {
		t.Fatalf("ImportPapers() error = %v", err)
	}
	if len(imported) != 1 {
		t.Fatalf("expected one imported paper, got %d", len(imported))
	}

	initialState, err := app.GetInitialState()
	if err != nil {
		t.Fatalf("GetInitialState() error = %v", err)
	}
	if len(initialState.Folders) < 2 {
		t.Fatalf("expected default folder and Agents folders, got %d", len(initialState.Folders))
	}

	app.llm = fakeLLM{translated: "你好，世界", summary: "测试摘要"}
	record, err := app.TranslatePaperSection(imported[0].ID, "Abstract", "Hello, world")
	if err != nil {
		t.Fatalf("TranslatePaperSection() error = %v", err)
	}
	if record.TranslatedText != "你好，世界" {
		t.Fatalf("expected fake translation to be used, got %q", record.TranslatedText)
	}

	translations, err := app.GetTranslations(imported[0].ID)
	if err != nil {
		t.Fatalf("GetTranslations() error = %v", err)
	}
	if len(translations) != 1 {
		t.Fatalf("expected one translation in history, got %d", len(translations))
	}
	if translations[0].Summary != "测试摘要" {
		t.Fatalf("expected summary to round-trip, got %q", translations[0].Summary)
	}
}

func TestAppImportPapersWithAssetsAllowsFolderCopiesAndSkipsSameFolderDuplicates(t *testing.T) {
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

	folderA, err := app.CreateFolder("Folder A")
	if err != nil {
		t.Fatalf("CreateFolder(folderA) error = %v", err)
	}
	folderB, err := app.CreateFolder("Folder B")
	if err != nil {
		t.Fatalf("CreateFolder(folderB) error = %v", err)
	}

	input := []SearchPaper{
		{
			ID:       "shared-paper-1",
			Title:    "Shared Paper",
			Authors:  "Tester",
			Abstract: "Abstract",
			Year:     2025,
			Journal:  "arXiv",
			URL:      "",
			Category: "test",
		},
	}

	first, err := app.ImportPapersWithAssets(folderA.ID, input)
	if err != nil {
		t.Fatalf("ImportPapersWithAssets(first) error = %v", err)
	}
	if len(first.Imported) != 1 || first.Queued != 1 {
		t.Fatalf("expected first import to enqueue one paper, got %+v", first)
	}

	second, err := app.ImportPapersWithAssets(folderA.ID, input)
	if err != nil {
		t.Fatalf("ImportPapersWithAssets(second) error = %v", err)
	}
	if len(second.Imported) != 0 || len(second.Skipped) != 1 {
		t.Fatalf("expected same-folder duplicate to be skipped, got %+v", second)
	}

	third, err := app.ImportPapersWithAssets(folderB.ID, input)
	if err != nil {
		t.Fatalf("ImportPapersWithAssets(third) error = %v", err)
	}
	if len(third.Imported) != 1 {
		t.Fatalf("expected cross-folder copy to be allowed, got %+v", third)
	}

	papersA, err := app.GetPapers(folderA.ID)
	if err != nil {
		t.Fatalf("GetPapers(folderA) error = %v", err)
	}
	papersB, err := app.GetPapers(folderB.ID)
	if err != nil {
		t.Fatalf("GetPapers(folderB) error = %v", err)
	}
	if len(papersA) != 1 || len(papersB) != 1 {
		t.Fatalf("expected one paper in each folder, got A=%d B=%d", len(papersA), len(papersB))
	}
	if papersA[0].ID == papersB[0].ID {
		t.Fatalf("expected folder copies to use different IDs, got %q", papersA[0].ID)
	}
	if papersA[0].SourcePaperID != "shared-paper-1" || papersB[0].SourcePaperID != "shared-paper-1" {
		t.Fatalf("expected source paper id to persist, got A=%q B=%q", papersA[0].SourcePaperID, papersB[0].SourcePaperID)
	}
}

func TestAppImportPapersWithAssetsDownloadsPDFAndUpdatesStorageOverview(t *testing.T) {
	pdfPayload := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdfPayload)
	}))
	defer server.Close()

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

	folder, err := app.CreateFolder("PDF Downloads")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	result, err := app.ImportPapersWithAssets(folder.ID, []SearchPaper{
		{
			ID:       "download-paper-1",
			Title:    "Download Paper",
			Authors:  "Tester",
			Abstract: "Abstract",
			Year:     2025,
			Journal:  "arXiv",
			URL:      server.URL + "/paper.pdf",
			Category: "test",
		},
	})
	if err != nil {
		t.Fatalf("ImportPapersWithAssets() error = %v", err)
	}
	if len(result.Imported) != 1 {
		t.Fatalf("expected one imported paper, got %+v", result)
	}

	var downloaded Paper
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		papers, err := app.GetPapers(folder.ID)
		if err != nil {
			t.Fatalf("GetPapers() error = %v", err)
		}
		if len(papers) > 0 && papers[0].DownloadStatus == "downloaded" {
			downloaded = papers[0]
			break
		}
		time.Sleep(80 * time.Millisecond)
	}

	if downloaded.ID == "" {
		t.Fatal("expected paper download to complete within timeout")
	}
	if strings.TrimSpace(downloaded.PDFPath) == "" {
		t.Fatal("expected downloaded paper to have pdf path")
	}
	content, err := os.ReadFile(downloaded.PDFPath)
	if err != nil {
		t.Fatalf("ReadFile(downloaded.PDFPath) error = %v", err)
	}
	if !strings.HasPrefix(string(content), "%PDF-1.4") {
		t.Fatalf("expected downloaded file to be pdf payload, got %q", string(content))
	}

	overview, err := app.GetLocalStorageOverview()
	if err != nil {
		t.Fatalf("GetLocalStorageOverview() error = %v", err)
	}
	if overview.Downloaded < 1 {
		t.Fatalf("expected at least one downloaded paper in overview, got %+v", overview)
	}

	expectedFolderPath := filepath.Join(config.DataPath, "papers", filepath.FromSlash(folder.Path))
	foundFolder := false
	for _, item := range overview.Folders {
		if item.FolderID == folder.ID {
			foundFolder = true
			if item.FolderPath != expectedFolderPath {
				t.Fatalf("expected folder path %q, got %q", expectedFolderPath, item.FolderPath)
			}
			if item.StoredFileCount < 1 {
				t.Fatalf("expected at least one stored file, got %+v", item)
			}
		}
	}
	if !foundFolder {
		t.Fatalf("expected overview to include folder %q", folder.ID)
	}
}

func TestAppDeepStartSessionFlow(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	folder, err := app.CreateFolder("Exploration")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	search := &fakeSearch{
		results: map[string][]SearchPaper{
			"llm agents": {
				{ID: "paper-1", Title: "Agents Survey", Year: 2024, Journal: "arXiv"},
				{ID: "paper-2", Title: "Agents Benchmark", Year: 2025, Journal: "arXiv"},
			},
			"llm agents benchmark": {
				{ID: "paper-2", Title: "Agents Benchmark", Year: 2025, Journal: "arXiv"},
			},
		},
	}
	app.search = search
	app.llm = fakeLLM{
		analysis: DeepStartAIResponse{
			Title: "LLM Agents",
			Analysis: DeepStartAnalysis{
				Overview: "这是测试分析。",
				Directions: []DeepStartDirection{
					{
						ID:       "mainline",
						Name:     "主线",
						Summary:  "代表性论文",
						Why:      "适合先看",
						PaperIDs: []string{"paper-1", "paper-2"},
					},
				},
				PaperNotes: []DeepStartPaperNote{
					{PaperID: "paper-1", Tier: "core", Reason: "适合作为入口", DirectionIDs: []string{"mainline"}},
				},
				FollowUpQuestions:   []string{"先看 survey"},
				SuggestedQueries:    []string{"llm agents benchmark"},
				RecommendedPaperIDs: []string{"paper-1"},
			},
		},
	}

	session, err := app.StartDeepStartSession("llm agents", folder.ID)
	if err != nil {
		t.Fatalf("StartDeepStartSession() error = %v", err)
	}
	if session.Summary.CurrentQuery != "llm agents" {
		t.Fatalf("expected current query to match prompt, got %q", session.Summary.CurrentQuery)
	}
	if len(session.Messages) != 2 {
		t.Fatalf("expected initial user+assistant messages, got %d", len(session.Messages))
	}
	if len(session.CurrentResults) != 2 {
		t.Fatalf("expected 2 current results, got %d", len(session.CurrentResults))
	}

	replied, err := app.ReplyDeepStartSession(session.Summary.ID, "先看 benchmark")
	if err != nil {
		t.Fatalf("ReplyDeepStartSession() error = %v", err)
	}
	if len(replied.Messages) != 4 {
		t.Fatalf("expected 4 messages after reply, got %d", len(replied.Messages))
	}
	if len(search.calls) != 1 {
		t.Fatalf("expected reply not to trigger search, got calls %+v", search.calls)
	}

	rerun, err := app.RerunDeepStartSearch(session.Summary.ID, "llm agents benchmark")
	if err != nil {
		t.Fatalf("RerunDeepStartSearch() error = %v", err)
	}
	if rerun.Summary.CurrentQuery != "llm agents benchmark" {
		t.Fatalf("expected rerun query to persist, got %q", rerun.Summary.CurrentQuery)
	}
	if len(rerun.CurrentResults) != 1 {
		t.Fatalf("expected rerun to replace current results, got %d", len(rerun.CurrentResults))
	}
	if len(search.calls) != 2 {
		t.Fatalf("expected rerun to trigger second search, got calls %+v", search.calls)
	}

	updated, err := app.UpdateDeepStartSelections(session.Summary.ID, []string{"paper-2"}, folder.ID)
	if err != nil {
		t.Fatalf("UpdateDeepStartSelections() error = %v", err)
	}
	if len(updated.SelectedPaperIDs) != 1 || updated.SelectedPaperIDs[0] != "paper-2" {
		t.Fatalf("expected selected paper to persist, got %+v", updated.SelectedPaperIDs)
	}

	initialState, err := app.GetInitialState()
	if err != nil {
		t.Fatalf("GetInitialState() error = %v", err)
	}
	if len(initialState.DeepStartSessions) != 1 {
		t.Fatalf("expected one DeepStart session in initial state, got %d", len(initialState.DeepStartSessions))
	}
	if initialState.ActiveDeepStartSession == nil {
		t.Fatal("expected active DeepStart session to hydrate")
	}
	if len(initialState.ActiveDeepStartSession.SelectedPaperIDs) != 1 {
		t.Fatalf("expected selected paper IDs to hydrate, got %+v", initialState.ActiveDeepStartSession.SelectedPaperIDs)
	}
}

func TestAppDeepStartReplyNarrowAndUndo(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	search := &fakeSearch{
		results: map[string][]SearchPaper{
			"embodied intelligence": {
				{ID: "paper-1", Title: "Embodied Survey", Year: 2024, Journal: "arXiv"},
				{ID: "paper-2", Title: "Embodied Benchmark", Year: 2025, Journal: "arXiv"},
				{ID: "paper-3", Title: "Embodied Dataset", Year: 2023, Journal: "arXiv"},
			},
		},
	}
	app.search = search
	app.llm = fakeLLM{
		analysis: DeepStartAIResponse{
			Title: "Embodied",
			Analysis: DeepStartAnalysis{
				Overview: "测试缩窄",
				Directions: []DeepStartDirection{
					{ID: "d-1", Name: "benchmark", Summary: "summary", Why: "why", PaperIDs: []string{"paper-2"}},
				},
				PaperNotes: []DeepStartPaperNote{
					{PaperID: "paper-2", Tier: "core", Reason: "keep", DirectionIDs: []string{"d-1"}},
				},
				FollowUpQuestions:   []string{"next"},
				SuggestedQueries:    []string{"embodied benchmark"},
				RecommendedPaperIDs: []string{"paper-2"},
				RetainedPaperIDs:    []string{"paper-2"},
			},
		},
	}

	session, err := app.StartDeepStartSession("embodied intelligence", "")
	if err != nil {
		t.Fatalf("StartDeepStartSession() error = %v", err)
	}
	if len(session.CurrentResults) != 3 {
		t.Fatalf("expected 3 initial results, got %d", len(session.CurrentResults))
	}

	narrowed, err := app.ReplyDeepStartSession(session.Summary.ID, "先只看 benchmark")
	if err != nil {
		t.Fatalf("ReplyDeepStartSession() error = %v", err)
	}
	if len(narrowed.CurrentResults) >= 3 {
		t.Fatalf("expected narrowed result set, got %d", len(narrowed.CurrentResults))
	}

	restored, err := app.UndoDeepStartNarrow(session.Summary.ID)
	if err != nil {
		t.Fatalf("UndoDeepStartNarrow() error = %v", err)
	}
	if len(restored.CurrentResults) != 3 {
		t.Fatalf("expected undo to restore 3 results, got %d", len(restored.CurrentResults))
	}
}

func TestAppCancelDeepStartStartRollback(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		app.cancelAllDeepStartTasks()
		_ = app.db.Close()
	})

	blocking := &blockingSearch{started: make(chan struct{}, 1)}
	app.search = blocking
	app.llm = panicDeepStartLLM{}

	errCh := make(chan error, 1)
	go func() {
		_, err := app.StartDeepStartSession("cancel me", "")
		errCh <- err
	}()

	select {
	case <-blocking.started:
	case <-time.After(2 * time.Second):
		t.Fatal("search did not start in time")
	}

	sessionID, err := waitForRunningDeepStartTaskID(app, 2*time.Second)
	if err != nil {
		t.Fatalf("waitForRunningDeepStartTaskID() error = %v", err)
	}
	if err := app.CancelDeepStartTask(sessionID); err != nil {
		t.Fatalf("CancelDeepStartTask() error = %v", err)
	}

	select {
	case err := <-errCh:
		if !isDeepStartCancelledError(err) {
			t.Fatalf("expected cancellation error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("StartDeepStartSession did not return after cancellation")
	}

	sessions, err := app.ListDeepStartSessions()
	if err != nil {
		t.Fatalf("ListDeepStartSessions() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no persisted sessions after cancellation, got %d", len(sessions))
	}

	var messageCount int
	if err := app.db.conn.QueryRow(`SELECT COUNT(*) FROM deepstart_messages`).Scan(&messageCount); err != nil {
		t.Fatalf("count deepstart_messages error = %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("expected no persisted messages after cancellation, got %d", messageCount)
	}
}

func TestAppCancelDeepStartRerunRollback(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		app.cancelAllDeepStartTasks()
		_ = app.db.Close()
	})

	initialSearch := &fakeSearch{
		results: map[string][]SearchPaper{
			"seed": {
				{ID: "paper-1", Title: "Seed Paper", Year: 2024, Journal: "arXiv"},
			},
		},
	}
	app.search = initialSearch
	app.llm = fakeLLM{
		analysis: DeepStartAIResponse{
			Title: "Seed Session",
			Analysis: DeepStartAnalysis{
				Overview:            "seed",
				Directions:          []DeepStartDirection{},
				PaperNotes:          []DeepStartPaperNote{},
				FollowUpQuestions:   []string{},
				SuggestedQueries:    []string{},
				RecommendedPaperIDs: []string{},
			},
		},
	}

	session, err := app.StartDeepStartSession("seed", "")
	if err != nil {
		t.Fatalf("StartDeepStartSession(seed) error = %v", err)
	}
	originalMessages := len(session.Messages)
	originalQuery := session.Summary.CurrentQuery
	originalResults := len(session.CurrentResults)

	var originalRounds int
	if err := app.db.conn.QueryRow(`SELECT COUNT(*) FROM deepstart_search_rounds WHERE session_id = ?`, session.Summary.ID).Scan(&originalRounds); err != nil {
		t.Fatalf("count original rounds error = %v", err)
	}

	blocking := &blockingSearch{started: make(chan struct{}, 1)}
	app.search = blocking

	errCh := make(chan error, 1)
	go func() {
		_, err := app.RerunDeepStartSearch(session.Summary.ID, "new query")
		errCh <- err
	}()

	select {
	case <-blocking.started:
	case <-time.After(2 * time.Second):
		t.Fatal("rerun search did not start in time")
	}

	if err := app.CancelDeepStartTask(session.Summary.ID); err != nil {
		t.Fatalf("CancelDeepStartTask() error = %v", err)
	}

	select {
	case err := <-errCh:
		if !isDeepStartCancelledError(err) {
			t.Fatalf("expected rerun cancellation error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RerunDeepStartSearch did not return after cancellation")
	}

	latest, err := app.GetDeepStartSession(session.Summary.ID)
	if err != nil {
		t.Fatalf("GetDeepStartSession() error = %v", err)
	}
	if latest.Summary.CurrentQuery != originalQuery {
		t.Fatalf("expected query rollback to %q, got %q", originalQuery, latest.Summary.CurrentQuery)
	}
	if len(latest.CurrentResults) != originalResults {
		t.Fatalf("expected results rollback to %d papers, got %d", originalResults, len(latest.CurrentResults))
	}
	if len(latest.Messages) != originalMessages {
		t.Fatalf("expected message rollback to %d messages, got %d", originalMessages, len(latest.Messages))
	}

	var latestRounds int
	if err := app.db.conn.QueryRow(`SELECT COUNT(*) FROM deepstart_search_rounds WHERE session_id = ?`, session.Summary.ID).Scan(&latestRounds); err != nil {
		t.Fatalf("count latest rounds error = %v", err)
	}
	if latestRounds != originalRounds {
		t.Fatalf("expected search rounds rollback to %d, got %d", originalRounds, latestRounds)
	}
}

func TestAppDeepStartUsesConfiguredResultLimitAndPersistsSearchStats(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.Search.DeepStartResultLimit = 200
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	search := &fakeSearch{
		results: map[string][]SearchPaper{
			"embodied intelligence": {
				{ID: "paper-1", Title: "Embodied Survey", Year: 2025, Source: "arxiv"},
				{ID: "paper-2", Title: "Embodied Benchmark", Year: 2024, Source: "semantic_scholar"},
			},
		},
	}
	app.search = search
	app.llm = nil

	session, err := app.StartDeepStartSession("embodied intelligence", "")
	if err != nil {
		t.Fatalf("StartDeepStartSession() error = %v", err)
	}
	if len(search.limits) != 1 {
		t.Fatalf("expected one search call, got %d", len(search.limits))
	}
	if search.limits[0] != 200 {
		t.Fatalf("expected deepstart search limit=200, got %d", search.limits[0])
	}
	if session.CurrentAnalysis == nil {
		t.Fatal("expected current analysis to exist")
	}
	if session.CurrentAnalysis.SearchStats.FinalCount != 2 {
		t.Fatalf("expected search stats final count=2, got %+v", session.CurrentAnalysis.SearchStats)
	}
}

func TestAppDeepStartFallsBackWithoutLLM(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	app.search = &fakeSearch{
		results: map[string][]SearchPaper{
			"world model": {
				{ID: "paper-1", Title: "World Model Survey", Year: 2024},
			},
		},
	}
	app.llm = nil

	session, err := app.StartDeepStartSession("world model", "")
	if err != nil {
		t.Fatalf("StartDeepStartSession() error = %v", err)
	}
	if session.CurrentAnalysis == nil {
		t.Fatal("expected fallback analysis to be generated")
	}
	if session.CurrentAnalysis.Overview == "" {
		t.Fatal("expected fallback overview to be populated")
	}
}

func TestAppDeepStartSkipsLLMWhenSearchFailsWithNoResults(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	app.search = &fakeSearch{results: map[string][]SearchPaper{}}
	app.llm = panicDeepStartLLM{}

	session, err := app.StartDeepStartSession("embodied intelligence", "")
	if err != nil {
		t.Fatalf("StartDeepStartSession() error = %v", err)
	}
	if session.CurrentAnalysis == nil {
		t.Fatal("expected fallback analysis to be present")
	}
	if !strings.Contains(session.CurrentAnalysis.Overview, "本轮检索暂时失败") {
		t.Fatalf("expected search failure warning in overview, got %q", session.CurrentAnalysis.Overview)
	}
}
