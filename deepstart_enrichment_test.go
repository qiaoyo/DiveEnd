package main

import (
	"strings"
	"testing"
)

func TestBuildPaperKeywordsFiltersIntentSentence(t *testing.T) {
	paper := SearchPaper{
		Title:    "Embodied Agent Benchmark",
		Abstract: "This paper studies embodied agent evaluation and benchmark design.",
	}

	keywords := buildPaperKeywords(
		paper,
		"我想梳理这两年具身智能领域的论文",
		[]string{"我想梳理这两年具身智能领域的论文", "embodied", "benchmark"},
	)

	for _, keyword := range keywords {
		if strings.Contains(keyword, "我想梳理这两年具身智能领域的论文") {
			t.Fatalf("unexpected intent sentence in keyword list: %+v", keywords)
		}
	}
	if !containsString(keywords, "embodied") || !containsString(keywords, "benchmark") {
		t.Fatalf("expected clean keywords to remain, got %+v", keywords)
	}
}

func TestBuildPaperKeywordsDoesNotFallbackToUserQuery(t *testing.T) {
	paper := SearchPaper{
		Title:    "Vision Language Action Models",
		Abstract: "A survey of VLA models and robotics policy learning.",
	}
	keywords := buildPaperKeywords(
		paper,
		"I want to sort out the papers in the field of embodied intelligence over the past two years.",
		nil,
	)

	joined := strings.ToLower(strings.Join(keywords, " "))
	if strings.Contains(joined, "sort out the papers") {
		t.Fatalf("keywords should not include raw user intent sentence: %+v", keywords)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

