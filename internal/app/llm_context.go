package app

import "context"

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
