package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (a *App) ListDeepStartSessions() ([]DeepStartSessionSummary, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.db.ListDeepStartSessions()
}

func (a *App) GetDeepStartSession(sessionID string) (*DeepStartSessionDetail, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.db.GetDeepStartSession(strings.TrimSpace(sessionID))
}

func (a *App) StartDeepStartSession(prompt, targetFolderID string) (*DeepStartSessionDetail, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}

	targetFolderID, targetFolderName, err := a.resolveFolder(targetFolderID)
	if err != nil {
		return nil, err
	}

	sessionID := uuid.NewString()
	taskCtx, taskToken, err := a.beginDeepStartTask(sessionID)
	if err != nil {
		return nil, err
	}
	shouldFinishTask := true
	defer func() {
		if shouldFinishTask {
			a.finishDeepStartTask(sessionID, taskToken)
		}
	}()

	startedAt := time.Now()
	abortIfCancelled := func(message string, stats *SearchRetrievalStats) error {
		return a.abortDeepStartIfCancelled(taskCtx, sessionID, startedAt, stats, message)
	}

	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:      sessionID,
		Phase:          "searching",
		Message:        "正在检索论文候选",
		ElapsedSeconds: 0,
		Total:          1,
		Completed:      0,
		OverallPercent: 3,
	})

	results, searchWarning, searchStats, searchErr := a.searchDeepStartResults(taskCtx, prompt, a.deepStartResultLimit())
	if searchErr != nil {
		if isDeepStartCancelledError(searchErr) {
			return nil, abortIfCancelled("本次探索已停止，检索结果未写入历史", &searchStats)
		}
		return nil, searchErr
	}
	if err := abortIfCancelled("本次探索已停止，检索结果未写入历史", &searchStats); err != nil {
		return nil, err
	}
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "searching",
		Message:                   "检索完成，正在补全论文元信息",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     1,
		Completed:                 1,
		OverallPercent:            18,
		Stats:                     &searchStats,
	})

	initialResults, backgroundResults := splitDeepStartInitialBatch(results, prompt, deepStartInitialReadyLimit)
	if len(initialResults) > 0 {
		enriched, enrichErr := a.enrichDeepStartResults(
			taskCtx,
			sessionID,
			startedAt,
			prompt,
			initialResults,
			searchStats,
		)
		if enrichErr != nil {
			if isDeepStartCancelledError(enrichErr) || isDeepStartCancelledError(taskCtx.Err()) {
				return nil, abortIfCancelled("本次探索已停止，补全结果未写入历史", &searchStats)
			}
			if strings.TrimSpace(searchWarning) == "" {
				searchWarning = fmt.Sprintf("机构补全阶段发生部分失败：%v", enrichErr)
			} else {
				searchWarning = strings.TrimSpace(searchWarning) + " 机构补全阶段发生部分失败。"
			}
		}
		initialResults = enriched
	}
	if err := abortIfCancelled("本次探索已停止，补全结果未写入历史", &searchStats); err != nil {
		return nil, err
	}

	if len(initialResults) > 0 {
		processed, batchStats, batchErr := a.preprocessDeepStartResults(
			taskCtx,
			sessionID,
			startedAt,
			initialResults,
			searchStats,
		)
		if batchErr != nil {
			if isDeepStartCancelledError(batchErr) || isDeepStartCancelledError(taskCtx.Err()) {
				return nil, abortIfCancelled("本次探索已停止，批处理结果未写入历史", &searchStats)
			}
			if errors.Is(batchErr, ErrDeepStartAIUnavailable) {
				return nil, batchErr
			}
			if strings.TrimSpace(searchWarning) == "" {
				searchWarning = fmt.Sprintf("PDF 批处理阶段发生部分失败：%v", batchErr)
			} else {
				searchWarning = strings.TrimSpace(searchWarning) + " PDF 批处理阶段发生部分失败。"
			}
		} else {
			initialResults = processed
			if batchStats.Failed > 0 {
				warn := fmt.Sprintf("首批批处理完成：成功 %d，失败 %d（无链接 %d）", batchStats.Success, batchStats.Failed, batchStats.NoPDFURL)
				if strings.TrimSpace(searchWarning) == "" {
					searchWarning = warn
				} else {
					searchWarning = strings.TrimSpace(searchWarning) + " " + warn
				}
			}
		}
	}
	if err := abortIfCancelled("本次探索已停止，批处理结果未写入历史", &searchStats); err != nil {
		return nil, err
	}
	results = initialResults

	summary := DeepStartSessionSummary{
		ID:                  sessionID,
		Title:               normalizeDeepStartTitle("", prompt, prompt),
		RootPrompt:          prompt,
		CurrentQuery:        prompt,
		TargetFolderID:      targetFolderID,
		ProcessingStatus:    "completed",
		InitialReadyCount:   len(results),
		TotalPlannedCount:   len(results) + len(backgroundResults),
		BackgroundRemaining: len(backgroundResults),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if len(backgroundResults) > 0 {
		summary.ProcessingStatus = "background_processing"
	}

	userMessage := DeepStartMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   prompt,
		CreatedAt: time.Now(),
	}
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "analyzing",
		Message:                   "正在生成 AI 概览与分类建议",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     len(results),
		Completed:                 len(results),
		OverallPercent:            82,
		Stats:                     &searchStats,
	})
	if err := abortIfCancelled("本次探索已停止，分析结果未写入历史", &searchStats); err != nil {
		return nil, err
	}
	analysisTitle, analysis, assistantContent, analysisErr := a.generateDeepStartAnalysis(
		taskCtx,
		summary,
		[]DeepStartMessage{userMessage},
		results,
		targetFolderName,
		searchWarning,
		searchStats,
	)
	if analysisErr != nil {
		return nil, fmt.Errorf("DeepStart AI 分析失败：%w", analysisErr)
	}
	summary.Title = analysisTitle
	if err := abortIfCancelled("本次探索已停止，分析结果未写入历史", &searchStats); err != nil {
		return nil, err
	}

	detail := &DeepStartSessionDetail{
		Summary:         summary,
		CurrentResults:  results,
		CurrentAnalysis: analysis,
	}
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "persisting",
		Message:                   "正在保存会话并准备进入详情",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     len(results),
		Completed:                 len(results),
		OverallPercent:            94,
		Stats:                     &searchStats,
	})
	if err := abortIfCancelled("本次探索已停止，未保存本轮探索数据", &searchStats); err != nil {
		return nil, err
	}
	if err := a.db.UpsertDeepStartSession(detail); err != nil {
		return nil, err
	}

	if err := a.db.SaveDeepStartMessage(&userMessage); err != nil {
		return nil, err
	}

	assistantMessage := DeepStartMessage{
		SessionID: detail.Summary.ID,
		Role:      "assistant",
		Content:   assistantContent,
		CreatedAt: time.Now(),
	}
	if err := a.db.SaveDeepStartMessage(&assistantMessage); err != nil {
		return nil, err
	}
	if err := a.db.SaveDeepStartSearchRound(detail.Summary.ID, detail.Summary.CurrentQuery, results, analysis); err != nil {
		return nil, err
	}

	loaded, err := a.db.GetDeepStartSession(detail.Summary.ID)
	if err != nil {
		return nil, err
	}

	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "completed",
		Message:                   "探索准备完成，正在进入详情页",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     len(results),
		Completed:                 len(results),
		OverallPercent:            100,
		Stats:                     &searchStats,
	})

	if len(backgroundResults) > 0 {
		a.emitDeepStartProgress(DeepStartProgressEvent{
			SessionID:                 sessionID,
			Phase:                     "initial_batch_ready",
			Message:                   fmt.Sprintf("首批 %d 篇已就绪，后台继续处理 %d 篇", len(results), len(backgroundResults)),
			ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
			EstimatedRemainingSeconds: estimateDeepStartETA(deepStartBatchStats{Total: len(backgroundResults), Completed: 0}),
			Total:                     len(results) + len(backgroundResults),
			Completed:                 len(results),
			OverallPercent:            100,
			InitialBatchTotal:         len(results),
			InitialBatchCompleted:     len(results),
			BackgroundCompleted:       0,
			Stats:                     &searchStats,
		})
		shouldFinishTask = false
		a.runDeepStartBackgroundProcessing(taskCtx, sessionID, taskToken, prompt, backgroundResults, searchStats)
	}
	return loaded, nil
}

func (a *App) ReplyDeepStartSession(sessionID, message string) (*DeepStartSessionDetail, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	sessionID = strings.TrimSpace(sessionID)
	message = strings.TrimSpace(message)
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}
	if message == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}

	detail, err := a.db.GetDeepStartSession(sessionID)
	if err != nil {
		return nil, err
	}

	userMessage := DeepStartMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   message,
		CreatedAt: time.Now(),
	}
	startedAt := time.Now()
	total := len(detail.CurrentResults)
	if total <= 0 {
		total = 1
	}
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "analyzing",
		Message:                   "正在基于当前候选池生成会话内缩窄建议",
		ElapsedSeconds:            0,
		EstimatedRemainingSeconds: 0,
		Total:                     total,
		Completed:                 0,
		OverallPercent:            35,
	})

	messages := append(append([]DeepStartMessage{}, detail.Messages...), userMessage)
	targetFolderName, _ := a.folderNameByID(detail.Summary.TargetFolderID)
	searchStats := SearchRetrievalStats{}
	if detail.CurrentAnalysis != nil {
		searchStats = detail.CurrentAnalysis.SearchStats
	}
	analysisTitle, analysis, assistantContent, analysisErr := a.generateDeepStartAnalysis(
		a.ctx,
		detail.Summary,
		messages,
		detail.CurrentResults,
		targetFolderName,
		"",
		searchStats,
	)
	if analysisErr != nil {
		return nil, fmt.Errorf("DeepStart AI 分析失败：%w", analysisErr)
	}
	beforeCount := len(detail.CurrentResults)
	narrowedResults, retainedIDs, narrowReason := applyDeepStartNarrowing(detail.CurrentResults, analysis, userMessage.Content)
	analysis.RetainedPaperIDs = retainedIDs
	afterCount := len(narrowedResults)
	removedCount := beforeCount - afterCount
	if removedCount < 0 {
		removedCount = 0
	}
	narrowSummary := fmt.Sprintf("本轮缩窄：剔除 %d 篇，保留 %d 篇。", removedCount, afterCount)
	if reason := strings.TrimSpace(narrowReason); reason != "" {
		narrowSummary = narrowSummary + " 主要依据：" + reason + "。"
	}
	if strings.TrimSpace(assistantContent) == "" {
		assistantContent = narrowSummary
	} else {
		assistantContent = strings.TrimSpace(assistantContent) + "\n\n" + narrowSummary
	}

	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "persisting",
		Message:                   "正在保存会话内筛选结果",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     len(narrowedResults),
		Completed:                 len(narrowedResults),
		OverallPercent:            85,
	})

	if err := a.db.SaveDeepStartMessage(&userMessage); err != nil {
		return nil, err
	}

	assistantMessage := DeepStartMessage{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   assistantContent,
		CreatedAt: time.Now(),
	}
	if err := a.db.SaveDeepStartMessage(&assistantMessage); err != nil {
		return nil, err
	}

	detail.Summary.Title = analysisTitle
	detail.Summary.UpdatedAt = time.Now()
	detail.CurrentAnalysis = analysis
	detail.CurrentResults = narrowedResults
	detail.SelectedPaperIDs = normalizeSelectedPaperIDs(detail.SelectedPaperIDs, detail.CurrentResults)
	if err := a.db.UpsertDeepStartSession(detail); err != nil {
		return nil, err
	}
	if err := a.db.SaveDeepStartSearchRound(sessionID, detail.Summary.CurrentQuery, detail.CurrentResults, detail.CurrentAnalysis); err != nil {
		return nil, err
	}

	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "completed",
		Message:                   "会话内缩窄完成",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     total,
		Completed:                 total,
		OverallPercent:            100,
	})

	return a.db.GetDeepStartSession(sessionID)
}

func (a *App) RerunDeepStartSearch(sessionID, query string) (*DeepStartSessionDetail, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	sessionID = strings.TrimSpace(sessionID)
	query = strings.TrimSpace(query)
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	detail, err := a.db.GetDeepStartSession(sessionID)
	if err != nil {
		return nil, err
	}

	taskCtx, taskToken, err := a.beginDeepStartTask(sessionID)
	if err != nil {
		return nil, err
	}
	shouldFinishTask := true
	defer func() {
		if shouldFinishTask {
			a.finishDeepStartTask(sessionID, taskToken)
		}
	}()

	startedAt := time.Now()
	abortIfCancelled := func(message string, stats *SearchRetrievalStats) error {
		return a.abortDeepStartIfCancelled(taskCtx, sessionID, startedAt, stats, message)
	}

	userMessage := DeepStartMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   "重新检索：" + query,
		CreatedAt: time.Now(),
	}
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:      sessionID,
		Phase:          "searching",
		Message:        "正在根据新 query 重新检索候选",
		ElapsedSeconds: 0,
		Total:          1,
		Completed:      0,
		OverallPercent: 3,
	})

	results, searchWarning, searchStats, searchErr := a.searchDeepStartResults(taskCtx, query, a.deepStartResultLimit())
	if searchErr != nil {
		if isDeepStartCancelledError(searchErr) {
			return nil, abortIfCancelled("本次重搜已停止，原会话保持不变", &searchStats)
		}
		return nil, searchErr
	}
	if err := abortIfCancelled("本次重搜已停止，原会话保持不变", &searchStats); err != nil {
		return nil, err
	}

	initialResults, backgroundResults := splitDeepStartInitialBatch(results, query, deepStartInitialReadyLimit)
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "searching",
		Message:                   "检索完成，正在补全论文元信息",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     1,
		Completed:                 1,
		OverallPercent:            18,
		Stats:                     &searchStats,
	})

	if len(initialResults) > 0 {
		enriched, enrichErr := a.enrichDeepStartResults(
			taskCtx,
			sessionID,
			startedAt,
			query,
			initialResults,
			searchStats,
		)
		if enrichErr != nil {
			if isDeepStartCancelledError(enrichErr) || isDeepStartCancelledError(taskCtx.Err()) {
				return nil, abortIfCancelled("本次重搜已停止，原会话保持不变", &searchStats)
			}
			if strings.TrimSpace(searchWarning) == "" {
				searchWarning = fmt.Sprintf("机构补全阶段发生部分失败：%v", enrichErr)
			} else {
				searchWarning = strings.TrimSpace(searchWarning) + " 机构补全阶段发生部分失败。"
			}
		}
		initialResults = enriched
	}
	if err := abortIfCancelled("本次重搜已停止，原会话保持不变", &searchStats); err != nil {
		return nil, err
	}

	if len(initialResults) > 0 {
		processed, batchStats, batchErr := a.preprocessDeepStartResults(
			taskCtx,
			sessionID,
			startedAt,
			initialResults,
			searchStats,
		)
		if batchErr != nil {
			if isDeepStartCancelledError(batchErr) || isDeepStartCancelledError(taskCtx.Err()) {
				return nil, abortIfCancelled("本次重搜已停止，原会话保持不变", &searchStats)
			}
			if errors.Is(batchErr, ErrDeepStartAIUnavailable) {
				return nil, batchErr
			}
			if strings.TrimSpace(searchWarning) == "" {
				searchWarning = fmt.Sprintf("PDF 批处理阶段发生部分失败：%v", batchErr)
			} else {
				searchWarning = strings.TrimSpace(searchWarning) + " PDF 批处理阶段发生部分失败。"
			}
		} else {
			initialResults = processed
			if batchStats.Failed > 0 {
				warn := fmt.Sprintf("首批批处理完成：成功 %d，失败 %d（无链接 %d）", batchStats.Success, batchStats.Failed, batchStats.NoPDFURL)
				if strings.TrimSpace(searchWarning) == "" {
					searchWarning = warn
				} else {
					searchWarning = strings.TrimSpace(searchWarning) + " " + warn
				}
			}
		}
	}
	if err := abortIfCancelled("本次重搜已停止，原会话保持不变", &searchStats); err != nil {
		return nil, err
	}
	results = initialResults

	detail.Summary.CurrentQuery = query
	detail.Summary.UpdatedAt = time.Now()
	detail.CurrentResults = results
	detail.Summary.InitialReadyCount = len(results)
	detail.Summary.TotalPlannedCount = len(results) + len(backgroundResults)
	detail.Summary.BackgroundRemaining = len(backgroundResults)
	detail.Summary.ProcessingStatus = "completed"
	if len(backgroundResults) > 0 {
		detail.Summary.ProcessingStatus = "background_processing"
	}
	detail.SelectedPaperIDs = normalizeSelectedPaperIDs(detail.SelectedPaperIDs, results)

	messages := append(append([]DeepStartMessage{}, detail.Messages...), userMessage)
	targetFolderName, _ := a.folderNameByID(detail.Summary.TargetFolderID)
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "analyzing",
		Message:                   "正在生成 AI 概览与分类建议",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     len(results),
		Completed:                 len(results),
		OverallPercent:            82,
		Stats:                     &searchStats,
	})
	if err := abortIfCancelled("本次重搜已停止，原会话保持不变", &searchStats); err != nil {
		return nil, err
	}
	analysisTitle, analysis, assistantContent, analysisErr := a.generateDeepStartAnalysis(
		taskCtx,
		detail.Summary,
		messages,
		results,
		targetFolderName,
		searchWarning,
		searchStats,
	)
	if analysisErr != nil {
		return nil, deepStartAIUnavailableError("强模型分析失败", analysisErr)
	}
	detail.Summary.Title = analysisTitle
	detail.CurrentAnalysis = analysis
	if err := abortIfCancelled("本次重搜已停止，原会话保持不变", &searchStats); err != nil {
		return nil, err
	}

	assistantMessage := DeepStartMessage{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   assistantContent,
		CreatedAt: time.Now(),
	}
	if err := abortIfCancelled("本次重搜已停止，原会话保持不变", &searchStats); err != nil {
		return nil, err
	}
	if err := a.db.SaveDeepStartMessage(&userMessage); err != nil {
		return nil, err
	}
	if err := a.db.SaveDeepStartMessage(&assistantMessage); err != nil {
		return nil, err
	}
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "persisting",
		Message:                   "正在保存会话并准备进入详情",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     len(results),
		Completed:                 len(results),
		OverallPercent:            94,
		Stats:                     &searchStats,
	})
	if err := abortIfCancelled("本次重搜已停止，原会话保持不变", &searchStats); err != nil {
		return nil, err
	}
	if err := a.db.UpsertDeepStartSession(detail); err != nil {
		return nil, err
	}
	if err := a.db.SaveDeepStartSearchRound(sessionID, query, results, analysis); err != nil {
		return nil, err
	}

	loaded, err := a.db.GetDeepStartSession(sessionID)
	if err != nil {
		return nil, err
	}

	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "completed",
		Message:                   "新一轮候选已准备完成",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     len(results),
		Completed:                 len(results),
		OverallPercent:            100,
		Stats:                     &searchStats,
	})

	if len(backgroundResults) > 0 {
		a.emitDeepStartProgress(DeepStartProgressEvent{
			SessionID:                 sessionID,
			Phase:                     "initial_batch_ready",
			Message:                   fmt.Sprintf("首批 %d 篇已就绪，后台继续处理 %d 篇", len(results), len(backgroundResults)),
			ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
			EstimatedRemainingSeconds: estimateDeepStartETA(deepStartBatchStats{Total: len(backgroundResults), Completed: 0}),
			Total:                     len(results) + len(backgroundResults),
			Completed:                 len(results),
			OverallPercent:            100,
			InitialBatchTotal:         len(results),
			InitialBatchCompleted:     len(results),
			BackgroundCompleted:       0,
			Stats:                     &searchStats,
		})
		shouldFinishTask = false
		a.runDeepStartBackgroundProcessing(taskCtx, sessionID, taskToken, query, backgroundResults, searchStats)
	}
	return loaded, nil
}

func (a *App) UpdateDeepStartSelections(sessionID string, selectedPaperIDs []string, targetFolderID string) (*DeepStartSessionDetail, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	detail, err := a.db.GetDeepStartSession(sessionID)
	if err != nil {
		return nil, err
	}

	if targetFolderID != "" {
		resolvedFolderID, _, err := a.resolveFolder(targetFolderID)
		if err != nil {
			return nil, err
		}
		detail.Summary.TargetFolderID = resolvedFolderID
	}

	detail.SelectedPaperIDs = normalizeSelectedPaperIDs(selectedPaperIDs, detail.CurrentResults)
	detail.Summary.UpdatedAt = time.Now()
	if err := a.db.UpsertDeepStartSession(detail); err != nil {
		return nil, err
	}

	return a.db.GetDeepStartSession(sessionID)
}

func (a *App) UndoDeepStartNarrow(sessionID string) (*DeepStartSessionDetail, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	detail, err := a.db.GetDeepStartSession(sessionID)
	if err != nil {
		return nil, err
	}

	rounds, err := a.db.ListDeepStartSearchRounds(sessionID, 2)
	if err != nil {
		return nil, err
	}
	if len(rounds) < 2 {
		return nil, fmt.Errorf("没有可回退的上一轮缩窄结果")
	}

	latestRound := rounds[0]
	previousRound := rounds[1]

	detail.CurrentResults = previousRound.Results
	detail.CurrentAnalysis = previousRound.Analysis
	if strings.TrimSpace(previousRound.Query) != "" {
		detail.Summary.CurrentQuery = strings.TrimSpace(previousRound.Query)
	}
	detail.SelectedPaperIDs = normalizeSelectedPaperIDs(detail.SelectedPaperIDs, detail.CurrentResults)
	detail.Summary.UpdatedAt = time.Now()
	if err := a.db.UpsertDeepStartSession(detail); err != nil {
		return nil, err
	}
	if err := a.db.DeleteDeepStartSearchRound(latestRound.ID); err != nil {
		return nil, err
	}
	if err := a.db.SaveDeepStartMessage(&DeepStartMessage{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   fmt.Sprintf("已回退上一轮缩窄，当前候选池恢复到 %d 篇。", len(detail.CurrentResults)),
		CreatedAt: time.Now(),
	}); err != nil {
		return nil, err
	}

	return a.db.GetDeepStartSession(sessionID)
}

func (a *App) resolveFolder(folderID string) (string, string, error) {
	folders, err := a.db.GetFolders()
	if err != nil {
		return "", "", err
	}
	if len(folders) == 0 {
		return "", "", fmt.Errorf("no folder available")
	}

	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return folders[0].ID, folders[0].Name, nil
	}

	for _, folder := range folders {
		if folder.ID == folderID {
			return folder.ID, folder.Name, nil
		}
	}

	return "", "", fmt.Errorf("folder not found")
}

func (a *App) folderNameByID(folderID string) (string, error) {
	if strings.TrimSpace(folderID) == "" {
		return "", nil
	}

	folders, err := a.db.GetFolders()
	if err != nil {
		return "", err
	}
	for _, folder := range folders {
		if folder.ID == folderID {
			return folder.Name, nil
		}
	}
	return "", fmt.Errorf("folder not found")
}

func (a *App) searchDeepStartResults(ctx context.Context, query string, limit int) ([]SearchPaper, string, SearchRetrievalStats, error) {
	results, warning, stats, err := a.deepStartSearchWithRewrittenQueries(ctx, query, limit, 0)
	if err != nil {
		if isDeepStartCancelledError(err) {
			return nil, "", stats, ErrDeepStartTaskCancelled
		}
		return nil, "", stats, err
	}
	return results, strings.TrimSpace(warning), stats, nil
}

func (a *App) deepStartResultLimit() int {
	limit := a.config.Search.DeepStartResultLimit
	if limit <= 0 {
		limit = defaultSearchAPIConfig().DeepStartResultLimit
	}
	if limit > 200 {
		limit = 200
	}
	return limit
}

func (a *App) enrichDeepStartResults(
	ctx context.Context,
	sessionID string,
	startedAt time.Time,
	query string,
	results []SearchPaper,
	stats SearchRetrievalStats,
) ([]SearchPaper, error) {
	if len(results) == 0 {
		return results, nil
	}
	if a.deepStartEnricher == nil {
		return results, nil
	}

	total := len(results)
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "enriching",
		Message:                   fmt.Sprintf("正在补全 %d 篇论文的机构信息", total),
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: maxInt(total/2, 1),
		Total:                     total,
		Completed:                 0,
		OverallPercent:            20,
		Stats:                     &stats,
	})

	return a.deepStartEnricher.EnrichPapers(ctx, query, results, func(completed, totalCount, etaSeconds int, message string) {
		overallPercent := 20
		if totalCount > 0 {
			overallPercent = 20 + int(float64(completed)/float64(totalCount)*50)
		}
		if overallPercent > 75 {
			overallPercent = 75
		}

		phaseMessage := fmt.Sprintf("正在导入 %d 篇论文并补全机构信息，预计剩余 %d 秒", totalCount, etaSeconds)
		if trimmed := strings.TrimSpace(message); trimmed != "" {
			phaseMessage = phaseMessage + " · " + trimmed
		}

		a.emitDeepStartProgress(DeepStartProgressEvent{
			SessionID:                 sessionID,
			Phase:                     "enriching",
			Message:                   phaseMessage,
			ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
			EstimatedRemainingSeconds: etaSeconds,
			Total:                     totalCount,
			Completed:                 completed,
			OverallPercent:            overallPercent,
			Stats:                     &stats,
		})
	})
}

func (a *App) emitDeepStartProgress(progress DeepStartProgressEvent) {
	a.emitEvent("deepstart-progress", progress)
}

func (a *App) generateDeepStartAnalysis(
	ctx context.Context,
	summary DeepStartSessionSummary,
	messages []DeepStartMessage,
	results []SearchPaper,
	targetFolderName,
	searchWarning string,
	searchStats SearchRetrievalStats,
) (string, *DeepStartAnalysis, string, error) {
	request := DeepStartAIRequest{
		RootPrompt:       summary.RootPrompt,
		CurrentQuery:     summary.CurrentQuery,
		TargetFolderName: targetFolderName,
		Messages:         trimDeepStartMessages(messages, 12),
		Results:          results,
	}

	strong := a.currentStrongLLM()
	if strong == nil {
		return "", nil, "", deepStartAIUnavailableError("强模型未初始化", nil)
	}
	if len(results) == 0 {
		if warning := strings.TrimSpace(searchWarning); warning != "" {
			return "", nil, "", deepStartAIUnavailableError("检索没有返回可供 AI 分析的论文："+warning, nil)
		}
		return "", nil, "", deepStartAIUnavailableError("检索没有返回可供 AI 分析的论文", nil)
	}

	response, err := analyzeDeepStartWithRetry(ctx, strong, request)
	if err != nil {
		return "", nil, "", deepStartAIUnavailableError("强模型分析请求失败", err)
	}
	if response == nil {
		return "", nil, "", fmt.Errorf("强模型返回了空的分析结果")
	}

	title := response.Title
	analysis := response.Analysis
	if warning := strings.TrimSpace(searchWarning); warning != "" {
		if strings.TrimSpace(analysis.Overview) == "" {
			analysis.Overview = warning
		} else {
			analysis.Overview = strings.TrimSpace(analysis.Overview) + " " + warning
		}
	}
	analysis.SearchStats = normalizeDeepStartSearchStats(searchStats, summary.CurrentQuery, len(results))
	if statsLine := formatDeepStartSearchStats(analysis.SearchStats, len(results)); statsLine != "" {
		if strings.TrimSpace(analysis.Overview) == "" {
			analysis.Overview = statsLine
		} else {
			analysis.Overview = strings.TrimSpace(analysis.Overview) + "\n\n" + statsLine
		}
	}

	if title == "" {
		title = normalizeDeepStartTitle("", summary.RootPrompt, summary.CurrentQuery)
	}
	assistantContent := renderDeepStartAssistantMessage(&analysis)

	return title, &analysis, assistantContent, nil
}

func trimDeepStartMessages(messages []DeepStartMessage, limit int) []DeepStartMessage {
	if len(messages) <= limit {
		return messages
	}
	return append([]DeepStartMessage{}, messages[len(messages)-limit:]...)
}

func renderDeepStartAssistantMessage(analysis *DeepStartAnalysis) string {
	if analysis == nil {
		return "我已经保留了这轮探索，但暂时没有可展示的分析。"
	}

	parts := []string{}
	if overview := strings.TrimSpace(analysis.Overview); overview != "" {
		parts = append(parts, overview)
	}

	if len(analysis.Directions) > 0 {
		names := make([]string, 0, len(analysis.Directions))
		for _, direction := range analysis.Directions {
			if strings.TrimSpace(direction.Name) != "" {
				names = append(names, direction.Name)
			}
			if len(names) >= 3 {
				break
			}
		}
		if len(names) > 0 {
			parts = append(parts, "我先按这些方向帮你整理："+strings.Join(names, "、")+"。")
		}
	}

	if len(analysis.FollowUpQuestions) > 0 {
		parts = append(parts, "你可以直接点下面的问题继续缩窄方向，或者自己补一句更具体的偏好。")
	}

	if len(parts) == 0 {
		return "我已经整理好这一轮结果，你可以继续告诉我你更想看哪条主线。"
	}

	return strings.Join(parts, "\n\n")
}

func normalizeSelectedPaperIDs(selectedPaperIDs []string, results []SearchPaper) []string {
	validPaperIDs := make(map[string]struct{}, len(results))
	for _, paper := range results {
		if strings.TrimSpace(paper.ID) != "" {
			validPaperIDs[paper.ID] = struct{}{}
		}
	}
	return filterExistingPaperIDs(selectedPaperIDs, validPaperIDs)
}

func normalizeDeepStartSearchStats(stats SearchRetrievalStats, query string, finalCount int) SearchRetrievalStats {
	stats.Query = strings.TrimSpace(firstNonEmpty(stats.Query, query))
	if stats.RawCount < 0 {
		stats.RawCount = 0
	}
	if stats.DedupCount < 0 {
		stats.DedupCount = 0
	}
	if stats.FinalCount < 0 {
		stats.FinalCount = 0
	}
	if stats.FinalCount == 0 {
		stats.FinalCount = finalCount
	}
	if stats.DedupCount == 0 {
		stats.DedupCount = stats.FinalCount
	}
	if stats.RawCount == 0 {
		stats.RawCount = stats.DedupCount
	}
	return stats
}

func formatDeepStartSearchStats(stats SearchRetrievalStats, currentPool int) string {
	if stats.RawCount == 0 && stats.DedupCount == 0 && stats.FinalCount == 0 && currentPool <= 0 {
		return ""
	}
	raw, dedup, final := stats.RawCount, stats.DedupCount, stats.FinalCount
	if currentPool > 0 {
		final = currentPool
	}
	return fmt.Sprintf("本轮从来源获取 %d 条记录，合并重复版本后得到 %d 篇候选，当前纳入 %d 篇。", raw, dedup, final)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func applyDeepStartNarrowing(results []SearchPaper, analysis *DeepStartAnalysis, userMessage string) ([]SearchPaper, []string, string) {
	if len(results) == 0 {
		return results, []string{}, ""
	}

	validPaperIDs := make(map[string]struct{}, len(results))
	allPaperIDs := make([]string, 0, len(results))
	for _, paper := range results {
		paperID := strings.TrimSpace(paper.ID)
		if paperID == "" {
			continue
		}
		validPaperIDs[paperID] = struct{}{}
		allPaperIDs = append(allPaperIDs, paperID)
	}
	if len(allPaperIDs) == 0 {
		return results, []string{}, ""
	}

	candidateIDs := []string{}
	strategy := "默认质量排序"
	if analysis != nil {
		candidateIDs = filterExistingPaperIDs(analysis.RetainedPaperIDs, validPaperIDs)
		if len(candidateIDs) > 0 {
			strategy = "AI 保留列表"
		}
		if len(candidateIDs) == 0 {
			candidateIDs = filterExistingPaperIDs(analysis.RecommendedPaperIDs, validPaperIDs)
			if len(candidateIDs) > 0 {
				strategy = "AI 推荐列表"
			}
		}
	}

	targetCount := defaultDeepStartNarrowTarget(len(allPaperIDs))
	messageMatchedIDs := matchPaperIDsByMessage(results, userMessage, targetCount)
	if len(messageMatchedIDs) > 0 && len(messageMatchedIDs) < len(allPaperIDs) {
		candidateIDs = messageMatchedIDs
		strategy = "会话消息 + 标签匹配"
	}

	if len(candidateIDs) == 0 {
		candidateIDs = allPaperIDs
		strategy = "保留全部候选（未命中缩窄条件）"
	}
	candidateIDs = filterExistingPaperIDs(candidateIDs, validPaperIDs)
	if len(candidateIDs) == 0 {
		candidateIDs = allPaperIDs
		strategy = "保留全部候选（候选回退）"
	}
	if len(allPaperIDs) > 1 && len(candidateIDs) >= len(allPaperIDs) {
		limit := defaultDeepStartNarrowTarget(len(allPaperIDs))
		if limit > 0 && limit < len(allPaperIDs) {
			candidateIDs = append([]string{}, allPaperIDs[:limit]...)
			strategy = "默认质量排序"
		}
	}

	retainedSet := make(map[string]struct{}, len(candidateIDs))
	for _, paperID := range candidateIDs {
		retainedSet[paperID] = struct{}{}
	}

	narrowed := make([]SearchPaper, 0, len(candidateIDs))
	for _, paper := range results {
		if _, keep := retainedSet[paper.ID]; keep {
			narrowed = append(narrowed, paper)
		}
	}
	if len(narrowed) == 0 {
		narrowed = append(narrowed, results...)
		strategy = "保留全部候选（缩窄结果为空）"
	}

	retainedIDs := make([]string, 0, len(narrowed))
	for _, paper := range narrowed {
		if strings.TrimSpace(paper.ID) != "" {
			retainedIDs = append(retainedIDs, paper.ID)
		}
	}
	return narrowed, retainedIDs, strategy
}

func defaultDeepStartNarrowTarget(total int) int {
	if total <= 8 {
		return total
	}
	target := total / 2
	switch {
	case total <= 20:
		target = (total * 3) / 4
	case total <= 60:
		target = (total * 2) / 3
	default:
		target = total / 2
	}
	if target < 6 {
		target = 6
	}
	if target >= total {
		target = total - 1
	}
	if target <= 0 {
		target = total
	}
	return target
}

func matchPaperIDsByMessage(results []SearchPaper, message string, targetCount int) []string {
	tokens := extractDeepStartMessageTokens(message)
	if len(tokens) == 0 {
		return []string{}
	}

	type scoredPaper struct {
		id    string
		score int
	}
	scored := make([]scoredPaper, 0, len(results))
	for _, paper := range results {
		score := deepStartPaperMatchScore(paper, tokens)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredPaper{id: paper.ID, score: score})
	}
	if len(scored) == 0 {
		return []string{}
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].id < scored[j].id
		}
		return scored[i].score > scored[j].score
	})

	limit := len(scored)
	if targetCount > 0 && targetCount < limit {
		limit = targetCount
	}
	ids := make([]string, 0, limit)
	for _, item := range scored[:limit] {
		if strings.TrimSpace(item.id) != "" {
			ids = append(ids, item.id)
		}
	}
	return ids
}

func extractDeepStartMessageTokens(message string) []string {
	normalized := normalizeSearchDelimiters(strings.ToLower(strings.TrimSpace(message)))
	parts := strings.Fields(normalized)
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, stop := searchEnglishStopWords[part]; stop {
			continue
		}
		if len([]rune(part)) < 2 {
			continue
		}
		tokens = append(tokens, part)
	}
	return compactStrings(tokens, 16)
}

func deepStartPaperMatchScore(paper SearchPaper, tokens []string) int {
	if len(tokens) == 0 {
		return 0
	}

	title := strings.ToLower(strings.TrimSpace(paper.Title))
	abstract := strings.ToLower(strings.TrimSpace(paper.Abstract))
	topic := strings.ToLower(strings.TrimSpace(paper.TopicLabel))
	method := strings.ToLower(strings.TrimSpace(paper.MethodLabel))
	task := strings.ToLower(strings.TrimSpace(paper.TaskLabel))
	domain := strings.ToLower(strings.TrimSpace(paper.DomainLabel))
	haystack := strings.ToLower(strings.Join([]string{
		paper.Title,
		paper.Abstract,
		paper.Authors,
		paper.Journal,
		paper.Category,
		paper.TopicLabel,
		paper.MethodLabel,
		paper.TaskLabel,
		paper.DomainLabel,
		strings.Join(paper.Tags, " "),
		strings.Join(paper.Keywords, " "),
		strings.Join(paper.Institutions, " "),
	}, " "))

	score := 0
	for _, token := range tokens {
		if strings.Contains(haystack, token) {
			score += maxInt(len([]rune(token)), 2)
		}
		if strings.Contains(title, token) {
			score += 4
		}
		if strings.Contains(abstract, token) {
			score += 2
		}
		if strings.Contains(topic, token) || strings.Contains(method, token) || strings.Contains(task, token) || strings.Contains(domain, token) {
			score += 6
		}
	}
	return score
}
