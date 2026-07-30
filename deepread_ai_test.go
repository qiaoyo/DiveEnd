package main

import (
	"context"
	"strings"
	"testing"
)

type fakeDeepReadRoutingLLM struct {
	name string
}

func (f *fakeDeepReadRoutingLLM) TranslateSection(string, string) (string, string, error) {
	return "", "", nil
}

func (f *fakeDeepReadRoutingLLM) AnalyzeDeepStart(DeepStartAIRequest) (*DeepStartAIResponse, error) {
	return nil, nil
}

func (f *fakeDeepReadRoutingLLM) AnalyzeScreening(ScreeningAIRequest) (*ScreeningDecisionNode, error) {
	return nil, nil
}

func (f *fakeDeepReadRoutingLLM) ExtractPaperProfile(string) (*PaperProfileExtraction, error) {
	return nil, nil
}

func (f *fakeDeepReadRoutingLLM) AnswerDeepReadWithContext(
	context.Context,
	string,
	string,
	string,
	string,
) (*DeepReadAIResponse, error) {
	return &DeepReadAIResponse{Answer: f.name}, nil
}

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

func TestBuildDeepReadAIContextUsesQuestionRelevantLateSection(t *testing.T) {
	sections := []DeepReadSection{
		{ID: "section-1", Title: "Introduction", Content: strings.Repeat("background ", 4000), Index: 0},
		{ID: "section-2", Title: "Method", Content: strings.Repeat("architecture ", 4000), Index: 1},
		{ID: "section-3", Title: "Ablation Study", Content: "The ablation removes the retrieval module.", Index: 2},
	}

	contextText := buildDeepReadAIContextForTask(sections, "", "论文的消融实验说明了什么？", "question", 1200)
	ablationIndex := strings.Index(contextText, "[section-3] Ablation Study")
	introductionIndex := strings.Index(contextText, "[section-1] Introduction")
	if ablationIndex != 0 {
		t.Fatalf("expected question-relevant ablation section first, got %q", contextText)
	}
	if introductionIndex <= ablationIndex {
		t.Fatalf("expected core sections to remain available after relevant section, got %q", contextText)
	}
}

func TestBuildDeepReadAIContextKeepsConclusionAfterLongAbstract(t *testing.T) {
	sections := []DeepReadSection{
		{ID: "section-1", Title: "Abstract", Content: strings.Repeat("abstract ", 4000), Index: 0},
		{ID: "section-2", Title: "Methods", Content: strings.Repeat("method ", 4000), Index: 1},
		{ID: "section-3", Title: "Conclusion", Content: "The final conclusion remains visible.", Index: 2},
	}

	contextText := buildDeepReadAIContextForTask(sections, "", "", "summary", 1200)
	if !strings.Contains(contextText, "[section-3] Conclusion") ||
		!strings.Contains(contextText, "The final conclusion remains visible.") {
		t.Fatalf("expected fair section allocation to retain conclusion, got %q", contextText)
	}
	if !strings.Contains(contextText, "[content omitted]") {
		t.Fatalf("expected long sections to preserve head and tail with an omission marker, got %q", contextText)
	}
}

func TestGroundDeepReadEvidenceRejectsInventedOrParaphrasedEvidence(t *testing.T) {
	contextText := "[section-1] Results\nAccuracy improved by 12 percent on ExampleBench.\n\n" +
		"[section-2] Conclusion\nThe method reduces inference cost."
	evidence := []DeepReadEvidence{
		{SectionID: "section-1", SectionTitle: "Wrong title", Excerpt: "Accuracy improved by 12 percent"},
		{SectionID: "section-9", SectionTitle: "Invented", Excerpt: "Fabricated quote"},
		{SectionID: "section-2", SectionTitle: "Conclusion", Excerpt: "Inference is much cheaper"},
	}

	grounded := groundDeepReadEvidence(evidence, contextText)
	if len(grounded) != 1 {
		t.Fatalf("expected only verbatim evidence from a real section, got %+v", grounded)
	}
	if grounded[0].SectionID != "section-1" || grounded[0].SectionTitle != "Results" {
		t.Fatalf("expected canonical section metadata, got %+v", grounded[0])
	}
}

func TestDeepReadAssistantRoutesQuestionsToWeakAndSummariesToStrong(t *testing.T) {
	app := NewApp()
	strong := &fakeDeepReadRoutingLLM{name: "strong"}
	weak := &fakeDeepReadRoutingLLM{name: "weak"}
	app.strongLLM = strong
	app.llm = strong
	app.weakLLM = weak

	if got := app.deepReadAssistantForMode("question"); got != weak {
		t.Fatalf("expected weak model for interactive question, got %#v", got)
	}
	if got := app.deepReadAssistantForMode("summary"); got != strong {
		t.Fatalf("expected strong model for full summary, got %#v", got)
	}
}
