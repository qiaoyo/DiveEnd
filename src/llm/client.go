package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"diveend/config"
)

// Client handles LLM API communication with fault tolerance
type Client struct {
	config     *config.LLMConfig
	httpClient *http.Client
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the API request body
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

// ChatResponse represents the API response
type ChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewClient creates a new LLM client
func NewClient(cfg *config.LLMConfig) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}
}

// Chat sends a chat request to the LLM API with retry logic
func (c *Client) Chat(ctx context.Context, systemPrompt string, userPrompt string) (*ChatResponse, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	var lastErr error
	backoff := c.config.Retry.BackoffBase

	for attempt := 0; attempt <= c.config.Retry.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			time.Sleep(backoff)
			backoff *= 2
		}

		resp, err := c.doRequest(ctx, messages)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable
		if !c.isRetryable(err, attempt) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("max retry attempts (%d) exceeded: %w", c.config.Retry.MaxAttempts, lastErr)
}

// doRequest performs the actual HTTP request
func (c *Client) doRequest(ctx context.Context, messages []Message) (*ChatResponse, error) {
	reqBody := ChatRequest{
		Model:       c.config.Model,
		Messages:    messages,
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	baseURL := c.config.BaseURL
	if baseURL == "" {
		if c.config.Provider == config.ProviderOpenAI {
			baseURL = "https://api.openai.com/v1"
		} else if c.config.Provider == config.ProviderAnthropic {
			baseURL = "https://api.anthropic.com/v1"
		}
	}

	var httpReq *http.Request
	var endpoint string

	if c.config.Provider == config.ProviderOpenAI {
		endpoint = baseURL + "/chat/completions"
		httpReq, err = http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	} else if c.config.Provider == config.ProviderAnthropic {
		endpoint = baseURL + "/messages"
		// Anthropic uses different request format, handle separately
		return nil, fmt.Errorf("anthropic provider not yet implemented")
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chatResp, nil
}

// isRetryable determines if an error should trigger a retry
func (c *Client) isRetryable(err error, attempt int) bool {
	if attempt >= c.config.Retry.MaxAttempts {
		return false
	}

	errStr := err.Error()
	for _, retryableErr := range c.config.Retry.RetryableErrors {
		if contains(errStr, retryableErr) {
			return true
		}
	}

	// Retry on network errors and 5xx status codes
	if contains(errStr, "timeout") ||
		contains(errStr, "connection refused") ||
		contains(errStr, "no such host") ||
		contains(errStr, "status 5") {
		return true
	}

	return false
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
