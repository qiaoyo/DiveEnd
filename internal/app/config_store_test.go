package app

import (
	"os"
	"path/filepath"
	"strings"
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
	if !config.Search.EnableSemanticScholar || !config.Search.EnableArxiv ||
		!config.Search.EnableOpenAlex || !config.Search.EnableOpenReview || !config.Search.EnableDBLP {
		t.Fatalf("unexpected default academic source set: %+v", config.Search)
	}
}

func TestResolveConfigAssetPathWalksParentDirectories(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "internal", "app")
	if err := os.MkdirAll(filepath.Join(child), 0755); err != nil {
		t.Fatalf("MkdirAll child error = %v", err)
	}
	seedPath := filepath.Join(root, "config", "strong_llm.json")
	if err := os.MkdirAll(filepath.Dir(seedPath), 0755); err != nil {
		t.Fatalf("MkdirAll config error = %v", err)
	}
	if err := os.WriteFile(seedPath, []byte(`{"model":"test"}`), 0600); err != nil {
		t.Fatalf("WriteFile seed error = %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error = %v", err)
	}
	if err := os.Chdir(child); err != nil {
		t.Fatalf("Chdir child error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	got := resolveConfigAssetPath(filepath.Join("config", "strong_llm.json"))
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks resolved asset error = %v", err)
	}
	wantResolved, err := filepath.EvalSymlinks(seedPath)
	if err != nil {
		t.Fatalf("EvalSymlinks seed asset error = %v", err)
	}
	if gotResolved != wantResolved {
		t.Fatalf("expected parent config asset %q, got %q", wantResolved, gotResolved)
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
	config.Search.OpenAlexAPIKey = "openalex-key"
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "baidu-token"
	config.GoogleDrive.Enabled = true
	config.GoogleDrive.ClientSecretPath = "/tmp/diveend-google-client.json"
	config.GoogleDrive.RootFolderName = "Research Backup"
	config.GoogleDrive.FallbackToBaidu = false

	if err := os.MkdirAll(filepath.Dir(configPathOverride), 0700); err != nil {
		t.Fatalf("MkdirAll config dir error = %v", err)
	}
	if err := os.WriteFile(configPathOverride, []byte(`{"theme":"light"}`), 0644); err != nil {
		t.Fatalf("WriteFile existing config error = %v", err)
	}

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
	if loaded.Search.OpenAlexAPIKey != "openalex-key" {
		t.Fatalf("expected OpenAlex key to round-trip")
	}
	if loaded.BaiduCloud.Token != "baidu-token" {
		t.Fatalf("expected Baidu token to round-trip")
	}
	if !loaded.GoogleDrive.Enabled || loaded.GoogleDrive.ClientSecretPath != config.GoogleDrive.ClientSecretPath || loaded.GoogleDrive.RootFolderName != config.GoogleDrive.RootFolderName || loaded.GoogleDrive.FallbackToBaidu {
		t.Fatalf("expected Google Drive settings to round-trip, got %+v", loaded.GoogleDrive)
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
	tempMatches, err := filepath.Glob(filepath.Join(filepath.Dir(configPathOverride), ".config.json.tmp-*"))
	if err != nil {
		t.Fatalf("Glob temp config files error = %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected no leftover temp config files, got %v", tempMatches)
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

func TestLLMConfigFromVolcengineSeedUsesChatCompletions(t *testing.T) {
	config := llmConfigFromSeed(llmSeedConfig{
		Provider: "openai",
		Model:    "doubao-seed-2.1-turbo",
		BaseURL:  "https://ark.cn-beijing.volces.com/api/coding/v3",
		APIKey:   "volc-key",
	})

	if config.ProviderID != "volcengine-coding" || config.ProviderName != "Volcengine Coding" {
		t.Fatalf("expected Volcengine route metadata, got id=%q name=%q", config.ProviderID, config.ProviderName)
	}
	if config.WireAPI != "chat_completions" {
		t.Fatalf("expected chat completions for Volcengine, got %q", config.WireAPI)
	}
	if config.BaseURL != "https://ark.cn-beijing.volces.com/api/coding/v3" {
		t.Fatalf("unexpected base URL %q", config.BaseURL)
	}
}

func TestMergeLLMSeedConfigReplacesStaleProviderRoute(t *testing.T) {
	stored := defaultOpenAICompatibleLLMConfig()
	stored.ProviderID = "duckcoding"
	stored.ProviderName = "DuckCoding"
	stored.BaseURL = "https://api.duckcoding.ai/v1"
	stored.WireAPI = "responses"
	stored.Model = "old-model"

	merged := mergeLLMSeedConfig(stored, llmSeedConfig{
		Provider: "openai",
		Model:    "deepseek-v4-flash",
		BaseURL:  "https://ark.cn-beijing.volces.com/api/coding/v3",
		APIKey:   "new-key",
	})

	if merged.ProviderID != "volcengine-coding" || merged.ProviderName != "Volcengine Coding" {
		t.Fatalf("stale provider route survived seed merge: %+v", merged)
	}
	if merged.WireAPI != "chat_completions" || merged.Model != "deepseek-v4-flash" {
		t.Fatalf("seed route/model not applied: wire=%q model=%q", merged.WireAPI, merged.Model)
	}
}

func TestSanitizeAppConfigRemovesSecretValues(t *testing.T) {
	config := defaultAppConfig()
	config.LLM.APIKey = "secret"
	config.WeakLLM.APIKey = "weak-secret"
	config.Search.SemanticScholarAPIKey = "semantic-secret"
	config.Search.OpenAlexAPIKey = "openalex-secret"
	config.BaiduCloud.Token = "token-secret"
	config.BaiduCloud.RefreshToken = "refresh-secret"
	config.BaiduCloud.ClientID = "client-id-secret"
	config.BaiduCloud.ClientSecret = "client-secret"
	config.OpenAIAPIKey = "legacy-openai-secret"
	config.AnthropicAPIKey = "legacy-anthropic-secret"
	config.SemanticScholarAPIKey = "legacy-semantic-secret"

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
	if sanitized.Search.OpenAlexAPIKey != "" || !sanitized.Search.HasOpenAlexAPIKey {
		t.Fatal("expected OpenAlex key to be redacted while preserving its presence flag")
	}
	if sanitized.BaiduCloud.Token != "" {
		t.Fatal("expected cloud token to be redacted")
	}
	if sanitized.BaiduCloud.RefreshToken != "" || sanitized.BaiduCloud.ClientID != "" || sanitized.BaiduCloud.ClientSecret != "" {
		t.Fatal("expected cloud refresh/client credentials to be redacted")
	}
	if !sanitized.BaiduCloud.HasToken {
		t.Fatal("expected cloud token presence flag to remain true")
	}
	if sanitized.OpenAIAPIKey != "" || sanitized.AnthropicAPIKey != "" || sanitized.SemanticScholarAPIKey != "" {
		t.Fatal("expected legacy secret fields to be redacted")
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

func TestLoadAppConfigRejectsOversizedConfigFile(t *testing.T) {
	useTestConfigPath(t)

	oversized := strings.Repeat(" ", int(appConfigFileLimitBytes)+1)
	if err := os.WriteFile(configPathOverride, []byte(oversized), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadAppConfig(); err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("expected config byte limit error, got %v", err)
	}
}

func TestLoadAppConfigRejectsSymlinkedConfigFile(t *testing.T) {
	tempDir := useTestConfigPath(t)

	target := filepath.Join(tempDir, "real-config.json")
	if err := os.WriteFile(target, []byte(`{"theme":"dark"}`), 0600); err != nil {
		t.Fatalf("WriteFile target error = %v", err)
	}
	if err := os.Symlink(target, configPathOverride); err != nil {
		t.Skipf("Symlink not supported in this environment: %v", err)
	}

	if _, err := LoadAppConfig(); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected config symlink rejection, got %v", err)
	}
}

func TestReadLimitedFileRejectsFileSwapDuringOpen(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "seed.json")
	if err := os.WriteFile(path, []byte(`{"api_key":"original"}`), 0600); err != nil {
		t.Fatalf("WriteFile original seed error = %v", err)
	}

	swapped := false
	_, err := readLimitedFileWithOpen(path, seedConfigFileLimitBytes, func(openPath string) (*os.File, error) {
		if !swapped {
			swapped = true
			if err := os.Remove(openPath); err != nil {
				t.Fatalf("Remove original seed error = %v", err)
			}
			if err := os.WriteFile(openPath, []byte(`{"api_key":"replacement"}`), 0600); err != nil {
				t.Fatalf("WriteFile replacement seed error = %v", err)
			}
		}
		return os.Open(openPath)
	})
	if err == nil || !strings.Contains(err.Error(), "changed while opening") {
		t.Fatalf("expected file swap rejection, got %v", err)
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

func TestLoadAppConfigBaiduSeedOverridesPersistedIncompleteToken(t *testing.T) {
	useTestConfigPath(t)

	persisted := defaultAppConfig()
	persisted.BaiduCloud.Enabled = true
	persisted.BaiduCloud.Token = "old-access-token"
	persisted.BaiduCloud.RefreshToken = ""
	persisted.BaiduCloud.ClientID = ""
	persisted.BaiduCloud.ClientSecret = ""
	if err := SaveAppConfig(persisted); err != nil {
		t.Fatalf("SaveAppConfig() error = %v", err)
	}
	seedJSON := `{"access_token":"new-access-token","refresh_token":"new-refresh-token","client_id":"new-client-id","client_secret":"new-client-secret"}`
	if err := os.WriteFile("baiduyun_token.json", []byte(seedJSON), 0600); err != nil {
		t.Fatalf("WriteFile baidu seed error = %v", err)
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}
	if config.BaiduCloud.Token != "new-access-token" || config.BaiduCloud.RefreshToken != "new-refresh-token" || config.BaiduCloud.ClientID != "new-client-id" || config.BaiduCloud.ClientSecret != "new-client-secret" {
		t.Fatalf("expected baidu seed to override persisted token fields, got %+v", config.BaiduCloud)
	}
}

func TestLoadAppConfigIgnoresOversizedSeedFiles(t *testing.T) {
	useTestConfigPath(t)

	if err := os.MkdirAll("config", 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	oversizedSuffix := strings.Repeat(" ", int(seedConfigFileLimitBytes)+1)
	if err := os.WriteFile(filepath.Join("config", "strong_llm.json"), []byte(`{"api_key":"oversized-strong"}`+oversizedSuffix), 0600); err != nil {
		t.Fatalf("WriteFile strong seed error = %v", err)
	}
	if err := os.WriteFile(filepath.Join("config", "weak_llm.json"), []byte(`{"api_key":"oversized-weak"}`+oversizedSuffix), 0600); err != nil {
		t.Fatalf("WriteFile weak seed error = %v", err)
	}
	if err := os.WriteFile("baiduyun_token.json", []byte(`{"access_token":"oversized-token"}`+oversizedSuffix), 0600); err != nil {
		t.Fatalf("WriteFile baidu seed error = %v", err)
	}
	appYAML := `search:
  enable_semantic_scholar: true
  semantic_scholar_key_path: "config/semantic_scholar.private.json"
`
	if err := os.WriteFile(filepath.Join("config", "app.yaml"), []byte(appYAML), 0644); err != nil {
		t.Fatalf("WriteFile app.yaml error = %v", err)
	}
	if err := os.WriteFile(filepath.Join("config", "semantic_scholar.private.json"), []byte(`{"api_key":"oversized-semantic"}`+oversizedSuffix), 0600); err != nil {
		t.Fatalf("WriteFile semantic key seed error = %v", err)
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if config.LLM.APIKey != "" {
		t.Fatalf("expected oversized strong seed to be ignored, got %q", config.LLM.APIKey)
	}
	if config.WeakLLM.APIKey != "" {
		t.Fatalf("expected oversized weak seed to be ignored, got %q", config.WeakLLM.APIKey)
	}
	if config.BaiduCloud.Token != "" {
		t.Fatalf("expected oversized baidu seed to be ignored, got %q", config.BaiduCloud.Token)
	}
	if config.Search.SemanticScholarAPIKey != "" {
		t.Fatalf("expected oversized semantic seed to be ignored, got %q", config.Search.SemanticScholarAPIKey)
	}
}

func TestLoadAppConfigIgnoresSymlinkedSeedFiles(t *testing.T) {
	tempDir := useTestConfigPath(t)

	if err := os.MkdirAll("config", 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	target := filepath.Join(tempDir, "external-strong-seed.json")
	if err := os.WriteFile(target, []byte(`{"api_key":"symlinked-strong-key"}`), 0600); err != nil {
		t.Fatalf("WriteFile seed target error = %v", err)
	}
	if err := os.Symlink(target, filepath.Join("config", "strong_llm.json")); err != nil {
		t.Skipf("Symlink not supported in this environment: %v", err)
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}
	if config.LLM.APIKey != "" {
		t.Fatalf("expected symlinked strong seed to be ignored, got %q", config.LLM.APIKey)
	}
}

func TestLoadAppConfigBootstrapsSearchFromAppYAMLAndSemanticKeyFile(t *testing.T) {
	useTestConfigPath(t)

	if err := os.MkdirAll("config", 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	appYAML := `search:
  enable_semantic_scholar: true
  enable_arxiv: false
  semantic_scholar_key_path: "config/semantic_scholar.private.json"
  per_source_result_limit: 120
  deepstart_result_limit: 200
  retry_duration_seconds: 45
  retry_interval_seconds: 1
`
	if err := os.WriteFile(filepath.Join("config", "app.yaml"), []byte(appYAML), 0644); err != nil {
		t.Fatalf("WriteFile app.yaml error = %v", err)
	}
	if err := os.WriteFile(filepath.Join("config", "semantic_scholar.private.json"), []byte(`{"api_key":"semantic-seed-key"}`), 0600); err != nil {
		t.Fatalf("WriteFile semantic key seed error = %v", err)
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if !config.Search.EnableSemanticScholar {
		t.Fatal("expected Semantic Scholar to be enabled from app.yaml")
	}
	if config.Search.EnableArxiv {
		t.Fatal("expected arXiv to be disabled from app.yaml")
	}
	if config.Search.SemanticScholarKeyPath != filepath.Join("config", "semantic_scholar.private.json") {
		t.Fatalf("expected semantic key path to apply from app.yaml, got %q", config.Search.SemanticScholarKeyPath)
	}
	if config.Search.SemanticScholarAPIKey != "semantic-seed-key" {
		t.Fatalf("expected semantic key to load from seed file, got %q", config.Search.SemanticScholarAPIKey)
	}
	if config.Search.PerSourceResultLimit != 120 {
		t.Fatalf("expected per source result limit 120, got %d", config.Search.PerSourceResultLimit)
	}
	if config.Search.DeepStartResultLimit != 200 {
		t.Fatalf("expected deepstart result limit 200, got %d", config.Search.DeepStartResultLimit)
	}
	if config.Search.RetryDurationSeconds != 45 {
		t.Fatalf("expected retry duration 45s, got %d", config.Search.RetryDurationSeconds)
	}
}

func TestLoadAppConfigDoesNotReadSemanticKeyWhenSourceDisabled(t *testing.T) {
	useTestConfigPath(t)

	if err := os.MkdirAll("config", 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	appYAML := `search:
  enable_semantic_scholar: false
  enable_arxiv: true
  semantic_scholar_key_path: "config/semantic_scholar.private.json"
`
	if err := os.WriteFile(filepath.Join("config", "app.yaml"), []byte(appYAML), 0644); err != nil {
		t.Fatalf("WriteFile app.yaml error = %v", err)
	}
	if err := os.WriteFile(filepath.Join("config", "semantic_scholar.private.json"), []byte(`{"api_key":"semantic-seed-key"}`), 0600); err != nil {
		t.Fatalf("WriteFile semantic key seed error = %v", err)
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if config.Search.EnableSemanticScholar {
		t.Fatal("expected Semantic Scholar to remain disabled")
	}
	if config.Search.SemanticScholarAPIKey != "" {
		t.Fatalf("expected semantic key to stay empty when semantic source disabled, got %q", config.Search.SemanticScholarAPIKey)
	}
}

func TestLoadAppConfigBootstrapsUniversalSearchSourcesAndOpenAlexKey(t *testing.T) {
	useTestConfigPath(t)

	if err := os.MkdirAll("config", 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	appYAML := `search:
  enable_semantic_scholar: false
  enable_arxiv: true
  enable_openalex: true
  enable_openreview: true
  enable_dblp: true
  openalex_key_path: "config/openalex.private.json"
`
	if err := os.WriteFile(filepath.Join("config", "app.yaml"), []byte(appYAML), 0644); err != nil {
		t.Fatalf("WriteFile app.yaml error = %v", err)
	}
	if err := os.WriteFile(filepath.Join("config", "openalex.private.json"), []byte(`{"api_key":"openalex-seed-key"}`), 0600); err != nil {
		t.Fatalf("WriteFile OpenAlex key seed error = %v", err)
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}
	if config.Search.EnableSemanticScholar {
		t.Fatal("expected Semantic Scholar to remain disabled")
	}
	if !config.Search.EnableArxiv || !config.Search.EnableOpenAlex ||
		!config.Search.EnableOpenReview || !config.Search.EnableDBLP {
		t.Fatalf("expected universal source set to be enabled, got %+v", config.Search)
	}
	if config.Search.OpenAlexAPIKey != "openalex-seed-key" {
		t.Fatal("expected OpenAlex key to load from ignored seed file")
	}
}
