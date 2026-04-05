package main

import "time"

const defaultFolderName = "Inbox"

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
	SemanticScholarAPIKey      string `json:"semanticScholarApiKey,omitempty"`
	HasSemanticScholarAPIKey   bool   `json:"hasSemanticScholarApiKey"`
	ClearSemanticScholarAPIKey bool   `json:"clearSemanticScholarApiKey,omitempty"`
}

type BaiduCloudConfig struct {
	Enabled    bool   `json:"enabled"`
	Token      string `json:"token,omitempty"`
	HasToken   bool   `json:"hasToken"`
	Quota      int    `json:"quota"`
	ClearToken bool   `json:"clearToken,omitempty"`
}

type AppConfig struct {
	LLM                   LLMConfig        `json:"llm"`
	Search                SearchAPIConfig  `json:"search"`
	BaiduCloud            BaiduCloudConfig `json:"baiduCloud"`
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

type InitialState struct {
	Config                 AppConfig                 `json:"config"`
	Folders                []Folder                  `json:"folders"`
	Papers                 []Paper                   `json:"papers"`
	ActiveFolderID         string                    `json:"activeFolderId"`
	DeepStartSessions      []DeepStartSessionSummary `json:"deepstartSessions"`
	ActiveDeepStartSession *DeepStartSessionDetail   `json:"activeDeepStartSession,omitempty"`
}

type ScreeningPaper struct {
	ID           string    `json:"id"`
	SessionID     string    `json:"sessionId"`
	FileName      string    `json:"fileName"`
	FilePath      string    `json:"filePath"`
	FileSize      int64     `json:"fileSize"`
	Status        string    `json:"status"` // "pending", "extracting", "extracted", "screening", "selected", "rejected"
	Title         string    `json:"title,omitempty"`
	Authors       string    `json:"authors,omitempty"`
	Abstract      string    `json:"abstract,omitempty"`
	FullText      string    `json:"fullText,omitempty"`
	SectionsJSON  string    `json:"sectionsJson,omitempty"`
	Selection     string    `json:"selection,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	TargetFolderID string    `json:"targetFolderId,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type ScreeningSession struct {
	ID               string    `json:"id"`
	Title             string    `json:"title"`
	Status            string    `json:"status"` // "upload", "extract", "screen", "complete"
	TotalPapers       int       `json:"totalPapers"`
	CurrentNodeJSON   string   `json:"currentNodeJson,omitempty"`
	SelectedOptionsJSON string   `json:"selectedOptionsJson,omitempty"`
	PathHistoryJSON   string    `json:"pathHistoryJson,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type ScreeningDecisionNode struct {
	ID               string    `json:"id"`
	Message          string    `json:"message"`
	Dimension        string    `json:"dimension"`
	OptionsJSON      string    `json:"optionsJson"`
	AllowMultiSelect  bool      `json:"allowMultiSelect"`
	AllowSkip       bool      `json:"allowSkip"`
	RemainingPaperIDsJSON string `json:"remainingPaperIdsJson,omitempty"`
}

type ExtractProgress struct {
	SessionID   string    `json:"sessionId"`
	Total       int       `json:"total"`
	Completed   int       `json:"completed"`
	Current     string    `json:"current"`
	Status      string    `json:"status"` // "processing", "completed", "error"
}

type ScreeningSessionDetail struct {
	Session       ScreeningSession        `json:"session"`
	Papers        []ScreeningPaper       `json:"papers"`
	CurrentNode   *ScreeningDecisionNode `json:"currentNode,omitempty"`
	PathHistory    []PathHistoryItem       `json:"pathHistory"`
}

type PathHistoryItem struct {
	Dimension string   `json:"dimension"`
	Choice   string   `json:"choice"`
}

type Folder struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Paper struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Authors   string    `json:"authors"`
	Abstract  string    `json:"abstract"`
	Year      int       `json:"year"`
	Journal   string    `json:"journal"`
	URL       string    `json:"url"`
	PDFPath   string    `json:"pdfPath,omitempty"`
	FolderID  string    `json:"folderId"`
	Category  string    `json:"category"`
	Tags      []string  `json:"tags"`
	AddedAt   time.Time `json:"addedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SearchPaper struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Authors  string   `json:"authors"`
	Abstract string   `json:"abstract"`
	Year     int      `json:"year"`
	Journal  string   `json:"journal"`
	URL      string   `json:"url"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

type DeepStartSessionSummary struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	RootPrompt     string    `json:"rootPrompt"`
	CurrentQuery   string    `json:"currentQuery"`
	TargetFolderID string    `json:"targetFolderId"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type DeepStartMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
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
}

type DeepStartSessionDetail struct {
	Summary          DeepStartSessionSummary `json:"summary"`
	Messages         []DeepStartMessage      `json:"messages"`
	CurrentResults   []SearchPaper           `json:"currentResults"`
	CurrentAnalysis  *DeepStartAnalysis      `json:"currentAnalysis,omitempty"`
	SelectedPaperIDs []string                `json:"selectedPaperIds"`
}

type TranslationRecord struct {
	ID             string    `json:"id"`
	PaperID        string    `json:"paperId"`
	Section        string    `json:"section"`
	OriginalText   string    `json:"originalText"`
	TranslatedText string    `json:"translatedText"`
	Summary        string    `json:"summary"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ========== Phase 6: Baidu Cloud Sync 相关数据模型 ==========

// BaiduToken 百度网盘 OAuth token
type BaiduToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ClientID    string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	ExpiresIn   int    `json:"expires_in"`
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
	ErrorMessage string    `json:"errorMessage,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	CompletedAt  time.Time `json:"completedAt,omitempty"`
}

// SyncConflict 同步冲突
type SyncConflict struct {
	ID          string    `json:"id"`
	FileName    string    `json:"fileName"`
	LocalPath   string    `json:"localPath"`
	LocalTime   time.Time `json:"localTime"`
	RemotePath  string    `json:"remotePath"`
	RemoteTime  time.Time `json:"remoteTime"`
	Resolution  string    `json:"resolution"` // "local", "remote", "skipped"
	ResolvedAt  time.Time `json:"resolvedAt,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// SyncStatus 同步状态
type SyncStatus struct {
	Enabled         bool       `json:"enabled"`
	Provider        string     `json:"provider"`
	LastSync        *time.Time  `json:"lastSync,omitempty"`
	SyncInProgress  bool       `json:"syncInProgress"`
	PendingFiles    int        `json:"pendingFiles"`
	Conflicts       int        `json:"conflicts"`
	TotalSynced     int        `json:"totalSynced"`
	TotalFailed     int        `json:"totalFailed"`
}

// SyncSettings 同步设置
type SyncSettings struct {
	AutoSync           bool   `json:"autoSync"`
	SyncOnStartup      bool   `json:"syncOnStartup"`
	SyncInterval       int    `json:"syncInterval"` // minutes
	ConflictResolution string `json:"conflictResolution"` // "timestamp" (always use newest)
}

// SyncProgress 同步进度
type SyncProgress struct {
	Total        int            `json:"total"`
	Completed    int            `json:"completed"`
	CurrentFile  string         `json:"currentFile"`
	Status       string         `json:"status"` // "preparing", "uploading", "downloading", "resolving", "complete", "error"
	Message      string         `json:"message,omitempty"`
}

// FileInfo 文件信息
type FileInfo struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"isDir"`
	Modified time.Time `json:"modified"`
	MD5      string    `json:"md5,omitempty"`
}
