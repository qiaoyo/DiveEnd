package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AppConfig main application configuration
type AppConfig struct {
	App struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
		DataDir string `yaml:"data_dir"`
	} `yaml:"app"`

	Sync struct {
		Enabled             bool   `yaml:"enabled"`
		Provider            string `yaml:"provider"`
		LocalSyncDir        string `yaml:"local_sync_dir"`
		RemotePath          string `yaml:"remote_path"`
		AutoSyncOnStartup   bool   `yaml:"auto_sync_on_startup"`
		AutoSyncOnChange    bool   `yaml:"auto_sync_on_change"`
		SyncDebounceSeconds int    `yaml:"sync_debounce_seconds"`
	} `yaml:"sync"`

	UI struct {
		Theme           string `yaml:"theme"`
		FontSize        int    `yaml:"font_size"`
		SidebarWidth    int    `yaml:"sidebar_width"`
		RightPanelWidth int    `yaml:"right_panel_width"`
	} `yaml:"ui"`

	Features struct {
		DeepstartEnabled         bool `yaml:"deepstart_enabled"`
		DeepreadEnabled          bool `yaml:"deepread_enabled"`
		ScreeningPipelineEnabled bool `yaml:"screening_pipeline_enabled"`
	} `yaml:"features"`

	PDFService struct {
		Host          string `yaml:"host"`
		Port          int    `yaml:"port"`
		AutoStart     bool   `yaml:"auto_start"`
		PythonPath    string `yaml:"python_path"`
		ServiceScript string `yaml:"service_script"`
	} `yaml:"pdf_service"`

	LLM struct {
		WeakConfigPath   string `yaml:"weak_config_path"`
		StrongConfigPath string `yaml:"strong_config_path"`
	} `yaml:"llm"`

	Search struct {
		EnableSemanticScholar  bool   `yaml:"enable_semantic_scholar"`
		EnableArxiv            bool   `yaml:"enable_arxiv"`
		SemanticScholarKeyPath string `yaml:"semantic_scholar_key_path"`
		PerSourceResultLimit   int    `yaml:"per_source_result_limit"`
		RetryDurationSeconds   int    `yaml:"retry_duration_seconds"`
		RetryIntervalSeconds   int    `yaml:"retry_interval_seconds"`
		RequestTimeoutSeconds  int    `yaml:"request_timeout_seconds"`
	} `yaml:"search"`
}

// LoadAppConfig loads application configuration from YAML file
func LoadAppConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Expand tilde in paths
	config.App.DataDir = expandTilde(config.App.DataDir)
	config.Sync.LocalSyncDir = expandTilde(config.Sync.LocalSyncDir)

	return &config, nil
}

// expandTilde expands ~ to home directory
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// Save saves configuration to file
func (c *AppConfig) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetWeakLLMConfigPath returns the absolute path to weak LLM config
func (c *AppConfig) GetWeakLLMConfigPath() string {
	if filepath.IsAbs(c.LLM.WeakConfigPath) {
		return c.LLM.WeakConfigPath
	}
	return filepath.Join(c.App.DataDir, c.LLM.WeakConfigPath)
}

// GetStrongLLMConfigPath returns the absolute path to strong LLM config
func (c *AppConfig) GetStrongLLMConfigPath() string {
	if filepath.IsAbs(c.LLM.StrongConfigPath) {
		return c.LLM.StrongConfigPath
	}
	return filepath.Join(c.App.DataDir, c.LLM.StrongConfigPath)
}
