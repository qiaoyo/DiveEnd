package app

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

	// The default source set contains five providers and DeepStart can execute
	// up to three rewritten queries sequentially. Allow provider backoff and
	// partial-source degradation to finish without treating a slow source as a
	// failed product search.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

func TestRealDBLPSearchE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DIVEEND_REAL_DBLP_E2E")) != "1" {
		t.Skip("set DIVEEND_REAL_DBLP_E2E=1 to run against the live DBLP Search API")
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("load configured search: %v", err)
	}
	config = normalizeAppConfig(config)
	config.Search.EnableSemanticScholar = false
	config.Search.EnableArxiv = false
	config.Search.EnableOpenAlex = false
	config.Search.EnableOpenReview = false
	config.Search.EnableDBLP = true

	client := NewSearchClient(config)
	client.retryMax = 1
	client.retryInterval = 0
	client.attemptTimeout = 15 * time.Second
	client.overallTimeout = 20 * time.Second

	var finalProgress SearchProgressEvent
	client.SetProgressReporter(func(progress SearchProgressEvent) {
		if progress.Phase == "completed" {
			finalProgress = progress
		}
	})

	results, err := client.Search("language model agent planning", 5)
	if err != nil {
		t.Fatalf("live DBLP search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("live DBLP search returned no papers")
	}
	if len(finalProgress.Sources) != 1 || finalProgress.Sources[0].Name != searchSourceDBLP ||
		!finalProgress.Sources[0].Success || finalProgress.Sources[0].Status != "success" {
		t.Fatalf("expected successful DBLP progress, got %+v", finalProgress)
	}
	t.Logf("DBLP returned %d papers; top result: %s", len(results), results[0].Title)
}
