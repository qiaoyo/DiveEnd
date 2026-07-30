package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRealDeepReadGroundingE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DIVEEND_REAL_DEEPREAD_E2E")) != "1" {
		t.Skip("set DIVEEND_REAL_DEEPREAD_E2E=1 to run grounded DeepRead against the configured strong LLM")
	}
	strongSeed, ok := readStrongLLMSeed()
	if !ok {
		t.Fatal("config/strong_llm.json is unavailable or incomplete")
	}
	client := newLLMClientFromConfig(llmConfigFromSeed(strongSeed))
	client.tokenBudget = newDailyTokenBudget(t.TempDir(), defaultDailyLLMTokenBudget)

	sectionContext := `[section-1] Results
Accuracy improved by 12 percent on ExampleBench after adding retrieval.

[section-2] Limitations
The evaluation only covers English programming tasks.`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	response, err := client.AnswerDeepReadWithContext(
		ctx,
		"Example Retrieval Agent",
		"question",
		"加入检索模块带来了什么结果？",
		sectionContext,
	)
	if err != nil {
		t.Fatalf("real DeepRead request failed: %v", err)
	}
	if strings.TrimSpace(response.Answer) == "" {
		t.Fatal("real DeepRead response is empty")
	}
	if len(response.Evidence) == 0 {
		t.Fatalf("expected at least one verbatim grounded evidence item, got %+v", response)
	}
	for _, evidence := range response.Evidence {
		section, ok := parseDeepReadContextSections(sectionContext)[evidence.SectionID]
		if !ok || !strings.Contains(normalizeEvidenceText(section.content), normalizeEvidenceText(evidence.Excerpt)) {
			t.Fatalf("response retained ungrounded evidence: %+v", evidence)
		}
	}
}

func TestRealDeepReadFastQuestionE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DIVEEND_REAL_DEEPREAD_E2E")) != "1" {
		t.Skip("set DIVEEND_REAL_DEEPREAD_E2E=1 to run grounded DeepRead against the configured weak LLM")
	}
	weakSeed, ok := readWeakLLMSeed()
	if !ok {
		t.Fatal("config/weak_llm.json is unavailable or incomplete")
	}
	client := newLLMClientFromConfig(llmConfigFromSeed(weakSeed))
	client.tokenBudget = newDailyTokenBudget(t.TempDir(), defaultDailyLLMTokenBudget)
	sectionContext := `[section-1] Results
Accuracy improved by 12 percent on ExampleBench after adding retrieval.

[section-2] Limitations
The evaluation only covers English programming tasks.`

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	startedAt := time.Now()
	response, err := client.AnswerDeepReadWithContext(
		ctx,
		"Example Retrieval Agent",
		"question",
		"加入检索模块带来了什么结果？",
		sectionContext,
	)
	if err != nil {
		t.Fatalf("real fast DeepRead request failed: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed >= time.Minute {
		t.Fatalf("interactive DeepRead exceeded latency budget: %v", elapsed)
	}
	if len(response.Evidence) == 0 {
		t.Fatalf("expected grounded evidence from weak model, got %+v", response)
	}
}
