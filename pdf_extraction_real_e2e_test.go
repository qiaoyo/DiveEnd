package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRealPDFExtractionBudgetE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DIVEEND_REAL_PDF_EXTRACTION_E2E")) != "1" {
		t.Skip("set DIVEEND_REAL_PDF_EXTRACTION_E2E=1 and DIVEEND_REAL_PDF_PATH to run real PDF extraction")
	}
	pdfPath := strings.TrimSpace(os.Getenv("DIVEEND_REAL_PDF_PATH"))
	if pdfPath == "" {
		t.Fatal("DIVEEND_REAL_PDF_PATH is required")
	}
	weakSeed, ok := readWeakLLMSeed()
	if !ok {
		t.Fatal("config/weak_llm.json is unavailable or incomplete")
	}

	client := NewPDFServiceClient(defaultPDFServiceURL())
	manager, err := newManagedPDFService(client)
	if err != nil {
		t.Fatalf("create managed PDF service: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := manager.EnsureRunning(ctx); err != nil {
		t.Fatalf("start managed PDF service: %v", err)
	}
	t.Cleanup(manager.Stop)

	parseResult, err := client.ParsePDFWithContext(ctx, pdfPath)
	if err != nil {
		t.Fatalf("parse real PDF: %v", err)
	}
	if strings.TrimSpace(parseResult.Markdown) == "" {
		t.Fatal("real PDF parse returned empty markdown")
	}

	config := defaultAppConfig()
	config.WeakLLM = llmConfigFromSeed(weakSeed)
	extractionConfig := pdfExtractionLLMConfigForApp(config)
	app := NewApp()
	app.llmTokenBudget = newDailyTokenBudget(t.TempDir(), defaultDailyLLMTokenBudget)
	reservation, err := app.reservePDFExtractionBudget(parseResult.Markdown, extractionConfig)
	if err != nil {
		t.Fatalf("reserve extraction budget: %v", err)
	}

	extractResult, err := client.ExtractContentWithContext(ctx, parseResult.Markdown, extractionConfig)
	if err != nil {
		reservation.Finish(0)
		t.Fatalf("extract real PDF: %v", err)
	}
	if extractResult.Data == nil || strings.TrimSpace(extractResult.Data.Metadata.Title) == "" {
		t.Fatalf("real extraction returned incomplete data: %+v", extractResult)
	}
	if extractResult.Usage.TotalTokens <= 0 {
		t.Fatalf("provider did not report token usage: %+v", extractResult.Usage)
	}
	reservation.Finish(extractResult.Usage.TotalTokens)
	if snapshot := app.llmTokenBudget.Snapshot(); snapshot.UsedTokens != extractResult.Usage.TotalTokens {
		t.Fatalf("budget did not settle to provider usage: usage=%+v snapshot=%+v", extractResult.Usage, snapshot)
	}
}
