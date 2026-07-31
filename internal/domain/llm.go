package domain

// DeepStartAIRequest is the application-level input for research-map analysis.
type DeepStartAIRequest struct {
	RootPrompt       string
	CurrentQuery     string
	TargetFolderName string
	Messages         []DeepStartMessage
	Results          []SearchPaper
}

// DeepStartAIResponse is the structured result returned by the strong model.
type DeepStartAIResponse struct {
	Title    string
	Analysis DeepStartAnalysis
}

// ScreeningAIRequest is the application-level input for the screening tree.
type ScreeningAIRequest struct {
	SessionTitle string
	Papers       []ScreeningPaper
	PathHistory  []PathHistoryItem
}
