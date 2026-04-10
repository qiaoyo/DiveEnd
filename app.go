package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct - Main Wails application
type App struct {
	ctx             context.Context
	config          AppConfig
	db              *DB
	llm             llmService
	search          paperSearchService
	pdfService      *PDFServiceClient
	syncManager     *SyncManager
	stateMu         sync.RWMutex
	extractProgress map[string]*ExtractProgress
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		extractProgress: map[string]*ExtractProgress{},
	}
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

func (a *App) GetSecretPrefill() (*ConfigSecretPrefill, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	return &ConfigSecretPrefill{
		StrongLLMAPIKey:    a.config.LLM.APIKey,
		HasStrongLLMAPIKey: strings.TrimSpace(a.config.LLM.APIKey) != "",
		WeakLLMAPIKey:      a.config.WeakLLM.APIKey,
		HasWeakLLMAPIKey:   strings.TrimSpace(a.config.WeakLLM.APIKey) != "",
		BaiduToken:         a.config.BaiduCloud.Token,
		HasBaiduToken:      strings.TrimSpace(a.config.BaiduCloud.Token) != "",
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

// EnhancedSearchPapers 增强版搜索API，支持多源、分页、过滤
func (a *App) EnhancedSearchPapers(query string, limit int, offset int, yearStart int, yearEnd int, sortBy string) (*EnhancedSearchResult, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.search == nil {
		return nil, fmt.Errorf("search not initialized")
	}

	// 参数验证和默认值
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	// 调用增强搜索
	return a.search.EnhancedSearch(query, limit, offset, yearStart, yearEnd, sortBy)
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
	searchClient := NewSearchClient(a.config)
	searchClient.SetProgressReporter(func(progress SearchProgressEvent) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "search-progress", progress)
		}
	})
	a.search = searchClient
	a.pdfService = NewPDFServiceClient(defaultPDFServiceURL())

	if !reloadDB {
		if a.db != nil {
			a.syncManager = NewSyncManager(a.db, a.config)
		}
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
	a.syncManager = NewSyncManager(a.db, a.config)
	return nil
}

func defaultPDFServiceURL() string {
	if raw := strings.TrimSpace(os.Getenv("DIVEEND_PDF_SERVICE_URL")); raw != "" {
		return raw
	}
	return "http://127.0.0.1:50051"
}
