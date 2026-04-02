package config

import (
	"sync"
)

// Manager handles configuration loading and caching
type Manager struct {
	mu              sync.RWMutex
	appConfig       *AppConfig
	weakLLMConfig   *LLMConfig
	strongLLMConfig *LLMConfig
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{}
}

// LoadAll loads all configurations from the specified paths
func (m *Manager) LoadAll(appConfigPath string) error {
	// Load application configuration
	appCfg, err := LoadAppConfig(appConfigPath)
	if err != nil {
		return err
	}

	// Load LLM configurations
	weakCfg, err := LoadFromFile(appCfg.GetWeakLLMConfigPath())
	if err != nil {
		return err
	}

	strongCfg, err := LoadFromFile(appCfg.GetStrongLLMConfigPath())
	if err != nil {
		return err
	}

	// Store configurations
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appConfig = appCfg
	m.weakLLMConfig = weakCfg
	m.strongLLMConfig = strongCfg

	return nil
}

// GetAppConfig returns the application configuration
func (m *Manager) GetAppConfig() *AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.appConfig
}

// GetWeakLLMConfig returns the weak LLM configuration
func (m *Manager) GetWeakLLMConfig() *LLMConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.weakLLMConfig
}

// GetStrongLLMConfig returns the strong LLM configuration
func (m *Manager) GetStrongLLMConfig() *LLMConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.strongLLMConfig
}
