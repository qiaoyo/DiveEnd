package main

import "time"

const defaultFolderName = "Cache"

type LLMConfig struct {
	ProviderID             string `json:"providerId"`
	ProviderName           string `json:"providerName"`
	ProviderType           string `json:"providerType"`
	BaseURL                string `json:"baseUrl"`
	WireAPI                string `json:"wireApi"`
	RequiresOpenAIAuth     bool   `json:"requiresOpenAIAuth"`
	APIKey                 string `json:"apiKey,omitempty"`
	HasAPIKey              bool   `json:"hasApiKey"`
	Model                  string `json:"model"`
	ReasoningEffort        string `json:"reasoningEffort"`
	DisableResponseStorage bool   `json:"disableResponseStorage"`
	ClearAPIKey            bool   `json:"clearApiKey,omitempty"`
}

type SearchAPIConfig struct {
	EnableSemanticScholar      bool   `json:"enableSemanticScholar"`
	EnableArxiv                bool   `json:"enableArxiv"`
	EnableOpenAlex             bool   `json:"enableOpenAlex"`
	EnableOpenReview           bool   `json:"enableOpenReview"`
	EnableDBLP                 bool   `json:"enableDBLP"`
	SemanticScholarKeyPath     string `json:"semanticScholarKeyPath"`
	OpenAlexKeyPath            string `json:"openAlexKeyPath"`
	PerSourceResultLimit       int    `json:"perSourceResultLimit"`
	DeepStartResultLimit       int    `json:"deepStartResultLimit"`
	RetryDurationSeconds       int    `json:"retryDurationSeconds"`
	RetryIntervalSeconds       int    `json:"retryIntervalSeconds"`
	RequestTimeoutSeconds      int    `json:"requestTimeoutSeconds"`
	SemanticScholarAPIKey      string `json:"semanticScholarApiKey,omitempty"`
	HasSemanticScholarAPIKey   bool   `json:"hasSemanticScholarApiKey"`
	ClearSemanticScholarAPIKey bool   `json:"clearSemanticScholarApiKey,omitempty"`
	OpenAlexAPIKey             string `json:"openAlexApiKey,omitempty"`
	HasOpenAlexAPIKey          bool   `json:"hasOpenAlexApiKey"`
	ClearOpenAlexAPIKey        bool   `json:"clearOpenAlexApiKey,omitempty"`
}

type BaiduCloudConfig struct {
	Enabled      bool   `json:"enabled"`
	Token        string `json:"token,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	HasToken     bool   `json:"hasToken"`
	Quota        int    `json:"quota"`
	ClearToken   bool   `json:"clearToken,omitempty"`
}

type AppConfig struct {
	LLM                   LLMConfig        `json:"llm"`
	WeakLLM               LLMConfig        `json:"weakLLM"`
	DailyLLMTokenBudget   int64            `json:"dailyLLMTokenBudget"`
	Search                SearchAPIConfig  `json:"search"`
	BaiduCloud            BaiduCloudConfig `json:"baiduCloud"`
	Sync                  SyncSettings     `json:"sync"`
	Theme                 string           `json:"theme"`
	LeftPanelWidth        int              `json:"leftPanelWidth"`
	RightPanelWidth       int              `json:"rightPanelWidth"`
	DataPath              string           `json:"dataPath"`
	SelectedProvider      string           `json:"selectedProvider,omitempty"`
	OpenAIAPIKey          string           `json:"openaiApiKey,omitempty"`
	OpenAIModel           string           `json:"openaiModel,omitempty"`
	AnthropicAPIKey       string           `json:"anthropicApiKey,omitempty"`
	AnthropicModel        string           `json:"anthropicModel,omitempty"`
	SemanticScholarAPIKey string           `json:"semanticScholarApiKey,omitempty"`
}

type SaveConfigResult struct {
	Config          AppConfig `json:"config"`
	RestartRequired bool      `json:"restartRequired"`
}

type ConfigSecretPrefill struct {
	StrongLLMAPIKey    string `json:"strongLLMApiKey,omitempty"`
	HasStrongLLMAPIKey bool   `json:"hasStrongLLMApiKey"`
	WeakLLMAPIKey      string `json:"weakLLMApiKey,omitempty"`
	HasWeakLLMAPIKey   bool   `json:"hasWeakLLMApiKey"`
	BaiduToken         string `json:"baiduToken,omitempty"`
	HasBaiduToken      bool   `json:"hasBaiduToken"`
}

type InitialState struct {
	Config                 AppConfig                 `json:"config"`
	Folders                []Folder                  `json:"folders"`
	Papers                 []Paper                   `json:"papers"`
	ActiveFolderID         string                    `json:"activeFolderId"`
	DeepStartSessions      []DeepStartSessionSummary `json:"deepstartSessions"`
	ActiveDeepStartSession *DeepStartSessionDetail   `json:"activeDeepStartSession,omitempty"`
}

type ScreeningPaper struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"sessionId"`
	FileName       string    `json:"fileName"`
	FilePath       string    `json:"filePath"`
	FileSize       int64     `json:"fileSize"`
	Status         string    `json:"status"` // "pending", "extracting", "extracted", "screening", "selected", "rejected"
	Title          string    `json:"title,omitempty"`
	Authors        string    `json:"authors,omitempty"`
	Abstract       string    `json:"abstract,omitempty"`
	FullText       string    `json:"fullText,omitempty"`
	SectionsJSON   string    `json:"sectionsJson,omitempty"`
	Selection      string    `json:"selection,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	TargetFolderID string    `json:"targetFolderId,omitempty"`
	CreatedAt      time.Time `json:"createdAt" ts_type:"string"`
	UpdatedAt      time.Time `json:"updatedAt" ts_type:"string"`
}

type ScreeningSession struct {
	ID                  string    `json:"id"`
	Title               string    `json:"title"`
	Status              string    `json:"status"` // "upload", "extract", "screen", "complete"
	TotalPapers         int       `json:"totalPapers"`
	CurrentNodeJSON     string    `json:"currentNodeJson,omitempty"`
	SelectedOptionsJSON string    `json:"selectedOptionsJson,omitempty"`
	PathHistoryJSON     string    `json:"pathHistoryJson,omitempty"`
	CreatedAt           time.Time `json:"createdAt" ts_type:"string"`
	UpdatedAt           time.Time `json:"updatedAt" ts_type:"string"`
}

type ScreeningDecisionOption struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	PaperIDs []string `json:"paperIds"`
	Count    int      `json:"count"`
}

type ScreeningDecisionNode struct {
	ID                string                    `json:"id"`
	NodeType          string                    `json:"nodeType"` // "branch" | "complete"
	Message           string                    `json:"message"`
	Dimension         string                    `json:"dimension"`
	Options           []ScreeningDecisionOption `json:"options"`
	AllowMultiSelect  bool                      `json:"allowMultiSelect"`
	AllowSkip         bool                      `json:"allowSkip"`
	RemainingPaperIDs []string                  `json:"remainingPaperIds"`
}

type ExtractProgress struct {
	SessionID    string `json:"sessionId"`
	Total        int    `json:"total"`
	Completed    int    `json:"completed"`
	CurrentFile  string `json:"currentFile"`
	Status       string `json:"status"` // "processing", "completed", "error"
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type ScreeningSessionDetail struct {
	Session     ScreeningSession       `json:"session"`
	Papers      []ScreeningPaper       `json:"papers"`
	CurrentNode *ScreeningDecisionNode `json:"currentNode,omitempty"`
	PathHistory []PathHistoryItem      `json:"pathHistory"`
}

type PathHistoryItem struct {
	Dimension string `json:"dimension"`
	Choice    string `json:"choice"`
}

type Folder struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ParentID  string    `json:"parentId,omitempty"`
	Path      string    `json:"path"`
	IsSystem  bool      `json:"isSystem"`
	CreatedAt time.Time `json:"createdAt" ts_type:"string"`
}

type FolderNode struct {
	Folder   Folder       `json:"folder"`
	Children []FolderNode `json:"children"`
}

type CreateFolderNodeRequest struct {
	ParentID string `json:"parentId,omitempty"`
	Path     string `json:"path,omitempty"`
	Name     string `json:"name,omitempty"`
}

type RenameFolderNodeRequest struct {
	FolderID string `json:"folderId"`
	Name     string `json:"name"`
}

type MoveFolderNodeRequest struct {
	FolderID string `json:"folderId"`
	ParentID string `json:"parentId,omitempty"`
}

type Paper struct {
	ID             string    `json:"id"`
	SourcePaperID  string    `json:"sourcePaperId"`
	Title          string    `json:"title"`
	Authors        string    `json:"authors"`
	Abstract       string    `json:"abstract"`
	Year           int       `json:"year"`
	Journal        string    `json:"journal"`
	URL            string    `json:"url"`
	PDFPath        string    `json:"pdfPath,omitempty"`
	DownloadStatus string    `json:"downloadStatus"`
	DownloadError  string    `json:"downloadError,omitempty"`
	FolderID       string    `json:"folderId"`
	Category       string    `json:"category"`
	Tags           []string  `json:"tags"`
	AddedAt        time.Time `json:"addedAt" ts_type:"string"`
	UpdatedAt      time.Time `json:"updatedAt" ts_type:"string"`
}

type SearchPaper struct {
	ID                       string            `json:"id"`
	Title                    string            `json:"title"`
	Authors                  string            `json:"authors"`
	Abstract                 string            `json:"abstract"`
	Year                     int               `json:"year"`
	Journal                  string            `json:"journal"`
	PublicationVenue         string            `json:"publicationVenue"`
	PublicationYear          int               `json:"publicationYear"`
	CitationCount            int               `json:"citationCount"`
	URL                      string            `json:"url"`
	Category                 string            `json:"category"`
	Tags                     []string          `json:"tags"`
	Source                   string            `json:"source"`
	Sources                  []string          `json:"sources,omitempty"`
	ExternalIDs              map[string]string `json:"externalIds,omitempty"`
	PDFCandidates            []string          `json:"pdfCandidates,omitempty"`
	Institutions             []string          `json:"institutions"`
	Keywords                 []string          `json:"keywords"`
	SourceLabel              string            `json:"sourceLabel"`
	MatchReason              string            `json:"matchReason,omitempty"`
	MatchedTerms             []string          `json:"matchedTerms,omitempty"`
	EnrichmentNote           string            `json:"enrichmentNote,omitempty"`
	PreprocessStatus         string            `json:"preprocessStatus,omitempty"` // pending | no_pdf_url | downloading | downloaded | parsing | parsed | extracting | extracted | failed
	LocalPDFPath             string            `json:"localPdfPath,omitempty"`
	MarkdownPath             string            `json:"markdownPath,omitempty"`
	ParseStatus              string            `json:"parseStatus,omitempty"` // pending | success | failed | skipped
	ParseError               string            `json:"parseError,omitempty"`
	ExtractStatus            string            `json:"extractStatus,omitempty"` // pending | success | failed | skipped
	ExtractError             string            `json:"extractError,omitempty"`
	Problem                  string            `json:"problem,omitempty"`
	Method                   string            `json:"method,omitempty"`
	TopicLabel               string            `json:"topicLabel,omitempty"`
	MethodLabel              string            `json:"methodLabel,omitempty"`
	TaskLabel                string            `json:"taskLabel,omitempty"`
	DomainLabel              string            `json:"domainLabel,omitempty"`
	ClassificationConfidence float64           `json:"classificationConfidence,omitempty"`
	ProcessingStage          string            `json:"processingStage,omitempty"`
	ProcessingError          string            `json:"processingError,omitempty"`
}

type PaperProfileExtraction struct {
	Title                    string   `json:"title"`
	Authors                  []string `json:"authors"`
	Abstract                 string   `json:"abstract"`
	Problem                  string   `json:"problem"`
	Method                   string   `json:"method"`
	Keywords                 []string `json:"keywords"`
	RelevanceTags            []string `json:"relevanceTags"`
	TopicLabel               string   `json:"topicLabel"`
	MethodLabel              string   `json:"methodLabel"`
	TaskLabel                string   `json:"taskLabel"`
	DomainLabel              string   `json:"domainLabel"`
	ClassificationConfidence float64  `json:"classificationConfidence"`
}

type ImportSkippedPaper struct {
	SourcePaperID string `json:"sourcePaperId"`
	Title         string `json:"title"`
	Reason        string `json:"reason"`
}

type ImportPapersWithAssetsResult struct {
	Imported []Paper              `json:"imported"`
	Skipped  []ImportSkippedPaper `json:"skipped"`
	Queued   int                  `json:"queued"`
	Message  string               `json:"message,omitempty"`
}

type LocalStorageFolderOverview struct {
	FolderID        string `json:"folderId"`
	FolderName      string `json:"folderName"`
	FolderPath      string `json:"folderPath"`
	TotalPapers     int    `json:"totalPapers"`
	Queued          int    `json:"queued"`
	Downloading     int    `json:"downloading"`
	Downloaded      int    `json:"downloaded"`
	Failed          int    `json:"failed"`
	StoredFileCount int    `json:"storedFileCount"`
}

type FolderStorageTreeNode struct {
	FolderID    string                  `json:"folderId"`
	FolderName  string                  `json:"folderName"`
	FolderPath  string                  `json:"folderPath"`
	PaperCount  int                     `json:"paperCount"`
	Queued      int                     `json:"queued"`
	Downloading int                     `json:"downloading"`
	Downloaded  int                     `json:"downloaded"`
	Failed      int                     `json:"failed"`
	Children    []FolderStorageTreeNode `json:"children"`
}

type FolderStorageTreeOverview struct {
	RootPath    string                  `json:"rootPath"`
	Directories []FolderStorageTreeNode `json:"directories"`
	GeneratedAt time.Time               `json:"generatedAt" ts_type:"string"`
}

type LocalStorageOverview struct {
	RootPath     string                       `json:"rootPath"`
	TotalFolders int                          `json:"totalFolders"`
	TotalFiles   int                          `json:"totalFiles"`
	Queued       int                          `json:"queued"`
	Downloading  int                          `json:"downloading"`
	Downloaded   int                          `json:"downloaded"`
	Failed       int                          `json:"failed"`
	Folders      []LocalStorageFolderOverview `json:"folders"`
	GeneratedAt  time.Time                    `json:"generatedAt" ts_type:"string"`
}

type SearchRetrievalStats struct {
	Query            string         `json:"query"`
	OriginalQuery    string         `json:"originalQuery,omitempty"`
	RewrittenQueries []string       `json:"rewrittenQueries,omitempty"`
	QueryHits        map[string]int `json:"queryHits,omitempty"`
	RawCount         int            `json:"rawCount"`
	DedupCount       int            `json:"dedupCount"`
	FinalCount       int            `json:"finalCount"`
}

type SearchSourceStatus struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Count   int    `json:"count"`
}

type EnhancedSearchResult struct {
	Papers    []SearchPaper        `json:"papers"`
	Total     int                  `json:"total"`
	HasMore   bool                 `json:"hasMore"`
	Sources   []SearchSourceStatus `json:"sources"`
	Query     string               `json:"query"`
	Limit     int                  `json:"limit"`
	Offset    int                  `json:"offset"`
	YearStart int                  `json:"yearStart,omitempty"`
	YearEnd   int                  `json:"yearEnd,omitempty"`
	SortBy    string               `json:"sortBy,omitempty"`
}

type DeepStartSessionSummary struct {
	ID                  string    `json:"id"`
	Title               string    `json:"title"`
	RootPrompt          string    `json:"rootPrompt"`
	CurrentQuery        string    `json:"currentQuery"`
	TargetFolderID      string    `json:"targetFolderId"`
	ProcessingStatus    string    `json:"processingStatus,omitempty"` // initializing | background_processing | completed
	InitialReadyCount   int       `json:"initialReadyCount,omitempty"`
	TotalPlannedCount   int       `json:"totalPlannedCount,omitempty"`
	BackgroundRemaining int       `json:"backgroundRemaining,omitempty"`
	CreatedAt           time.Time `json:"createdAt" ts_type:"string"`
	UpdatedAt           time.Time `json:"updatedAt" ts_type:"string"`
}

type DeepStartMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt" ts_type:"string"`
}

type DeepStartDirection struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Summary  string   `json:"summary"`
	Why      string   `json:"why"`
	PaperIDs []string `json:"paperIds"`
}

type DeepStartPaperNote struct {
	PaperID      string   `json:"paperId"`
	Tier         string   `json:"tier"`
	Reason       string   `json:"reason"`
	DirectionIDs []string `json:"directionIds"`
}

type DeepStartAnalysis struct {
	Overview            string               `json:"overview"`
	Directions          []DeepStartDirection `json:"directions"`
	PaperNotes          []DeepStartPaperNote `json:"paperNotes"`
	FollowUpQuestions   []string             `json:"followUpQuestions"`
	SuggestedQueries    []string             `json:"suggestedQueries"`
	RecommendedPaperIDs []string             `json:"recommendedPaperIds"`
	RetainedPaperIDs    []string             `json:"retainedPaperIds"`
	SearchStats         SearchRetrievalStats `json:"searchStats"`
}

type DeepStartSessionDetail struct {
	Summary          DeepStartSessionSummary `json:"summary"`
	Messages         []DeepStartMessage      `json:"messages"`
	CurrentResults   []SearchPaper           `json:"currentResults"`
	CurrentAnalysis  *DeepStartAnalysis      `json:"currentAnalysis,omitempty"`
	SelectedPaperIDs []string                `json:"selectedPaperIds"`
}

type DeepStartProgressEvent struct {
	SessionID                 string                `json:"sessionId,omitempty"`
	Phase                     string                `json:"phase"` // searching | enriching | downloading | parsing | weak_extracting | initial_batch_ready | background_processing | analyzing | persisting | cancelling | cancelled | completed
	Message                   string                `json:"message,omitempty"`
	ElapsedSeconds            int                   `json:"elapsedSeconds"`
	EstimatedRemainingSeconds int                   `json:"estimatedRemainingSeconds"`
	Total                     int                   `json:"total"`
	Completed                 int                   `json:"completed"`
	OverallPercent            int                   `json:"overallPercent"`
	SuccessCount              int                   `json:"successCount,omitempty"`
	FailedCount               int                   `json:"failedCount,omitempty"`
	NoPDFURLCount             int                   `json:"noPdfUrlCount,omitempty"`
	DownloadedCount           int                   `json:"downloadedCount,omitempty"`
	ParsedCount               int                   `json:"parsedCount,omitempty"`
	ExtractedCount            int                   `json:"extractedCount,omitempty"`
	InitialBatchTotal         int                   `json:"initialBatchTotal,omitempty"`
	InitialBatchCompleted     int                   `json:"initialBatchCompleted,omitempty"`
	BackgroundCompleted       int                   `json:"backgroundCompleted,omitempty"`
	Stats                     *SearchRetrievalStats `json:"stats,omitempty"`
}

type DeepStartEnrichmentCache struct {
	CacheKey          string
	Institutions      []string
	Keywords          []string
	SourceLabel       string
	PublicationVenue  string
	PublicationYear   int
	CitationCount     int
	OpenAlexAttempted bool
	CrossrefAttempted bool
	ErrorMessage      string
	UpdatedAt         time.Time
}

type TranslationRecord struct {
	ID             string    `json:"id"`
	PaperID        string    `json:"paperId"`
	Section        string    `json:"section"`
	OriginalText   string    `json:"originalText"`
	TranslatedText string    `json:"translatedText"`
	Summary        string    `json:"summary"`
	CreatedAt      time.Time `json:"createdAt" ts_type:"string"`
	UpdatedAt      time.Time `json:"updatedAt" ts_type:"string"`
}

type DeepReadSection struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Index   int    `json:"index"`
}

type DeepReadParseCache struct {
	PaperID        string            `json:"paperId"`
	PDFPath        string            `json:"pdfPath"`
	Status         string            `json:"status"` // idle | preparing | ready | failed | missing_pdf
	ErrorMessage   string            `json:"errorMessage,omitempty"`
	Markdown       string            `json:"markdown,omitempty"`
	Sections       []DeepReadSection `json:"sections"`
	LastPreparedAt time.Time         `json:"lastPreparedAt" ts_type:"string"`
}

type DeepReadNote struct {
	ID        string    `json:"id"`
	PaperID   string    `json:"paperId"`
	Section   string    `json:"section"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt" ts_type:"string"`
	UpdatedAt time.Time `json:"updatedAt" ts_type:"string"`
}

type DeepReadEvidence struct {
	SectionID    string `json:"sectionId"`
	SectionTitle string `json:"sectionTitle"`
	Excerpt      string `json:"excerpt"`
}

type DeepReadAIResponse struct {
	Mode        string             `json:"mode"`
	Answer      string             `json:"answer"`
	Takeaway    string             `json:"takeaway"`
	Evidence    []DeepReadEvidence `json:"evidence"`
	Limitations []string           `json:"limitations"`
}

type DeepReadState struct {
	PaperID        string              `json:"paperId"`
	HasPDF         bool                `json:"hasPdf"`
	PDFPath        string              `json:"pdfPath"`
	ParseStatus    string              `json:"parseStatus"`
	ParseError     string              `json:"parseError,omitempty"`
	Sections       []DeepReadSection   `json:"sections"`
	Markdown       string              `json:"markdown,omitempty"`
	Translations   []TranslationRecord `json:"translations"`
	Notes          []DeepReadNote      `json:"notes"`
	LastPreparedAt time.Time           `json:"lastPreparedAt" ts_type:"string"`
}

// ========== Phase 6: Baidu Cloud Sync 相关数据模型 ==========

// BaiduToken 百度网盘 OAuth token
type BaiduToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	ExpiresIn    int    `json:"expires_in"`
}

// SyncRecord 同步记录
type SyncRecord struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"` // "upload", "download", "conflict"
	FileName     string    `json:"fileName"`
	FileSize     int64     `json:"fileSize"`
	RemotePath   string    `json:"remotePath"`
	LocalPath    string    `json:"localPath"`
	Status       string    `json:"status"` // "pending", "success", "failed"
	Message      string    `json:"message,omitempty"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	CreatedAt    time.Time `json:"createdAt" ts_type:"string"`
	CompletedAt  time.Time `json:"completedAt,omitempty" ts_type:"string"`
}

// SyncConflict 同步冲突
type SyncConflict struct {
	ID         string    `json:"id"`
	FileName   string    `json:"fileName"`
	FileKind   string    `json:"fileKind,omitempty"`
	LocalPath  string    `json:"localPath"`
	LocalSize  int64     `json:"localSize"`
	LocalTime  time.Time `json:"localTime" ts_type:"string"`
	RemotePath string    `json:"remotePath"`
	RemoteSize int64     `json:"remoteSize"`
	RemoteTime time.Time `json:"remoteTime" ts_type:"string"`
	NewerSide  string    `json:"newerSide,omitempty"` // "local", "remote", "equal"
	Resolution string    `json:"resolution"`          // "local", "remote", "skipped"
	ResolvedAt time.Time `json:"resolvedAt,omitempty" ts_type:"string"`
	CreatedAt  time.Time `json:"createdAt" ts_type:"string"`
}

// SyncStatus 同步状态
type SyncStatus struct {
	Enabled        bool       `json:"enabled"`
	Provider       string     `json:"provider"`
	LastSync       *time.Time `json:"lastSync,omitempty" ts_type:"string"`
	SyncInProgress bool       `json:"syncInProgress"`
	PendingFiles   int        `json:"pendingFiles"`
	Conflicts      int        `json:"conflicts"`
	TotalSynced    int        `json:"totalSynced"`
	TotalFailed    int        `json:"totalFailed"`
}

// SyncSettings 同步设置
type SyncSettings struct {
	AutoSync           bool   `json:"autoSync"`
	SyncOnStartup      bool   `json:"syncOnStartup"`
	SyncBeforeExit     bool   `json:"syncBeforeExit"`
	SyncInterval       int    `json:"syncInterval"`       // minutes
	ConflictResolution string `json:"conflictResolution"` // "timestamp" (always use newest)
}

// SyncProgress 同步进度
type SyncProgress struct {
	Total       int    `json:"total"`
	Completed   int    `json:"completed"`
	CurrentFile string `json:"currentFile"`
	Status      string `json:"status"` // "preparing", "uploading", "downloading", "resolving", "complete", "error"
	Message     string `json:"message,omitempty"`
}

type BaiduTokenRefreshStatus struct {
	Enabled         bool      `json:"enabled"`
	TokenFile       string    `json:"tokenFile"`
	HasAccessToken  bool      `json:"hasAccessToken"`
	HasRefreshToken bool      `json:"hasRefreshToken"`
	HasClientID     bool      `json:"hasClientId"`
	HasClientSecret bool      `json:"hasClientSecret"`
	Refreshed       bool      `json:"refreshed"`
	CheckedAt       time.Time `json:"checkedAt" ts_type:"string"`
	Message         string    `json:"message,omitempty"`
}

type SyncPreviewFile struct {
	Key        string `json:"key"`
	FileName   string `json:"fileName"`
	Kind       string `json:"kind"`
	Size       int64  `json:"size"`
	RemotePath string `json:"remotePath"`
}

type SyncPreview struct {
	Enabled       bool              `json:"enabled"`
	DataPath      string            `json:"dataPath"`
	RemoteRoot    string            `json:"remoteRoot"`
	TokenFile     string            `json:"tokenFile"`
	TotalFiles    int               `json:"totalFiles"`
	TotalBytes    int64             `json:"totalBytes"`
	DatabaseBytes int64             `json:"databaseBytes"`
	PaperPDFCount int               `json:"paperPdfCount"`
	PaperPDFBytes int64             `json:"paperPdfBytes"`
	OtherFiles    int               `json:"otherFiles"`
	Files         []SyncPreviewFile `json:"files"`
	CheckedAt     time.Time         `json:"checkedAt" ts_type:"string"`
	Warning       string            `json:"warning,omitempty"`
}

type PDFServiceStatus struct {
	Enabled   bool              `json:"enabled"`
	URL       string            `json:"url"`
	Healthy   bool              `json:"healthy"`
	Ready     bool              `json:"ready"`
	Checks    map[string]string `json:"checks,omitempty"`
	CheckedAt time.Time         `json:"checkedAt" ts_type:"string"`
	Message   string            `json:"message,omitempty"`
}

type DatabaseRestoreStatus struct {
	Pending     bool      `json:"pending"`
	Applied     bool      `json:"applied"`
	StagedPath  string    `json:"stagedPath,omitempty"`
	BackupPath  string    `json:"backupPath,omitempty"`
	RemotePath  string    `json:"remotePath,omitempty"`
	ScheduledAt time.Time `json:"scheduledAt,omitempty" ts_type:"string"`
	AppliedAt   time.Time `json:"appliedAt,omitempty" ts_type:"string"`
	Message     string    `json:"message,omitempty"`
}

// FileInfo 文件信息
type FileInfo struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"isDir"`
	Modified time.Time `json:"modified" ts_type:"string"`
	MD5      string    `json:"md5,omitempty"`
}
