package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// LLMProvider supported providers
type LLMProvider string

const (
	ProviderOpenAI    LLMProvider = "openai"
	ProviderAnthropic LLMProvider = "anthropic"
)

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxAttempts     int           `json:"max_attempts"`
	BackoffBase     time.Duration `json:"backoff_base_seconds"`
	RetryableErrors []string      `json:"retryable_errors"`
}

// LLMConfig complete configuration for an LLM client
type LLMConfig struct {
	Provider    LLMProvider `json:"provider"`
	Model       string      `json:"model"`
	APIKey      string      `json:"api_key"`
	BaseURL     string      `json:"base_url,omitempty"`
	MaxTokens   int         `json:"max_tokens"`
	Temperature float64     `json:"temperature"`
	Timeout     int         `json:"timeout_seconds"`
	Retry       RetryPolicy `json:"retry"`
}

// Validate checks configuration validity
func (c *LLMConfig) Validate() error {
	if c.Provider != ProviderOpenAI && c.Provider != ProviderAnthropic {
		return fmt.Errorf("unsupported provider: %s", c.Provider)
	}
	if c.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	if c.Model == "" {
		return fmt.Errorf("model is required")
	}
	if c.MaxTokens <= 0 {
		c.MaxTokens = 2000
	}
	if c.Temperature < 0 || c.Temperature > 2 {
		c.Temperature = 0.0
	}
	if c.Timeout <= 0 {
		c.Timeout = 30
	}
	return nil
}

// LoadFromFile loads configuration from JSON file
func LoadFromFile(path string) (*LLMConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config LLMConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Save saves configuration to file
func (c *LLMConfig) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
