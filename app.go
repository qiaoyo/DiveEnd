package main

import (
	"context"
	"fmt"
	"time"
)

// App struct - Main Wails application
type App struct {
	ctx       context.Context
	db        *DB
	llm       *LLMClient
	search    *SearchClient
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	
	// Initialize database
	db, err := NewDB("./data")
	if err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		return
	}
	a.db = db
	
	// Initialize LLM client (default to Anthropic)
	a.llm = NewLLMClient("", "claude-3-5-sonnet-20241022", "anthropic")
	
	// Initialize search client
	a.search = NewSearchClient("")
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

// Greet returns a greeting
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// SearchPapers searches for papers based on query
func (a *App) SearchPapers(query string, limit int) ([]SearchPaper, error) {
	if a.search == nil {
		return nil, fmt.Errorf("search not initialized")
	}
	
	papers, err := a.search.Search(query, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	
	return papers, nil
}

// TranslateText translates text using LLM
func (a *App) TranslateText(text string, targetLang string) (string, error) {
	if a.llm == nil {
		return "", fmt.Errorf("LLM not initialized")
	}
	
	result, err := a.llm.Translate(text, targetLang)
	if err != nil {
		return "", fmt.Errorf("translation failed: %w", err)
	}
	
	return result, nil
}

// GetPapers returns all papers from the database
func (a *App) GetPapers() ([]Paper, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	
	return a.db.GetPapers()
}

// AddPaper adds a paper to the database
func (a *App) AddPaper(paper *Paper) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	
	if paper.AddedAt.IsZero() {
		paper.AddedAt = time.Now()
	}
	if paper.UpdatedAt.IsZero() {
		paper.UpdatedAt = time.Now()
	}
	
	return a.db.AddPaper(paper)
}

// DeletePaper deletes a paper from the database
func (a *App) DeletePaper(id string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	
	return a.db.DeletePaper(id)
}

// GetConfig returns the configuration value for a key
func (a *App) GetConfig(key string) (string, error) {
	if a.db == nil {
		return "", fmt.Errorf("database not initialized")
	}
	
	return a.db.GetConfig(key)
}

// SetConfig sets a configuration value
func (a *App) SetConfig(key, value string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	
	return a.db.SetConfig(key, value)
}

// SetLLMProvider sets the LLM provider and API key
func (a *App) SetLLMProvider(providerType, apiKey, model string) error {
	a.llm = NewLLMClient(apiKey, model, providerType)
	return nil
}

// SetSearchAPIKey sets the Semantic Scholar API key
func (a *App) SetSearchAPIKey(apiKey string) error {
	a.search = NewSearchClient(apiKey)
	return nil
}