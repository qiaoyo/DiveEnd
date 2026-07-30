package main

import (
	"context"
	"testing"
)

func TestDeepReadAITaskReplacementAndCancellation(t *testing.T) {
	app := NewApp()

	firstContext, firstToken := app.beginDeepReadAI()
	secondContext, secondToken := app.beginDeepReadAI()

	if firstContext.Err() != context.Canceled {
		t.Fatal("starting a replacement task did not cancel the previous request")
	}
	if firstToken == secondToken {
		t.Fatal("replacement task reused the previous token")
	}

	app.finishDeepReadAI(firstToken)
	if !app.cancelDeepReadAI() {
		t.Fatal("finishing the previous task cleared the active replacement")
	}
	select {
	case <-secondContext.Done():
	default:
		t.Fatal("explicit cancellation did not cancel the active request")
	}
	if app.cancelDeepReadAI() {
		t.Fatal("cancelling an empty task should report false")
	}
}
