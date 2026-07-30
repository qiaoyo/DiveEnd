package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type blockingScreeningLLM struct {
	started chan struct{}
}

func (b *blockingScreeningLLM) TranslateSection(string, string) (string, string, error) {
	return "", "", nil
}

func (b *blockingScreeningLLM) AnalyzeDeepStart(DeepStartAIRequest) (*DeepStartAIResponse, error) {
	return nil, nil
}

func (b *blockingScreeningLLM) AnalyzeScreening(ScreeningAIRequest) (*ScreeningDecisionNode, error) {
	return nil, fmt.Errorf("context-aware screening expected")
}

func (b *blockingScreeningLLM) AnalyzeScreeningWithContext(
	ctx context.Context,
	_ ScreeningAIRequest,
) (*ScreeningDecisionNode, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestScreeningTaskCancellationIsSessionScoped(t *testing.T) {
	app := NewApp()

	firstContext, firstToken, err := app.beginScreeningTask("session-1")
	if err != nil {
		t.Fatalf("begin first task: %v", err)
	}
	if _, _, err := app.beginScreeningTask("session-1"); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected duplicate task rejection, got %v", err)
	}
	secondContext, secondToken, err := app.beginScreeningTask("session-2")
	if err != nil {
		t.Fatalf("begin second session task: %v", err)
	}

	app.db = &DB{}
	if err := app.CancelScreeningTask("session-1"); err != nil {
		t.Fatalf("cancel first task: %v", err)
	}
	if firstContext.Err() != context.Canceled {
		t.Fatal("first session context was not cancelled")
	}
	if secondContext.Err() != nil {
		t.Fatal("cancelling one session affected another session")
	}

	app.finishScreeningTask("session-1", firstToken)
	app.finishScreeningTask("session-2", secondToken)
}

func TestCancelScreeningTaskAndWaitObservesCompletion(t *testing.T) {
	app := NewApp()
	ctx, token, err := app.beginScreeningTask("session-1")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		<-ctx.Done()
		app.finishScreeningTask("session-1", token)
	}()

	startedAt := time.Now()
	if !app.cancelScreeningTaskAndWait("session-1", time.Second) {
		t.Fatal("expected task completion before timeout")
	}
	if time.Since(startedAt) >= time.Second {
		t.Fatal("wait did not observe task completion")
	}
}

func TestCancelScreeningTaskAndWaitReportsTimeout(t *testing.T) {
	app := NewApp()
	_, token, err := app.beginScreeningTask("session-1")
	if err != nil {
		t.Fatal(err)
	}
	if app.cancelScreeningTaskAndWait("session-1", time.Millisecond) {
		t.Fatal("expected timeout while task cleanup has not completed")
	}
	app.finishScreeningTask("session-1", token)
}

func TestCancelledScreeningChoiceLeavesPersistedDecisionUnchanged(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	llm := &blockingScreeningLLM{started: make(chan struct{}, 1)}
	app.strongLLM = llm
	app.llm = llm

	leftIDs := make([]string, 0, 8)
	rightIDs := make([]string, 0, 4)
	papers := make([]ScreeningPaper, 0, 12)
	for index := 0; index < 12; index++ {
		id := fmt.Sprintf("paper-%02d", index)
		if index < 8 {
			leftIDs = append(leftIDs, id)
		} else {
			rightIDs = append(rightIDs, id)
		}
		papers = append(papers, ScreeningPaper{
			ID:        id,
			SessionID: "session-1",
			FileName:  id + ".pdf",
			Status:    "screening",
		})
	}
	node := ScreeningDecisionNode{
		ID:                "original-node",
		NodeType:          "branch",
		Message:           "original decision",
		Dimension:         "topic",
		AllowMultiSelect:  true,
		RemainingPaperIDs: append(append([]string{}, leftIDs...), rightIDs...),
		Options: []ScreeningDecisionOption{
			{Key: "left", Label: "Left", PaperIDs: leftIDs, Count: len(leftIDs)},
			{Key: "right", Label: "Right", PaperIDs: rightIDs, Count: len(rightIDs)},
		},
	}
	nodeJSON, _ := json.Marshal(node)
	if err := app.db.UpsertScreeningSession(&ScreeningSession{
		ID:                  "session-1",
		Title:               "Atomic screening",
		Status:              "screen",
		TotalPapers:         len(papers),
		CurrentNodeJSON:     string(nodeJSON),
		SelectedOptionsJSON: "[]",
		PathHistoryJSON:     "[]",
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.db.BatchUpsertScreeningPapers(papers); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		_, err := app.ApplyScreeningChoice("session-1", []string{"left"})
		result <- err
	}()
	select {
	case <-llm.started:
	case <-time.After(time.Second):
		t.Fatal("screening choice did not reach the model")
	}
	if err := app.CancelScreeningTask("session-1"); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !isScreeningTaskCancelled(err) {
		t.Fatalf("expected cancellation, got %v", err)
	}

	detail, err := app.db.GetScreeningSessionDetail("session-1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.CurrentNode == nil || detail.CurrentNode.ID != "original-node" {
		t.Fatalf("current node changed after cancellation: %+v", detail.CurrentNode)
	}
	if len(detail.PathHistory) != 0 {
		t.Fatalf("path history changed after cancellation: %+v", detail.PathHistory)
	}
	for _, paper := range detail.Papers {
		if paper.Status != "screening" || paper.Selection != "" || paper.Reason != "" {
			t.Fatalf("paper changed after cancellation: %+v", paper)
		}
	}
}

func TestCancelledInitialScreeningLeavesPersistedStateUnchanged(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	llm := &blockingScreeningLLM{started: make(chan struct{}, 1)}
	app.strongLLM = llm
	app.llm = llm

	papers := make([]ScreeningPaper, 0, 6)
	for index := 0; index < 6; index++ {
		id := fmt.Sprintf("paper-%02d", index)
		papers = append(papers, ScreeningPaper{
			ID:        id,
			SessionID: "session-1",
			FileName:  id + ".pdf",
			Status:    "extracted",
			Abstract:  "A completed extraction ready for screening.",
		})
	}
	if err := app.db.UpsertScreeningSession(&ScreeningSession{
		ID:                  "session-1",
		Title:               "Initial atomic screening",
		Status:              "extract",
		TotalPapers:         len(papers),
		SelectedOptionsJSON: "[]",
		PathHistoryJSON:     "[]",
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.db.BatchUpsertScreeningPapers(papers); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		_, err := app.AnalyzePapers("session-1")
		result <- err
	}()
	select {
	case <-llm.started:
	case <-time.After(time.Second):
		t.Fatal("initial screening did not reach the model")
	}
	if err := app.CancelScreeningTask("session-1"); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !isScreeningTaskCancelled(err) {
		t.Fatalf("expected cancellation, got %v", err)
	}

	detail, err := app.db.GetScreeningSessionDetail("session-1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Session.Status != "extract" || detail.CurrentNode != nil {
		t.Fatalf("session changed after cancellation: %+v", detail.Session)
	}
	for _, paper := range detail.Papers {
		if paper.Status != "extracted" {
			t.Fatalf("paper changed after cancellation: %+v", paper)
		}
	}
}
