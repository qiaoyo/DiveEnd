package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const screeningCompletionThreshold = 5

type storedScreeningContent struct {
	Sections      []string             `json:"sections"`
	ParseMetadata map[string]any       `json:"parseMetadata,omitempty"`
	Metadata      *PDFExtractMetadata  `json:"metadata,omitempty"`
	Metrics       []PDFExtractMetric   `json:"metrics,omitempty"`
	Baselines     []PDFExtractBaseline `json:"baselines,omitempty"`
	RelevanceTags []string             `json:"relevanceTags,omitempty"`
}

func (a *App) CreateScreeningSession(title string) (*ScreeningSession, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	title = strings.TrimSpace(title)
	if title == "" {
		title = fmt.Sprintf("Screening %s", time.Now().Format("2006-01-02 15:04"))
	}

	session := &ScreeningSession{
		ID:                  uuid.NewString(),
		Title:               title,
		Status:              "upload",
		TotalPapers:         0,
		SelectedOptionsJSON: "[]",
		PathHistoryJSON:     "[]",
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if err := a.db.UpsertScreeningSession(session); err != nil {
		return nil, err
	}

	return session, nil
}

func (a *App) UploadScreeningFiles(sessionID string, filePaths []string) (*ScreeningSessionDetail, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID is required")
	}
	if _, err := a.db.GetScreeningSession(sessionID); err != nil {
		return nil, err
	}

	normalizedPaths := normalizeScreeningPaths(filePaths)
	if len(normalizedPaths) == 0 {
		return nil, fmt.Errorf("no valid PDF file paths provided")
	}

	var papers []ScreeningPaper
	now := time.Now()
	for _, path := range normalizedPaths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to access %s: %w", path, err)
		}
		if info.IsDir() {
			continue
		}

		papers = append(papers, ScreeningPaper{
			ID:        uuid.NewString(),
			SessionID: sessionID,
			FileName:  filepath.Base(path),
			FilePath:  path,
			FileSize:  info.Size(),
			Status:    "pending",
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	if len(papers) == 0 {
		return nil, fmt.Errorf("no PDF files were selected")
	}

	if err := a.db.BatchUpsertScreeningPapers(papers); err != nil {
		return nil, err
	}
	if err := a.db.UpdateScreeningSessionStatus(sessionID, "extract", len(papers)); err != nil {
		return nil, err
	}

	return a.db.GetScreeningSessionDetail(sessionID)
}

func (a *App) ExtractPaperContent(sessionID string) (*ExtractProgress, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.pdfService == nil {
		return nil, fmt.Errorf("pdf service client is not initialized")
	}

	detail, err := a.db.GetScreeningSessionDetail(sessionID)
	if err != nil {
		return nil, err
	}
	if len(detail.Papers) == 0 {
		return nil, fmt.Errorf("no screening papers found")
	}

	progress := &ExtractProgress{
		SessionID: sessionID,
		Total:     len(detail.Papers),
		Status:    "processing",
	}
	for _, paper := range detail.Papers {
		if paper.Status == "extracted" || paper.Status == "selected" {
			progress.Completed++
		}
	}
	a.storeExtractProgress(progress)

	extractionLLMConfig := pdfExtractionLLMConfigForApp(a.config)
	var firstErr error
	var failedCount int

	for _, paper := range detail.Papers {
		if paper.Status == "extracted" || paper.Status == "selected" {
			continue
		}

		paper.Status = "extracting"
		paper.Reason = ""
		paper.UpdatedAt = time.Now()
		if err := a.db.UpsertScreeningPaper(&paper); err != nil {
			return nil, err
		}

		progress.CurrentFile = paper.FileName
		progress.Status = "processing"
		progress.ErrorMessage = ""
		a.storeExtractProgress(progress)

		parseResult, err := a.pdfService.ParsePDF(paper.FilePath)
		if err != nil {
			failedCount++
			if firstErr == nil {
				firstErr = err
			}
			paper.Status = "pending"
			paper.Reason = err.Error()
			paper.UpdatedAt = time.Now()
			if upsertErr := a.db.UpsertScreeningPaper(&paper); upsertErr != nil {
				return nil, upsertErr
			}
			continue
		}

		extractResult, err := a.pdfService.ExtractContent(parseResult.Markdown, extractionLLMConfig)
		if err != nil {
			failedCount++
			if firstErr == nil {
				firstErr = err
			}
			paper.Status = "pending"
			paper.Reason = err.Error()
			paper.UpdatedAt = time.Now()
			if upsertErr := a.db.UpsertScreeningPaper(&paper); upsertErr != nil {
				return nil, upsertErr
			}
			continue
		}

		contentJSON, err := buildStoredScreeningContent(parseResult, extractResult)
		if err != nil {
			failedCount++
			if firstErr == nil {
				firstErr = err
			}
			paper.Status = "pending"
			paper.Reason = err.Error()
			paper.UpdatedAt = time.Now()
			if upsertErr := a.db.UpsertScreeningPaper(&paper); upsertErr != nil {
				return nil, upsertErr
			}
			continue
		}

		paper.Status = "extracted"
		paper.Title = strings.TrimSpace(extractResult.Data.Metadata.Title)
		if paper.Title == "" {
			paper.Title = firstNonBlankString(metadataString(parseResult.Metadata, "title"), trimPDFSuffix(paper.FileName))
		}
		paper.Authors = strings.Join(extractResult.Data.Metadata.Authors, ", ")
		paper.Abstract = strings.TrimSpace(extractResult.Data.Metadata.Abstract)
		paper.FullText = parseResult.Markdown
		paper.SectionsJSON = contentJSON
		paper.Reason = ""
		paper.UpdatedAt = time.Now()
		if err := a.db.UpsertScreeningPaper(&paper); err != nil {
			return nil, err
		}

		progress.Completed++
		progress.CurrentFile = paper.FileName
		a.storeExtractProgress(progress)
	}

	if firstErr != nil {
		progress.Status = "error"
		progress.ErrorMessage = fmt.Sprintf("完成 %d/%d，失败 %d：%v", progress.Completed, progress.Total, failedCount, firstErr)
		a.storeExtractProgress(progress)
		return cloneExtractProgress(progress), fmt.Errorf(progress.ErrorMessage)
	}

	progress.Status = "completed"
	progress.ErrorMessage = ""
	progress.CurrentFile = ""
	a.storeExtractProgress(progress)
	return cloneExtractProgress(progress), nil
}

func (a *App) GetExtractProgress(sessionID string) (*ExtractProgress, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	if cached := a.loadExtractProgress(sessionID); cached != nil {
		return cached, nil
	}

	papers, err := a.db.GetScreeningPapers(sessionID)
	if err != nil {
		return nil, err
	}
	progress := &ExtractProgress{
		SessionID: sessionID,
		Total:     len(papers),
		Status:    "processing",
	}
	for _, paper := range papers {
		if paper.Status == "extracted" || paper.Status == "selected" {
			progress.Completed++
		}
		if paper.Status == "extracting" {
			progress.CurrentFile = paper.FileName
		}
	}
	if progress.Total == 0 {
		progress.Status = "error"
		progress.ErrorMessage = "no screening papers found"
	} else if progress.Completed == progress.Total {
		progress.Status = "completed"
	}

	return progress, nil
}

func (a *App) AnalyzePapers(sessionID string) (*ScreeningDecisionNode, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.currentStrongLLM() == nil {
		return nil, fmt.Errorf("LLM client is not initialized")
	}

	detail, err := a.db.GetScreeningSessionDetail(sessionID)
	if err != nil {
		return nil, err
	}

	candidates := screeningCandidatePapers(detail.Papers)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no extracted papers available for screening")
	}

	node, err := a.buildNextScreeningNode(detail.Session.Title, candidates, detail.PathHistory)
	if err != nil {
		return nil, err
	}

	if err := a.persistScreeningNode(sessionID, node, nil); err != nil {
		return nil, err
	}
	if err := a.db.UpdateScreeningSessionStatus(sessionID, "screen", detail.Session.TotalPapers); err != nil {
		return nil, err
	}
	for _, paper := range candidates {
		if paper.Status != "screening" {
			paper.Status = "screening"
			paper.UpdatedAt = time.Now()
			if err := a.db.UpsertScreeningPaper(&paper); err != nil {
				return nil, err
			}
		}
	}

	return node, nil
}

func (a *App) ApplyScreeningChoice(sessionID string, selectedOptions []string) (*ScreeningDecisionNode, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.currentStrongLLM() == nil {
		return nil, fmt.Errorf("LLM client is not initialized")
	}

	detail, err := a.db.GetScreeningSessionDetail(sessionID)
	if err != nil {
		return nil, err
	}
	if detail.CurrentNode == nil {
		return nil, fmt.Errorf("screening node has not been generated")
	}
	if detail.CurrentNode.NodeType == "complete" {
		return detail.CurrentNode, nil
	}

	selectedIDs, selectedLabels := filterPaperIDsBySelection(detail.CurrentNode, selectedOptions)
	if len(selectedIDs) == 0 {
		return nil, fmt.Errorf("no screening options selected")
	}

	currentSet := detail.CurrentNode.RemainingPaperIDs
	if len(currentSet) == 0 {
		currentSet = unionOptionPaperIDs(detail.CurrentNode.Options)
	}
	currentSetMap := make(map[string]struct{}, len(currentSet))
	for _, id := range currentSet {
		currentSetMap[id] = struct{}{}
	}
	selectedMap := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		selectedMap[id] = struct{}{}
	}

	var nextPapers []ScreeningPaper
	for _, paper := range detail.Papers {
		if _, ok := currentSetMap[paper.ID]; !ok {
			if paper.Status != "rejected" {
				nextPapers = append(nextPapers, paper)
			}
			continue
		}

		if _, keep := selectedMap[paper.ID]; keep {
			paper.Status = "screening"
			paper.Selection = ""
			paper.Reason = ""
			nextPapers = append(nextPapers, paper)
		} else {
			paper.Status = "rejected"
			paper.Selection = "rejected"
			paper.Reason = fmt.Sprintf("Filtered out by %s", detail.CurrentNode.Dimension)
		}
		paper.UpdatedAt = time.Now()
		if err := a.db.UpsertScreeningPaper(&paper); err != nil {
			return nil, err
		}
	}

	pathHistory := append([]PathHistoryItem{}, detail.PathHistory...)
	pathHistory = append(pathHistory, PathHistoryItem{
		Dimension: detail.CurrentNode.Dimension,
		Choice:    strings.Join(selectedLabels, ", "),
	})
	pathHistoryJSON, err := json.Marshal(pathHistory)
	if err != nil {
		return nil, err
	}
	if err := a.db.UpdateScreeningSessionPathHistory(sessionID, string(pathHistoryJSON)); err != nil {
		return nil, err
	}

	selectedOptionsJSON, err := json.Marshal(selectedOptions)
	if err != nil {
		return nil, err
	}

	node, err := a.buildNextScreeningNode(detail.Session.Title, nextPapers, pathHistory)
	if err != nil {
		return nil, err
	}
	if err := a.persistScreeningNode(sessionID, node, selectedOptionsJSON); err != nil {
		return nil, err
	}

	return node, nil
}

func (a *App) CompleteScreening(sessionID string, targetFolderID string) ([]Paper, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	detail, err := a.db.GetScreeningSessionDetail(sessionID)
	if err != nil {
		return nil, err
	}

	remainingIDs := remainingPaperIDsForCompletion(detail)
	if len(remainingIDs) == 0 {
		return nil, fmt.Errorf("no papers remain for import")
	}

	folderID, err := a.resolveFolderID(targetFolderID)
	if err != nil {
		return nil, err
	}

	remainingMap := make(map[string]struct{}, len(remainingIDs))
	for _, id := range remainingIDs {
		remainingMap[id] = struct{}{}
	}

	var imported []Paper
	for _, paper := range detail.Papers {
		if _, ok := remainingMap[paper.ID]; !ok {
			continue
		}

		libraryPaper := screeningPaperToLibraryPaper(paper, folderID)
		if err := a.db.UpsertPaper(&libraryPaper); err != nil {
			return nil, err
		}

		paper.Status = "selected"
		paper.Selection = "selected"
		paper.TargetFolderID = folderID
		paper.UpdatedAt = time.Now()
		if err := a.db.UpsertScreeningPaper(&paper); err != nil {
			return nil, err
		}

		imported = append(imported, libraryPaper)
	}

	if err := a.db.UpdateScreeningSessionStatus(sessionID, "complete", detail.Session.TotalPapers); err != nil {
		return nil, err
	}

	return imported, nil
}

func (a *App) ListScreeningSessions() ([]ScreeningSession, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.db.ListScreeningSessions()
}

func (a *App) GetScreeningSession(sessionID string) (*ScreeningSessionDetail, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.db.GetScreeningSessionDetail(sessionID)
}

func (a *App) CancelScreening(sessionID string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}

	a.stateMu.Lock()
	delete(a.extractProgress, sessionID)
	a.stateMu.Unlock()

	return a.db.DeleteScreeningSession(sessionID)
}

func (a *App) buildNextScreeningNode(sessionTitle string, papers []ScreeningPaper, history []PathHistoryItem) (*ScreeningDecisionNode, error) {
	activePapers := screeningCandidatePapers(papers)
	if len(activePapers) == 0 {
		return nil, fmt.Errorf("no candidate papers remain")
	}
	if len(activePapers) <= screeningCompletionThreshold {
		return buildCompletionNode(activePapers), nil
	}

	strong := a.currentStrongLLM()
	if strong == nil {
		return nil, fmt.Errorf("LLM client is not initialized")
	}

	node, err := strong.AnalyzeScreening(ScreeningAIRequest{
		SessionTitle: sessionTitle,
		Papers:       activePapers,
		PathHistory:  history,
	})
	if err != nil {
		return nil, err
	}

	return normalizeScreeningNode(node, activePapers)
}

func (a *App) persistScreeningNode(sessionID string, node *ScreeningDecisionNode, selectedOptionsJSON []byte) error {
	payload, err := json.Marshal(node)
	if err != nil {
		return err
	}

	selectedPayload := "[]"
	if len(selectedOptionsJSON) > 0 {
		selectedPayload = string(selectedOptionsJSON)
	}

	return a.db.UpdateScreeningSessionNode(sessionID, string(payload), selectedPayload)
}

func (a *App) resolveFolderID(folderID string) (string, error) {
	folderID = strings.TrimSpace(folderID)
	if folderID != "" {
		return folderID, nil
	}

	folders, err := a.db.GetFolders()
	if err != nil {
		return "", err
	}
	if len(folders) == 0 {
		return "", fmt.Errorf("no folders available")
	}
	return folders[0].ID, nil
}

func (a *App) storeExtractProgress(progress *ExtractProgress) {
	cloned := cloneExtractProgress(progress)

	a.stateMu.Lock()
	a.extractProgress[progress.SessionID] = cloned
	a.stateMu.Unlock()

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "extract-progress", cloned)
	}
}

func (a *App) loadExtractProgress(sessionID string) *ExtractProgress {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()

	progress := a.extractProgress[sessionID]
	if progress == nil {
		return nil
	}
	return cloneExtractProgress(progress)
}

func cloneExtractProgress(progress *ExtractProgress) *ExtractProgress {
	if progress == nil {
		return nil
	}
	cloned := *progress
	return &cloned
}

func normalizeScreeningPaths(filePaths []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		cleaned := strings.TrimSpace(filePath)
		if cleaned == "" {
			continue
		}
		if !strings.EqualFold(filepath.Ext(cleaned), ".pdf") {
			continue
		}
		if abs, err := filepath.Abs(cleaned); err == nil {
			cleaned = abs
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		normalized = append(normalized, cleaned)
	}
	sort.Strings(normalized)
	return normalized
}

func screeningCandidatePapers(papers []ScreeningPaper) []ScreeningPaper {
	candidates := make([]ScreeningPaper, 0, len(papers))
	for _, paper := range papers {
		switch paper.Status {
		case "extracted", "screening", "selected":
			if paper.Selection != "rejected" {
				candidates = append(candidates, paper)
			}
		}
	}
	return candidates
}

func buildCompletionNode(papers []ScreeningPaper) *ScreeningDecisionNode {
	remaining := make([]string, 0, len(papers))
	for _, paper := range papers {
		remaining = append(remaining, paper.ID)
	}

	return &ScreeningDecisionNode{
		ID:                fmt.Sprintf("screen-node-%d", time.Now().UnixNano()),
		NodeType:          "complete",
		Message:           fmt.Sprintf("筛选完成，当前剩余 %d 篇论文，请确认导入。", len(papers)),
		Dimension:         "结果确认",
		Options:           []ScreeningDecisionOption{},
		AllowMultiSelect:  false,
		AllowSkip:         false,
		RemainingPaperIDs: remaining,
	}
}

func normalizeScreeningNode(node *ScreeningDecisionNode, papers []ScreeningPaper) (*ScreeningDecisionNode, error) {
	if node == nil {
		return nil, fmt.Errorf("screening node is nil")
	}

	allowed := make(map[string]ScreeningPaper, len(papers))
	orderedIDs := make([]string, 0, len(papers))
	for _, paper := range papers {
		allowed[paper.ID] = paper
		orderedIDs = append(orderedIDs, paper.ID)
	}

	if strings.TrimSpace(node.ID) == "" {
		node.ID = fmt.Sprintf("screen-node-%d", time.Now().UnixNano())
	}
	if node.NodeType == "" {
		node.NodeType = "branch"
	}
	if node.NodeType == "complete" {
		if len(node.RemainingPaperIDs) == 0 {
			node.RemainingPaperIDs = orderedIDs
		}
		return node, nil
	}

	keyCounts := map[string]int{}
	normalizedOptions := make([]ScreeningDecisionOption, 0, len(node.Options))
	union := make([]string, 0, len(papers))
	unionSeen := map[string]struct{}{}
	for index, option := range node.Options {
		filteredIDs := make([]string, 0, len(option.PaperIDs))
		seenIDs := map[string]struct{}{}
		for _, paperID := range option.PaperIDs {
			if _, ok := allowed[paperID]; !ok {
				continue
			}
			if _, ok := seenIDs[paperID]; ok {
				continue
			}
			seenIDs[paperID] = struct{}{}
			filteredIDs = append(filteredIDs, paperID)
			if _, ok := unionSeen[paperID]; !ok {
				unionSeen[paperID] = struct{}{}
				union = append(union, paperID)
			}
		}
		if len(filteredIDs) == 0 {
			continue
		}

		key := slugKey(option.Key)
		if key == "" {
			key = slugKey(option.Label)
		}
		if key == "" {
			key = fmt.Sprintf("option-%d", index+1)
		}
		keyCounts[key]++
		if keyCounts[key] > 1 {
			key = fmt.Sprintf("%s-%d", key, keyCounts[key])
		}

		label := strings.TrimSpace(option.Label)
		if label == "" {
			label = key
		}

		normalizedOptions = append(normalizedOptions, ScreeningDecisionOption{
			Key:      key,
			Label:    label,
			PaperIDs: filteredIDs,
			Count:    len(filteredIDs),
		})
	}

	if len(normalizedOptions) == 0 {
		return nil, fmt.Errorf("LLM returned no usable screening options")
	}

	node.Options = normalizedOptions
	if strings.TrimSpace(node.Message) == "" {
		node.Message = "请继续缩小筛选范围。"
	}
	if strings.TrimSpace(node.Dimension) == "" {
		node.Dimension = "研究维度"
	}
	if len(node.RemainingPaperIDs) == 0 {
		node.RemainingPaperIDs = union
	}

	return node, nil
}

func filterPaperIDsBySelection(node *ScreeningDecisionNode, selectedOptions []string) ([]string, []string) {
	selected := map[string]struct{}{}
	labels := make([]string, 0, len(selectedOptions))
	for _, key := range selectedOptions {
		selected[strings.TrimSpace(key)] = struct{}{}
	}

	resultIDs := make([]string, 0)
	resultSeen := map[string]struct{}{}
	for _, option := range node.Options {
		if _, ok := selected[option.Key]; !ok {
			continue
		}
		labels = append(labels, option.Label)
		for _, paperID := range option.PaperIDs {
			if _, ok := resultSeen[paperID]; ok {
				continue
			}
			resultSeen[paperID] = struct{}{}
			resultIDs = append(resultIDs, paperID)
		}
	}
	return resultIDs, labels
}

func unionOptionPaperIDs(options []ScreeningDecisionOption) []string {
	var result []string
	seen := map[string]struct{}{}
	for _, option := range options {
		for _, paperID := range option.PaperIDs {
			if _, ok := seen[paperID]; ok {
				continue
			}
			seen[paperID] = struct{}{}
			result = append(result, paperID)
		}
	}
	return result
}

func remainingPaperIDsForCompletion(detail *ScreeningSessionDetail) []string {
	if detail.CurrentNode != nil && detail.CurrentNode.NodeType == "complete" && len(detail.CurrentNode.RemainingPaperIDs) > 0 {
		return detail.CurrentNode.RemainingPaperIDs
	}

	var remaining []string
	for _, paper := range detail.Papers {
		if paper.Status == "screening" || paper.Status == "selected" || paper.Status == "extracted" {
			if paper.Selection != "rejected" {
				remaining = append(remaining, paper.ID)
			}
		}
	}
	return remaining
}

func buildStoredScreeningContent(parseResult *PDFParseResponse, extractResult *PDFExtractResponse) (string, error) {
	payload := storedScreeningContent{
		Sections:      parseResult.Sections,
		ParseMetadata: parseResult.Metadata,
		Metrics:       []PDFExtractMetric{},
		Baselines:     []PDFExtractBaseline{},
		RelevanceTags: []string{},
	}
	if extractResult != nil && extractResult.Data != nil {
		metadata := extractResult.Data.Metadata
		payload.Metadata = &metadata
		payload.Metrics = extractResult.Data.Metrics
		payload.Baselines = extractResult.Data.Baselines
		payload.RelevanceTags = extractResult.Data.RelevanceTags
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func screeningPaperToLibraryPaper(paper ScreeningPaper, folderID string) Paper {
	metadata := decodeStoredScreeningContent(paper.SectionsJSON)
	tags := dedupeStrings(metadata.RelevanceTags)
	category := ""
	if len(tags) > 0 {
		category = tags[0]
	}

	url := ""
	if metadata.Metadata != nil {
		url = firstNonBlankString(metadata.Metadata.ArxivURL, metadata.Metadata.GitHubURL)
	}

	return Paper{
		ID:        paper.ID,
		Title:     firstNonBlankString(paper.Title, trimPDFSuffix(paper.FileName)),
		Authors:   paper.Authors,
		Abstract:  paper.Abstract,
		Year:      0,
		Journal:   "",
		URL:       url,
		PDFPath:   paper.FilePath,
		FolderID:  folderID,
		Category:  category,
		Tags:      tags,
		AddedAt:   time.Now(),
		UpdatedAt: time.Now(),
	}
}

func decodeStoredScreeningContent(raw string) storedScreeningContent {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return storedScreeningContent{}
	}

	var payload storedScreeningContent
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return storedScreeningContent{}
	}
	return payload
}

func pdfExtractionLLMConfigForApp(config AppConfig) PDFExtractionLLMConfig {
	llmConfig := normalizeLLMConfig(config.WeakLLM)
	if strings.TrimSpace(llmConfig.APIKey) == "" {
		llmConfig = normalizeLLMConfig(config.LLM)
	}

	provider := "openai"
	if llmConfig.ProviderType == "anthropic" {
		provider = "anthropic"
	}

	return PDFExtractionLLMConfig{
		Provider:    provider,
		Model:       llmConfig.Model,
		APIKey:      llmConfig.APIKey,
		BaseURL:     llmConfig.BaseURL,
		MaxTokens:   4096,
		Temperature: 0,
		Timeout:     120,
	}
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func trimPDFSuffix(filename string) string {
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

func firstNonBlankString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func slugKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			builder.WriteRune(r)
			lastDash = false
		case !lastDash:
			builder.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}
