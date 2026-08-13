package app

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func appContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func translateSectionWithContext(ctx context.Context, service interface {
	TranslateSection(section, originalText string) (string, string, error)
}, section, text string) (string, string, error) {
	ctx = appContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	if serviceWithContext, ok := service.(interface {
		TranslateSectionWithContext(context.Context, string, string) (string, string, error)
	}); ok {
		return serviceWithContext.TranslateSectionWithContext(ctx, section, text)
	}
	return service.TranslateSection(section, text)
}

func extractPaperProfileWithContext(ctx context.Context, service interface {
	ExtractPaperProfile(markdown string) (*PaperProfileExtraction, error)
}, markdown string) (*PaperProfileExtraction, error) {
	ctx = appContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if serviceWithContext, ok := service.(interface {
		ExtractPaperProfileWithContext(context.Context, string) (*PaperProfileExtraction, error)
	}); ok {
		return serviceWithContext.ExtractPaperProfileWithContext(ctx, markdown)
	}
	return service.ExtractPaperProfile(markdown)
}

func analyzeDeepStartWithContext(ctx context.Context, service interface {
	AnalyzeDeepStart(request DeepStartAIRequest) (*DeepStartAIResponse, error)
}, request DeepStartAIRequest) (*DeepStartAIResponse, error) {
	ctx = appContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if serviceWithContext, ok := service.(interface {
		AnalyzeDeepStartWithContext(context.Context, DeepStartAIRequest) (*DeepStartAIResponse, error)
	}); ok {
		return serviceWithContext.AnalyzeDeepStartWithContext(ctx, request)
	}
	return service.AnalyzeDeepStart(request)
}

func analyzeDeepStartWithRetry(ctx context.Context, service interface {
	AnalyzeDeepStart(DeepStartAIRequest) (*DeepStartAIResponse, error)
}, request DeepStartAIRequest) (*DeepStartAIResponse, error) {
	const maxAttempts = 2
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		response, err := analyzeDeepStartWithContext(ctx, service, request)
		if err == nil {
			if response == nil {
				return nil, fmt.Errorf("AI returned an empty DeepStart analysis")
			}
			return response, nil
		}
		lastErr = err
		if ctx == nil || ctx.Err() != nil || attempt == maxAttempts || !isRetryableLLMError(err) {
			break
		}
		if err := sleepWithContext(ctx, 700*time.Millisecond); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

func isRetryableLLMError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"429", "rate limit", "timeout", "timed out", "deadline exceeded",
		"connection reset", "connection refused", "temporarily unavailable",
		"service unavailable", "status 500", "status 502", "status 503", "status 504",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func analyzeScreeningWithContext(ctx context.Context, service interface {
	AnalyzeScreening(request ScreeningAIRequest) (*ScreeningDecisionNode, error)
}, request ScreeningAIRequest) (*ScreeningDecisionNode, error) {
	ctx = appContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if serviceWithContext, ok := service.(interface {
		AnalyzeScreeningWithContext(context.Context, ScreeningAIRequest) (*ScreeningDecisionNode, error)
	}); ok {
		return serviceWithContext.AnalyzeScreeningWithContext(ctx, request)
	}
	return service.AnalyzeScreening(request)
}
