package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

var deepReadBase64FallbackMaxBytes int64 = 16 * 1024 * 1024

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
