package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

var configPathOverride string
var userConfigDirFunc = os.UserConfigDir
var userHomeDirFunc = os.UserHomeDir

func defaultAnthropicLLMConfig() LLMConfig {
	return LLMConfig{
		ProviderID:             "anthropic",
		ProviderName:           "Anthropic",
		ProviderType:           "anthropic",
		BaseURL:                "https://api.anthropic.com/v1",
		WireAPI:                "anthropic_messages",
		Model:                  "claude-3-5-sonnet-20241022",
		DisableResponseStorage: true,
	}
}

func defaultOpenAICompatibleLLMConfig() LLMConfig {
	return LLMConfig{
		ProviderID:             "openai",
		ProviderName:           "OpenAI",
		ProviderType:           "openai_compatible",
		BaseURL:                "https://api.openai.com/v1",
		WireAPI:                "responses",
		RequiresOpenAIAuth:     true,
		Model:                  "gpt-4o-mini",
		DisableResponseStorage: true,
	}
}

func defaultAppConfig() AppConfig {
	homeDir, err := userHomeDirFunc()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		homeDir = "."
	}

	return AppConfig{
		LLM:             defaultAnthropicLLMConfig(),
		Search:          SearchAPIConfig{},
		Theme:           "light",
		LeftPanelWidth:  280,
		RightPanelWidth: 340,
		DataPath:        filepath.Join(homeDir, "DiveEndData"),
		BaiduCloud: BaiduCloudConfig{
			Enabled: false,
			Quota:   0,
		},
	}
}

func normalizeAppConfig(config AppConfig) AppConfig {
	defaults := defaultAppConfig()
	config = migrateLegacyConfig(config)
	config.LLM = normalizeLLMConfig(config.LLM)
	config.Search = normalizeSearchAPIConfig(config.Search)
	config.BaiduCloud = normalizeBaiduCloudConfig(config.BaiduCloud)
	if config.Theme != "dark" && config.Theme != "light" {
		config.Theme = defaults.Theme
	}
	if config.LeftPanelWidth < 220 {
		config.LeftPanelWidth = defaults.LeftPanelWidth
	}
	if config.RightPanelWidth < 280 {
		config.RightPanelWidth = defaults.RightPanelWidth
	}
	if strings.TrimSpace(config.DataPath) == "" {
		config.DataPath = defaults.DataPath
	}

	config.SelectedProvider = ""
	config.OpenAIAPIKey = ""
	config.OpenAIModel = ""
	config.AnthropicAPIKey = ""
	config.AnthropicModel = ""
	config.SemanticScholarAPIKey = ""

	return config
}

func migrateLegacyConfig(config AppConfig) AppConfig {
	if config.LLM.ProviderType == "" {
		switch strings.TrimSpace(config.SelectedProvider) {
		case "openai":
			config.LLM = defaultOpenAICompatibleLLMConfig()
			config.LLM.WireAPI = "chat_completions"
			if strings.TrimSpace(config.OpenAIAPIKey) != "" {
				config.LLM.APIKey = config.OpenAIAPIKey
			}
			if strings.TrimSpace(config.OpenAIModel) != "" {
				config.LLM.Model = config.OpenAIModel
			}
		case "anthropic", "":
			config.LLM = defaultAnthropicLLMConfig()
			if strings.TrimSpace(config.AnthropicAPIKey) != "" {
				config.LLM.APIKey = config.AnthropicAPIKey
			}
			if strings.TrimSpace(config.AnthropicModel) != "" {
				config.LLM.Model = config.AnthropicModel
			}
		}
	}

	if strings.TrimSpace(config.Search.SemanticScholarAPIKey) == "" && strings.TrimSpace(config.SemanticScholarAPIKey) != "" {
		config.Search.SemanticScholarAPIKey = config.SemanticScholarAPIKey
	}

	return config
}

func normalizeLLMConfig(config LLMConfig) LLMConfig {
	config.ProviderID = strings.TrimSpace(config.ProviderID)
	config.ProviderName = strings.TrimSpace(config.ProviderName)
	config.ProviderType = strings.TrimSpace(config.ProviderType)
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.WireAPI = strings.TrimSpace(config.WireAPI)
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.Model = strings.TrimSpace(config.Model)
	config.ReasoningEffort = strings.TrimSpace(config.ReasoningEffort)

	if config.ProviderType != "openai_compatible" && config.ProviderType != "anthropic" {
		config.ProviderType = defaultAnthropicLLMConfig().ProviderType
	}

	var defaults LLMConfig
	switch config.ProviderType {
	case "openai_compatible":
		defaults = defaultOpenAICompatibleLLMConfig()
		if config.WireAPI != "responses" && config.WireAPI != "chat_completions" {
			config.WireAPI = defaults.WireAPI
		}
	case "anthropic":
		defaults = defaultAnthropicLLMConfig()
		config.WireAPI = defaults.WireAPI
		config.RequiresOpenAIAuth = false
	}

	if config.ProviderID == "" {
		config.ProviderID = defaults.ProviderID
	}
	if config.ProviderName == "" {
		config.ProviderName = defaults.ProviderName
	}
	if config.BaseURL == "" {
		config.BaseURL = defaults.BaseURL
	}
	if config.Model == "" {
		config.Model = defaults.Model
	}

	config.HasAPIKey = config.APIKey != ""
	config.ClearAPIKey = false
	return config
}

func normalizeSearchAPIConfig(config SearchAPIConfig) SearchAPIConfig {
	config.SemanticScholarAPIKey = strings.TrimSpace(config.SemanticScholarAPIKey)
	config.HasSemanticScholarAPIKey = config.SemanticScholarAPIKey != ""
	config.ClearSemanticScholarAPIKey = false
	return config
}

func normalizeBaiduCloudConfig(config BaiduCloudConfig) BaiduCloudConfig {
	config.Token = strings.TrimSpace(config.Token)
	config.HasToken = config.Token != ""
	config.ClearToken = false
	return config
}

func mergeAppConfigSecrets(existing AppConfig, incoming AppConfig) AppConfig {
	merged := incoming

	if incoming.LLM.ClearAPIKey {
		merged.LLM.APIKey = ""
	} else if strings.TrimSpace(incoming.LLM.APIKey) == "" {
		merged.LLM.APIKey = existing.LLM.APIKey
	}

	if incoming.Search.ClearSemanticScholarAPIKey {
		merged.Search.SemanticScholarAPIKey = ""
	} else if strings.TrimSpace(incoming.Search.SemanticScholarAPIKey) == "" {
		merged.Search.SemanticScholarAPIKey = existing.Search.SemanticScholarAPIKey
	}

	if incoming.BaiduCloud.ClearToken {
		merged.BaiduCloud.Token = ""
	} else if strings.TrimSpace(incoming.BaiduCloud.Token) == "" {
		merged.BaiduCloud.Token = existing.BaiduCloud.Token
	}

	return merged
}

func sanitizeAppConfig(config AppConfig) AppConfig {
	safe := normalizeAppConfig(config)
	safe.LLM.APIKey = ""
	safe.Search.SemanticScholarAPIKey = ""
	safe.BaiduCloud.Token = ""
	return safe
}

func LoadAppConfig() (AppConfig, error) {
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return defaultAppConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return AppConfig{}, err
	}

	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return AppConfig{}, err
	}

	return normalizeAppConfig(config), nil
}

func SaveAppConfig(config AppConfig) error {
	configPath := getConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(normalizeAppConfig(config), "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

func getConfigPath() string {
	if configPathOverride != "" {
		return configPathOverride
	}

	configDir, err := userConfigDirFunc()
	if err != nil || strings.TrimSpace(configDir) == "" {
		return filepath.Join(".", "config.json")
	}

	return filepath.Join(configDir, "DiveEnd", "config.json")
}
