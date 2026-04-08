package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type llmSeedConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
}

type baiduTokenSeed struct {
	AccessToken string `json:"access_token"`
}

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

func defaultWeakLLMConfig() LLMConfig {
	config := defaultOpenAICompatibleLLMConfig()
	config.ProviderID = "weak-llm"
	config.ProviderName = "Weak LLM"
	config.Model = "gpt-4o-mini"
	return config
}

func defaultAppConfig() AppConfig {
	homeDir, err := userHomeDirFunc()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		homeDir = "."
	}

	return AppConfig{
		LLM:             defaultAnthropicLLMConfig(),
		WeakLLM:         defaultWeakLLMConfig(),
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
	if strings.TrimSpace(config.WeakLLM.ProviderType) == "" && strings.TrimSpace(config.WeakLLM.ProviderID) == "" && strings.TrimSpace(config.WeakLLM.Model) == "" && strings.TrimSpace(config.WeakLLM.BaseURL) == "" {
		config.WeakLLM = defaults.WeakLLM
	} else {
		config.WeakLLM = normalizeLLMConfig(config.WeakLLM)
	}
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

	if incoming.WeakLLM.ClearAPIKey {
		merged.WeakLLM.APIKey = ""
	} else if strings.TrimSpace(incoming.WeakLLM.APIKey) == "" {
		merged.WeakLLM.APIKey = existing.WeakLLM.APIKey
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
	safe.WeakLLM.APIKey = ""
	safe.Search.SemanticScholarAPIKey = ""
	safe.BaiduCloud.Token = ""
	return safe
}

func LoadAppConfig() (AppConfig, error) {
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return loadBootstrapConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return AppConfig{}, err
	}

	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return AppConfig{}, err
	}

	return mergeSeedSecrets(normalizeAppConfig(config)), nil
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

func loadBootstrapConfig() AppConfig {
	config := defaultAppConfig()

	if seed, ok := readStrongLLMSeed(); ok {
		config.LLM = llmConfigFromSeed(seed)
	}

	if seed, ok := readWeakLLMSeed(); ok {
		config.WeakLLM = llmConfigFromSeed(seed)
	}

	if token, ok := readBaiduTokenSeed(); ok {
		config.BaiduCloud.Enabled = true
		config.BaiduCloud.Token = token
	}

	return normalizeAppConfig(config)
}

func mergeSeedSecrets(config AppConfig) AppConfig {
	if seed, ok := readStrongLLMSeed(); ok {
		config.LLM = mergeLLMSeedConfig(config.LLM, seed)
	}

	if seed, ok := readWeakLLMSeed(); ok {
		config.WeakLLM = mergeLLMSeedConfig(config.WeakLLM, seed)
	}

	if token, ok := readBaiduTokenSeed(); ok {
		if strings.TrimSpace(config.BaiduCloud.Token) == "" {
			config.BaiduCloud.Token = token
		}
		if !config.BaiduCloud.Enabled {
			config.BaiduCloud.Enabled = true
		}
	}

	return normalizeAppConfig(config)
}

func readStrongLLMSeed() (llmSeedConfig, bool) {
	return readLLMSeed(filepath.Join("config", "strong_llm.json"))
}

func readWeakLLMSeed() (llmSeedConfig, bool) {
	return readLLMSeed(filepath.Join("config", "weak_llm.json"))
}

func readLLMSeed(path string) (llmSeedConfig, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return llmSeedConfig{}, false
	}

	var seed llmSeedConfig
	if err := json.Unmarshal(data, &seed); err != nil {
		return llmSeedConfig{}, false
	}

	if strings.TrimSpace(seed.BaseURL) == "" && strings.TrimSpace(seed.APIKey) == "" && strings.TrimSpace(seed.Model) == "" {
		return llmSeedConfig{}, false
	}

	return seed, true
}

func readBaiduTokenSeed() (string, bool) {
	data, err := os.ReadFile("baiduyun_token.json")
	if err != nil {
		return "", false
	}

	var seed baiduTokenSeed
	if err := json.Unmarshal(data, &seed); err != nil {
		return "", false
	}

	token := strings.TrimSpace(seed.AccessToken)
	return token, token != ""
}

func llmConfigFromSeed(seed llmSeedConfig) LLMConfig {
	provider := strings.TrimSpace(strings.ToLower(seed.Provider))
	baseURL := strings.TrimSpace(seed.BaseURL)
	model := strings.TrimSpace(seed.Model)
	apiKey := strings.TrimSpace(seed.APIKey)

	if provider == "anthropic" {
		config := defaultAnthropicLLMConfig()
		if baseURL != "" {
			config.BaseURL = baseURL
		}
		if model != "" {
			config.Model = model
		}
		config.APIKey = apiKey
		return normalizeLLMConfig(config)
	}

	config := defaultOpenAICompatibleLLMConfig()
	if strings.Contains(baseURL, "duckcoding.ai") {
		config.ProviderID = "duckcoding"
		config.ProviderName = "DuckCoding"
	} else if strings.Contains(baseURL, "ark.cn-beijing.volces.com") {
		config.ProviderID = "volcengine-coding"
		config.ProviderName = "Volcengine Coding"
	}
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	if model != "" {
		config.Model = model
	}
	config.APIKey = apiKey
	return normalizeLLMConfig(config)
}

func mergeLLMSeedConfig(config LLMConfig, seed llmSeedConfig) LLMConfig {
	seedConfig := llmConfigFromSeed(seed)

	if strings.TrimSpace(config.ProviderType) == "" {
		config.ProviderType = seedConfig.ProviderType
	}
	if strings.TrimSpace(config.ProviderID) == "" {
		config.ProviderID = seedConfig.ProviderID
	}
	if strings.TrimSpace(config.ProviderName) == "" {
		config.ProviderName = seedConfig.ProviderName
	}
	if strings.TrimSpace(config.BaseURL) == "" {
		config.BaseURL = seedConfig.BaseURL
	}
	if strings.TrimSpace(config.WireAPI) == "" {
		config.WireAPI = seedConfig.WireAPI
	}
	if strings.TrimSpace(config.Model) == "" {
		config.Model = seedConfig.Model
	}
	if strings.TrimSpace(config.APIKey) == "" {
		config.APIKey = seedConfig.APIKey
	}
	if strings.TrimSpace(config.ReasoningEffort) == "" {
		config.ReasoningEffort = seedConfig.ReasoningEffort
	}
	if !config.RequiresOpenAIAuth {
		config.RequiresOpenAIAuth = seedConfig.RequiresOpenAIAuth
	}
	if !config.DisableResponseStorage {
		config.DisableResponseStorage = seedConfig.DisableResponseStorage
	}

	return normalizeLLMConfig(config)
}
