package contracts

import (
	"context"

	"github.com/qiaoyo/DiveEnd/internal/domain"
)

// LLMService is the strong-model port used by application workflows.
type LLMService interface {
	TranslateSection(section, originalText string) (translated string, summary string, err error)
	AnalyzeDeepStart(request domain.DeepStartAIRequest) (*domain.DeepStartAIResponse, error)
	AnalyzeScreening(request domain.ScreeningAIRequest) (*domain.ScreeningDecisionNode, error)
}

// WeakLLMService is the low-latency model port used by discovery and reading.
type WeakLLMService interface {
	TranslateSection(section, originalText string) (translated string, summary string, err error)
	ExtractPaperProfile(markdown string) (*domain.PaperProfileExtraction, error)
}

// PaperSearchService is the retrieval port exposed to application workflows.
type PaperSearchService interface {
	Search(query string, limit int) ([]domain.SearchPaper, error)
	SearchWithContext(ctx context.Context, query string, limit int) ([]domain.SearchPaper, error)
	EnhancedSearch(query string, limit int, offset int, yearStart int, yearEnd int, sortBy string) (*domain.EnhancedSearchResult, error)
	LastSearchStats() domain.SearchRetrievalStats
}

// ContextLLMService is the cancellable strong-model port.
type ContextLLMService interface {
	TranslateSectionWithContext(ctx context.Context, section, originalText string) (translated string, summary string, err error)
	AnalyzeDeepStartWithContext(ctx context.Context, request domain.DeepStartAIRequest) (*domain.DeepStartAIResponse, error)
	AnalyzeScreeningWithContext(ctx context.Context, request domain.ScreeningAIRequest) (*domain.ScreeningDecisionNode, error)
}

// ContextWeakLLMService is the cancellable weak-model port.
type ContextWeakLLMService interface {
	TranslateSectionWithContext(ctx context.Context, section, originalText string) (translated string, summary string, err error)
	ExtractPaperProfileWithContext(ctx context.Context, markdown string) (*domain.PaperProfileExtraction, error)
}

// QueryRewriter is the optional synchronous query-rewrite capability.
type QueryRewriter interface {
	RewriteSearchQueries(query string) ([]string, error)
}

// ContextQueryRewriter is the cancellable query-rewrite capability.
type ContextQueryRewriter interface {
	RewriteSearchQueriesWithContext(ctx context.Context, query string) ([]string, error)
}

// DeepReadAssistant is the model port for grounded paper questions.
type DeepReadAssistant interface {
	AnswerDeepReadWithContext(
		ctx context.Context,
		paperTitle string,
		mode string,
		question string,
		sectionContext string,
	) (*domain.DeepReadAIResponse, error)
}
