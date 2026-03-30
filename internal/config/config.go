package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds all application configuration
type Config struct {
	Port        int         `json:"port"`
	DataPath    string      `json:"data_path"`
	LLM         LLMConfig   `json:"llm"`
	BaiduCloud  BaiduConfig `json:"baidu_cloud"`
}

// LLMConfig holds LLM API configuration
type LLMConfig struct {
	Provider string `json:"provider"` // "openai" or "anthropic"
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
}

// BaiduConfig holds Baidu Cloud sync configuration
type BaiduConfig struct {
	Enabled   bool   `json:"enabled"`
	AK        string `json:"ak"` // Access Key
	SK        string `json:"sk"` // Secret Key
	Bucket    string `json:"bucket"`
	RemoteDir string `json:"remote_dir"`
}

// Default returns default configuration
func Default() *Config {
	homeDir, _ := os.UserHomeDir()
	defaultDataPath := filepath.Join(homeDir, "DiveEndData")

	return &Config{
		Port:     8080,
		DataPath: defaultDataPath,
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o",
		},
		BaiduCloud: BaiduConfig{
			Enabled:   false,
			RemoteDir: "DiveEndData",
		},
	}
}

// Load loads configuration from file
func Load() (*Config, error) {
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Default(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save saves configuration to file
func Save(cfg *Config) error {
	configPath := getConfigPath()
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

func getConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	return filepath.Join(configDir, "DiveEnd", "config.json")
}
