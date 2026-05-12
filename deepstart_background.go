package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (a *App) runDeepStartBackgroundProcessing(
	taskCtx context.Context,
	sessionID string,
	taskToken string,
	query string,
	backgroundResults []SearchPaper,
	searchStats SearchRetrievalStats,
) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	pending := append([]SearchPaper{}, backgroundResults...)
	if len(pending) == 0 {
		a.finishDeepStartTask(sessionID, taskToken)
		return
	}

	go func() {
		defer a.finishDeepStartTask(sessionID, taskToken)

		startedAt := time.Now()
		abortIfCancelled := func(message string) bool {
			if err := a.abortDeepStartIfCancelled(taskCtx, sessionID, startedAt, &searchStats, message); err != nil {
				_ = a.markDeepStartBackgroundComplete(sessionID, 0, false)
				return true
			}
			return false
		}

		if abortIfCancelled("后台补全已停止，本次补充未写入会话") {
			return
		}

		a.emitDeepStartProgress(DeepStartProgressEvent{
			SessionID:                 sessionID,
			Phase:                     "background_processing",
			Message:                   fmt.Sprintf("后台正在补全剩余 %d 篇论文", len(pending)),
			ElapsedSeconds:            0,
			EstimatedRemainingSeconds: estimateDeepStartETA(deepStartBatchStats{Total: len(pending), Completed: 0}),
			Total:                     len(pending),
			Completed:                 0,
			OverallPercent:            100,
			BackgroundCompleted:       0,
			Stats:                     &searchStats,
		})

		enriched := pending
		if len(enriched) > 0 {
			results, enrichErr := a.enrichDeepStartResults(taskCtx, sessionID, startedAt, query, enriched, searchStats)
			if isDeepStartCancelledError(enrichErr) || isDeepStartCancelledError(taskCtx.Err()) {
				abortIfCancelled("后台补全已停止，本次补充未写入会话")
				return
			}
			if enrichErr == nil {
				enriched = results
			}
		}
		if abortIfCancelled("后台补全已停止，本次补充未写入会话") {
			return
		}

		processed := enriched
		batch := deepStartBatchStats{Total: len(enriched), Completed: 0}
		if len(enriched) > 0 {
			results, batchStats, batchErr := a.preprocessDeepStartResults(taskCtx, sessionID, startedAt, enriched, searchStats)
			batch = batchStats
			if isDeepStartCancelledError(batchErr) || isDeepStartCancelledError(taskCtx.Err()) {
				abortIfCancelled("后台批处理已停止，本次补充未写入会话")
				return
			}
			if batchErr == nil {
				processed = results
			} else if len(results) > 0 {
				processed = results
			}
		}
		if abortIfCancelled("后台批处理已停止，本次补充未写入会话") {
			return
		}

		if err := a.mergeDeepStartBackgroundResults(sessionID, processed); err != nil {
			a.emitDeepStartProgress(DeepStartProgressEvent{
				SessionID:                 sessionID,
				Phase:                     "background_processing",
				Message:                   fmt.Sprintf("后台合并结果失败：%v", err),
				ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
				EstimatedRemainingSeconds: 0,
				Total:                     len(processed),
				Completed:                 batch.Completed,
				OverallPercent:            100,
				SuccessCount:              batch.Success,
				FailedCount:               batch.Failed,
				NoPDFURLCount:             batch.NoPDFURL,
				BackgroundCompleted:       batch.Completed,
				Stats:                     &searchStats,
			})
			_ = a.markDeepStartBackgroundComplete(sessionID, 0, false)
			return
		}

		a.emitDeepStartProgress(DeepStartProgressEvent{
			SessionID:                 sessionID,
			Phase:                     "background_processing",
			Message:                   fmt.Sprintf("后台处理完成：补全并合并 %d 篇，成功 %d，失败 %d", len(processed), batch.Success, batch.Failed),
			ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
			EstimatedRemainingSeconds: 0,
			Total:                     len(processed),
			Completed:                 maxInt(batch.Completed, len(processed)),
			OverallPercent:            100,
			SuccessCount:              batch.Success,
			FailedCount:               batch.Failed,
			NoPDFURLCount:             batch.NoPDFURL,
			DownloadedCount:           batch.Downloaded,
			ParsedCount:               batch.Parsed,
			ExtractedCount:            batch.Extracted,
			BackgroundCompleted:       maxInt(batch.Completed, len(processed)),
			Stats:                     &searchStats,
		})
		a.emitDeepStartProgress(DeepStartProgressEvent{
			SessionID:                 sessionID,
			Phase:                     "completed",
			Message:                   "后台处理完成，候选池已更新",
			ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
			EstimatedRemainingSeconds: 0,
			Total:                     len(processed),
			Completed:                 len(processed),
			OverallPercent:            100,
			BackgroundCompleted:       len(processed),
			Stats:                     &searchStats,
		})
	}()
}

func (a *App) mergeDeepStartBackgroundResults(sessionID string, added []SearchPaper) error {
	if len(added) == 0 {
		return a.markDeepStartBackgroundComplete(sessionID, 0, false)
	}

	detail, err := a.db.GetDeepStartSession(sessionID)
	if err != nil {
		return err
	}

	merged := mergeSearchPaperPools(detail.CurrentResults, added)
	detail.CurrentResults = merged
	detail.SelectedPaperIDs = normalizeSelectedPaperIDs(detail.SelectedPaperIDs, merged)
	detail.Summary.UpdatedAt = time.Now()
	detail.Summary.ProcessingStatus = "completed"
	detail.Summary.BackgroundRemaining = 0
	if detail.Summary.InitialReadyCount <= 0 {
		detail.Summary.InitialReadyCount = len(merged)
	}
	if detail.Summary.TotalPlannedCount < len(merged) {
		detail.Summary.TotalPlannedCount = len(merged)
	}

	if err := a.db.UpsertDeepStartSession(detail); err != nil {
		return err
	}
	return nil
}

func (a *App) markDeepStartBackgroundComplete(sessionID string, backgroundRemaining int, preserveProcessing bool) error {
	detail, err := a.db.GetDeepStartSession(sessionID)
	if err != nil {
		return err
	}
	if backgroundRemaining < 0 {
		backgroundRemaining = 0
	}
	detail.Summary.BackgroundRemaining = backgroundRemaining
	if !preserveProcessing {
		detail.Summary.ProcessingStatus = "completed"
	}
	detail.Summary.UpdatedAt = time.Now()
	if detail.Summary.TotalPlannedCount < detail.Summary.InitialReadyCount {
		detail.Summary.TotalPlannedCount = detail.Summary.InitialReadyCount
	}
	return a.db.UpsertDeepStartSession(detail)
}
