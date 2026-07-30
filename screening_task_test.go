package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

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
