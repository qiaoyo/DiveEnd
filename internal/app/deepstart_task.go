package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrDeepStartTaskCancelled = errors.New("deepstart task cancelled")

type deepStartTaskHandle struct {
	cancel context.CancelFunc
	token  string
}

func isDeepStartCancelledError(err error) bool {
	return errors.Is(err, ErrDeepStartTaskCancelled) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

func (a *App) beginDeepStartTask(sessionID string) (context.Context, string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, "", fmt.Errorf("session ID cannot be empty")
	}

	a.deepStartTaskMu.Lock()
	defer a.deepStartTaskMu.Unlock()

	if existing, exists := a.deepStartTasks[sessionID]; exists && existing.cancel != nil {
		existing.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	token := uuid.NewString()
	a.deepStartTasks[sessionID] = deepStartTaskHandle{cancel: cancel, token: token}
	return ctx, token, nil
}

func (a *App) finishDeepStartTask(sessionID string, token string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	a.deepStartTaskMu.Lock()
	if current, ok := a.deepStartTasks[sessionID]; ok && current.token == token {
		delete(a.deepStartTasks, sessionID)
	}
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
	handle, exists := a.deepStartTasks[sessionID]
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

	if handle.cancel != nil {
		handle.cancel()
	}
	return nil
}

func (a *App) cancelAllDeepStartTasks() {
	a.deepStartTaskMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(a.deepStartTasks))
	for _, handle := range a.deepStartTasks {
		if handle.cancel != nil {
			cancels = append(cancels, handle.cancel)
		}
	}
	a.deepStartTasks = map[string]deepStartTaskHandle{}
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
