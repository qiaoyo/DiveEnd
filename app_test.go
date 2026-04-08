package main

import (
	"fmt"
	"testing"
)

type fakeLLM struct {
	translated string
	summary    string
	analysis   DeepStartAIResponse
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

type fakeSearch struct {
	calls   []string
	results map[string][]SearchPaper
}

func (f *fakeSearch) Search(query string, limit int) ([]SearchPaper, error) {
	f.calls = append(f.calls, query)
	if results, ok := f.results[query]; ok {
		return results, nil
	}
	return nil, fmt.Errorf("query not found: %s", query)
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
		t.Fatalf("expected Inbox and Agents folders, got %d", len(initialState.Folders))
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
