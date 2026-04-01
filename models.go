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
	DeepStartSessions      []DeepStartSessionSummary `json:"deepStartSessions"`
	ActiveDeepStartSession *DeepStartSessionDetail   `json:"activeDeepStartSession,omitempty"`
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
