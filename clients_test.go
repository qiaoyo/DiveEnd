package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func twoSourceSearchTestConfig() AppConfig {
	config := defaultAppConfig()
	config.Search.EnableSemanticScholar = true
	config.Search.EnableArxiv = true
	config.Search.EnableOpenAlex = false
	config.Search.EnableOpenReview = false
	config.Search.EnableDBLP = false
	return config
}

func TestRankSearchPapersExplainsQueryMatches(t *testing.T) {
	papers := []SearchPaper{
		{ID: "abstract", Title: "A Recent Systems Paper", Abstract: "We evaluate code agent reliability.", Year: 2026},
		{ID: "title", Title: "Code Agent Benchmark", Abstract: "A benchmark suite.", Year: 2024},
		{ID: "fallback", Title: "Unrelated Study", Abstract: "No matching terms.", Year: 2027},
	}

	ranked := rankSearchPapersByQueries(papers, []string{"code agent benchmark", "agent reliability"})
	if ranked[0].ID != "title" {
		t.Fatalf("expected direct title phrase match first, got %+v", ranked)
	}
	if !strings.Contains(ranked[0].MatchReason, "标题直接匹配") {
		t.Fatalf("expected title match explanation, got %q", ranked[0].MatchReason)
	}
	if len(ranked[0].MatchedTerms) == 0 {
		t.Fatalf("expected matched terms, got %+v", ranked[0])
	}

	var fallback SearchPaper
	for _, paper := range ranked {
		if paper.ID == "fallback" {
			fallback = paper
		}
	}
	if !strings.Contains(fallback.MatchReason, "来源召回") {
		t.Fatalf("expected transparent fallback explanation, got %q", fallback.MatchReason)
	}
}

func TestParseArXivXML(t *testing.T) {
	xmlData := []byte(`
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2401.12345v1</id>
    <title>
      Example Paper Title
    </title>
    <summary>
      Example summary text with
      multiple lines.
    </summary>
    <published>2024-01-15T00:00:00Z</published>
    <author><name>Author One</name></author>
    <author><name>Author Two</name></author>
  </entry>
</feed>
`)

	papers, err := parseArXivXML(xmlData)
	if err != nil {
		t.Fatalf("parseArXivXML() error = %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper, got %d", len(papers))
	}

	paper := papers[0]
	if paper.ID != "2401.12345v1" {
		t.Fatalf("expected arXiv short ID, got %q", paper.ID)
	}
	if paper.Title != "Example Paper Title" {
		t.Fatalf("expected trimmed title, got %q", paper.Title)
	}
	if paper.Authors != "Author One, Author Two" {
		t.Fatalf("expected authors to join correctly, got %q", paper.Authors)
	}
	if paper.Year != 2024 {
		t.Fatalf("expected year 2024, got %d", paper.Year)
	}
}

func TestLLMClientTranslateSectionUsesResponsesAPI(t *testing.T) {
	config := defaultAppConfig()
	config.LLM = defaultOpenAICompatibleLLMConfig()
	config.LLM.ProviderID = "duckcoding"
	config.LLM.ProviderName = "DuckCoding"
	config.LLM.BaseURL = "https://example.com/v1"
	config.LLM.APIKey = "sk-test"
	config.LLM.Model = "gpt-5.3-codex"
	config.LLM.ReasoningEffort = "xhigh"

	client := NewLLMClient(config)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/responses" {
				t.Fatalf("expected /v1/responses, got %s", r.URL.Path)
			}
			if auth := r.Header.Get("Authorization"); auth != "Bearer sk-test" {
				t.Fatalf("expected bearer auth header, got %q", auth)
			}

			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload["model"] != "gpt-5.3-codex" {
				t.Fatalf("expected model gpt-5.3-codex, got %#v", payload["model"])
			}
			if payload["store"] != false {
				t.Fatalf("expected store=false when response storage is disabled, got %#v", payload["store"])
			}

			reasoning, ok := payload["reasoning"].(map[string]any)
			if !ok || reasoning["effort"] != "xhigh" {
				t.Fatalf("expected reasoning effort xhigh, got %#v", payload["reasoning"])
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(bytes.NewBufferString(`{
  "output": [
    {
      "content": [
        {
          "type": "output_text",
          "text": "{\"translation\":\"测试译文\",\"summary\":\"测试摘要\"}"
        }
      ]
    }
  ]
}`)),
				Request: r,
			}, nil
		}),
	}
	translated, summary, err := client.TranslateSection("Intro", "Hello")
	if err != nil {
		t.Fatalf("TranslateSection() error = %v", err)
	}
	if translated != "测试译文" {
		t.Fatalf("expected translated text, got %q", translated)
	}
	if summary != "测试摘要" {
		t.Fatalf("expected summary, got %q", summary)
	}
}

func TestLLMClientTranslateSectionUsesChatCompletions(t *testing.T) {
	config := defaultAppConfig()
	config.LLM = defaultOpenAICompatibleLLMConfig()
	config.LLM.BaseURL = "https://example.com/v1"
	config.LLM.WireAPI = "chat_completions"
	config.LLM.APIKey = "sk-chat"
	config.LLM.Model = "gpt-test"

	client := NewLLMClient(config)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/chat/completions" {
				t.Fatalf("expected /v1/chat/completions, got %s", r.URL.Path)
			}
			if auth := r.Header.Get("Authorization"); auth != "Bearer sk-chat" {
				t.Fatalf("expected bearer auth header, got %q", auth)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(bytes.NewBufferString(`{
  "choices": [
    {
      "message": {
        "content": "{\"translation\":\"聊天译文\",\"summary\":\"聊天摘要\"}"
      }
    }
  ]
}`)),
				Request: r,
			}, nil
		}),
	}
	translated, summary, err := client.TranslateSection("Intro", "Hello")
	if err != nil {
		t.Fatalf("TranslateSection() error = %v", err)
	}
	if translated != "聊天译文" {
		t.Fatalf("expected translated text, got %q", translated)
	}
	if summary != "聊天摘要" {
		t.Fatalf("expected summary, got %q", summary)
	}
}

func TestLLMClientRejectsOversizedResponseBody(t *testing.T) {
	previousLimit := externalHTTPBodyLimitBytes
	externalHTTPBodyLimitBytes = 32
	t.Cleanup(func() { externalHTTPBodyLimitBytes = previousLimit })

	config := defaultAppConfig()
	config.LLM = defaultOpenAICompatibleLLMConfig()
	config.LLM.BaseURL = "https://example.com/v1"
	config.LLM.WireAPI = "chat_completions"
	config.LLM.APIKey = "sk-chat"
	config.LLM.Model = "gpt-test"

	client := NewLLMClient(config)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 33))),
				Request:    r,
			}, nil
		}),
	}

	_, _, err := client.TranslateSection("Intro", "Hello")
	if err == nil {
		t.Fatal("expected oversized LLM response to fail")
	}
	if !strings.Contains(err.Error(), "response body exceeds 32 byte limit") {
		t.Fatalf("expected response body limit error, got %v", err)
	}
}

func TestLLMClientRedactsErrorResponseBody(t *testing.T) {
	config := defaultAppConfig()
	config.LLM = defaultOpenAICompatibleLLMConfig()
	config.LLM.BaseURL = "https://example.com/v1"
	config.LLM.WireAPI = "chat_completions"
	config.LLM.APIKey = "sk-chat"
	config.LLM.Model = "gpt-test"

	client := NewLLMClient(config)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					`{"error":"upstream rejected api_key=sk-upstream-secret token=provider-token Authorization: Bearer bearer-secret-12345"}`,
				)),
				Request: r,
			}, nil
		}),
	}

	_, _, err := client.TranslateSection("Intro", "Hello")
	if err == nil {
		t.Fatal("expected LLM error")
	}
	message := err.Error()
	for _, leaked := range []string{"sk-upstream-secret", "provider-token", "bearer-secret-12345"} {
		if strings.Contains(message, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, message)
		}
	}
	if !strings.Contains(message, redactedValue) {
		t.Fatalf("expected redacted marker in %q", message)
	}
}

func TestLLMClientRedactsProviderLabelAndURLQueriesFromNetworkErrors(t *testing.T) {
	const (
		labelSecret = "sk-llm-label-secret"
		urlSecret   = "llm-url-token-secret"
		querySecret = "private-llm-query"
	)

	cases := []struct {
		name      string
		configure func(*AppConfig)
	}{
		{
			name: "responses",
			configure: func(config *AppConfig) {
				config.LLM = defaultOpenAICompatibleLLMConfig()
				config.LLM.WireAPI = "responses"
				config.LLM.RequiresOpenAIAuth = true
				config.LLM.APIKey = "sk-test"
			},
		},
		{
			name: "chat completions",
			configure: func(config *AppConfig) {
				config.LLM = defaultOpenAICompatibleLLMConfig()
				config.LLM.WireAPI = "chat_completions"
				config.LLM.RequiresOpenAIAuth = true
				config.LLM.APIKey = "sk-test"
			},
		},
		{
			name: "anthropic",
			configure: func(config *AppConfig) {
				config.LLM = defaultAnthropicLLMConfig()
				config.LLM.APIKey = "anthropic-test"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := defaultAppConfig()
			tc.configure(&config)
			config.LLM.ProviderName = "Provider api_key=" + labelSecret
			config.LLM.BaseURL = "https://example.test/v1"
			config.LLM.Model = "model-test"

			client := NewLLMClient(config)
			client.httpClient = &http.Client{
				Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("network failed for %s?query=%s&access_token=%s", r.URL.String(), querySecret, urlSecret)
				}),
			}

			_, err := client.chatWithContext(context.Background(), []llmMessage{{Role: "user", Content: "hello"}})
			if err == nil {
				t.Fatal("expected LLM network error")
			}
			message := err.Error()
			for _, leaked := range []string{labelSecret, urlSecret, querySecret} {
				if strings.Contains(message, leaked) {
					t.Fatalf("expected %q to be redacted from %q", leaked, message)
				}
			}
			if !strings.Contains(message, redactedValue) {
				t.Fatalf("expected redacted marker in %q", message)
			}
		})
	}
}

func TestLLMClientAnalyzeDeepStartParsesStructuredJSON(t *testing.T) {
	config := defaultAppConfig()
	config.LLM = defaultOpenAICompatibleLLMConfig()
	config.LLM.BaseURL = "https://example.com/v1"
	config.LLM.WireAPI = "chat_completions"
	config.LLM.APIKey = "sk-chat"
	config.LLM.Model = "gpt-test"

	client := NewLLMClient(config)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(bytes.NewBufferString(`{
  "choices": [
    {
      "message": {
        "content": "{\"title\":\"LLM Agents\",\"overview\":\"测试概览\",\"directions\":[{\"id\":\"main\",\"name\":\"主线\",\"summary\":\"代表作\",\"why\":\"值得先看\",\"paperIds\":[\"paper-1\"]}],\"paperNotes\":[{\"paperId\":\"paper-1\",\"tier\":\"core\",\"reason\":\"入口材料\",\"directionIds\":[\"main\"]}],\"followUpQuestions\":[\"先看 survey\"],\"suggestedQueries\":[\"llm agents survey\"],\"recommendedPaperIds\":[\"paper-1\"]}"
      }
    }
  ]
}`)),
				Request: r,
			}, nil
		}),
	}

	response, err := client.AnalyzeDeepStart(DeepStartAIRequest{
		RootPrompt:   "llm agents",
		CurrentQuery: "llm agents",
		Results: []SearchPaper{
			{ID: "paper-1", Title: "Agents Survey"},
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeDeepStart() error = %v", err)
	}
	if response.Title != "LLM Agents" {
		t.Fatalf("expected title to parse, got %q", response.Title)
	}
	if len(response.Analysis.Directions) != 1 {
		t.Fatalf("expected one direction, got %d", len(response.Analysis.Directions))
	}
	if len(response.Analysis.RecommendedPaperIDs) != 1 || response.Analysis.RecommendedPaperIDs[0] != "paper-1" {
		t.Fatalf("expected recommended paper IDs to parse, got %+v", response.Analysis.RecommendedPaperIDs)
	}
}

func TestSearchClientSearchUsesFocusedDefaultPerSourceLimit(t *testing.T) {
	config := twoSourceSearchTestConfig()
	config.Search.SemanticScholarAPIKey = "semantic-key"
	client := NewSearchClient(config)

	var semanticCalled bool
	var arxivCalled bool
	var arxivSanityCalled bool
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Host, "api.semanticscholar.org"):
				semanticCalled = true
				if got := r.URL.Query().Get("limit"); got != "20" {
					t.Fatalf("expected semantic limit=20, got %s", got)
				}
				if got := r.Header.Get("x-api-key"); got != "semantic-key" {
					t.Fatalf("expected semantic api key header, got %q", got)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`{
  "data": [
    {
      "paperId": "sem-1",
      "title": "Semantic Paper",
      "authors": [{"name":"Author A"}],
      "abstract": "Semantic abstract",
      "year": 2025,
      "venue": "NeurIPS",
      "journal": {"name":"NeurIPS"},
      "openAccessPdf": {"url":"https://example.com/sem-1.pdf"},
      "url": "https://www.semanticscholar.org/paper/sem-1",
      "externalIds": {"ArXiv":"2501.00001","DOI":"10.1000/xyz123","CorpusId":123456789},
      "citationCount": 321
    }
  ]
}`)),
					Request: r,
				}, nil
			case strings.Contains(r.URL.Host, "export.arxiv.org"):
				arxivCalled = true
				if got := r.URL.Query().Get("max_results"); got != "20" {
					t.Fatalf("expected arxiv max_results=20, got %s", got)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2501.00001v1</id>
    <title>ArXiv Paper</title>
    <summary>Arxiv abstract</summary>
    <published>2025-01-01T00:00:00Z</published>
    <author><name>Author B</name></author>
  </entry>
</feed>
`)),
					Request: r,
				}, nil
			case strings.Contains(r.URL.Host, "arxiv-sanity-lite.com"):
				arxivSanityCalled = true
				return nil, fmt.Errorf("unexpected call to arxiv-sanity-lite")
			default:
				return nil, fmt.Errorf("unexpected host: %s", r.URL.Host)
			}
		}),
	}

	papers, err := client.Search("vla", 40)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !semanticCalled || !arxivCalled {
		t.Fatalf("expected semantic and arxiv to be called, semantic=%v arxiv=%v", semanticCalled, arxivCalled)
	}
	if arxivSanityCalled {
		t.Fatal("did not expect arxiv-sanity-lite to be called in two-source workflow")
	}
	if len(papers) != 1 {
		t.Fatalf("expected Semantic Scholar and arXiv versions to merge by arXiv ID, got %d", len(papers))
	}
	var semanticPaper *SearchPaper
	for idx := range papers {
		if papers[idx].ID == "sem-1" {
			semanticPaper = &papers[idx]
			break
		}
	}
	if semanticPaper == nil {
		t.Fatalf("expected semantic paper in results, got %+v", papers)
	}
	if semanticPaper.PublicationVenue == "" || semanticPaper.PublicationYear == 0 {
		t.Fatalf("expected publication metadata to be populated, got %+v", semanticPaper)
	}
	if semanticPaper.CitationCount != 321 {
		t.Fatalf("expected citation count to be parsed, got %+v", semanticPaper)
	}
	if semanticPaper.ExternalIDs["ArXiv"] != "2501.00001" {
		t.Fatalf("expected semantic external ids to be parsed, got %+v", semanticPaper.ExternalIDs)
	}
	if semanticPaper.ExternalIDs["CorpusId"] != "123456789" {
		t.Fatalf("expected numeric semantic external ids to be normalized as string, got %+v", semanticPaper.ExternalIDs)
	}
	if len(semanticPaper.PDFCandidates) == 0 {
		t.Fatalf("expected semantic pdf candidates, got %+v", semanticPaper.PDFCandidates)
	}
	candidateSet := map[string]bool{}
	for _, candidate := range semanticPaper.PDFCandidates {
		candidateSet[candidate] = true
	}
	if !candidateSet["https://example.com/sem-1.pdf"] {
		t.Fatalf("expected OA pdf candidate, got %+v", semanticPaper.PDFCandidates)
	}
	if !candidateSet["https://arxiv.org/pdf/2501.00001.pdf"] {
		t.Fatalf("expected arxiv fallback candidate, got %+v", semanticPaper.PDFCandidates)
	}
	if !candidateSet["https://doi.org/10.1000/xyz123"] {
		t.Fatalf("expected doi fallback candidate, got %+v", semanticPaper.PDFCandidates)
	}
}

func TestSearchClientSearchWithContextCancellation(t *testing.T) {
	config := twoSourceSearchTestConfig()
	client := NewSearchClient(config)

	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			select {
			case <-r.Context().Done():
				return nil, r.Context().Err()
			case <-time.After(2 * time.Second):
				return nil, fmt.Errorf("unexpected long wait")
			}
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	startedAt := time.Now()
	_, err := client.SearchWithContext(ctx, "embodied", 100)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "canceled") && !strings.Contains(strings.ToLower(err.Error()), "cancelled") {
		t.Fatalf("expected cancellation error, got %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("expected SearchWithContext to stop quickly, took %v", elapsed)
	}
}

func TestSearchClientUsesPerSourceLimitWhenOverallLimitIs200(t *testing.T) {
	config := twoSourceSearchTestConfig()
	config.Search.PerSourceResultLimit = 100
	client := NewSearchClient(config)

	var semanticLimit string
	var arxivLimit string
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Host, "api.semanticscholar.org"):
				semanticLimit = r.URL.Query().Get("limit")
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`{
  "data": [
    {
      "paperId": "sem-100",
      "title": "Semantic Paper",
      "authors": [{"name":"Author"}],
      "abstract": "Abstract",
      "year": 2025,
      "venue": "ICLR",
      "journal": {"name":"ICLR"},
      "openAccessPdf": {"url":"https://example.com/sem-100.pdf"}
    }
  ]
}`)),
					Request: r,
				}, nil
			case strings.Contains(r.URL.Host, "export.arxiv.org"):
				arxivLimit = r.URL.Query().Get("max_results")
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2501.00011v1</id>
    <title>ArXiv Paper</title>
    <summary>Arxiv abstract</summary>
    <published>2025-01-01T00:00:00Z</published>
    <author><name>Author B</name></author>
  </entry>
</feed>
`)),
					Request: r,
				}, nil
			default:
				return nil, fmt.Errorf("unexpected host: %s", r.URL.Host)
			}
		}),
	}

	papers, err := client.Search("vla", 200)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(papers) != 2 {
		t.Fatalf("expected 2 papers, got %d", len(papers))
	}
	if semanticLimit != "100" {
		t.Fatalf("expected semantic per-source limit=100, got %q", semanticLimit)
	}
	if arxivLimit != "100" {
		t.Fatalf("expected arxiv per-source limit=100, got %q", arxivLimit)
	}
}

func TestSearchClientRedactsProviderErrorBodies(t *testing.T) {
	config := twoSourceSearchTestConfig()
	client := NewSearchClient(config)
	client.retryMax = 1
	client.retryInterval = 0
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					`provider failed api_key=sk-search-secret access_token=search-token Authorization: Bearer bearer-secret-12345`,
				)),
				Request: r,
			}, nil
		}),
	}

	for name, run := range map[string]func() error{
		"semantic": func() error {
			_, err := client.searchSemanticScholar(context.Background(), "robot papers", 1, nil)
			return err
		},
		"arxiv": func() error {
			_, err := client.searchArXiv(context.Background(), "robot papers", 1, nil)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := run()
			if err == nil {
				t.Fatal("expected provider error")
			}
			message := err.Error()
			for _, leaked := range []string{"sk-search-secret", "search-token", "bearer-secret-12345"} {
				if strings.Contains(message, leaked) {
					t.Fatalf("expected %q to be redacted from %q", leaked, message)
				}
			}
			if !strings.Contains(message, redactedValue) {
				t.Fatalf("expected redacted marker in %q", message)
			}
		})
	}
}

func TestDeepStartEnricherRedactsMetadataRequestNetworkErrors(t *testing.T) {
	const (
		querySecret = "private-enrichment-query"
		tokenSecret = "enrichment-token-secret"
	)
	enricher := &DeepStartEnricher{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network failed for %s", req.URL.String())
		})},
	}

	_, err := enricher.getJSON(
		context.Background(),
		"https://example.test/works?query="+querySecret+"&access_token="+tokenSecret,
		"application/json",
	)
	if err == nil {
		t.Fatal("expected metadata request network error")
	}
	message := err.Error()
	for _, leaked := range []string{querySecret, tokenSecret} {
		if strings.Contains(message, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, message)
		}
	}
	if !strings.Contains(message, redactedValue) && !strings.Contains(message, "%5Bredacted%5D") {
		t.Fatalf("expected redacted marker in %q", message)
	}
}

func TestEnhancedSearchReportsOnlyActiveSources(t *testing.T) {
	config := twoSourceSearchTestConfig()
	client := NewSearchClient(config)
	client.retryMax = 1

	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Host, "api.semanticscholar.org"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`{
  "data": [
    {
      "paperId": "sem-enhanced",
      "title": "Semantic Enhanced",
      "authors": [{"name":"Author A"}],
      "abstract": "Abstract",
      "year": 2026,
      "venue": "ICLR",
      "openAccessPdf": {"url":"https://example.com/semantic.pdf"}
    }
  ]
}`)),
					Request: r,
				}, nil
			case strings.Contains(r.URL.Host, "export.arxiv.org"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2601.00001v1</id>
    <title>ArXiv Enhanced</title>
    <summary>Arxiv abstract</summary>
    <published>2026-01-01T00:00:00Z</published>
    <author><name>Author B</name></author>
  </entry>
</feed>
`)),
					Request: r,
				}, nil
			case strings.Contains(r.URL.Host, "arxiv-sanity-lite.com"):
				return nil, fmt.Errorf("unexpected call to arxiv-sanity-lite")
			default:
				return nil, fmt.Errorf("unexpected host: %s", r.URL.Host)
			}
		}),
	}

	result, err := client.EnhancedSearch("vla", 20, 0, 0, 0, "relevance")
	if err != nil {
		t.Fatalf("EnhancedSearch() error = %v", err)
	}
	if len(result.Sources) != 2 {
		t.Fatalf("expected exactly two active sources, got %+v", result.Sources)
	}
	for _, source := range result.Sources {
		if strings.Contains(strings.ToLower(source.Name), "sanity") {
			t.Fatalf("unexpected inactive source in enhanced search response: %+v", result.Sources)
		}
	}
}

func TestSearchClientRetryStopsAfterPerSourceSuccessAndEmitsProgress(t *testing.T) {
	config := twoSourceSearchTestConfig()
	client := NewSearchClient(config)

	semanticAttempts := 0
	arxivAttempts := 0
	progressEvents := make([]SearchProgressEvent, 0, 4)
	client.SetProgressReporter(func(progress SearchProgressEvent) {
		progressEvents = append(progressEvents, progress)
	})

	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Host, "api.semanticscholar.org"):
				semanticAttempts++
				if semanticAttempts < 3 {
					return &http.Response{
						StatusCode: http.StatusTooManyRequests,
						Header:     make(http.Header),
						Body:       io.NopCloser(bytes.NewBufferString(`{"error":"rate limited"}`)),
						Request:    r,
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`{
  "data": [
    {
      "paperId": "sem-2",
      "title": "Semantic Success",
      "authors": [{"name":"Author S"}],
      "abstract": "Semantic abstract",
      "year": 2024,
      "venue": "ICLR",
      "journal": {"name":"ICLR"},
      "openAccessPdf": {"url":"https://example.com/sem-2.pdf"}
    }
  ]
}`)),
					Request: r,
				}, nil
			case strings.Contains(r.URL.Host, "export.arxiv.org"):
				arxivAttempts++
				if arxivAttempts < 2 {
					return nil, fmt.Errorf("temporary arxiv timeout")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2502.00002v1</id>
    <title>ArXiv Success</title>
    <summary>Arxiv abstract</summary>
    <published>2025-02-01T00:00:00Z</published>
    <author><name>Author X</name></author>
  </entry>
</feed>
`)),
					Request: r,
				}, nil
			default:
				return nil, fmt.Errorf("unexpected host: %s", r.URL.Host)
			}
		}),
	}

	papers, err := client.Search("agent", 100)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(papers) != 2 {
		t.Fatalf("expected 2 papers, got %d", len(papers))
	}
	if semanticAttempts != 3 {
		t.Fatalf("expected semantic to stop retrying after success at attempt 3, got %d", semanticAttempts)
	}
	if arxivAttempts != 2 {
		t.Fatalf("expected arxiv to stop retrying after success at attempt 2, got %d", arxivAttempts)
	}

	if len(progressEvents) == 0 {
		t.Fatal("expected search progress events to be emitted")
	}
	finalEvent := progressEvents[len(progressEvents)-1]
	if finalEvent.Phase != "completed" {
		t.Fatalf("expected final progress phase completed, got %q", finalEvent.Phase)
	}
	if finalEvent.TotalSources != 2 {
		t.Fatalf("expected 2 sources in progress summary, got %d", finalEvent.TotalSources)
	}
	if finalEvent.CompletedSources != 2 {
		t.Fatalf("expected completed sources = 2, got %d", finalEvent.CompletedSources)
	}
	if len(finalEvent.Sources) != 2 {
		t.Fatalf("expected two source entries, got %d", len(finalEvent.Sources))
	}
}

func TestSearchClientStopsPermanentSourceFailureAndKeepsPartialResults(t *testing.T) {
	config := twoSourceSearchTestConfig()
	client := NewSearchClient(config)
	client.retryMax = 60
	client.retryInterval = 0

	semanticAttempts := 0
	arxivAttempts := 0
	var finalProgress SearchProgressEvent
	client.SetProgressReporter(func(progress SearchProgressEvent) {
		if progress.Phase == "completed" {
			finalProgress = progress
		}
	})

	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Host, "api.semanticscholar.org"):
				semanticAttempts++
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewBufferString(`{"message":"Forbidden"}`)),
					Request:    r,
				}, nil
			case strings.Contains(r.URL.Host, "export.arxiv.org"):
				arxivAttempts++
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2607.00001v1</id>
    <title>Partial Search Still Works</title>
    <summary>ArXiv remains available.</summary>
    <published>2026-07-01T00:00:00Z</published>
    <author><name>Author A</name></author>
  </entry>
</feed>
`)),
					Request: r,
				}, nil
			default:
				return nil, fmt.Errorf("unexpected host: %s", r.URL.Host)
			}
		}),
	}

	papers, err := client.Search("reliable agents", 20)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(papers) != 1 || papers[0].Source != "arxiv" {
		t.Fatalf("expected one arXiv partial result, got %+v", papers)
	}
	if semanticAttempts != 1 {
		t.Fatalf("expected permanent 403 to stop after one attempt, got %d", semanticAttempts)
	}
	if arxivAttempts != 1 {
		t.Fatalf("expected arXiv to complete once, got %d attempts", arxivAttempts)
	}

	var semanticProgress *SearchSourceProgress
	for index := range finalProgress.Sources {
		if finalProgress.Sources[index].Name == searchSourceSemantic {
			semanticProgress = &finalProgress.Sources[index]
			break
		}
	}
	if semanticProgress == nil {
		t.Fatal("expected Semantic Scholar progress in final event")
	}
	if !semanticProgress.Done || semanticProgress.Status != "failed" || semanticProgress.Attempt != 1 {
		t.Fatalf("expected terminal first-attempt failure, got %+v", semanticProgress)
	}
}

func TestSearchClientRedactsProgressLogsAndAggregatedErrors(t *testing.T) {
	const (
		accessToken = "search-access-secret"
		bearerToken = "search-bearer-secret"
		query       = "private robotics query"
	)
	config := twoSourceSearchTestConfig()
	client := NewSearchClient(config)
	client.retryMax = 1
	client.retryInterval = 0
	client.overallTimeout = time.Second
	client.attemptTimeout = time.Second

	progressEvents := make([]SearchProgressEvent, 0, 4)
	client.SetProgressReporter(func(progress SearchProgressEvent) {
		progressEvents = append(progressEvents, progress)
	})

	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network failed for %s&access_token=%s with Authorization: Bearer %s", r.URL.String(), accessToken, bearerToken)
		}),
	}

	var logs bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	}()

	_, err := client.Search(query, 10)
	if err == nil {
		t.Fatal("expected search failure")
	}

	combined := err.Error() + "\n" + logs.String()
	for _, event := range progressEvents {
		for _, source := range event.Sources {
			combined += "\n" + source.Error
		}
	}
	for _, leaked := range []string{accessToken, bearerToken, "private+robotics+query", query} {
		if strings.Contains(combined, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, combined)
		}
	}
	if !strings.Contains(combined, redactedValue) {
		t.Fatalf("expected redacted marker in %q", combined)
	}
}

func TestSearchClientDedupeUsesTitleAndYearAndTracksStats(t *testing.T) {
	config := twoSourceSearchTestConfig()
	client := NewSearchClient(config)

	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Host, "api.semanticscholar.org"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`{
  "data": [
    {
      "paperId": "sem-dup-1",
      "title": "Unified Embodied Agent",
      "authors": [{"name":"Author S"}],
      "abstract": "Semantic variant",
      "year": 2025,
      "venue": "ICRA",
      "journal": {"name":"ICRA"},
      "openAccessPdf": {"url":"https://example.com/sem-dup-1.pdf"}
    }
  ]
}`)),
					Request: r,
				}, nil
			case strings.Contains(r.URL.Host, "export.arxiv.org"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2501.10001v1</id>
    <title>Unified Embodied Agent</title>
    <summary>Arxiv duplicate same year</summary>
    <published>2025-01-01T00:00:00Z</published>
    <author><name>Author A</name></author>
  </entry>
  <entry>
    <id>http://arxiv.org/abs/2401.10002v1</id>
    <title>Unified Embodied Agent</title>
    <summary>Arxiv prior year</summary>
    <published>2024-01-01T00:00:00Z</published>
    <author><name>Author B</name></author>
  </entry>
</feed>
`)),
					Request: r,
				}, nil
			default:
				return nil, fmt.Errorf("unexpected host: %s", r.URL.Host)
			}
		}),
	}

	papers, err := client.Search("embodied", 200)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(papers) != 2 {
		t.Fatalf("expected 2 papers after title+year dedupe, got %d", len(papers))
	}

	stats := client.LastSearchStats()
	if stats.RawCount != 3 {
		t.Fatalf("expected raw count 3, got %+v", stats)
	}
	if stats.DedupCount != 2 {
		t.Fatalf("expected dedup count 2, got %+v", stats)
	}
	if stats.FinalCount != 2 {
		t.Fatalf("expected final count 2, got %+v", stats)
	}
}

func TestSearchClientHonorsSourceEnableFlagsFromConfig(t *testing.T) {
	config := twoSourceSearchTestConfig()
	config.Search.EnableSemanticScholar = false
	config.Search.EnableArxiv = true
	client := NewSearchClient(config)

	var semanticCalled bool
	var arxivCalled bool
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Host, "api.semanticscholar.org"):
				semanticCalled = true
				return nil, fmt.Errorf("semantic should be disabled by config")
			case strings.Contains(r.URL.Host, "export.arxiv.org"):
				arxivCalled = true
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(bytes.NewBufferString(`
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2503.00003v1</id>
    <title>ArXiv Only</title>
    <summary>Arxiv abstract</summary>
    <published>2025-03-01T00:00:00Z</published>
    <author><name>Author A</name></author>
  </entry>
</feed>
`)),
					Request: r,
				}, nil
			default:
				return nil, fmt.Errorf("unexpected host: %s", r.URL.Host)
			}
		}),
	}

	papers, err := client.Search("robotics", 100)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if semanticCalled {
		t.Fatal("did not expect semantic source call when disabled")
	}
	if !arxivCalled {
		t.Fatal("expected arxiv source to be called")
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper, got %d", len(papers))
	}
}

func TestBuildSearchQueryCandidatesCompactsSentencePrompt(t *testing.T) {
	candidates := buildSearchQueryCandidates("I want to sort out the papers in the field of embodied intelligence over the past two years.")
	if len(candidates) < 2 {
		t.Fatalf("expected at least two query candidates, got %v", candidates)
	}
	if candidates[1] == candidates[0] {
		t.Fatalf("expected compacted keyword candidate, got %v", candidates)
	}
}

func TestBuildSearchQueryCandidatesExtractsASCIIFromMixedLanguagePrompt(t *testing.T) {
	candidates := buildSearchQueryCandidates("我想梳理这两年关于 world model的论文.")
	found := false
	for _, candidate := range candidates {
		if candidate == "world model" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected mixed-language prompt to produce english keyword variant, got %v", candidates)
	}
}

func TestSearchClientSemanticRetriesOnEmptyResultsWithQueryVariants(t *testing.T) {
	config := twoSourceSearchTestConfig()
	config.Search.EnableArxiv = false
	config.Search.EnableSemanticScholar = true
	config.Search.RetryDurationSeconds = 3
	config.Search.RetryIntervalSeconds = 1
	client := NewSearchClient(config)

	seenQueries := make([]string, 0, 3)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if !strings.Contains(r.URL.Host, "api.semanticscholar.org") {
				return nil, fmt.Errorf("unexpected host: %s", r.URL.Host)
			}
			query := r.URL.Query().Get("query")
			seenQueries = append(seenQueries, query)

			if len(seenQueries) == 1 {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewBufferString(`{"data":[]}`)),
					Request:    r,
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(bytes.NewBufferString(`{
  "data": [
    {
      "paperId": "sem-retry-1",
      "title": "Embodied Success",
      "authors": [{"name":"Author Z"}],
      "abstract": "Recovered after query variant retry",
      "year": 2025,
      "venue": "ICRA",
      "journal": {"name":"ICRA"},
      "openAccessPdf": {"url":"https://example.com/sem-retry-1.pdf"}
    }
  ]
}`)),
				Request: r,
			}, nil
		}),
	}

	papers, err := client.Search("I want to sort out the papers in the field of embodied intelligence over the past two years.", 100)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected 1 paper after retry, got %d", len(papers))
	}
	if len(seenQueries) < 2 {
		t.Fatalf("expected at least two semantic attempts, got %d", len(seenQueries))
	}
	if seenQueries[0] == seenQueries[1] {
		t.Fatalf("expected second attempt to use query variant, got queries=%v", seenQueries)
	}
}

func TestSearchClientSemanticStopsAfterTryingAllEmptyQueryVariants(t *testing.T) {
	config := twoSourceSearchTestConfig()
	config.Search.EnableArxiv = false
	config.Search.EnableSemanticScholar = true
	config.Search.RetryDurationSeconds = 60
	config.Search.RetryIntervalSeconds = 1
	client := NewSearchClient(config)
	client.retryInterval = 0

	query := "我想梳理这两年关于 world model的论文."
	candidates := buildSearchQueryCandidates(query)
	attempts := 0

	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if !strings.Contains(r.URL.Host, "api.semanticscholar.org") {
				return nil, fmt.Errorf("unexpected host: %s", r.URL.Host)
			}
			attempts++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(`{"data":[]}`)),
				Request:    r,
			}, nil
		}),
	}

	_, err := client.Search(query, 100)
	if err == nil {
		t.Fatal("expected search to fail when all query variants return empty results")
	}
	if !strings.Contains(err.Error(), "no results after trying") {
		t.Fatalf("expected terminal no-result error, got %v", err)
	}
	if attempts != len(candidates) {
		t.Fatalf("expected %d semantic attempts (one per query variant), got %d", len(candidates), attempts)
	}
}
