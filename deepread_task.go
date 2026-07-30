package main

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var errDeepReadAICancelled = errors.New("deepread AI request cancelled")

type deepReadAITaskHandle struct {
	token  string
	cancel context.CancelFunc
}

func (a *App) beginDeepReadAI() (context.Context, string) {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	token := uuid.NewString()

	a.deepReadAITaskMu.Lock()
	previous := a.deepReadAITask.cancel
	a.deepReadAITask = deepReadAITaskHandle{token: token, cancel: cancel}
	a.deepReadAITaskMu.Unlock()

	if previous != nil {
		previous()
	}
	return ctx, token
}

func (a *App) finishDeepReadAI(token string) {
	a.deepReadAITaskMu.Lock()
	if a.deepReadAITask.token == token {
		a.deepReadAITask = deepReadAITaskHandle{}
	}
	a.deepReadAITaskMu.Unlock()
}

func (a *App) cancelDeepReadAI() bool {
	a.deepReadAITaskMu.Lock()
	cancel := a.deepReadAITask.cancel
	a.deepReadAITask = deepReadAITaskHandle{}
	a.deepReadAITaskMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (a *App) CancelDeepReadAI() bool {
	return a.cancelDeepReadAI()
}
