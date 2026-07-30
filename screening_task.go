package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrScreeningTaskCancelled = errors.New("screening task cancelled")

type screeningTaskHandle struct {
	token  string
	cancel context.CancelFunc
	done   chan struct{}
}

func (a *App) beginScreeningTask(sessionID string) (context.Context, string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, "", fmt.Errorf("sessionID is required")
	}
	base := a.ctx
	if base == nil {
		base = context.Background()
	}

	a.screeningTaskMu.Lock()
	defer a.screeningTaskMu.Unlock()
	if existing, ok := a.screeningTasks[sessionID]; ok && existing.cancel != nil {
		return nil, "", fmt.Errorf("a screening task is already running for this session")
	}

	ctx, cancel := context.WithCancel(base)
	token := uuid.NewString()
	a.screeningTasks[sessionID] = screeningTaskHandle{
		token:  token,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	return ctx, token, nil
}

func (a *App) finishScreeningTask(sessionID, token string) {
	a.screeningTaskMu.Lock()
	handle, ok := a.screeningTasks[sessionID]
	if ok && handle.token == token {
		delete(a.screeningTasks, sessionID)
		close(handle.done)
	}
	a.screeningTaskMu.Unlock()
}

func (a *App) CancelScreeningTask(sessionID string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("sessionID is required")
	}

	a.screeningTaskMu.Lock()
	handle, ok := a.screeningTasks[sessionID]
	a.screeningTaskMu.Unlock()
	if ok && handle.cancel != nil {
		handle.cancel()
	}
	return nil
}

func (a *App) cancelScreeningTaskAndWait(sessionID string, timeout time.Duration) bool {
	a.screeningTaskMu.Lock()
	handle, ok := a.screeningTasks[sessionID]
	a.screeningTaskMu.Unlock()
	if !ok {
		return true
	}
	if handle.cancel != nil {
		handle.cancel()
	}
	if timeout <= 0 {
		return false
	}
	select {
	case <-handle.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (a *App) cancelAllScreeningTasks() {
	a.screeningTaskMu.Lock()
	handles := make([]screeningTaskHandle, 0, len(a.screeningTasks))
	for _, handle := range a.screeningTasks {
		handles = append(handles, handle)
	}
	a.screeningTaskMu.Unlock()
	for _, handle := range handles {
		if handle.cancel != nil {
			handle.cancel()
		}
	}

	deadline := time.After(10 * time.Second)
	for _, handle := range handles {
		if handle.done == nil {
			continue
		}
		select {
		case <-handle.done:
		case <-deadline:
			return
		}
	}
}

func isScreeningTaskCancelled(err error) bool {
	return errors.Is(err, ErrScreeningTaskCancelled) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}
