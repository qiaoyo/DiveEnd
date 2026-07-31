package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	appConfigFileLimitBytes  int64 = 1024 * 1024
	seedConfigFileLimitBytes int64 = 256 * 1024
)

type llmSeedConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
}

type baiduTokenSeed struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type semanticScholarSeed struct {
	APIKey string `json:"api_key"`
}

type appYAMLSearchConfig struct {
	Search struct {
		EnableSemanticScholar  *bool  `yaml:"enable_semantic_scholar"`
		EnableArxiv            *bool  `yaml:"enable_arxiv"`
		EnableOpenAlex         *bool  `yaml:"enable_openalex"`
		EnableOpenReview       *bool  `yaml:"enable_openreview"`
		EnableDBLP             *bool  `yaml:"enable_dblp"`
		SemanticScholarKeyPath string `yaml:"semantic_scholar_key_path"`
		OpenAlexKeyPath        string `yaml:"openalex_key_path"`
		PerSourceResultLimit   int    `yaml:"per_source_result_limit"`
		DeepStartResultLimit   int    `yaml:"deepstart_result_limit"`
		RetryDurationSeconds   int    `yaml:"retry_duration_seconds"`
		RetryIntervalSeconds   int    `yaml:"retry_interval_seconds"`
		RequestTimeoutSeconds  int    `yaml:"request_timeout_seconds"`
	} `yaml:"search"`
}

type appYAMLSearchSettings struct {
	EnableSemanticScholar  *bool
	EnableArxiv            *bool
	EnableOpenAlex         *bool
	EnableOpenReview       *bool
	EnableDBLP             *bool
	SemanticScholarKeyPath string
	OpenAlexKeyPath        string
	PerSourceResultLimit   int
	DeepStartResultLimit   int
	RetryDurationSeconds   int
	RetryIntervalSeconds   int
	RequestTimeoutSeconds  int
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

func defaultSearchAPIConfig() SearchAPIConfig {
	return SearchAPIConfig{
		EnableSemanticScholar:  true,
		EnableArxiv:            true,
		EnableOpenAlex:         true,
		EnableOpenReview:       true,
		EnableDBLP:             true,
		SemanticScholarKeyPath: filepath.Join("config", "semantic_scholar.json"),
		OpenAlexKeyPath:        filepath.Join("config", "openalex.json"),
		PerSourceResultLimit:   20,
		DeepStartResultLimit:   20,
		RetryDurationSeconds:   60,
		RetryIntervalSeconds:   1,
		RequestTimeoutSeconds:  5,
	}
}

func defaultAppConfig() AppConfig {
	homeDir, err := userHomeDirFunc()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		homeDir = "."
	}

	return AppConfig{
		LLM:                 defaultAnthropicLLMConfig(),
		WeakLLM:             defaultWeakLLMConfig(),
		DailyLLMTokenBudget: defaultDailyLLMTokenBudget,
		Search:              defaultSearchAPIConfig(),
		Theme:               "light",
		LeftPanelWidth:      280,
		RightPanelWidth:     340,
		DataPath:            filepath.Join(homeDir, "DiveEndData"),
		BaiduCloud: BaiduCloudConfig{
			Enabled: false,
			Quota:   0,
		},
		Sync: defaultSyncSettings(),
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
	if config.DailyLLMTokenBudget <= 0 {
		config.DailyLLMTokenBudget = defaults.DailyLLMTokenBudget
	}
	config.Search = normalizeSearchAPIConfig(config.Search)
	config.BaiduCloud = normalizeBaiduCloudConfig(config.BaiduCloud)
	config.Sync = normalizeSyncSettings(config.Sync)
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
	config.DataPath = normalizeDataPath(config.DataPath, defaults.DataPath)

	config.SelectedProvider = ""
	config.OpenAIAPIKey = ""
	config.OpenAIModel = ""
	config.AnthropicAPIKey = ""
	config.AnthropicModel = ""
	config.SemanticScholarAPIKey = ""

	return config
}

func defaultSyncSettings() SyncSettings {
	return SyncSettings{
		AutoSync:           false,
		SyncOnStartup:      false,
		SyncBeforeExit:     false,
		SyncInterval:       30,
		ConflictResolution: "timestamp",
	}
}

func normalizeSyncSettings(settings SyncSettings) SyncSettings {
	defaults := defaultSyncSettings()
	if settings.SyncInterval <= 0 {
		settings.SyncInterval = defaults.SyncInterval
	}
	switch strings.TrimSpace(settings.ConflictResolution) {
	case "timestamp", "local", "remote", "manual":
	default:
		settings.ConflictResolution = defaults.ConflictResolution
	}
	return settings
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
	defaults := defaultSearchAPIConfig()
	if !config.EnableSemanticScholar && !config.EnableArxiv && !config.EnableOpenAlex &&
		!config.EnableOpenReview && !config.EnableDBLP &&
		strings.TrimSpace(config.SemanticScholarKeyPath) == "" &&
		strings.TrimSpace(config.OpenAlexKeyPath) == "" &&
		config.PerSourceResultLimit == 0 &&
		config.RetryDurationSeconds == 0 &&
		config.RetryIntervalSeconds == 0 {
		config.EnableSemanticScholar = defaults.EnableSemanticScholar
		config.EnableArxiv = defaults.EnableArxiv
		config.EnableOpenAlex = defaults.EnableOpenAlex
		config.EnableOpenReview = defaults.EnableOpenReview
		config.EnableDBLP = defaults.EnableDBLP
	}

	config.SemanticScholarKeyPath = strings.TrimSpace(config.SemanticScholarKeyPath)
	if config.SemanticScholarKeyPath == "" {
		config.SemanticScholarKeyPath = defaults.SemanticScholarKeyPath
	}
	config.OpenAlexKeyPath = strings.TrimSpace(config.OpenAlexKeyPath)
	if config.OpenAlexKeyPath == "" {
		config.OpenAlexKeyPath = defaults.OpenAlexKeyPath
	}
	if config.PerSourceResultLimit <= 0 {
		config.PerSourceResultLimit = defaults.PerSourceResultLimit
	}
	if config.DeepStartResultLimit <= 0 {
		config.DeepStartResultLimit = defaults.DeepStartResultLimit
	}
	if config.RetryIntervalSeconds <= 0 {
		config.RetryIntervalSeconds = defaults.RetryIntervalSeconds
	}
	if config.RetryDurationSeconds <= 0 {
		config.RetryDurationSeconds = defaults.RetryDurationSeconds
	}
	if config.RetryDurationSeconds < config.RetryIntervalSeconds {
		config.RetryDurationSeconds = defaults.RetryDurationSeconds
	}
	if config.RequestTimeoutSeconds <= 0 {
		config.RequestTimeoutSeconds = defaults.RequestTimeoutSeconds
	}

	config.SemanticScholarAPIKey = strings.TrimSpace(config.SemanticScholarAPIKey)
	config.HasSemanticScholarAPIKey = config.SemanticScholarAPIKey != ""
	config.ClearSemanticScholarAPIKey = false
	config.OpenAlexAPIKey = strings.TrimSpace(config.OpenAlexAPIKey)
	config.HasOpenAlexAPIKey = config.OpenAlexAPIKey != ""
	config.ClearOpenAlexAPIKey = false
	return config
}

func normalizeBaiduCloudConfig(config BaiduCloudConfig) BaiduCloudConfig {
	config.Token = strings.TrimSpace(config.Token)
	config.RefreshToken = strings.TrimSpace(config.RefreshToken)
	config.ClientID = strings.TrimSpace(config.ClientID)
	config.ClientSecret = strings.TrimSpace(config.ClientSecret)
	config.HasToken = config.Token != ""
	config.ClearToken = false
	return config
}

func normalizeDataPath(dataPath, fallback string) string {
	dataPath = strings.TrimSpace(dataPath)
	if dataPath == "" {
		dataPath = fallback
	}
	if filepath.IsAbs(dataPath) {
		return filepath.Clean(dataPath)
	}
	homeDir, err := userHomeDirFunc()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return filepath.Clean(dataPath)
	}
	return filepath.Join(homeDir, dataPath)
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
	if incoming.Search.ClearOpenAlexAPIKey {
		merged.Search.OpenAlexAPIKey = ""
	} else if strings.TrimSpace(incoming.Search.OpenAlexAPIKey) == "" {
		merged.Search.OpenAlexAPIKey = existing.Search.OpenAlexAPIKey
	}

	if incoming.BaiduCloud.ClearToken {
		merged.BaiduCloud.Token = ""
		merged.BaiduCloud.RefreshToken = ""
		merged.BaiduCloud.ClientID = ""
		merged.BaiduCloud.ClientSecret = ""
	} else {
		if strings.TrimSpace(incoming.BaiduCloud.Token) == "" {
			merged.BaiduCloud.Token = existing.BaiduCloud.Token
		}
		if strings.TrimSpace(incoming.BaiduCloud.RefreshToken) == "" {
			merged.BaiduCloud.RefreshToken = existing.BaiduCloud.RefreshToken
		}
		if strings.TrimSpace(incoming.BaiduCloud.ClientID) == "" {
			merged.BaiduCloud.ClientID = existing.BaiduCloud.ClientID
		}
		if strings.TrimSpace(incoming.BaiduCloud.ClientSecret) == "" {
			merged.BaiduCloud.ClientSecret = existing.BaiduCloud.ClientSecret
		}
	}

	return merged
}

func sanitizeAppConfig(config AppConfig) AppConfig {
	safe := normalizeAppConfig(config)
	safe.LLM.APIKey = ""
	safe.WeakLLM.APIKey = ""
	safe.Search.SemanticScholarAPIKey = ""
	safe.Search.OpenAlexAPIKey = ""
	safe.BaiduCloud.Token = ""
	safe.BaiduCloud.RefreshToken = ""
	safe.BaiduCloud.ClientID = ""
	safe.BaiduCloud.ClientSecret = ""
	return safe
}

func LoadAppConfig() (AppConfig, error) {
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return loadBootstrapConfig(), nil
	}

	data, err := readLimitedFile(configPath, appConfigFileLimitBytes)
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
	data, err := json.MarshalIndent(normalizeAppConfig(config), "", "  ")
	if err != nil {
		return err
	}

	return writeFileAtomic(configPath, data, 0600)
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
	config.Search = mergeSearchConfigWithAppYAML(config.Search)

	if seed, ok := readStrongLLMSeed(); ok {
		config.LLM = llmConfigFromSeed(seed)
	}

	if seed, ok := readWeakLLMSeed(); ok {
		config.WeakLLM = llmConfigFromSeed(seed)
	}

	if token, ok := readBaiduTokenSeed(); ok {
		config.BaiduCloud.Enabled = true
		config.BaiduCloud.Token = token.AccessToken
		config.BaiduCloud.RefreshToken = token.RefreshToken
		config.BaiduCloud.ClientID = token.ClientID
		config.BaiduCloud.ClientSecret = token.ClientSecret
	}

	if config.Search.EnableSemanticScholar {
		if key, ok := readSemanticScholarSeed(config.Search.SemanticScholarKeyPath); ok {
			config.Search.SemanticScholarAPIKey = key
		}
	}
	if config.Search.EnableOpenAlex {
		if key, ok := readSearchAPIKeySeed(config.Search.OpenAlexKeyPath); ok {
			config.Search.OpenAlexAPIKey = key
		}
	}

	return normalizeAppConfig(config)
}

func mergeSeedSecrets(config AppConfig) AppConfig {
	config.Search = mergeSearchConfigWithAppYAML(config.Search)

	if seed, ok := readStrongLLMSeed(); ok {
		config.LLM = mergeLLMSeedConfig(config.LLM, seed)
	}

	if seed, ok := readWeakLLMSeed(); ok {
		config.WeakLLM = mergeLLMSeedConfig(config.WeakLLM, seed)
	}

	if token, ok := readBaiduTokenSeed(); ok {
		config.BaiduCloud.Token = token.AccessToken
		config.BaiduCloud.RefreshToken = token.RefreshToken
		config.BaiduCloud.ClientID = token.ClientID
		config.BaiduCloud.ClientSecret = token.ClientSecret
		config.BaiduCloud.Enabled = true
	}

	if config.Search.EnableSemanticScholar && strings.TrimSpace(config.Search.SemanticScholarAPIKey) == "" {
		if key, ok := readSemanticScholarSeed(config.Search.SemanticScholarKeyPath); ok {
			config.Search.SemanticScholarAPIKey = key
		}
	}
	if config.Search.EnableOpenAlex && strings.TrimSpace(config.Search.OpenAlexAPIKey) == "" {
		if key, ok := readSearchAPIKeySeed(config.Search.OpenAlexKeyPath); ok {
			config.Search.OpenAlexAPIKey = key
		}
	}

	return normalizeAppConfig(config)
}

func mergeSearchConfigWithAppYAML(searchConfig SearchAPIConfig) SearchAPIConfig {
	yamlConfig, ok := readAppYAMLSearchConfig()
	if !ok {
		return normalizeSearchAPIConfig(searchConfig)
	}
	if yamlConfig.EnableSemanticScholar != nil {
		searchConfig.EnableSemanticScholar = *yamlConfig.EnableSemanticScholar
	}
	if yamlConfig.EnableArxiv != nil {
		searchConfig.EnableArxiv = *yamlConfig.EnableArxiv
	}
	if yamlConfig.EnableOpenAlex != nil {
		searchConfig.EnableOpenAlex = *yamlConfig.EnableOpenAlex
	}
	if yamlConfig.EnableOpenReview != nil {
		searchConfig.EnableOpenReview = *yamlConfig.EnableOpenReview
	}
	if yamlConfig.EnableDBLP != nil {
		searchConfig.EnableDBLP = *yamlConfig.EnableDBLP
	}
	if strings.TrimSpace(yamlConfig.SemanticScholarKeyPath) != "" {
		searchConfig.SemanticScholarKeyPath = strings.TrimSpace(yamlConfig.SemanticScholarKeyPath)
	}
	if strings.TrimSpace(yamlConfig.OpenAlexKeyPath) != "" {
		searchConfig.OpenAlexKeyPath = strings.TrimSpace(yamlConfig.OpenAlexKeyPath)
	}
	if yamlConfig.EnableSemanticScholar != nil && !*yamlConfig.EnableSemanticScholar {
		searchConfig.SemanticScholarAPIKey = ""
	}
	if yamlConfig.PerSourceResultLimit > 0 {
		searchConfig.PerSourceResultLimit = yamlConfig.PerSourceResultLimit
	}
	if yamlConfig.DeepStartResultLimit > 0 {
		searchConfig.DeepStartResultLimit = yamlConfig.DeepStartResultLimit
	}
	if yamlConfig.RetryDurationSeconds > 0 {
		searchConfig.RetryDurationSeconds = yamlConfig.RetryDurationSeconds
	}
	if yamlConfig.RetryIntervalSeconds > 0 {
		searchConfig.RetryIntervalSeconds = yamlConfig.RetryIntervalSeconds
	}
	if yamlConfig.RequestTimeoutSeconds > 0 {
		searchConfig.RequestTimeoutSeconds = yamlConfig.RequestTimeoutSeconds
	}
	return normalizeSearchAPIConfig(searchConfig)
}

func readAppYAMLSearchConfig() (appYAMLSearchSettings, bool) {
	data, err := readLimitedFile(filepath.Join("config", "app.yaml"), seedConfigFileLimitBytes)
	if err != nil {
		return appYAMLSearchSettings{}, false
	}

	var raw appYAMLSearchConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return appYAMLSearchSettings{}, false
	}

	return appYAMLSearchSettings{
		EnableSemanticScholar:  raw.Search.EnableSemanticScholar,
		EnableArxiv:            raw.Search.EnableArxiv,
		EnableOpenAlex:         raw.Search.EnableOpenAlex,
		EnableOpenReview:       raw.Search.EnableOpenReview,
		EnableDBLP:             raw.Search.EnableDBLP,
		SemanticScholarKeyPath: strings.TrimSpace(raw.Search.SemanticScholarKeyPath),
		OpenAlexKeyPath:        strings.TrimSpace(raw.Search.OpenAlexKeyPath),
		PerSourceResultLimit:   raw.Search.PerSourceResultLimit,
		DeepStartResultLimit:   raw.Search.DeepStartResultLimit,
		RetryDurationSeconds:   raw.Search.RetryDurationSeconds,
		RetryIntervalSeconds:   raw.Search.RetryIntervalSeconds,
		RequestTimeoutSeconds:  raw.Search.RequestTimeoutSeconds,
	}, true
}

func readStrongLLMSeed() (llmSeedConfig, bool) {
	return readLLMSeed(filepath.Join("config", "strong_llm.json"))
}

func readWeakLLMSeed() (llmSeedConfig, bool) {
	return readLLMSeed(filepath.Join("config", "weak_llm.json"))
}

func readLLMSeed(path string) (llmSeedConfig, bool) {
	data, err := readLimitedFile(path, seedConfigFileLimitBytes)
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

func readBaiduTokenSeed() (BaiduToken, bool) {
	data, err := readLimitedFile(defaultBaiduTokenPath(), seedConfigFileLimitBytes)
	if err != nil {
		return BaiduToken{}, false
	}

	var seed baiduTokenSeed
	if err := json.Unmarshal(data, &seed); err != nil {
		return BaiduToken{}, false
	}

	token := BaiduToken{
		AccessToken:  strings.TrimSpace(seed.AccessToken),
		RefreshToken: strings.TrimSpace(seed.RefreshToken),
		ClientID:     strings.TrimSpace(seed.ClientID),
		ClientSecret: strings.TrimSpace(seed.ClientSecret),
	}
	return token, token.AccessToken != ""
}

func defaultBaiduTokenPath() string {
	return "baiduyun_token.json"
}

func readSemanticScholarSeed(path string) (string, bool) {
	return readSearchAPIKeySeed(path)
}

func readSearchAPIKeySeed(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}

	data, err := readLimitedFile(path, seedConfigFileLimitBytes)
	if err != nil {
		return "", false
	}

	var seed semanticScholarSeed
	if err := json.Unmarshal(data, &seed); err != nil {
		return "", false
	}

	key := strings.TrimSpace(seed.APIKey)
	return key, key != ""
}

func readLimitedFile(path string, limit int64) ([]byte, error) {
	return readLimitedFileWithOpen(path, limit, os.Open)
}

func readLimitedFileWithOpen(path string, limit int64, openFile func(string) (*os.File, error)) ([]byte, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("file byte limit must be positive")
	}
	if openFile == nil {
		return nil, fmt.Errorf("file open function cannot be nil")
	}

	linkInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s is a symbolic link", path)
	}
	if !linkInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if linkInfo.Size() > limit {
		return nil, fmt.Errorf("%s exceeds %d byte limit", path, limit)
	}

	file, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if !os.SameFile(linkInfo, info) {
		return nil, fmt.Errorf("%s changed while opening", path)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s exceeds %d byte limit", path, limit)
	}

	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d byte limit", path, limit)
	}
	return data, nil
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
