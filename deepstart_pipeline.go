package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const deepStartBatchDefaultPerSourceLimit = 20
const deepStartInitialReadyLimit = 20
const deepStartEagerPreprocessLimit = 4

var deepStartCacheNamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type deepStartBatchStats struct {
	Total      int
	Completed  int
	Success    int
	Failed     int
	NoPDFURL   int
	Downloaded int
	Parsed     int
	Extracted  int
}

type scoredDeepStartPaper struct {
	paper SearchPaper
	score int
}

func splitDeepStartInitialBatch(results []SearchPaper, query string, limit int) ([]SearchPaper, []SearchPaper) {
	if len(results) == 0 {
		return []SearchPaper{}, []SearchPaper{}
	}
	if limit <= 0 {
		limit = deepStartInitialReadyLimit
	}
	if len(results) <= limit {
		return append([]SearchPaper{}, results...), []SearchPaper{}
	}

	tokens := extractDeepStartMessageTokens(query)
	scored := make([]scoredDeepStartPaper, 0, len(results))
	for _, paper := range results {
		scored = append(scored, scoredDeepStartPaper{
			paper: paper,
			score: deepStartInitialPaperScore(paper, tokens),
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].paper.Year > scored[j].paper.Year
		}
		return scored[i].score > scored[j].score
	})

	initial := make([]SearchPaper, 0, limit)
	remaining := make([]SearchPaper, 0, len(results)-limit)
	for idx, item := range scored {
		if idx < limit {
			initial = append(initial, item.paper)
		} else {
			remaining = append(remaining, item.paper)
		}
	}
	return initial, remaining
}

func deepStartInitialPaperScore(paper SearchPaper, tokens []string) int {
	score := searchPaperQualityScore(paper)
	if paper.Year > 0 {
		score += (paper.Year - 2015) * 3
	}
	if paper.CitationCount > 0 {
		score += minInt(paper.CitationCount, 200)
	}
	if strings.TrimSpace(paper.URL) != "" || len(paper.PDFCandidates) > 0 {
		score += 24
	}

	title := strings.ToLower(strings.TrimSpace(paper.Title))
	if strings.Contains(title, "survey") || strings.Contains(title, "review") || strings.Contains(title, "benchmark") {
		score += 12
	}
	if len(tokens) > 0 {
		score += deepStartPaperMatchScore(paper, tokens)
	}
	return score
}

func fillSearchPaperClassification(paper *SearchPaper, profile *PaperProfileExtraction) {
	if paper == nil {
		return
	}

	keywordFallback := func(defaultValue string) string {
		defaultValue = strings.TrimSpace(defaultValue)
		if defaultValue != "" {
			return defaultValue
		}
		if len(paper.Keywords) > 0 {
			return strings.TrimSpace(paper.Keywords[0])
		}
		if len(paper.Tags) > 0 {
			return strings.TrimSpace(paper.Tags[0])
		}
		return ""
	}

	if profile != nil {
		paper.TopicLabel = keywordFallback(profile.TopicLabel)
		paper.MethodLabel = keywordFallback(profile.MethodLabel)
		paper.TaskLabel = keywordFallback(profile.TaskLabel)
		paper.DomainLabel = keywordFallback(profile.DomainLabel)
		paper.ClassificationConfidence = profile.ClassificationConfidence
	}

	if strings.TrimSpace(paper.TopicLabel) == "" {
		paper.TopicLabel = keywordFallback(paper.Category)
	}
	if strings.TrimSpace(paper.MethodLabel) == "" {
		paper.MethodLabel = keywordFallback(paper.Method)
	}
	if strings.TrimSpace(paper.TaskLabel) == "" {
		paper.TaskLabel = keywordFallback(paper.Problem)
	}
	if strings.TrimSpace(paper.DomainLabel) == "" {
		paper.DomainLabel = keywordFallback(paper.Journal)
	}

	paper.TopicLabel = strings.TrimSpace(paper.TopicLabel)
	paper.MethodLabel = strings.TrimSpace(paper.MethodLabel)
	paper.TaskLabel = strings.TrimSpace(paper.TaskLabel)
	paper.DomainLabel = strings.TrimSpace(paper.DomainLabel)

	if paper.ClassificationConfidence <= 0 {
		if strings.TrimSpace(paper.TopicLabel) != "" ||
			strings.TrimSpace(paper.MethodLabel) != "" ||
			strings.TrimSpace(paper.TaskLabel) != "" ||
			strings.TrimSpace(paper.DomainLabel) != "" {
			paper.ClassificationConfidence = 0.55
		}
	}
	if paper.ClassificationConfidence < 0 {
		paper.ClassificationConfidence = 0
	}
	if paper.ClassificationConfidence > 1 {
		paper.ClassificationConfidence = 1
	}
}

func splitDeepStartEagerPreprocess(papers []SearchPaper, limit int) ([]SearchPaper, []SearchPaper) {
	if limit <= 0 {
		limit = deepStartEagerPreprocessLimit
	}
	if len(papers) <= limit {
		return append([]SearchPaper{}, papers...), []SearchPaper{}
	}

	eager := append([]SearchPaper{}, papers[:limit]...)
	deferred := append([]SearchPaper{}, papers[limit:]...)
	for index := range deferred {
		deferred[index].PreprocessStatus = "deferred"
		deferred[index].ParseStatus = "deferred"
		deferred[index].ExtractStatus = "deferred"
		deferred[index].ProcessingStage = "ready"
		deferred[index].ProcessingError = ""
		deferred[index].ParseError = ""
		deferred[index].ExtractError = ""
		fillSearchPaperClassification(&deferred[index], nil)
	}
	return eager, deferred
}

func (a *App) searchDeepStartResultsWithPerSourceLimit(
	ctx context.Context,
	query string,
	limit int,
	perSourceLimit int,
) ([]SearchPaper, string, SearchRetrievalStats, error) {
	results, warning, stats, err := a.deepStartSearchWithRewrittenQueries(ctx, query, limit, perSourceLimit)
	if err != nil {
		if isDeepStartCancelledError(err) {
			return nil, "", stats, ErrDeepStartTaskCancelled
		}
		return nil, "", stats, err
	}
	return results, strings.TrimSpace(warning), stats, nil
}

func (a *App) preprocessDeepStartResults(
	ctx context.Context,
	sessionID string,
	startedAt time.Time,
	papers []SearchPaper,
	stats SearchRetrievalStats,
) ([]SearchPaper, deepStartBatchStats, error) {
	eagerPapers, deferredPapers := splitDeepStartEagerPreprocess(papers, deepStartEagerPreprocessLimit)
	batch := deepStartBatchStats{
		Total: len(eagerPapers),
	}
	if len(eagerPapers) == 0 {
		return papers, batch, nil
	}

	cacheSessionID := safeSyncSegment(sessionID)
	cacheRoot := filepath.Join(a.config.DataPath, "deepstart_cache")

	processed := make([]SearchPaper, 0, len(papers))
	weak := a.currentWeakLLM()
	for idx := range eagerPapers {
		if err := ctx.Err(); err != nil {
			return nil, batch, err
		}

		paper := eagerPapers[idx]
		cacheID := deepStartCachePaperID(paper, idx)
		pdfPath, err := ensureManagedFileParent(
			cacheRoot,
			filepath.Join(cacheRoot, cacheSessionID, "pdf", cacheID+".pdf"),
		)
		if err != nil {
			return processed, batch, err
		}
		markdownPath, err := ensureManagedFileParent(
			cacheRoot,
			filepath.Join(cacheRoot, cacheSessionID, "markdown", cacheID+".md"),
		)
		if err != nil {
			return processed, batch, err
		}

		paper.PreprocessStatus = "pending"
		paper.ParseStatus = "pending"
		paper.ExtractStatus = "pending"
		paper.ProcessingStage = "pending"
		paper.ProcessingError = ""
		paper.LocalPDFPath = ""
		paper.MarkdownPath = ""
		paper.ParseError = ""
		paper.ExtractError = ""

		candidates := buildPDFCandidateURLs(
			strings.TrimSpace(paper.URL),
			normalizeSourcePaperID(paper),
			normalizeExternalIDMap(paper.ExternalIDs),
			compactStrings(paper.PDFCandidates, 12),
		)
		if len(candidates) == 0 {
			batch.NoPDFURL++
			batch.Failed++
			batch.Completed++
			paper.PreprocessStatus = "failed"
			paper.ParseStatus = "skipped"
			paper.ExtractStatus = "skipped"
			paper.ParseError = "no downloadable pdf url"
			paper.ExtractError = "skip extract because parse skipped"
			paper.ProcessingStage = "failed"
			paper.ProcessingError = "no downloadable pdf url"

			a.emitDeepStartBatchProgress(
				sessionID,
				"downloading",
				fmt.Sprintf("论文 %d/%d 缺少可下载 PDF 链接，已跳过", batch.Completed, batch.Total),
				startedAt,
				batch,
				stats,
			)
			processed = append(processed, paper)
			continue
		}

		a.emitDeepStartBatchProgress(
			sessionID,
			"downloading",
			fmt.Sprintf("正在下载论文 %d/%d", batch.Completed+1, batch.Total),
			startedAt,
			batch,
			stats,
		)
		if err := deepStartDownloadWithCandidates(ctx, candidates, pdfPath); err != nil {
			if isDeepStartCancelledError(err) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return processed, batch, err
			}
			batch.Failed++
			batch.Completed++
			paper.PreprocessStatus = "failed"
			paper.ParseStatus = "failed"
			paper.ExtractStatus = "skipped"
			paper.ParseError = err.Error()
			paper.ExtractError = "skip extract because parse failed"
			paper.ProcessingStage = "failed"
			paper.ProcessingError = err.Error()

			a.emitDeepStartBatchProgress(
				sessionID,
				"downloading",
				fmt.Sprintf("论文 %d/%d 下载失败：%s", batch.Completed, batch.Total, trimErrorForProgress(err)),
				startedAt,
				batch,
				stats,
			)
			processed = append(processed, paper)
			continue
		}

		batch.Downloaded++
		paper.PreprocessStatus = "downloaded"
		paper.LocalPDFPath = pdfPath
		paper.ProcessingStage = "downloading"

		if a.pdfService == nil {
			batch.Failed++
			batch.Completed++
			paper.PreprocessStatus = "failed"
			paper.ParseStatus = "failed"
			paper.ExtractStatus = "skipped"
			paper.ParseError = "pdf service client is not initialized"
			paper.ExtractError = "skip extract because parse failed"
			paper.ProcessingStage = "failed"
			paper.ProcessingError = paper.ParseError
			processed = append(processed, paper)
			continue
		}

		a.emitDeepStartBatchProgress(
			sessionID,
			"parsing",
			fmt.Sprintf("正在解析论文 %d/%d", batch.Completed+1, batch.Total),
			startedAt,
			batch,
			stats,
		)
		parseResult, err := a.pdfService.ParsePDFWithContext(ctx, pdfPath)
		if err != nil {
			if isDeepStartCancelledError(err) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return processed, batch, err
			}
			batch.Failed++
			batch.Completed++
			paper.PreprocessStatus = "failed"
			paper.ParseStatus = "failed"
			paper.ExtractStatus = "skipped"
			paper.ParseError = err.Error()
			paper.ExtractError = "skip extract because parse failed"
			paper.ProcessingStage = "failed"
			paper.ProcessingError = err.Error()
			processed = append(processed, paper)
			continue
		}

		markdown := strings.TrimSpace(parseResult.Markdown)
		if markdown == "" {
			batch.Failed++
			batch.Completed++
			paper.PreprocessStatus = "failed"
			paper.ParseStatus = "failed"
			paper.ExtractStatus = "skipped"
			paper.ParseError = "empty markdown from parser"
			paper.ExtractError = "skip extract because parse failed"
			paper.ProcessingStage = "failed"
			paper.ProcessingError = paper.ParseError
			processed = append(processed, paper)
			continue
		}

		if err := writeFileAtomic(markdownPath, []byte(parseResult.Markdown), 0600); err != nil {
			batch.Failed++
			batch.Completed++
			paper.PreprocessStatus = "failed"
			paper.ParseStatus = "failed"
			paper.ExtractStatus = "skipped"
			paper.ParseError = fmt.Sprintf("failed to persist markdown cache: %v", err)
			paper.ExtractError = "skip extract because parse failed"
			paper.ProcessingStage = "failed"
			paper.ProcessingError = paper.ParseError
			processed = append(processed, paper)
			continue
		}

		batch.Parsed++
		paper.PreprocessStatus = "parsed"
		paper.ParseStatus = "success"
		paper.MarkdownPath = markdownPath
		paper.Title = firstNonBlankString(strings.TrimSpace(paper.Title), metadataString(parseResult.Metadata, "title"))
		paper.ProcessingStage = "parsing"

		if weak == nil {
			paper.ExtractStatus = "skipped"
			paper.ExtractError = "weak llm unavailable"
			paper.PreprocessStatus = "parsed"
			fillSearchPaperClassification(&paper, nil)
			paper.ProcessingStage = "ready"
			paper.ProcessingError = ""
			batch.Success++
			batch.Completed++
			processed = append(processed, paper)
			continue
		}

		a.emitDeepStartBatchProgress(
			sessionID,
			"weak_extracting",
			fmt.Sprintf("正在弱模型抽取论文 %d/%d", batch.Completed+1, batch.Total),
			startedAt,
			batch,
			stats,
		)
		profile, err := extractPaperProfileWithContext(ctx, weak, markdown)
		if err != nil {
			if isDeepStartCancelledError(err) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return processed, batch, err
			}
			batch.Failed++
			batch.Completed++
			paper.PreprocessStatus = "failed"
			paper.ExtractStatus = "failed"
			paper.ExtractError = err.Error()
			paper.ProcessingStage = "failed"
			paper.ProcessingError = err.Error()
			processed = append(processed, paper)
			continue
		}

		paper.Title = firstNonBlankString(strings.TrimSpace(profile.Title), strings.TrimSpace(paper.Title))
		if strings.TrimSpace(paper.Abstract) == "" {
			paper.Abstract = strings.TrimSpace(profile.Abstract)
		}
		paper.Problem = strings.TrimSpace(profile.Problem)
		paper.Method = strings.TrimSpace(profile.Method)
		paper.Keywords = uniqueStrings(append(paper.Keywords, profile.Keywords...))
		paper.Tags = uniqueStrings(append(paper.Tags, profile.RelevanceTags...))
		fillSearchPaperClassification(&paper, profile)
		paper.ExtractStatus = "success"
		paper.ExtractError = ""
		paper.PreprocessStatus = "extracted"
		paper.ProcessingStage = "ready"
		paper.ProcessingError = ""

		batch.Extracted++
		batch.Success++
		batch.Completed++
		processed = append(processed, paper)
	}

	a.emitDeepStartBatchProgress(
		sessionID,
		"weak_extracting",
		fmt.Sprintf("批处理已完成：成功 %d，失败 %d", batch.Success, batch.Failed),
		startedAt,
		batch,
		stats,
	)
	processed = append(processed, deferredPapers...)
	return processed, batch, nil
}

func (a *App) emitDeepStartBatchProgress(
	sessionID string,
	phase string,
	message string,
	startedAt time.Time,
	batch deepStartBatchStats,
	stats SearchRetrievalStats,
) {
	total := batch.Total
	if total <= 0 {
		total = 1
	}
	completed := batch.Completed
	if completed > total {
		completed = total
	}

	base := 20
	span := 58
	switch phase {
	case "downloading":
		base, span = 20, 18
	case "parsing":
		base, span = 38, 20
	case "weak_extracting":
		base, span = 58, 20
	}

	overall := base + int(float64(completed)/float64(total)*float64(span))
	if overall > 80 {
		overall = 80
	}

	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     phase,
		Message:                   strings.TrimSpace(message),
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: estimateDeepStartETAWithElapsed(batch, time.Since(startedAt)),
		Total:                     batch.Total,
		Completed:                 batch.Completed,
		OverallPercent:            overall,
		SuccessCount:              batch.Success,
		FailedCount:               batch.Failed,
		NoPDFURLCount:             batch.NoPDFURL,
		DownloadedCount:           batch.Downloaded,
		ParsedCount:               batch.Parsed,
		ExtractedCount:            batch.Extracted,
		Stats:                     &stats,
	})
}

func estimateDeepStartETA(batch deepStartBatchStats) int {
	return estimateDeepStartETAWithElapsed(batch, 0)
}

func estimateDeepStartETAWithElapsed(batch deepStartBatchStats, elapsed time.Duration) int {
	remaining := batch.Total - batch.Completed
	if remaining <= 0 {
		return 0
	}
	secondsPerPaper := 20
	if batch.Completed > 0 && elapsed > 0 {
		secondsPerPaper = int(elapsed.Seconds()) / batch.Completed
		if secondsPerPaper < 5 {
			secondsPerPaper = 5
		}
		if secondsPerPaper > 90 {
			secondsPerPaper = 90
		}
	}
	eta := remaining * secondsPerPaper
	if eta < 1 {
		return 1
	}
	return eta
}

func trimErrorForProgress(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if utf8Len(message) <= 120 {
		return message
	}
	runes := []rune(message)
	return string(runes[:120]) + "..."
}

var deepStartDownloadWithCandidates = downloadWithCandidates

func downloadWithCandidates(ctx context.Context, candidates []string, targetPath string) error {
	if len(candidates) == 0 {
		return fmt.Errorf("no downloadable pdf url")
	}

	var lastErr error
	for attempt := 1; attempt <= pdfDownloadMaxRetry; attempt++ {
		for _, candidate := range candidates {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := downloadPDFToFile(ctx, candidate, targetPath); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}

		if attempt < pdfDownloadMaxRetry {
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			if err := sleepWithContext(ctx, backoff); err != nil {
				return err
			}
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown download error")
	}
	return lastErr
}

func deepStartCachePaperID(paper SearchPaper, idx int) string {
	parts := []string{
		strings.TrimSpace(paper.ID),
		strings.TrimSpace(normalizeSourcePaperID(paper)),
		strings.TrimSpace(dedupeSearchPaperKey(paper)),
		fmt.Sprintf("paper-%d", idx+1),
	}
	raw := ""
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			raw = part
			break
		}
	}
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "/", "_")
	raw = strings.ReplaceAll(raw, "\\", "_")
	raw = deepStartCacheNamePattern.ReplaceAllString(raw, "_")
	raw = strings.Trim(raw, "._- ")
	if raw == "" {
		raw = fmt.Sprintf("paper-%d", idx+1)
	}
	if len(raw) > 96 {
		raw = raw[:96]
	}
	return raw
}

func mergeSearchPaperPools(existing []SearchPaper, added []SearchPaper) []SearchPaper {
	merged := make(map[string]SearchPaper, len(existing)+len(added))
	order := make([]string, 0, len(existing)+len(added))
	appendPaper := func(paper SearchPaper) {
		key := dedupeSearchPaperKey(paper)
		if key == "" {
			key = strings.TrimSpace(paper.ID)
		}
		if key == "" {
			key = strings.TrimSpace(paper.URL)
		}
		if key == "" {
			key = fmt.Sprintf("fallback-%d", len(order)+1)
		}

		if prev, ok := merged[key]; ok {
			if deepStartPaperPoolScore(paper) > deepStartPaperPoolScore(prev) {
				merged[key] = paper
			}
			return
		}
		merged[key] = paper
		order = append(order, key)
	}

	for _, paper := range existing {
		appendPaper(paper)
	}
	for _, paper := range added {
		appendPaper(paper)
	}

	result := make([]SearchPaper, 0, len(order))
	for _, key := range order {
		result = append(result, merged[key])
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Year > result[j].Year
	})
	return result
}

func deepStartPaperPoolScore(paper SearchPaper) int {
	score := searchPaperQualityScore(paper)
	switch strings.TrimSpace(paper.PreprocessStatus) {
	case "extracted":
		score += 60
	case "parsed":
		score += 40
	case "downloaded":
		score += 20
	}
	if strings.TrimSpace(paper.MarkdownPath) != "" {
		score += 6
	}
	if strings.TrimSpace(paper.Problem) != "" {
		score += 4
	}
	if strings.TrimSpace(paper.Method) != "" {
		score += 4
	}
	return score
}

func (a *App) SupplementDeepStartSearch(sessionID, query string, perSourceLimit int) (*DeepStartSessionDetail, error) {
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
	if perSourceLimit <= 0 {
		perSourceLimit = deepStartBatchDefaultPerSourceLimit
	}
	if perSourceLimit > 100 {
		perSourceLimit = 100
	}

	detail, err := a.db.GetDeepStartSession(sessionID)
	if err != nil {
		return nil, err
	}

	taskCtx, taskToken, err := a.beginDeepStartTask(sessionID)
	if err != nil {
		return nil, err
	}
	defer a.finishDeepStartTask(sessionID, taskToken)

	startedAt := time.Now()
	abortIfCancelled := func(message string, stats *SearchRetrievalStats) error {
		return a.abortDeepStartIfCancelled(taskCtx, sessionID, startedAt, stats, message)
	}

	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:      sessionID,
		Phase:          "searching",
		Message:        fmt.Sprintf("正在发起补充检索（每源 %d 篇）", perSourceLimit),
		ElapsedSeconds: 0,
		Total:          1,
		Completed:      0,
		OverallPercent: 3,
	})

	limit := perSourceLimit * 2
	if limit < 20 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	added, searchWarning, searchStats, searchErr := a.searchDeepStartResultsWithPerSourceLimit(taskCtx, query, limit, perSourceLimit)
	if searchErr != nil {
		if isDeepStartCancelledError(searchErr) {
			return nil, abortIfCancelled("补充检索已停止，原会话保持不变", &searchStats)
		}
		return nil, searchErr
	}
	if err := abortIfCancelled("补充检索已停止，原会话保持不变", &searchStats); err != nil {
		return nil, err
	}

	if len(added) > 0 {
		enriched, enrichErr := a.enrichDeepStartResults(taskCtx, sessionID, startedAt, query, added, searchStats)
		if enrichErr != nil {
			if isDeepStartCancelledError(enrichErr) || isDeepStartCancelledError(taskCtx.Err()) {
				return nil, abortIfCancelled("补充检索已停止，原会话保持不变", &searchStats)
			}
			if strings.TrimSpace(searchWarning) == "" {
				searchWarning = fmt.Sprintf("机构补全阶段发生部分失败：%v", enrichErr)
			}
		} else {
			added = enriched
		}
	}

	processed, batch, batchErr := a.preprocessDeepStartResults(taskCtx, sessionID, startedAt, added, searchStats)
	if batchErr != nil {
		if isDeepStartCancelledError(batchErr) || isDeepStartCancelledError(taskCtx.Err()) {
			return nil, abortIfCancelled("补充检索已停止，原会话保持不变", &searchStats)
		}
		if strings.TrimSpace(searchWarning) == "" {
			searchWarning = fmt.Sprintf("补充批处理阶段发生部分失败：%v", batchErr)
		}
	} else {
		if batch.Failed > 0 {
			warn := fmt.Sprintf("补充批处理完成：成功 %d，失败 %d（无链接 %d）", batch.Success, batch.Failed, batch.NoPDFURL)
			if strings.TrimSpace(searchWarning) == "" {
				searchWarning = warn
			} else {
				searchWarning = strings.TrimSpace(searchWarning) + " " + warn
			}
		}
		added = processed
	}

	merged := mergeSearchPaperPools(detail.CurrentResults, added)
	detail.CurrentResults = merged
	detail.SelectedPaperIDs = normalizeSelectedPaperIDs(detail.SelectedPaperIDs, merged)
	detail.Summary.CurrentQuery = query
	detail.Summary.UpdatedAt = time.Now()

	userMessage := DeepStartMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   fmt.Sprintf("补充检索：%s（每源 %d）", query, perSourceLimit),
		CreatedAt: time.Now(),
	}
	messages := append(append([]DeepStartMessage{}, detail.Messages...), userMessage)
	targetFolderName, _ := a.folderNameByID(detail.Summary.TargetFolderID)

	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "analyzing",
		Message:                   "补充检索已合并，正在生成新的 AI 建议",
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     len(detail.CurrentResults),
		Completed:                 len(detail.CurrentResults),
		OverallPercent:            88,
		Stats:                     &searchStats,
	})

	analysisTitle, analysis, assistantContent := a.generateDeepStartAnalysis(
		taskCtx,
		detail.Summary,
		messages,
		detail.CurrentResults,
		targetFolderName,
		searchWarning,
		searchStats,
	)
	detail.Summary.Title = analysisTitle
	detail.CurrentAnalysis = analysis

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
	if err := a.db.UpsertDeepStartSession(detail); err != nil {
		return nil, err
	}
	if err := a.db.SaveDeepStartSearchRound(sessionID, query, detail.CurrentResults, detail.CurrentAnalysis); err != nil {
		return nil, err
	}

	a.emitDeepStartProgress(DeepStartProgressEvent{
		SessionID:                 sessionID,
		Phase:                     "completed",
		Message:                   fmt.Sprintf("补充检索完成，当前候选池 %d 篇", len(detail.CurrentResults)),
		ElapsedSeconds:            int(time.Since(startedAt).Seconds()),
		EstimatedRemainingSeconds: 0,
		Total:                     len(detail.CurrentResults),
		Completed:                 len(detail.CurrentResults),
		OverallPercent:            100,
		Stats:                     &searchStats,
	})

	return a.db.GetDeepStartSession(sessionID)
}
