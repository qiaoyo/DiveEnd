package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrDeepStartTaskCancelled = errors.New("deepstart task cancelled")

func isDeepStartCancelledError(err error) bool {
	return errors.Is(err, ErrDeepStartTaskCancelled) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

func (a *App) beginDeepStartTask(sessionID string) (context.Context, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	a.deepStartTaskMu.Lock()
	defer a.deepStartTaskMu.Unlock()

	if _, exists := a.deepStartTasks[sessionID]; exists {
		return nil, fmt.Errorf("deepstart task already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.deepStartTasks[sessionID] = cancel
	return ctx, nil
}

func (a *App) finishDeepStartTask(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	a.deepStartTaskMu.Lock()
	delete(a.deepStartTasks, sessionID)
	a.deepStartTaskMu.Unlock()
}

func (a *App) CancelDeepStartTask(sessionID string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	a.deepStartTaskMu.Lock()
	cancel, exists := a.deepStartTasks[sessionID]
	a.deepStartTaskMu.Unlock()
	if !exists {
		return nil
	}

	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "cancelling",
		Message:                   "正在停止本次探索并回滚暂存结果",
		ElapsedSeconds:            0,
		EstimatedRemainingSeconds: 1,
		Total:                     0,
		Completed:                 0,
		OverallPercent:            0,
	})

	cancel()
	return nil
}

func (a *App) cancelAllDeepStartTasks() {
	a.deepStartTaskMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(a.deepStartTasks))
	for _, cancel := range a.deepStartTasks {
		cancels = append(cancels, cancel)
	}
	a.deepStartTasks = map[string]context.CancelFunc{}
	a.deepStartTaskMu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (a *App) abortDeepStartIfCancelled(
	ctx context.Context,
	sessionID string,
	startedAt time.Time,
	stats *SearchRetrievalStats,
	message string,
) error {
	if ctx == nil || ctx.Err() == nil {
		return nil
	}

	progress := DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "cancelled",
		Message:                   strings.TrimSpace(message),
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     0,
		Completed:                 0,
		OverallPercent:            0,
		Stats:                     stats,
	}
	if progress.Message == "" {
		progress.Message = "本次探索已停止，所有暂存结果已撤销"
	}
	a.emitDeepStartProgress(progress)
	return ErrDeepStartTaskCancelled
}
