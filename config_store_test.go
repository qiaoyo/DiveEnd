package main

import (
	"os"
	"path/filepath"
	"testing"
)

func useTestConfigPath(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	oldConfigPathOverride := configPathOverride
	oldUserConfigDirFunc := userConfigDirFunc
	oldUserHomeDirFunc := userHomeDirFunc

	configPathOverride = filepath.Join(tempDir, "config.json")
	userConfigDirFunc = func() (string, error) { return tempDir, nil }
	userHomeDirFunc = func() (string, error) { return tempDir, nil }
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	t.Cleanup(func() {
		configPathOverride = oldConfigPathOverride
		userConfigDirFunc = oldUserConfigDirFunc
		userHomeDirFunc = oldUserHomeDirFunc
		_ = os.Chdir(oldWD)
	})

	return tempDir
}

func TestLoadAppConfigReturnsDefaultWhenMissing(t *testing.T) {
	tempDir := useTestConfigPath(t)

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if config.LLM.ProviderType != "anthropic" {
		t.Fatalf("expected default provider type anthropic, got %q", config.LLM.ProviderType)
	}
	if config.DataPath != filepath.Join(tempDir, "DiveEndData") {
		t.Fatalf("expected default data path in temp home, got %q", config.DataPath)
	}
}

func TestSaveAndLoadAppConfigRoundTrip(t *testing.T) {
	tempDir := useTestConfigPath(t)

	config := defaultAppConfig()
	config.LLM = defaultOpenAICompatibleLLMConfig()
	config.LLM.ProviderID = "duckcoding"
	config.LLM.ProviderName = "DuckCoding"
	config.LLM.BaseURL = "https://api.duckcoding.ai/v1"
	config.LLM.APIKey = "sk-test"
	config.LLM.Model = "gpt-5.3-codex"
	config.LLM.ReasoningEffort = "xhigh"
	config.WeakLLM = defaultOpenAICompatibleLLMConfig()
	config.WeakLLM.ProviderID = "duckcoding-lite"
	config.WeakLLM.ProviderName = "DuckCoding Lite"
	config.WeakLLM.BaseURL = "https://api.duckcoding.ai/v1"
	config.WeakLLM.APIKey = "sk-weak"
	config.WeakLLM.Model = "gpt-4o-mini"
	config.Theme = "dark"
	config.DataPath = filepath.Join(tempDir, "LibraryData")
	config.Search.SemanticScholarAPIKey = "semantic-key"
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "baidu-token"

	if err := SaveAppConfig(config); err != nil {
		t.Fatalf("SaveAppConfig() error = %v", err)
	}

	loaded, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if loaded.LLM.ProviderID != config.LLM.ProviderID {
		t.Fatalf("expected provider ID %q, got %q", config.LLM.ProviderID, loaded.LLM.ProviderID)
	}
	if loaded.LLM.APIKey != config.LLM.APIKey {
		t.Fatalf("expected llm key to round-trip")
	}
	if loaded.LLM.BaseURL != config.LLM.BaseURL {
		t.Fatalf("expected base URL %q, got %q", config.LLM.BaseURL, loaded.LLM.BaseURL)
	}
	if loaded.LLM.ReasoningEffort != "xhigh" {
		t.Fatalf("expected reasoning effort to round-trip, got %q", loaded.LLM.ReasoningEffort)
	}
	if loaded.WeakLLM.APIKey != "sk-weak" {
		t.Fatalf("expected weak llm key to round-trip")
	}
	if loaded.WeakLLM.Model != "gpt-4o-mini" {
		t.Fatalf("expected weak llm model to round-trip, got %q", loaded.WeakLLM.Model)
	}
	if loaded.Search.SemanticScholarAPIKey != "semantic-key" {
		t.Fatalf("expected Semantic Scholar key to round-trip")
	}
	if loaded.BaiduCloud.Token != "baidu-token" {
		t.Fatalf("expected Baidu token to round-trip")
	}
	if loaded.DataPath != config.DataPath {
		t.Fatalf("expected data path %q, got %q", config.DataPath, loaded.DataPath)
	}
	if loaded.Theme != "dark" {
		t.Fatalf("expected dark theme, got %q", loaded.Theme)
	}

	info, err := os.Stat(configPathOverride)
	if err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected config permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestLoadAppConfigMigratesLegacyProviderFields(t *testing.T) {
	useTestConfigPath(t)

	legacyJSON := `{
  "selectedProvider": "openai",
  "openaiApiKey": "legacy-key",
  "openaiModel": "legacy-model",
  "semanticScholarApiKey": "legacy-semantic"
}`
	if err := os.WriteFile(configPathOverride, []byte(legacyJSON), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if config.LLM.ProviderType != "openai_compatible" {
		t.Fatalf("expected legacy openai to migrate to openai_compatible, got %q", config.LLM.ProviderType)
	}
	if config.LLM.WireAPI != "chat_completions" {
		t.Fatalf("expected legacy openai to keep chat completions, got %q", config.LLM.WireAPI)
	}
	if config.LLM.APIKey != "legacy-key" {
		t.Fatalf("expected legacy key to migrate")
	}
	if config.Search.SemanticScholarAPIKey != "legacy-semantic" {
		t.Fatalf("expected legacy Semantic Scholar key to migrate")
	}
}

func TestSanitizeAppConfigRemovesSecretValues(t *testing.T) {
	config := defaultAppConfig()
	config.LLM.APIKey = "secret"
	config.WeakLLM.APIKey = "weak-secret"
	config.Search.SemanticScholarAPIKey = "semantic-secret"
	config.BaiduCloud.Token = "token-secret"

	sanitized := sanitizeAppConfig(config)
	if sanitized.LLM.APIKey != "" {
		t.Fatal("expected llm api key to be redacted")
	}
	if !sanitized.LLM.HasAPIKey {
		t.Fatal("expected llm api key presence flag to remain true")
	}
	if sanitized.WeakLLM.APIKey != "" {
		t.Fatal("expected weak llm api key to be redacted")
	}
	if !sanitized.WeakLLM.HasAPIKey {
		t.Fatal("expected weak llm api key presence flag to remain true")
	}
	if sanitized.Search.SemanticScholarAPIKey != "" {
		t.Fatal("expected search api key to be redacted")
	}
	if !sanitized.Search.HasSemanticScholarAPIKey {
		t.Fatal("expected search api key presence flag to remain true")
	}
	if sanitized.BaiduCloud.Token != "" {
		t.Fatal("expected cloud token to be redacted")
	}
	if !sanitized.BaiduCloud.HasToken {
		t.Fatal("expected cloud token presence flag to remain true")
	}
}

func TestLoadAppConfigInvalidJSON(t *testing.T) {
	useTestConfigPath(t)

	if err := os.WriteFile(configPathOverride, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadAppConfig(); err == nil {
		t.Fatal("expected invalid JSON to return an error")
	}
}

func TestLoadAppConfigBootstrapsStrongWeakAndBaiduSeeds(t *testing.T) {
	useTestConfigPath(t)

	if err := os.MkdirAll("config", 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join("config", "strong_llm.json"), []byte(`{"provider":"anthropic","model":"claude-3-7-sonnet","api_key":"strong-key","base_url":"https://api.anthropic.com/v1"}`), 0600); err != nil {
		t.Fatalf("WriteFile strong seed error = %v", err)
	}
	if err := os.WriteFile(filepath.Join("config", "weak_llm.json"), []byte(`{"provider":"openai_compatible","model":"gpt-4o-mini","api_key":"weak-key","base_url":"https://api.openai.com/v1"}`), 0600); err != nil {
		t.Fatalf("WriteFile weak seed error = %v", err)
	}
	if err := os.WriteFile("baiduyun_token.json", []byte(`{"access_token":"baidu-seed-token"}`), 0600); err != nil {
		t.Fatalf("WriteFile baidu seed error = %v", err)
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if config.LLM.APIKey != "strong-key" {
		t.Fatalf("expected strong llm key to bootstrap, got %q", config.LLM.APIKey)
	}
	if config.WeakLLM.APIKey != "weak-key" {
		t.Fatalf("expected weak llm key to bootstrap, got %q", config.WeakLLM.APIKey)
	}
	if config.BaiduCloud.Token != "baidu-seed-token" {
		t.Fatalf("expected baidu token to bootstrap, got %q", config.BaiduCloud.Token)
	}
}
