package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
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
