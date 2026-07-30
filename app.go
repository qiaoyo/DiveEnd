package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type runtimeEventEmitter func(context.Context, string, ...interface{})

// App struct - Main Wails application
type App struct {
	ctx                  context.Context
	config               AppConfig
	db                   *DB
	llm                  llmService // legacy alias for strong llm
	strongLLM            llmService
	weakLLM              weakLLMService
	llmTokenBudget       *dailyTokenBudget
	search               paperSearchService
	deepStartEnricher    deepStartEnricher
	pdfService           *PDFServiceClient
	pdfServiceProcess    *managedPDFService
	syncManager          *SyncManager
	stateMu              sync.RWMutex
	extractProgress      map[string]*ExtractProgress
	downloadQueue        chan paperDownloadJob
	downloadCancel       context.CancelFunc
	downloadWG           sync.WaitGroup
	downloadMu           sync.Mutex
	deepStartTaskMu      sync.Mutex
	deepStartTasks       map[string]deepStartTaskHandle
	pdfResourceMu        sync.Mutex
	pdfResources         map[string]deepReadPDFResource
	closeMu              sync.Mutex
	closeBypass          bool
	closeSyncing         bool
	periodicSyncLifeMu   sync.Mutex
	periodicSyncMu       sync.Mutex
	periodicSyncCancel   context.CancelFunc
	periodicSyncWG       sync.WaitGroup
	periodicSyncInterval time.Duration
	emitRuntimeEvent     runtimeEventEmitter
	pendingRestartConfig *AppConfig
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		extractProgress:  map[string]*ExtractProgress{},
		deepStartTasks:   map[string]deepStartTaskHandle{},
		pdfResources:     map[string]deepReadPDFResource{},
		emitRuntimeEvent: runtime.EventsEmit,
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	config, err := LoadAppConfig()
	if err != nil {
		log.Printf("Failed to load config, fallback to defaults: %v", err)
		config = defaultAppConfig()
	}
	config = normalizeAppConfig(config)
	if restoreStatus, err := applyPendingDatabaseRestore(config.DataPath); err != nil {
		log.Printf("Failed to apply pending database restore: %v", err)
	} else if restoreStatus.Applied {
		log.Printf("Applied pending database restore; local backup created")
	}

	if err := a.applyConfig(config, true); err != nil {
		log.Printf("Failed to initialize app: %v", err)
	}
	a.startManagedPDFService()
	if a.config.Sync.SyncOnStartup && syncConfigured(a.config) {
		go func() {
			if manager := a.ensureSyncManager(); manager != nil {
				_ = manager.SyncOnStartup()
			}
		}()
	}
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	a.stopPeriodicSyncLoop()
	a.cancelAllDeepStartTasks()
	a.stopDownloadWorkers()
	if a.pdfServiceProcess != nil {
		a.pdfServiceProcess.Stop()
		a.pdfServiceProcess = nil
	}
	a.closeMu.Lock()
	skipShutdownSync := a.closeBypass
	a.closeMu.Unlock()
	if !skipShutdownSync && a.shouldRunShutdownSync() {
		manager := a.ensureSyncManager()
		if manager != nil && !syncProgressActive(manager.GetSyncProgress()) {
			_ = manager.SyncToCloudIfIdle()
		}
	}
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
		Config:                 sanitizeAppConfig(a.displayConfig()),
		Folders:                folders,
		Papers:                 papers,
		ActiveFolderID:         activeFolderID,
		DeepStartSessions:      deepStartSessions,
		ActiveDeepStartSession: activeDeepStartSession,
	}, nil
}

func (a *App) displayConfig() AppConfig {
	if a.pendingRestartConfig != nil {
		return *a.pendingRestartConfig
	}
	return a.config
}

func (a *App) SaveConfig(config AppConfig) (*SaveConfigResult, error) {
	previousDataPath := a.config.DataPath
	existingConfig := a.config
	if a.pendingRestartConfig != nil {
		existingConfig = *a.pendingRestartConfig
	}
	mergedConfig := mergeAppConfigSecrets(existingConfig, config)
	mergedConfig = normalizeAppConfig(mergedConfig)

	restartRequired := a.db != nil && strings.TrimSpace(previousDataPath) != "" && previousDataPath != mergedConfig.DataPath
	runtimeConfig := mergedConfig
	if restartRequired {
		runtimeConfig.DataPath = previousDataPath
	}
	if a.syncManagerActive() && syncManagerConfigChanged(a.config, runtimeConfig) {
		return nil, fmt.Errorf("cannot change sync configuration while cloud sync is in progress")
	}

	if err := SaveAppConfig(mergedConfig); err != nil {
		return nil, err
	}

	if err := a.applyConfig(runtimeConfig, false); err != nil {
		return nil, err
	}
	if restartRequired {
		pendingConfig := mergedConfig
		a.pendingRestartConfig = &pendingConfig
	} else {
		a.pendingRestartConfig = nil
	}

	return &SaveConfigResult{
		Config:          sanitizeAppConfig(mergedConfig),
		RestartRequired: restartRequired,
	}, nil
}

func (a *App) GetSecretPrefill() (*ConfigSecretPrefill, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	return &ConfigSecretPrefill{
		StrongLLMAPIKey:    "",
		HasStrongLLMAPIKey: strings.TrimSpace(a.config.LLM.APIKey) != "",
		WeakLLMAPIKey:      "",
		HasWeakLLMAPIKey:   strings.TrimSpace(a.config.WeakLLM.APIKey) != "",
		BaiduToken:         "",
		HasBaiduToken:      strings.TrimSpace(a.config.BaiduCloud.Token) != "",
	}, nil
}

func (a *App) GetPDFServiceStatus() (*PDFServiceStatus, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.pdfService == nil {
		return &PDFServiceStatus{Enabled: false, CheckedAt: time.Now(), Message: "PDF 服务未初始化"}, nil
	}
	return a.pdfService.Status(context.Background()), nil
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

	resolvedFolderID, err := a.resolveFolderID(folderID)
	if err != nil {
		return nil, err
	}
	folderID = resolvedFolderID

	imported := make([]Paper, 0, len(papers))
	for _, searchPaper := range papers {
		sourcePaperID := normalizeSourcePaperID(searchPaper)
		paper := Paper{
			ID:             strings.TrimSpace(searchPaper.ID),
			SourcePaperID:  sourcePaperID,
			Title:          strings.TrimSpace(searchPaper.Title),
			Authors:        strings.TrimSpace(searchPaper.Authors),
			Abstract:       strings.TrimSpace(searchPaper.Abstract),
			Year:           searchPaper.Year,
			Journal:        strings.TrimSpace(searchPaper.Journal),
			URL:            strings.TrimSpace(searchPaper.URL),
			FolderID:       folderID,
			Category:       strings.TrimSpace(searchPaper.Category),
			Tags:           searchPaper.Tags,
			DownloadStatus: "queued",
			AddedAt:        time.Now(),
			UpdatedAt:      time.Now(),
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
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}

	paper, err := a.db.GetPaperByID(id)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	pdfPath, shouldRemove, err := a.managedPaperPDFCleanupCandidate(*paper)
	if err != nil {
		return err
	}
	if shouldRemove {
		if err := removeManagedFileIfPresent(a.config.DataPath, pdfPath); err != nil {
			return err
		}
	}
	return a.db.DeletePaper(id)
}

func (a *App) MovePaperToFolder(paperID, targetFolderID string) (*Paper, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return nil, fmt.Errorf("paper id cannot be empty")
	}

	paper, err := a.db.GetPaperByID(paperID)
	if err != nil {
		return nil, err
	}
	resolvedFolderID, err := a.resolveFolderID(targetFolderID)
	if err != nil {
		return nil, err
	}
	if resolvedFolderID == paper.FolderID {
		current := *paper
		return &current, nil
	}

	if strings.TrimSpace(paper.SourcePaperID) != "" {
		existing, err := a.db.GetPaperByFolderAndSource(resolvedFolderID, paper.SourcePaperID)
		if err == nil && existing != nil && existing.ID != paper.ID {
			return nil, fmt.Errorf("paper already exists in target folder")
		}
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	}

	targetFolder, err := a.db.GetFolderByID(resolvedFolderID)
	if err != nil {
		return nil, err
	}
	nextPDFPath, copiedPDF, oldPDFPath, err := a.copyManagedPaperPDFForMove(*paper, targetFolder)
	if err != nil {
		return nil, err
	}
	shouldRemoveOldPDF := false
	if copiedPDF {
		cleanupPath, shouldRemove, err := a.managedPaperPDFCleanupCandidate(*paper)
		if err != nil {
			_ = removeManagedFileIfPresent(a.config.DataPath, nextPDFPath)
			return nil, err
		}
		shouldRemoveOldPDF = shouldRemove && sameExistingLocalPath(cleanupPath, oldPDFPath)
	}

	tx, err := a.db.conn.Begin()
	if err != nil {
		if copiedPDF {
			_ = removeManagedFileIfPresent(a.config.DataPath, nextPDFPath)
		}
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()
	if _, err := tx.Exec(`
		UPDATE papers
		SET folder_id = ?, pdf_path = ?, updated_at = ?
		WHERE id = ?
	`, resolvedFolderID, nullIfBlank(nextPDFPath), now, paper.ID); err != nil {
		if copiedPDF {
			_ = removeManagedFileIfPresent(a.config.DataPath, nextPDFPath)
		}
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE deepread_parse_cache SET pdf_path = ? WHERE paper_id = ?`, nullIfBlank(nextPDFPath), paper.ID); err != nil {
		if copiedPDF {
			_ = removeManagedFileIfPresent(a.config.DataPath, nextPDFPath)
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		if copiedPDF {
			_ = removeManagedFileIfPresent(a.config.DataPath, nextPDFPath)
		}
		return nil, err
	}

	if copiedPDF && shouldRemoveOldPDF {
		if err := removeManagedFileIfPresent(a.config.DataPath, oldPDFPath); err != nil {
			log.Printf("failed to remove old managed PDF after paper move: %s", redactErrorText(err))
		}
	}

	return a.db.GetPaperByID(paper.ID)
}

func (a *App) MovePapersToFolder(paperIDs []string, targetFolderID string) ([]Paper, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	ids := uniqueTrimmedStrings(paperIDs)
	if len(ids) == 0 {
		return []Paper{}, nil
	}

	resolvedFolderID, err := a.resolveFolderID(targetFolderID)
	if err != nil {
		return nil, err
	}
	targetFolder, err := a.db.GetFolderByID(resolvedFolderID)
	if err != nil {
		return nil, err
	}

	plans, err := a.planPaperFolderMoves(ids, resolvedFolderID, targetFolder)
	if err != nil {
		return nil, err
	}

	copiedPDFs := make([]string, 0)
	copiedByTarget := make(map[string]bool)
	for _, plan := range plans {
		if !plan.NeedsCopy {
			continue
		}
		targetKey := filepath.Clean(plan.NextPDFPath)
		if copiedByTarget[targetKey] {
			continue
		}
		if err := copyManagedPaperPDF(plan.OldPDFPath, plan.NextPDFPath); err != nil {
			for _, copied := range copiedPDFs {
				_ = removeManagedFileIfPresent(a.config.DataPath, copied)
			}
			return nil, err
		}
		copiedByTarget[targetKey] = true
		copiedPDFs = append(copiedPDFs, plan.NextPDFPath)
	}

	tx, err := a.db.conn.Begin()
	if err != nil {
		for _, copied := range copiedPDFs {
			_ = removeManagedFileIfPresent(a.config.DataPath, copied)
		}
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()
	for _, plan := range plans {
		if _, err := tx.Exec(`
			UPDATE papers
			SET folder_id = ?, pdf_path = ?, updated_at = ?
			WHERE id = ?
		`, resolvedFolderID, nullIfBlank(plan.NextPDFPath), now, plan.Paper.ID); err != nil {
			for _, copied := range copiedPDFs {
				_ = removeManagedFileIfPresent(a.config.DataPath, copied)
			}
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE deepread_parse_cache SET pdf_path = ? WHERE paper_id = ?`, nullIfBlank(plan.NextPDFPath), plan.Paper.ID); err != nil {
			for _, copied := range copiedPDFs {
				_ = removeManagedFileIfPresent(a.config.DataPath, copied)
			}
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		for _, copied := range copiedPDFs {
			_ = removeManagedFileIfPresent(a.config.DataPath, copied)
		}
		return nil, err
	}

	seenOldPDFs := make(map[string]bool)
	for _, plan := range plans {
		if !plan.NeedsCopy {
			continue
		}
		oldKey := filepath.Clean(plan.OldPDFPath)
		if seenOldPDFs[oldKey] {
			continue
		}
		seenOldPDFs[oldKey] = true
		referenced, err := a.managedPDFPathReferencedByAnyPaper(plan.OldPDFPath)
		if err != nil {
			log.Printf("failed to check old managed PDF references after batch move: %s", redactErrorText(err))
			continue
		}
		if !referenced {
			if err := removeManagedFileIfPresent(a.config.DataPath, plan.OldPDFPath); err != nil {
				log.Printf("failed to remove old managed PDF after batch paper move: %s", redactErrorText(err))
			}
		}
	}

	moved := make([]Paper, 0, len(plans))
	for _, plan := range plans {
		paper, err := a.db.GetPaperByID(plan.Paper.ID)
		if err != nil {
			return nil, err
		}
		moved = append(moved, *paper)
	}
	return moved, nil
}

type paperFolderMovePlan struct {
	Paper       Paper
	NextPDFPath string
	NeedsCopy   bool
	OldPDFPath  string
}

func (a *App) planPaperFolderMoves(paperIDs []string, targetFolderID string, targetFolder Folder) ([]paperFolderMovePlan, error) {
	selectedIDs := make(map[string]bool, len(paperIDs))
	plans := make([]paperFolderMovePlan, 0, len(paperIDs))
	sourceOwners := make(map[string]string, len(paperIDs))
	targetOwners := make(map[string]string, len(paperIDs))

	for _, paperID := range paperIDs {
		if selectedIDs[paperID] {
			continue
		}
		selectedIDs[paperID] = true

		paper, err := a.db.GetPaperByID(paperID)
		if err != nil {
			return nil, err
		}
		if sourcePaperID := strings.TrimSpace(paper.SourcePaperID); sourcePaperID != "" {
			if existingPaperID, ok := sourceOwners[sourcePaperID]; ok && existingPaperID != paper.ID {
				return nil, fmt.Errorf("selected papers contain duplicate source paper")
			}
			sourceOwners[sourcePaperID] = paper.ID

			existing, err := a.db.GetPaperByFolderAndSource(targetFolderID, sourcePaperID)
			if err == nil && existing != nil && !selectedIDs[existing.ID] && existing.ID != paper.ID {
				return nil, fmt.Errorf("paper already exists in target folder")
			}
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
		}

		nextPDFPath, needsCopy, oldPDFPath, err := a.planManagedPaperPDFForMove(*paper, targetFolder)
		if err != nil {
			return nil, err
		}
		if needsCopy {
			targetKey := filepath.Clean(nextPDFPath)
			oldKey := filepath.Clean(oldPDFPath)
			if existingOldKey, ok := targetOwners[targetKey]; ok && !sameExistingLocalPath(existingOldKey, oldKey) {
				return nil, fmt.Errorf("target paper PDF already exists in batch")
			}
			targetOwners[targetKey] = oldKey
		}
		plans = append(plans, paperFolderMovePlan{
			Paper:       *paper,
			NextPDFPath: nextPDFPath,
			NeedsCopy:   needsCopy,
			OldPDFPath:  oldPDFPath,
		})
	}

	return plans, nil
}

func (a *App) copyManagedPaperPDFForMove(paper Paper, targetFolder Folder) (nextPDFPath string, copied bool, oldPDFPath string, err error) {
	nextPDFPath, needsCopy, oldPDFPath, err := a.planManagedPaperPDFForMove(paper, targetFolder)
	if err != nil || !needsCopy {
		return nextPDFPath, false, oldPDFPath, err
	}
	if err := copyManagedPaperPDF(oldPDFPath, nextPDFPath); err != nil {
		return "", false, "", err
	}
	return nextPDFPath, true, oldPDFPath, nil
}

func (a *App) planManagedPaperPDFForMove(paper Paper, targetFolder Folder) (nextPDFPath string, needsCopy bool, oldPDFPath string, err error) {
	pdfPath := strings.TrimSpace(paper.PDFPath)
	if pdfPath == "" {
		return "", false, "", nil
	}

	rootPath := filepath.Join(a.config.DataPath, "papers")
	managedPath, ok := managedPathInsideRoot(rootPath, pdfPath)
	if !ok {
		return pdfPath, false, "", nil
	}
	managedPath, err = managedPathWithPlainExistingParent(rootPath, managedPath)
	if err != nil {
		return "", false, "", err
	}
	if _, err := os.Lstat(managedPath); err != nil {
		if os.IsNotExist(err) {
			return pdfPath, false, "", nil
		}
		return "", false, "", err
	}
	if err := validateLocalPDFFile(managedPath); err != nil {
		return "", false, "", err
	}

	targetFolderPath := normalizeFolderPath(targetFolder.Path)
	if targetFolderPath == "" {
		targetFolderPath = normalizeFolderPath(targetFolder.Name)
	}
	targetPath := filepath.Join(rootPath, filepath.FromSlash(targetFolderPath), filepath.Base(managedPath))
	targetPath, err = ensureManagedFileParent(rootPath, targetPath)
	if err != nil {
		return "", false, "", err
	}
	if sameExistingLocalPath(managedPath, targetPath) {
		return managedPath, false, "", nil
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return "", false, "", fmt.Errorf("target paper PDF already exists")
	} else if err != nil && !os.IsNotExist(err) {
		return "", false, "", err
	}

	return targetPath, true, managedPath, nil
}

func copyManagedPaperPDF(sourcePath, targetPath string) error {
	sourcePath = strings.TrimSpace(sourcePath)
	targetPath = strings.TrimSpace(targetPath)
	if sourcePath == "" || targetPath == "" {
		return fmt.Errorf("source and target PDF paths cannot be empty")
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return fmt.Errorf("target paper PDF already exists")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	source, _, err := openValidatedLocalPDFFile(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	if err := writeFileAtomicWithWriter(targetPath, 0600, func(tmp *os.File) error {
		_, err := io.Copy(tmp, source)
		return err
	}, func(_ *os.File, path string) error {
		return validateLocalPDFFile(path)
	}); err != nil {
		return err
	}
	return nil
}

func (a *App) managedPaperPDFCleanupCandidate(paper Paper) (string, bool, error) {
	pdfPath := strings.TrimSpace(paper.PDFPath)
	if pdfPath == "" {
		return "", false, nil
	}

	rootPath := filepath.Join(a.config.DataPath, "papers")
	managedPath, ok := managedPathInsideRoot(rootPath, pdfPath)
	if !ok {
		return "", false, nil
	}
	managedPath, err := managedPathWithPlainExistingParent(rootPath, managedPath)
	if err != nil {
		return "", false, err
	}

	papers, err := a.db.GetPapers("")
	if err != nil {
		return "", false, err
	}
	for _, candidate := range papers {
		if candidate.ID == paper.ID {
			continue
		}
		if strings.TrimSpace(candidate.PDFPath) == "" {
			continue
		}
		if sameExistingLocalPath(managedPath, candidate.PDFPath) {
			return managedPath, false, nil
		}
	}

	return managedPath, true, nil
}

func (a *App) managedPDFPathReferencedByAnyPaper(pdfPath string) (bool, error) {
	pdfPath = strings.TrimSpace(pdfPath)
	if pdfPath == "" {
		return false, nil
	}
	papers, err := a.db.GetPapers("")
	if err != nil {
		return false, err
	}
	for _, paper := range papers {
		if strings.TrimSpace(paper.PDFPath) == "" {
			continue
		}
		if sameExistingLocalPath(pdfPath, paper.PDFPath) {
			return true, nil
		}
	}
	return false, nil
}

func uniqueTrimmedStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		unique = append(unique, trimmed)
	}
	return unique
}

func (a *App) TranslatePaperSection(paperID, section, text string) (*TranslationRecord, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.llm != nil && a.llm != a.strongLLM {
		section = strings.TrimSpace(section)
		if section == "" {
			section = "Untitled Section"
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, fmt.Errorf("text cannot be empty")
		}
		translated, summary, err := translateSectionWithContext(a.ctx, a.llm, section, text)
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

	translator := a.currentWeakLLM()
	if translator == nil {
		return nil, fmt.Errorf("weak LLM not initialized")
	}

	section = strings.TrimSpace(section)
	if section == "" {
		section = "Untitled Section"
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	translated, summary, err := translateSectionWithContext(a.ctx, translator, section, text)
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
	a.stopPeriodicSyncLoop()
	previousConfig := a.config
	a.config = normalizeAppConfig(config)
	a.llmTokenBudget = newDailyTokenBudget(a.config.DataPath, a.config.DailyLLMTokenBudget)
	strongLLM := NewStrongLLMClient(a.config)
	strongLLM.tokenBudget = a.llmTokenBudget
	weakLLM := NewWeakLLMClient(a.config)
	weakLLM.tokenBudget = a.llmTokenBudget
	a.strongLLM = strongLLM
	a.weakLLM = weakLLM
	a.llm = a.strongLLM
	searchClient := NewSearchClient(a.config)
	searchClient.SetProgressReporter(func(progress SearchProgressEvent) {
		a.emitEvent("search-progress", progress)
	})
	a.search = searchClient
	a.pdfService = NewPDFServiceClient(defaultPDFServiceURL())
	a.deepStartEnricher = NewDeepStartEnricher(a.db)

	if !reloadDB {
		if a.db != nil {
			a.deepStartEnricher = NewDeepStartEnricher(a.db)
			if a.syncManager == nil || syncManagerConfigChanged(previousConfig, a.config) {
				if !a.syncManagerActive() {
					a.syncManager = NewSyncManager(a.db, a.config)
					a.configureSyncManager()
				}
			}
			a.configurePeriodicSyncLoop()
			a.startDownloadWorkers()
		}
		return nil
	}

	if a.db != nil {
		a.stopDownloadWorkers()
		_ = a.db.Close()
		a.db = nil
	}

	db, err := NewDB(a.config.DataPath)
	if err != nil {
		return err
	}
	a.db = db
	if recovered, err := a.db.RecoverInterruptedDeepStartSessions(); err != nil {
		log.Printf("Failed to recover interrupted DeepStart sessions: %v", err)
	} else if recovered > 0 {
		log.Printf("Recovered %d interrupted DeepStart session(s)", recovered)
	}
	a.deepStartEnricher = NewDeepStartEnricher(a.db)
	a.syncManager = NewSyncManager(a.db, a.config)
	a.configureSyncManager()
	a.configurePeriodicSyncLoop()
	a.startDownloadWorkers()
	return nil
}

func (a *App) GetLLMUsage() LLMUsageSnapshot {
	if a.llmTokenBudget == nil {
		return LLMUsageSnapshot{Limit: defaultDailyLLMTokenBudget, Remaining: defaultDailyLLMTokenBudget}
	}
	return a.llmTokenBudget.Snapshot()
}

func (a *App) configureSyncManager() {
	if a.syncManager == nil {
		return
	}
	a.syncManager.SetProgressReporter(func(progress SyncProgress) {
		a.emitEvent("sync-progress", progress)
	})
}

func (a *App) syncManagerActive() bool {
	return a.syncManager != nil && syncProgressActive(a.syncManager.GetSyncProgress())
}

func (a *App) emitEvent(eventName string, optionalData ...interface{}) {
	if a.ctx == nil || a.emitRuntimeEvent == nil {
		return
	}
	a.emitRuntimeEvent(a.ctx, eventName, optionalData...)
}

func defaultPDFServiceURL() string {
	if raw := strings.TrimSpace(os.Getenv("DIVEEND_PDF_SERVICE_URL")); raw != "" {
		return raw
	}
	return "http://127.0.0.1:50051"
}

func (a *App) currentStrongLLM() llmService {
	if a.llm != nil {
		return a.llm
	}
	return a.strongLLM
}

func (a *App) currentWeakLLM() weakLLMService {
	if a.weakLLM != nil {
		return a.weakLLM
	}
	if strong := a.currentStrongLLM(); strong != nil {
		if client, ok := strong.(*LLMClient); ok {
			return client
		}
	}
	return nil
}
