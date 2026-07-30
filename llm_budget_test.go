package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDailyTokenBudgetReservesSettlesAndPersists(t *testing.T) {
	dataPath := t.TempDir()
	budget := newDailyTokenBudget(dataPath, 100)

	reservation, err := budget.Reserve([]llmMessage{{Role: "user", Content: "0123456789"}}, 20)
	if err != nil {
		t.Fatalf("reserve budget: %v", err)
	}
	reservation.Finish(7)

	snapshot := budget.Snapshot()
	if snapshot.UsedTokens != 7 || snapshot.Remaining != 93 {
		t.Fatalf("unexpected snapshot after settlement: %+v", snapshot)
	}

	reloaded := newDailyTokenBudget(dataPath, 100)
	reloadedSnapshot := reloaded.Snapshot()
	if reloadedSnapshot.UsedTokens != 7 {
		t.Fatalf("expected persisted usage, got %+v", reloadedSnapshot)
	}
}

func TestDailyTokenBudgetCancelsFailedRequestAndRejectsOverflow(t *testing.T) {
	budget := newDailyTokenBudget(t.TempDir(), 10)
	reservation, err := budget.Reserve(nil, 8)
	if err != nil {
		t.Fatalf("reserve budget: %v", err)
	}
	reservation.Cancel()
	if snapshot := budget.Snapshot(); snapshot.UsedTokens != 0 {
		t.Fatalf("expected cancellation to release reservation, got %+v", snapshot)
	}

	_, err = budget.Reserve([]llmMessage{{Role: "user", Content: strings.Repeat("x", 40)}}, 1)
	if err == nil {
		t.Fatal("expected request over daily budget to be rejected")
	}
}

func TestDailyTokenBudgetSettlesRequestThatCrossesDateBoundary(t *testing.T) {
	budget := newDailyTokenBudget(t.TempDir(), 100)
	reservation := &tokenReservation{
		budget:   budget,
		reserved: 20,
		date:     "2000-01-01",
	}
	reservation.Finish(7)

	if snapshot := budget.Snapshot(); snapshot.UsedTokens != 7 {
		t.Fatalf("expected new-day usage without subtracting old reservation, got %+v", snapshot)
	}
}

func TestEstimateTextTokensTreatsNonASCIIConservatively(t *testing.T) {
	if got := estimateTextTokens("论文检索"); got < 4 {
		t.Fatalf("expected at least one token per non-ASCII rune, got %d", got)
	}
}

func TestPDFExtractionSharesDailyTokenBudget(t *testing.T) {
	app := NewApp()
	app.llmTokenBudget = newDailyTokenBudget(t.TempDir(), 100_000)

	reservation, err := app.reservePDFExtractionBudget(
		strings.Repeat("paper ", 100),
		PDFExtractionLLMConfig{Provider: "openai", Model: "test", APIKey: "key", MaxTokens: 100},
	)
	if err != nil {
		t.Fatalf("reserve PDF extraction budget: %v", err)
	}
	if snapshot := app.llmTokenBudget.Snapshot(); snapshot.UsedTokens <= 200 {
		t.Fatalf("expected input and two output reservations, got %+v", snapshot)
	}
	reservation.Finish(50)
	if snapshot := app.llmTokenBudget.Snapshot(); snapshot.UsedTokens != 50 {
		t.Fatalf("expected provider usage settlement, got %+v", snapshot)
	}
}

func TestLLMFailureKeepsConservativeReservation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream failed", http.StatusBadGateway)
	}))
	defer server.Close()

	budget := newDailyTokenBudget(t.TempDir(), 20_000)
	client := &LLMClient{
		model:        "test-model",
		providerName: "test-provider",
		baseURL:      server.URL,
		wireAPI:      "chat_completions",
		httpClient:   server.Client(),
		tokenBudget:  budget,
	}
	if _, err := client.chat([]llmMessage{{Role: "user", Content: "test"}}); err == nil {
		t.Fatal("expected upstream failure")
	}
	if snapshot := budget.Snapshot(); snapshot.UsedTokens < 8192 {
		t.Fatalf("expected failed sent request to retain conservative reservation, got %+v", snapshot)
	}
}
