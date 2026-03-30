package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/qiaoyo/DiveEnd/internal/config"
)

// Provider is an LLM provider interface
type Provider interface {
	Chat(messages []Message) (string, error)
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client is an LLM client
type Client struct {
	cfg config.LLMConfig
	httpClient *http.Client
}

// NewClient creates a new LLM client
func NewClient(cfg config.LLMConfig) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Chat sends a chat request and returns the response
func (c *Client) Chat(messages []Message) (string, error) {
	switch c.cfg.Provider {
	case "openai":
		return c.chatOpenAI(messages)
	case "anthropic":
		return c.chatAnthropic(messages)
	default:
		return "", fmt.Errorf("unknown provider: %s", c.cfg.Provider)
	}
}

// OpenAI request/response
type openAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) chatOpenAI(messages []Message) (string, error) {
	reqBody := openAIRequest{
		Model:    c.cfg.Model,
		Messages: messages,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.getBaseURL(), bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("openai error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from openai")
	}

	return result.Choices[0].Message.Content, nil
}

// Anthropic request/response
type anthropicRequest struct {
	Model      string    `json:"model"`
	Messages   []Message `json:"messages"`
	MaxTokens  int       `json:"max_tokens"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) chatAnthropic(messages []Message) (string, error) {
	reqBody := anthropicRequest{
		Model:    c.cfg.Model,
		Messages: messages,
		MaxTokens: 16000,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	baseURL := c.getBaseURL()
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1/messages"
	}

	req, err := http.NewRequest("POST", baseURL, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("anthropic error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response from anthropic")
	}

	var fullText string
	for _, content := range result.Content {
		if content.Type == "text" {
			fullText += content.Text
		}
	}

	return fullText, nil
}

func (c *Client) getBaseURL() string {
	if c.cfg.BaseURL != "" {
		return c.cfg.BaseURL
	}
	return "https://api.openai.com/v1/chat/completions"
}

// SummarizePapers generates a summary and classification of papers
func (c *Client) ClassifyPapers(query string, papers []PaperInfo) ([]PaperClassification, error) {
	var prompt string = fmt.Sprintf(`
User query: %s

Below is a list of %d papers found from search. Please help classify them into technical categories or research directions.
For each paper, assign 1-3 categories from the content, and give a relevance score from 0-10 (10 being most relevant to the query).
Also briefly explain why it's relevant.

Return the result as a JSON array with format:
[{"title": "paper title", "categories": ["category1", "category2"], "relevance": 8, "reasoning": "brief explanation"}]

Papers:
`, query, len(papers))

	for i, p := range papers {
		prompt += fmt.Sprintf("\n%d. Title: %s\nAuthors: %s\nAbstract: %s\n", i+1, p.Title, p.Authors, p.Abstract)
	}

	messages := []Message{
		{Role: "user", Content: prompt},
	}

	response, err := c.Chat(messages)
	if err != nil {
		return nil, err
	}

	// Parse JSON from response (handle possible markdown wrapping)
	var classifications []PaperClassification
	err = json.Unmarshal([]byte(extractJSON(response)), &classifications)
	if err != nil {
		// Try to extract from code block
		clean := extractCodeBlock(response)
		if clean != "" {
			err = json.Unmarshal([]byte(clean), &classifications)
		}
	}

	return classifications, err
}

// TranslatePaperSection translates a paper section and summarizes
func (c *Client) TranslateAndSummarize(section, originalText string) (translated string, summary string, err error) {
	prompt := fmt.Sprintf(`
Please translate the following academic paper section from English to Chinese, then provide a brief summary (100-200 words) of the key points.

Section: %s

Original text:
%s

Return the result in JSON format:
{
  "translation": "full Chinese translation here",
  "summary": "brief summary of key points in Chinese"
}
`, section, originalText)

	messages := []Message{{Role: "user", Content: prompt}}
	response, err := c.Chat(messages)
	if err != nil {
		return "", "", err
	}

	var result struct {
		Translation string `json:"translation"`
		Summary     string `json:"summary"`
	}

	err = json.Unmarshal([]byte(extractJSON(response)), &result)
	if err != nil {
		clean := extractCodeBlock(response)
		if clean != "" {
			err = json.Unmarshal([]byte(clean), &result)
		}
	}

	return result.Translation, result.Summary, err
}

// PaperInfo is paper information for classification
type PaperInfo struct {
	Title    string `json:"title"`
	Authors  string `json:"authors"`
	Abstract string `json:"abstract"`
}

// PaperClassification is classification result
type PaperClassification struct {
	Title       string   `json:"title"`
	Categories  []string `json:"categories"`
	Relevance   int      `json:"relevance"`
	Reasoning   string   `json:"reasoning"`
}

func extractJSON(s string) string {
	// Find first { and last }
	start := 0
	for i, c := range s {
		if c == '{' {
			start = i
			break
		}
	}
	end := len(s)
	for i := len(s)-1; i >= 0; i-- {
		if s[i] == '}' {
			end = i+1
			break
		}
	}
	return s[start:end]
}

func extractCodeBlock(s string) string {
	inBlock := false
	var result []rune
	for _, line := range splitLines(s) {
		if len(line) >= 3 && line[:3] == "```" {
			inBlock = !inBlock
			continue
		}
		if inBlock {
			result = append(result, []rune(line)...)
			result = append(result, '\n')
		}
	}
	return string(result)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i+1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
