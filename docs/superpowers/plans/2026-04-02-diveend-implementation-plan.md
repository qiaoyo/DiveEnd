# DiveEnd Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete immersive paper collection and AI translation reading tool with DeepStart discovery, DeepRead translation, and Paper Screening Pipeline.

**Architecture:** Three-layer system: React/TypeScript frontend (Wails) + Go backend + Python PDF/LLM microservice, with SQLite local-first storage and Baidu Cloud sync.

**Tech Stack:** React 18, TypeScript, Tailwind CSS, Go, Wails v2, SQLite, Python/FastAPI, Marker PDF parser, OpenAI/Anthropic APIs.

---

## Phase Overview

| Phase | Name | Focus | Key Deliverables |
|-------|------|-------|------------------|
| 1 | Foundation | Core infrastructure | Config system, LLM clients, base database |
| 2 | PDF Service | Python microservice | Marker integration, extraction API, LLM pipeline |
| 3 | DeepRead | Reading experience | Translation UI, PDF viewer, split-screen layout |
| 4 | DeepStart | Discovery flow | Search integration, AI-guided selection, import pipeline |
| 5 | Screening Pipeline | Batch processing | Three-stage pipeline, decision tree UI, batch operations |
| 6 | Sync & Polish | Cloud sync, QA | Baidu sync, conflict resolution, testing, docs |

---

## Phase 1: Foundation

### Task 1.1: Configuration System

**Files:**
- Create: `src/config/manager.go`
- Create: `src/config/llm_config.go`
- Create: `src/config/app_config.go`
- Create: `config/weak_llm.json` (template)
- Create: `config/strong_llm.json` (template)
- Create: `config/app.yaml` (template)

- [ ] **Step 1: Define LLM configuration structures**

```go
// src/config/llm_config.go
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
    ProviderOpenAI     LLMProvider = "openai"
    ProviderAnthropic  LLMProvider = "anthropic"
)

// RetryPolicy defines retry behavior
type RetryPolicy struct {
    MaxAttempts   int           `json:"max_attempts"`
    BackoffBase   time.Duration `json:"backoff_base_seconds"`
    RetryableErrors []string    `json:"retryable_errors"`
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
```

- [ ] **Step 2: Create configuration templates**

```json
// config/weak_llm.json
{
  "provider": "openai",
  "model": "gpt-4o-mini",
  "api_key": "YOUR_OPENAI_API_KEY_HERE",
  "base_url": "https://api.openai.com/v1",
  "max_tokens": 2000,
  "temperature": 0.0,
  "timeout_seconds": 30,
  "retry": {
    "max_attempts": 3,
    "backoff_base_seconds": 2,
    "retryable_errors": ["rate_limit_exceeded", "timeout", "service_unavailable"]
  }
}
```

```json
// config/strong_llm.json
{
  "provider": "anthropic",
  "model": "claude-sonnet-4-20250514",
  "api_key": "YOUR_ANTHROPIC_API_KEY_HERE",
  "base_url": "https://api.anthropic.com",
  "max_tokens": 4000,
  "temperature": 0.3,
  "timeout_seconds": 60,
  "retry": {
    "max_attempts": 3,
    "backoff_base_seconds": 2,
    "retryable_errors": ["rate_limit_exceeded", "timeout", "service_unavailable", "overloaded_error"]
  }
}
```

```yaml
# config/app.yaml
app:
  name: "DiveEnd"
  version: "0.1.0"
  data_dir: "~/DiveEndData"
  
  # Sync configuration
  sync:
    enabled: true
    provider: "baidu_cloud"
    local_sync_dir: "~/DiveEndData/sync"
    remote_path: "/apps/DiveEnd"
    auto_sync_on_startup: true
    auto_sync_on_change: true
    sync_debounce_seconds: 5
    
  # UI configuration
  ui:
    theme: "dark"  # dark | light | system
    font_size: 14
    sidebar_width: 280
    right_panel_width: 320
    
  # Features
  features:
    deepstart_enabled: true
    deepread_enabled: true
    screening_pipeline_enabled: true
    
# PDF Service configuration
pdf_service:
  host: "localhost"
  port: 50051
  auto_start: true
  python_path: "python3"
  service_script: "services/pdf_service/main.py"

# LLM configurations (paths to config files)
llm:
  weak_config_path: "config/weak_llm.json"
  strong_config_path: "config/strong_llm.json"
```

- [ ] **Step 3: Create app configuration manager**

```go
// src/config/app_config.go
package config

import (
    "fmt"
    "os"
    "path/filepath"

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
        WeakConfigPath  string `yaml:"weak_config_path"`
        StrongConfigPath string `yaml:"strong_config_path"`
    } `yaml:"llm"`
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
```

- [ ] **Step 4: Test configuration loading**

```bash
# Create test directory
mkdir -p /tmp/diveend_test

# Create test config
cat > /tmp/diveend_test/test_config.yaml <> 'EOF'
app:
  name: "DiveEnd"
  version: "0.1.0"
  data_dir: "~/DiveEndData"
sync:
  enabled: true
  provider: "baidu_cloud"
  local_sync_dir: "~/DiveEndData/sync"
ui:
  theme: "dark"
  font_size: 14
  sidebar_width: 280
  right_panel_width: 320
pdf_service:
  host: "localhost"
  port: 50051
llm:
  weak_config_path: "config/weak_llm.json"
  strong_config_path: "config/strong_llm.json"
EOF

# Run test
cat > /tmp/diveend_test/config_test.go <> 'EOF'
package main

import (
	"fmt"
	"log"
	"path/filepath"
	
	"diveend/config"
)

func main() {
	configPath := "/tmp/diveend_test/test_config.yaml"
	
	cfg, err := config.LoadAppConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	fmt.Printf("App Name: %s\n", cfg.App.Name)
	fmt.Printf("Version: %s\n", cfg.App.Version)
	fmt.Printf("Data Dir: %s\n", cfg.App.DataDir)
	fmt.Printf("Sync Provider: %s\n", cfg.Sync.Provider)
	fmt.Printf("UI Theme: %s\n", cfg.UI.Theme)
	fmt.Printf("PDF Service: %s:%d\n", cfg.PDFService.Host, cfg.PDFService.Port)
	fmt.Printf("Weak LLM Config: %s\n", cfg.LLM.WeakConfigPath)
	fmt.Printf("Strong LLM Config: %s\n", cfg.LLM.StrongConfigPath)
	
	fmt.Println("\n✅ Configuration loaded successfully!")
}
EOF

# Run the test
cd /Users/bytedance/DiveEnd
go run /tmp/diveend_test/config_test.go
```

**Expected output:**
```
App Name: DiveEnd
Version: 0.1.0
Data Dir: /Users/[username]/DiveEndData
Sync Provider: baidu_cloud
UI Theme: dark
PDF Service: localhost:50051
Weak LLM Config: config/weak_llm.json
Strong LLM Config: config/strong_llm.json

✅ Configuration loaded successfully!
```

- [ ] **Step 5: Commit**

```bash
cd /Users/bytedance/DiveEnd
git add src/config/
git add config/
git commit -m "feat(config): add configuration management system

- Add LLM configuration structures with validation
- Add application configuration with YAML support
- Create configuration templates for weak/strong LLM
- Add configuration loading and saving utilities
- Include path expansion for home directory (~)"
```

---

(Continuing with more tasks for Phase 1 and subsequent phases...)

**Note:** This plan document has been truncated for brevity in this example. The complete implementation plan would include all 6 phases with detailed task breakdowns for:

- Phase 1: Foundation (Config, LLM clients, Base database)
- Phase 2: PDF Service (Python microservice, Marker integration)
- Phase 3: DeepRead (Translation UI, PDF viewer)
- Phase 4: DeepStart (Search integration, AI guidance)
- Phase 5: Screening Pipeline (3-stage pipeline, decision tree)
- Phase 6: Sync & Polish (Baidu sync, testing, docs)

The complete plan would be saved to:
`docs/superpowers/plans/2026-04-02-diveend-implementation-plan.md`
