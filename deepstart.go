package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	startedAt := time.Now()
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:      sessionID,
		Phase:          "searching",
		Message:        "正在检索论文候选",
		ElapsedSeconds: 0,
		Total:          1,
		Completed:      0,
		OverallPercent: 3,
	})

	results, searchWarning, searchStats := a.searchDeepStartResults(prompt, a.deepStartResultLimit())
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

	if len(results) > 0 {
		enriched, enrichErr := a.enrichDeepStartResults(
			context.Background(),
			sessionID,
			startedAt,
			prompt,
			results,
			searchStats,
		)
		if enrichErr != nil {
			if strings.TrimSpace(searchWarning) == "" {
				searchWarning = fmt.Sprintf("机构补全阶段发生部分失败：%v", enrichErr)
			} else {
				searchWarning = strings.TrimSpace(searchWarning) + " 机构补全阶段发生部分失败。"
			}
		}
		results = enriched
	}

	summary := DeepStartSessionSummary{
		ID:             sessionID,
		Title:          normalizeDeepStartTitle("", prompt, prompt),
		RootPrompt:     prompt,
		CurrentQuery:   prompt,
		TargetFolderID: targetFolderID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
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
		EstimatedRemainingSeconds: 2,
		Total:                     len(results),
		Completed:                 len(results),
		OverallPercent:            82,
		Stats:                     &searchStats,
	})
	analysisTitle, analysis, assistantContent := a.generateDeepStartAnalysis(
		summary,
		[]DeepStartMessage{userMessage},
		results,
		targetFolderName,
		searchWarning,
		searchStats,
	)
	summary.Title = analysisTitle

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
		EstimatedRemainingSeconds: 1,
		Total:                     len(results),
		Completed:                 len(results),
		OverallPercent:            94,
		Stats:                     &searchStats,
	})
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
	if err := a.db.SaveDeepStartMessage(&userMessage); err != nil {
		return nil, err
	}

	messages := append(append([]DeepStartMessage{}, detail.Messages...), userMessage)
	targetFolderName, _ := a.folderNameByID(detail.Summary.TargetFolderID)
	searchStats := SearchRetrievalStats{}
	if detail.CurrentAnalysis != nil {
		searchStats = detail.CurrentAnalysis.SearchStats
	}
	analysisTitle, analysis, assistantContent := a.generateDeepStartAnalysis(
		detail.Summary,
		messages,
		detail.CurrentResults,
		targetFolderName,
		"",
		searchStats,
	)

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
	if err := a.db.UpsertDeepStartSession(detail); err != nil {
		return nil, err
	}

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

	userMessage := DeepStartMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   "重新检索：" + query,
		CreatedAt: time.Now(),
	}
	if err := a.db.SaveDeepStartMessage(&userMessage); err != nil {
		return nil, err
	}

	startedAt := time.Now()
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:      sessionID,
		Phase:          "searching",
		Message:        "正在根据新 query 重新检索候选",
		ElapsedSeconds: 0,
		Total:          1,
		Completed:      0,
		OverallPercent: 3,
	})

	results, searchWarning, searchStats := a.searchDeepStartResults(query, a.deepStartResultLimit())
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

	if len(results) > 0 {
		enriched, enrichErr := a.enrichDeepStartResults(
			context.Background(),
			sessionID,
			startedAt,
			query,
			results,
			searchStats,
		)
		if enrichErr != nil {
			if strings.TrimSpace(searchWarning) == "" {
				searchWarning = fmt.Sprintf("机构补全阶段发生部分失败：%v", enrichErr)
			} else {
				searchWarning = strings.TrimSpace(searchWarning) + " 机构补全阶段发生部分失败。"
			}
		}
		results = enriched
	}

	detail.Summary.CurrentQuery = query
	detail.Summary.UpdatedAt = time.Now()
	detail.CurrentResults = results
	detail.SelectedPaperIDs = normalizeSelectedPaperIDs(detail.SelectedPaperIDs, results)

	messages := append(append([]DeepStartMessage{}, detail.Messages...), userMessage)
	targetFolderName, _ := a.folderNameByID(detail.Summary.TargetFolderID)
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "analyzing",
		Message:                   "正在生成 AI 概览与分类建议",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 2,
		Total:                     len(results),
		Completed:                 len(results),
		OverallPercent:            82,
		Stats:                     &searchStats,
	})
	analysisTitle, analysis, assistantContent := a.generateDeepStartAnalysis(
		detail.Summary,
		messages,
		results,
		targetFolderName,
		searchWarning,
		searchStats,
	)
	detail.Summary.Title = analysisTitle
	detail.CurrentAnalysis = analysis

	assistantMessage := DeepStartMessage{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   assistantContent,
		CreatedAt: time.Now(),
	}
	if err := a.db.SaveDeepStartMessage(&assistantMessage); err != nil {
		return nil, err
	}
	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "persisting",
		Message:                   "正在保存会话并准备进入详情",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 1,
		Total:                     len(results),
		Completed:                 len(results),
		OverallPercent:            94,
		Stats:                     &searchStats,
	})
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

func (a *App) searchDeepStartResults(query string, limit int) ([]SearchPaper, string, SearchRetrievalStats) {
	stats := SearchRetrievalStats{Query: strings.TrimSpace(query)}
	if a.search == nil {
		return []SearchPaper{}, "搜索服务当前不可用。", stats
	}

	results, err := a.search.Search(query, limit)
	stats = a.search.LastSearchStats()
	if strings.TrimSpace(stats.Query) == "" {
		stats.Query = strings.TrimSpace(query)
	}
	if stats.RawCount == 0 && len(results) > 0 {
		stats.RawCount = len(results)
	}
	if stats.DedupCount == 0 && len(results) > 0 {
		stats.DedupCount = len(results)
	}
	if stats.FinalCount == 0 && len(results) > 0 {
		stats.FinalCount = len(results)
	}
	if err != nil {
		return []SearchPaper{}, fmt.Sprintf("本轮检索暂时失败：%v", err), stats
	}
	return results, "", stats
}

func (a *App) deepStartResultLimit() int {
	limit := a.config.Search.DeepStartResultLimit
	if limit < 100 {
		limit = 200
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
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "deepstart-progress", progress)
}

func (a *App) generateDeepStartAnalysis(
	summary DeepStartSessionSummary,
	messages []DeepStartMessage,
	results []SearchPaper,
	targetFolderName,
	searchWarning string,
	searchStats SearchRetrievalStats,
) (string, *DeepStartAnalysis, string) {
	request := DeepStartAIRequest{
		RootPrompt:       summary.RootPrompt,
		CurrentQuery:     summary.CurrentQuery,
		TargetFolderName: targetFolderName,
		Messages:         trimDeepStartMessages(messages, 12),
		Results:          results,
	}

	var (
		title    string
		analysis DeepStartAnalysis
	)

	shouldSkipLLM := len(results) == 0 && strings.TrimSpace(searchWarning) != ""
	if a.llm != nil && !shouldSkipLLM {
		if response, err := a.llm.AnalyzeDeepStart(request); err == nil {
			title = response.Title
			analysis = response.Analysis
			if searchWarning != "" {
				if strings.TrimSpace(analysis.Overview) == "" {
					analysis.Overview = searchWarning
				} else {
					analysis.Overview = strings.TrimSpace(analysis.Overview) + " " + searchWarning
				}
			}
		} else {
			analysis = buildFallbackDeepStartAnalysis(summary.RootPrompt, summary.CurrentQuery, results, err.Error(), searchWarning, searchStats)
		}
	} else {
		aiWarning := "AI 辅助暂不可用"
		if shouldSkipLLM {
			aiWarning = ""
		}
		analysis = buildFallbackDeepStartAnalysis(summary.RootPrompt, summary.CurrentQuery, results, aiWarning, searchWarning, searchStats)
	}
	analysis.SearchStats = normalizeDeepStartSearchStats(searchStats, summary.CurrentQuery, len(results))
	if statsLine := formatDeepStartSearchStats(analysis.SearchStats); statsLine != "" {
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

	return title, &analysis, assistantContent
}

func trimDeepStartMessages(messages []DeepStartMessage, limit int) []DeepStartMessage {
	if len(messages) <= limit {
		return messages
	}
	return append([]DeepStartMessage{}, messages[len(messages)-limit:]...)
}

func buildFallbackDeepStartAnalysis(
	rootPrompt,
	currentQuery string,
	results []SearchPaper,
	aiWarning,
	searchWarning string,
	searchStats SearchRetrievalStats,
) DeepStartAnalysis {
	query := strings.TrimSpace(currentQuery)
	if query == "" {
		query = strings.TrimSpace(rootPrompt)
	}

	recommended := rankDeepStartResults(results)
	questions := compactStrings([]string{
		fmt.Sprintf("如果你更看重综述和全景地图，我可以优先帮你挑 `%s` 方向的 survey 吗？", query),
		"你更想先看代表性方法、最新进展，还是落地应用？",
		"你希望我优先筛掉过旧、过泛，还是偏工程实现的论文？",
	}, 3)

	suggestedQueries := compactStrings([]string{
		query + " survey",
		query + " benchmark",
		query + " recent progress",
		query + " tutorial",
	}, 4)

	var overviewParts []string
	switch {
	case len(results) == 0 && searchWarning != "":
		overviewParts = append(overviewParts, "这一轮暂时没有检索到稳定结果。")
	case len(results) == 0:
		overviewParts = append(overviewParts, "这一轮还没有拿到候选论文。")
	default:
		overviewParts = append(overviewParts, fmt.Sprintf("我先把当前检索到的 %d 篇候选论文整理成一个可继续收窄的阅读面板。", len(results)))
	}
	if searchWarning != "" {
		overviewParts = append(overviewParts, searchWarning)
	}
	if aiWarning != "" {
		overviewParts = append(overviewParts, fmt.Sprintf("AI 辅助当前走降级路径：%s。你仍然可以继续手动筛选和导入。", aiWarning))
	}

	directions := buildFallbackDeepStartDirections(results, recommended)
	notes := buildFallbackDeepStartNotes(results, directions)

	return DeepStartAnalysis{
		Overview:            strings.Join(overviewParts, " "),
		Directions:          directions,
		PaperNotes:          notes,
		FollowUpQuestions:   questions,
		SuggestedQueries:    suggestedQueries,
		RecommendedPaperIDs: recommended,
		SearchStats:         normalizeDeepStartSearchStats(searchStats, currentQuery, len(results)),
	}
}

func rankDeepStartResults(results []SearchPaper) []string {
	type rankedPaper struct {
		ID    string
		Score int
	}

	ranked := make([]rankedPaper, 0, len(results))
	for index, paper := range results {
		score := 100 - index
		title := strings.ToLower(strings.TrimSpace(paper.Title))
		if strings.Contains(title, "survey") || strings.Contains(title, "review") || strings.Contains(title, "overview") {
			score += 40
		}
		if strings.Contains(title, "benchmark") || strings.Contains(title, "leaderboard") {
			score += 20
		}
		score += paper.Year
		ranked = append(ranked, rankedPaper{ID: paper.ID, Score: score})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	recommended := make([]string, 0, minInt(5, len(ranked)))
	for _, paper := range ranked {
		if strings.TrimSpace(paper.ID) == "" {
			continue
		}
		recommended = append(recommended, paper.ID)
		if len(recommended) >= 5 {
			break
		}
	}

	return recommended
}

func buildFallbackDeepStartDirections(results []SearchPaper, recommended []string) []DeepStartDirection {
	if len(results) == 0 {
		return []DeepStartDirection{
			{
				ID:       "refocus",
				Name:     "重新聚焦问题",
				Summary:  "先把检索词收窄到 survey、benchmark、recent progress 之类的词组，更容易拿到稳定结果。",
				Why:      "当前没有候选论文，先优化问题表述最有效。",
				PaperIDs: []string{},
			},
		}
	}

	categoryBuckets := map[string][]string{}
	categoryOrder := []string{}
	for _, paper := range results {
		category := strings.TrimSpace(paper.Category)
		if category == "" {
			continue
		}
		if _, exists := categoryBuckets[category]; !exists {
			categoryOrder = append(categoryOrder, category)
		}
		categoryBuckets[category] = append(categoryBuckets[category], paper.ID)
	}

	directions := make([]DeepStartDirection, 0, 3)
	if len(categoryOrder) >= 2 {
		for index, category := range categoryOrder {
			paperIDs := uniqueStrings(categoryBuckets[category])
			if len(paperIDs) == 0 {
				continue
			}
			directions = append(directions, DeepStartDirection{
				ID:       fmt.Sprintf("category-%d", index+1),
				Name:     category,
				Summary:  "这一组候选更接近同一条技术支线。",
				Why:      "先按主题拆开看，更容易判断该不该入库。",
				PaperIDs: paperIDs,
			})
			if len(directions) >= 3 {
				break
			}
		}
	}

	if len(directions) == 0 {
		directions = append(directions, DeepStartDirection{
			ID:       "priority",
			Name:     "优先阅读",
			Summary:  "先从更可能搭建领域地图的论文开始。",
			Why:      "这样能最快建立这轮探索的主线。",
			PaperIDs: recommended,
		})

		var remaining []string
		recommendedSet := make(map[string]struct{}, len(recommended))
		for _, id := range recommended {
			recommendedSet[id] = struct{}{}
		}
		for _, paper := range results {
			if _, exists := recommendedSet[paper.ID]; exists {
				continue
			}
			remaining = append(remaining, paper.ID)
		}
		if len(remaining) > 0 {
			directions = append(directions, DeepStartDirection{
				ID:       "extended",
				Name:     "扩展阅读",
				Summary:  "这些候选适合在主线确认后再补充。",
				Why:      "可以避免第一次筛选时信息过载。",
				PaperIDs: remaining,
			})
		}
	}

	return directions
}

func buildFallbackDeepStartNotes(results []SearchPaper, directions []DeepStartDirection) []DeepStartPaperNote {
	directionByPaper := make(map[string][]string)
	for _, direction := range directions {
		for _, paperID := range direction.PaperIDs {
			directionByPaper[paperID] = append(directionByPaper[paperID], direction.ID)
		}
	}

	notes := make([]DeepStartPaperNote, 0, len(results))
	for index, paper := range results {
		tier := "optional"
		if index == 0 {
			tier = "core"
		} else if index < 4 {
			tier = "important"
		}

		reason := "适合作为补充阅读。"
		title := strings.ToLower(paper.Title)
		switch {
		case strings.Contains(title, "survey") || strings.Contains(title, "review") || strings.Contains(title, "overview"):
			reason = "标题看起来更像综述或全景材料，适合先建立整体地图。"
			tier = "core"
		case strings.Contains(title, "benchmark") || strings.Contains(title, "leaderboard"):
			reason = "更适合用来判断这个方向的评测基线和对比方式。"
		case paper.Year >= time.Now().Year()-1:
			reason = "相对较新，适合快速补最近一轮进展。"
		default:
			reason = "能帮助你补齐这轮主题里的代表性样本。"
		}

		notes = append(notes, DeepStartPaperNote{
			PaperID:      paper.ID,
			Tier:         tier,
			Reason:       reason,
			DirectionIDs: uniqueStrings(directionByPaper[paper.ID]),
		})
	}

	return notes
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

func formatDeepStartSearchStats(stats SearchRetrievalStats) string {
	if stats.RawCount == 0 && stats.DedupCount == 0 && stats.FinalCount == 0 {
		return ""
	}
	return fmt.Sprintf(
		"检索统计：原始候选 %d 篇，去重后 %d 篇，本轮进入探索区 %d 篇。",
		stats.RawCount,
		stats.DedupCount,
		stats.FinalCount,
	)
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
