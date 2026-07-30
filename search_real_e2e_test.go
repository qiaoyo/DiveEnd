package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRealDeepStartRetrievalE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DIVEEND_REAL_SEARCH_E2E")) != "1" {
		t.Skip("set DIVEEND_REAL_SEARCH_E2E=1 to run against configured search and weak LLM providers")
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("load configured search: %v", err)
	}
	config = normalizeAppConfig(config)
	weak := NewWeakLLMClient(config)
	weak.tokenBudget = newDailyTokenBudget(t.TempDir(), defaultDailyLLMTokenBudget)

	app := NewApp()
	app.weakLLM = weak
	app.search = NewSearchClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	results, warning, stats, err := app.deepStartSearchWithRewrittenQueries(
		ctx,
		"LLM code agent benchmark",
		20,
		20,
	)
	if err != nil {
		t.Fatalf("real retrieval failed: %v", err)
	}
	if len(results) < 5 {
		t.Fatalf("expected useful partial retrieval, got %d results (warning present=%t)", len(results), warning != "")
	}
	if len(stats.RewrittenQueries) < 2 || len(stats.QueryHits) != len(stats.RewrittenQueries) {
		t.Fatalf("expected every rewritten query to be attempted, got %+v", stats)
	}
	if results[0].ID == "" || strings.TrimSpace(results[0].Title) == "" {
		t.Fatalf("expected a ranked paper at the top, got %+v", results[0])
	}
	relevantTopResults := 0
	for index, paper := range results {
		if index >= 5 {
			break
		}
		if strings.TrimSpace(paper.MatchReason) == "" {
			t.Fatalf("rank %d is missing a retrieval explanation: %+v", index+1, paper)
		}
		haystack := strings.ToLower(paper.Title + " " + paper.Abstract)
		if strings.Contains(haystack, "code") ||
			strings.Contains(haystack, "software") ||
			strings.Contains(haystack, "agent") ||
			strings.Contains(haystack, "benchmark") {
			relevantTopResults++
		}
		t.Logf("rank %d: %s (%s)", index+1, paper.Title, paper.MatchReason)
	}
	if relevantTopResults < 4 {
		t.Fatalf("expected at least four of the top five papers to match the research topic, got %d", relevantTopResults)
	}
}
