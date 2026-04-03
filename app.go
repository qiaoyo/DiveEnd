package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// App struct - Main Wails application
type App struct {
	ctx    context.Context
	config AppConfig
	db     *DB
	llm    llmService
	search paperSearchService
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	config, err := LoadAppConfig()
	if err != nil {
		fmt.Printf("Failed to load config, fallback to defaults: %v\n", err)
		config = defaultAppConfig()
	}

	if err := a.applyConfig(config, true); err != nil {
		fmt.Printf("Failed to initialize app: %v\n", err)
	}
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		_ = a.db.Close()
	}
}

// Greet returns a greeting
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) GetInitialState() (*InitialState, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	folders, err := a.db.GetFolders()
	if err != nil {
		return nil, err
	}

	deepStartSessions, err := a.db.ListDeepStartSessions()
	if err != nil {
		return nil, err
	}

	var activeDeepStartSession *DeepStartSessionDetail
	if len(deepStartSessions) > 0 {
		activeDeepStartSession, err = a.db.GetDeepStartSession(deepStartSessions[0].ID)
		if err != nil {
			return nil, err
		}
	}

	activeFolderID := ""
	if activeDeepStartSession != nil && strings.TrimSpace(activeDeepStartSession.Summary.TargetFolderID) != "" {
		activeFolderID = activeDeepStartSession.Summary.TargetFolderID
	} else if len(folders) > 0 {
		activeFolderID = folders[0].ID
	}

	papers, err := a.db.GetPapers(activeFolderID)
	if err != nil {
		return nil, err
	}

	return &InitialState{
		Config:                 sanitizeAppConfig(a.config),
		Folders:                folders,
		Papers:                 papers,
		ActiveFolderID:         activeFolderID,
		DeepStartSessions:      deepStartSessions,
		ActiveDeepStartSession: activeDeepStartSession,
	}, nil
}

func (a *App) SaveConfig(config AppConfig) (*SaveConfigResult, error) {
	previousDataPath := a.config.DataPath
	mergedConfig := mergeAppConfigSecrets(a.config, config)
	mergedConfig = normalizeAppConfig(mergedConfig)

	if err := SaveAppConfig(mergedConfig); err != nil {
		return nil, err
	}

	restartRequired := a.db != nil && strings.TrimSpace(previousDataPath) != "" && previousDataPath != mergedConfig.DataPath
	if err := a.applyConfig(mergedConfig, !restartRequired); err != nil {
		return nil, err
	}

	return &SaveConfigResult{
		Config:          sanitizeAppConfig(a.config),
		RestartRequired: restartRequired,
	}, nil
}

func (a *App) SearchPapers(query string, limit int) ([]SearchPaper, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.search == nil {
		return nil, fmt.Errorf("search not initialized")
	}

	return a.search.Search(query, limit)
}

func (a *App) GetFolders() ([]Folder, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.db.GetFolders()
}

func (a *App) CreateFolder(name string) (*Folder, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	folder, err := a.db.CreateFolder(name)
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

func (a *App) GetPapers(folderID string) ([]Paper, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.db.GetPapers(folderID)
}

func (a *App) ImportPapers(folderID string, papers []SearchPaper) ([]Paper, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		folders, err := a.db.GetFolders()
		if err != nil {
			return nil, err
		}
		if len(folders) == 0 {
			return nil, fmt.Errorf("no folder available")
		}
		folderID = folders[0].ID
	}

	imported := make([]Paper, 0, len(papers))
	for _, searchPaper := range papers {
		paper := Paper{
			ID:        strings.TrimSpace(searchPaper.ID),
			Title:     strings.TrimSpace(searchPaper.Title),
			Authors:   strings.TrimSpace(searchPaper.Authors),
			Abstract:  strings.TrimSpace(searchPaper.Abstract),
			Year:      searchPaper.Year,
			Journal:   strings.TrimSpace(searchPaper.Journal),
			URL:       strings.TrimSpace(searchPaper.URL),
			FolderID:  folderID,
			Category:  strings.TrimSpace(searchPaper.Category),
			Tags:      searchPaper.Tags,
			AddedAt:   time.Now(),
			UpdatedAt: time.Now(),
		}

		if paper.ID == "" {
			paper.ID = fmt.Sprintf("paper-%d", time.Now().UnixNano())
		}
		if err := a.db.UpsertPaper(&paper); err != nil {
			return nil, err
		}

		imported = append(imported, paper)
	}

	return imported, nil
}

func (a *App) DeletePaper(id string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	return a.db.DeletePaper(id)
}

func (a *App) TranslatePaperSection(paperID, section, text string) (*TranslationRecord, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.llm == nil {
		return nil, fmt.Errorf("LLM not initialized")
	}

	section = strings.TrimSpace(section)
	if section == "" {
		section = "Untitled Section"
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	translated, summary, err := a.llm.TranslateSection(section, text)
	if err != nil {
		return nil, err
	}

	record := &TranslationRecord{
		PaperID:        paperID,
		Section:        section,
		OriginalText:   text,
		TranslatedText: translated,
		Summary:        summary,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := a.db.SaveTranslation(record); err != nil {
		return nil, err
	}

	return record, nil
}

func (a *App) GetTranslations(paperID string) ([]TranslationRecord, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.db.GetTranslations(paperID)
}

func (a *App) ensureReady() error {
	if a.db == nil {
		return a.applyConfig(a.config, true)
	}
	return nil
}

func (a *App) applyConfig(config AppConfig, reloadDB bool) error {
	a.config = normalizeAppConfig(config)
	a.llm = NewLLMClient(a.config)
	a.search = NewSearchClient(a.config)

	if !reloadDB {
		return nil
	}

	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}

	db, err := NewDB(a.config.DataPath)
	if err != nil {
		return err
	}
	a.db = db
	return nil
}

// SearchPapers searches for papers across multiple sources
func (a *App) SearchPapers(query string, limit int) ([]SearchResult, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.search == nil {
		return nil, fmt.Errorf("search service not initialized")
	}

	results, err := a.search.Search(query, limit)
	if err != nil {
		return nil, err
	}

	// Convert to SearchResult format
	searchResults := make([]SearchResult, len(results))
	for i, r := range results {
		searchResults[i] = SearchResult{
			ID:          r.ID,
			Title:       r.Title,
			Authors:     r.Authors,
			Abstract:    r.Abstract,
			Year:        r.Year,
			Journal:     r.Journal,
			URL:         r.URL,
			Source:      r.Source,
			Citations:   r.Citations,
			PDFURL:      r.PDFURL,
		}
	}

	return searchResults, nil
}
