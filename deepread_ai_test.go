package main

import (
	"strings"
	"testing"
)

func TestBuildDeepReadAIContextPrioritizesSelectedAndCoreSections(t *testing.T) {
	sections := []DeepReadSection{
		{ID: "section-1", Title: "Related Work", Content: strings.Repeat("related ", 20), Index: 0},
		{ID: "section-2", Title: "Methods", Content: "selected method details", Index: 1},
		{ID: "section-3", Title: "Abstract", Content: "abstract details", Index: 2},
		{ID: "section-4", Title: "Appendix", Content: strings.Repeat("appendix ", 20), Index: 3},
	}

	contextText := buildDeepReadAIContext(sections, "section-2", 220)
	selectedIndex := strings.Index(contextText, "[section-2] Methods")
	abstractIndex := strings.Index(contextText, "[section-3] Abstract")
	if selectedIndex != 0 {
		t.Fatalf("expected selected section first, got context %q", contextText)
	}
	if abstractIndex <= selectedIndex {
		t.Fatalf("expected abstract after selected section, got context %q", contextText)
	}
	if len([]rune(contextText)) > 220 {
		t.Fatalf("expected context to respect rune budget, got %d", len([]rune(contextText)))
	}
}

func TestBuildDeepReadAIContextUsesStableFallbackSectionID(t *testing.T) {
	contextText := buildDeepReadAIContext([]DeepReadSection{
		{Title: "Conclusion", Content: "final result", Index: 4},
	}, "", 200)

	if !strings.Contains(contextText, "[section-5] Conclusion") {
		t.Fatalf("expected generated stable section id, got %q", contextText)
	}
}
