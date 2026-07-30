package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

var deepReadBase64FallbackMaxBytes int64 = 16 * 1024 * 1024

const deepReadAIContextMaxRunes = 48000

func (a *App) GetDeepReadState(paperID string) (*DeepReadState, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return nil, fmt.Errorf("paper id cannot be empty")
	}

	paper, err := a.db.GetPaperByID(paperID)
	if err != nil {
		return nil, err
	}

	translations, err := a.db.GetTranslations(paperID)
	if err != nil {
		return nil, err
	}
	notes, err := a.db.GetDeepReadNotes(paperID)
	if err != nil {
		return nil, err
	}

	parseCache, err := a.db.GetDeepReadParseCache(paperID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	hasPDF := a.deepReadManagedPDFAvailable(paper.PDFPath)
	state := &DeepReadState{
		PaperID:      paperID,
		HasPDF:       hasPDF,
		PDFPath:      strings.TrimSpace(paper.PDFPath),
		ParseStatus:  "idle",
		Sections:     []DeepReadSection{},
		Translations: translations,
		Notes:        notes,
	}

	if parseCache != nil {
		state.ParseStatus = strings.TrimSpace(parseCache.Status)
		if state.ParseStatus == "" {
			state.ParseStatus = "idle"
		}
		state.ParseError = strings.TrimSpace(parseCache.ErrorMessage)
		state.Markdown = parseCache.Markdown
		state.Sections = parseCache.Sections
		state.LastPreparedAt = parseCache.LastPreparedAt
	}

	if hasPDF && parseCache != nil && strings.TrimSpace(parseCache.Status) == "missing_pdf" {
		state.ParseStatus = "idle"
		state.ParseError = ""
		_ = a.db.UpsertDeepReadParseCache(&DeepReadParseCache{
			PaperID:        paperID,
			PDFPath:        strings.TrimSpace(paper.PDFPath),
			Status:         "idle",
			ErrorMessage:   "",
			Markdown:       parseCache.Markdown,
			Sections:       parseCache.Sections,
			LastPreparedAt: time.Now(),
		})
	}

	if !hasPDF {
		state.ParseStatus = "missing_pdf"
		state.ParseError = "本地 PDF 不可用，请先等待下载完成或手动导入 PDF。"
		if parseCache == nil {
			_ = a.db.UpsertDeepReadParseCache(&DeepReadParseCache{
				PaperID:        paperID,
				PDFPath:        strings.TrimSpace(paper.PDFPath),
				Status:         "missing_pdf",
				ErrorMessage:   state.ParseError,
				Sections:       []DeepReadSection{},
				LastPreparedAt: time.Now(),
			})
		}
	}

	return state, nil
}

func (a *App) PrepareDeepReadPaper(paperID string) (*DeepReadState, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.pdfService == nil {
		return nil, fmt.Errorf("pdf service client is not initialized")
	}

	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return nil, fmt.Errorf("paper id cannot be empty")
	}

	paper, err := a.db.GetPaperByID(paperID)
	if err != nil {
		return nil, err
	}
	pdfPath, pdfErr := a.resolveDeepReadPDFPath(paper.PDFPath)
	if pdfErr != nil {
		errorMessage := "本地 PDF 不可用或不在托管目录，请先等待下载完成或手动导入 PDF。"
		_ = a.db.UpsertDeepReadParseCache(&DeepReadParseCache{
			PaperID:        paperID,
			PDFPath:        strings.TrimSpace(paper.PDFPath),
			Status:         "missing_pdf",
			ErrorMessage:   errorMessage,
			Sections:       []DeepReadSection{},
			LastPreparedAt: time.Now(),
		})
		return a.GetDeepReadState(paperID)
	}

	if existing, err := a.db.GetDeepReadParseCache(paperID); err == nil && existing != nil {
		if existing.Status == "ready" && strings.TrimSpace(existing.PDFPath) == pdfPath && len(existing.Sections) > 0 {
			return a.GetDeepReadState(paperID)
		}
	}

	_ = a.db.UpsertDeepReadParseCache(&DeepReadParseCache{
		PaperID:        paperID,
		PDFPath:        pdfPath,
		Status:         "preparing",
		ErrorMessage:   "",
		Sections:       []DeepReadSection{},
		LastPreparedAt: time.Now(),
	})

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := a.ensureManagedPDFServiceReady(ctx); err != nil {
		_ = a.db.UpsertDeepReadParseCache(&DeepReadParseCache{
			PaperID:        paperID,
			PDFPath:        pdfPath,
			Status:         "failed",
			ErrorMessage:   err.Error(),
			Sections:       []DeepReadSection{},
			LastPreparedAt: time.Now(),
		})
		return a.GetDeepReadState(paperID)
	}
	parseResult, err := a.pdfService.ParsePDFWithContext(ctx, pdfPath)
	if err != nil {
		_ = a.db.UpsertDeepReadParseCache(&DeepReadParseCache{
			PaperID:        paperID,
			PDFPath:        pdfPath,
			Status:         "failed",
			ErrorMessage:   err.Error(),
			Sections:       []DeepReadSection{},
			LastPreparedAt: time.Now(),
		})
		return a.GetDeepReadState(paperID)
	}

	sections := buildDeepReadSections(parseResult.Markdown, parseResult.Sections)
	if len(sections) == 0 {
		sections = []DeepReadSection{
			{
				ID:      "section-fulltext",
				Title:   "Full Text",
				Content: strings.TrimSpace(parseResult.Markdown),
				Index:   0,
			},
		}
	}

	if err := a.db.UpsertDeepReadParseCache(&DeepReadParseCache{
		PaperID:        paperID,
		PDFPath:        pdfPath,
		Status:         "ready",
		ErrorMessage:   "",
		Markdown:       parseResult.Markdown,
		Sections:       sections,
		LastPreparedAt: time.Now(),
	}); err != nil {
		return nil, err
	}

	return a.GetDeepReadState(paperID)
}

func (a *App) SaveDeepReadNote(paperID, section, content string) (*DeepReadNote, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	paperID = strings.TrimSpace(paperID)
	section = strings.TrimSpace(section)
	content = strings.TrimSpace(content)
	if paperID == "" {
		return nil, fmt.Errorf("paper id cannot be empty")
	}
	if content == "" {
		return nil, fmt.Errorf("note content cannot be empty")
	}
	if section == "" {
		section = "General"
	}
	if _, err := a.db.GetPaperByID(paperID); err != nil {
		return nil, err
	}

	note := &DeepReadNote{
		ID:        uuid.NewString(),
		PaperID:   paperID,
		Section:   section,
		Content:   content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := a.db.SaveDeepReadNote(note); err != nil {
		return nil, err
	}
	return note, nil
}

func (a *App) AskDeepReadPaper(paperID, sectionID, question, mode string) (*DeepReadAIResponse, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return nil, fmt.Errorf("paper id cannot be empty")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "question"
	}
	if mode != "question" && mode != "summary" {
		return nil, fmt.Errorf("mode must be question or summary")
	}
	question = strings.TrimSpace(question)
	if mode == "question" && question == "" {
		return nil, fmt.Errorf("question cannot be empty")
	}

	paper, err := a.db.GetPaperByID(paperID)
	if err != nil {
		return nil, err
	}
	state, err := a.GetDeepReadState(paperID)
	if err != nil {
		return nil, err
	}
	if state.ParseStatus != "ready" || len(state.Sections) == 0 {
		return nil, fmt.Errorf("paper content is not ready; prepare the PDF before using the AI reading assistant")
	}

	assistant := a.deepReadAssistantForMode(mode)
	if assistant == nil {
		return nil, fmt.Errorf("configured LLMs do not support the DeepRead assistant")
	}
	contextText := buildDeepReadAIContextForTask(
		state.Sections,
		sectionID,
		question,
		mode,
		deepReadAIContextMaxRunes,
	)
	if strings.TrimSpace(contextText) == "" {
		return nil, fmt.Errorf("paper context is empty")
	}

	ctx, token := a.beginDeepReadAI()
	defer a.finishDeepReadAI(token)
	response, err := assistant.AnswerDeepReadWithContext(ctx, paper.Title, mode, question, contextText)
	if errors.Is(err, context.Canceled) {
		return nil, errDeepReadAICancelled
	}
	return response, err
}

func (a *App) deepReadAssistantForMode(mode string) deepReadAssistant {
	if strings.EqualFold(strings.TrimSpace(mode), "question") {
		if weak, ok := a.currentWeakLLM().(deepReadAssistant); ok && weak != nil {
			return weak
		}
	}
	if strong, ok := a.currentStrongLLM().(deepReadAssistant); ok && strong != nil {
		return strong
	}
	if weak, ok := a.currentWeakLLM().(deepReadAssistant); ok && weak != nil {
		return weak
	}
	return nil
}

func buildDeepReadAIContext(sections []DeepReadSection, selectedSectionID string, maxRunes int) string {
	return buildDeepReadAIContextForTask(sections, selectedSectionID, "", "", maxRunes)
}

func buildDeepReadAIContextForTask(
	sections []DeepReadSection,
	selectedSectionID string,
	question string,
	mode string,
	maxRunes int,
) string {
	if maxRunes <= 0 {
		return ""
	}
	selectedSectionID = strings.TrimSpace(selectedSectionID)
	priorityTerms := []string{"abstract", "introduction", "method", "approach", "experiment", "result", "discussion", "conclusion", "limitation"}

	ordered := make([]DeepReadSection, 0, len(sections))
	seen := make(map[string]struct{}, len(sections))
	appendSection := func(section DeepReadSection) {
		if strings.TrimSpace(section.Content) == "" {
			return
		}
		key := strings.TrimSpace(section.ID)
		if key == "" {
			key = fmt.Sprintf("section-%d", section.Index+1)
			section.ID = key
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		ordered = append(ordered, section)
	}

	if selectedSectionID != "" {
		for _, section := range sections {
			if strings.TrimSpace(section.ID) == selectedSectionID {
				appendSection(section)
				break
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(mode), "question") && strings.TrimSpace(question) != "" {
		type scoredSection struct {
			section DeepReadSection
			score   int
		}
		terms := deepReadQuestionTerms(question)
		scored := make([]scoredSection, 0, len(sections))
		for _, section := range sections {
			if score := deepReadSectionRelevance(section, terms); score > 0 {
				scored = append(scored, scoredSection{section: section, score: score})
			}
		}
		sort.SliceStable(scored, func(i, j int) bool {
			if scored[i].score != scored[j].score {
				return scored[i].score > scored[j].score
			}
			return scored[i].section.Index < scored[j].section.Index
		})
		for index, item := range scored {
			if index >= 6 {
				break
			}
			appendSection(item.section)
		}
	}
	for _, term := range priorityTerms {
		for _, section := range sections {
			if strings.Contains(strings.ToLower(section.Title), term) {
				appendSection(section)
			}
		}
	}
	for _, section := range sections {
		appendSection(section)
	}
	if len(ordered) > 16 {
		ordered = ordered[:16]
	}

	var builder strings.Builder
	remaining := maxRunes
	for index, section := range ordered {
		header := fmt.Sprintf("[%s] %s\n", section.ID, strings.TrimSpace(section.Title))
		headerRunes := len([]rune(header))
		sectionsLeft := len(ordered) - index
		targetRunes := remaining / sectionsLeft
		contentBudget := targetRunes - headerRunes - 2
		if contentBudget < 64 {
			contentBudget = remaining - headerRunes - 2
		}
		if contentBudget <= 0 {
			break
		}
		content := truncateDeepReadSection(strings.TrimSpace(section.Content), contentBudget)
		block := header + content + "\n\n"
		runes := []rune(block)
		if len(runes) > remaining {
			if remaining <= headerRunes+64 {
				break
			}
			runes = runes[:remaining]
		}
		builder.WriteString(string(runes))
		remaining -= len(runes)
		if remaining <= 0 {
			break
		}
	}
	return strings.TrimSpace(builder.String())
}

func deepReadQuestionTerms(question string) []string {
	terms := extractDeepStartMessageTokens(question)
	lowered := strings.ToLower(question)
	synonyms := map[string][]string{
		"消融": {"ablation"},
		"局限": {"limitation", "limitations"},
		"方法": {"method", "approach"},
		"实验": {"experiment", "experiments", "result", "results"},
		"数据": {"data", "dataset"},
		"结论": {"conclusion"},
		"基线": {"baseline"},
		"训练": {"training"},
		"架构": {"architecture"},
		"贡献": {"contribution"},
	}
	for source, expanded := range synonyms {
		if strings.Contains(lowered, source) {
			terms = append(terms, expanded...)
		}
	}
	return uniqueStrings(terms)
}

func deepReadSectionRelevance(section DeepReadSection, terms []string) int {
	title := strings.ToLower(section.Title)
	content := strings.ToLower(section.Content)
	score := 0
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if strings.Contains(title, term) {
			score += 20
		}
		matches := strings.Count(content, term)
		if matches > 3 {
			matches = 3
		}
		score += matches * 2
	}
	return score
}

func truncateDeepReadSection(content string, limit int) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= limit {
		return string(runes)
	}
	if limit < 96 {
		return string(runes[:limit])
	}
	marker := []rune("\n[content omitted]\n")
	headLength := (limit - len(marker)) * 2 / 3
	tailLength := limit - len(marker) - headLength
	return string(runes[:headLength]) + string(marker) + string(runes[len(runes)-tailLength:])
}

func (a *App) GetDeepReadPDFBytes(paperID string) (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}

	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return "", fmt.Errorf("paper id cannot be empty")
	}

	paper, err := a.db.GetPaperByID(paperID)
	if err != nil {
		return "", err
	}

	pdfPath, err := a.resolveDeepReadPDFPath(paper.PDFPath)
	if err != nil {
		return "", err
	}

	content, err := readLimitedFile(pdfPath, deepReadBase64FallbackMaxBytes)
	if err != nil {
		if strings.Contains(err.Error(), "exceeds") {
			return "", fmt.Errorf("local pdf is too large for base64 fallback: %w; use the DeepRead asset URL instead", err)
		}
		return "", fmt.Errorf("read local pdf failed: %w", err)
	}
	if len(content) == 0 {
		return "", fmt.Errorf("local pdf file is empty")
	}
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		return "", fmt.Errorf("local file is not a valid pdf")
	}

	return base64.StdEncoding.EncodeToString(content), nil
}

func (a *App) GetDeepReadPDFURL(paperID string) (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}

	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return "", fmt.Errorf("paper id cannot be empty")
	}

	paper, err := a.db.GetPaperByID(paperID)
	if err != nil {
		return "", err
	}

	pdfPath, err := a.resolveDeepReadPDFPath(paper.PDFPath)
	if err != nil {
		return "", err
	}

	token := uuid.NewString()
	a.storeDeepReadPDFResource(token, deepReadPDFResource{
		PaperID:   paperID,
		Path:      pdfPath,
		ExpiresAt: time.Now().Add(deepReadPDFResourceTTL),
	})
	return "/deepread/pdf/" + token, nil
}

func deepReadPDFAvailable(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func buildDeepReadSections(markdown string, hints []string) []DeepReadSection {
	content := strings.TrimSpace(markdown)
	if content == "" {
		return []DeepReadSection{}
	}

	lines := strings.Split(content, "\n")
	sections := make([]DeepReadSection, 0, 12)
	currentTitle := "Full Text"
	buffer := make([]string, 0, 48)
	flush := func() {
		text := strings.TrimSpace(strings.Join(buffer, "\n"))
		if strings.TrimSpace(currentTitle) == "" || text == "" {
			buffer = buffer[:0]
			return
		}
		index := len(sections)
		sections = append(sections, DeepReadSection{
			ID:      fmt.Sprintf("section-%d", index+1),
			Title:   currentTitle,
			Content: text,
			Index:   index,
		})
		buffer = buffer[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			flush()
			currentTitle = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if currentTitle == "" {
				currentTitle = "Untitled Section"
			}
			continue
		}
		buffer = append(buffer, line)
	}
	flush()

	if len(sections) == 0 {
		if len(hints) > 0 {
			sections = append(sections, DeepReadSection{
				ID:      "section-1",
				Title:   strings.TrimSpace(hints[0]),
				Content: content,
				Index:   0,
			})
			return sections
		}
		return []DeepReadSection{
			{
				ID:      "section-1",
				Title:   "Full Text",
				Content: content,
				Index:   0,
			},
		}
	}

	return sections
}
