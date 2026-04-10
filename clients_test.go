package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
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

func TestSearchClientSearchRequestsAtLeast100FromSemanticAndArxiv(t *testing.T) {
	config := defaultAppConfig()
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
				if got := r.URL.Query().Get("limit"); got != "100" {
					t.Fatalf("expected semantic limit=100, got %s", got)
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
      "openAccessPdf": {"url":"https://example.com/sem-1.pdf"}
    }
  ]
}`)),
					Request: r,
				}, nil
			case strings.Contains(r.URL.Host, "export.arxiv.org"):
				arxivCalled = true
				if got := r.URL.Query().Get("max_results"); got != "100" {
					t.Fatalf("expected arxiv max_results=100, got %s", got)
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
	if len(papers) != 2 {
		t.Fatalf("expected 2 papers, got %d", len(papers))
	}
}

func TestSearchClientRetryStopsAfterPerSourceSuccessAndEmitsProgress(t *testing.T) {
	config := defaultAppConfig()
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

func TestSearchClientHonorsSourceEnableFlagsFromConfig(t *testing.T) {
	config := defaultAppConfig()
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

func TestSearchClientSemanticRetriesOnEmptyResultsWithQueryVariants(t *testing.T) {
	config := defaultAppConfig()
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
