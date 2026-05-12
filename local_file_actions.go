package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) SelectScreeningPDFs() ([]string, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.ctx == nil {
		return nil, fmt.Errorf("native file dialog is unavailable")
	}

	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择待筛选 PDF",
		Filters: []runtime.FileFilter{
			{DisplayName: "PDF Files (*.pdf)", Pattern: "*.pdf"},
		},
	})
	if err != nil {
		return nil, err
	}
	return normalizeScreeningPaths(paths), nil
}

func (a *App) SelectAndAttachPaperPDF(paperID string) (*Paper, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if a.ctx == nil {
		return nil, fmt.Errorf("native file dialog is unavailable")
	}

	sourcePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择本地 PDF",
		Filters: []runtime.FileFilter{
			{DisplayName: "PDF Files (*.pdf)", Pattern: "*.pdf"},
		},
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sourcePath) == "" {
		return nil, fmt.Errorf("no pdf selected")
	}
	return a.AttachLocalPDFToPaper(paperID, sourcePath)
}

func (a *App) AttachLocalPDFToPaper(paperID, sourcePath string) (*Paper, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	paperID = strings.TrimSpace(paperID)
	sourcePath = strings.TrimSpace(sourcePath)
	if paperID == "" {
		return nil, fmt.Errorf("paper id cannot be empty")
	}
	if sourcePath == "" {
		return nil, fmt.Errorf("pdf path cannot be empty")
	}
	if strings.ToLower(filepath.Ext(sourcePath)) != ".pdf" {
		return nil, fmt.Errorf("selected file is not a PDF")
	}

	paper, err := a.db.GetPaperByID(paperID)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to access selected PDF: %w", err)
	}
	if info.IsDir() || info.Size() == 0 {
		return nil, fmt.Errorf("selected PDF is empty or invalid")
	}

	folderPath := paper.FolderID
	if folder, folderErr := a.db.GetFolderByID(paper.FolderID); folderErr == nil {
		folderPath = normalizeFolderPath(folder.Path)
	}
	targetPath := filepath.Join(a.config.DataPath, "papers", filepath.FromSlash(folderPath), paper.ID+".pdf")
	if err := copyLocalPDF(sourcePath, targetPath); err != nil {
		return nil, err
	}

	paper.PDFPath = targetPath
	paper.DownloadStatus = "downloaded"
	paper.DownloadError = ""
	paper.UpdatedAt = time.Now()
	if err := a.db.UpsertPaper(paper); err != nil {
		return nil, err
	}

	_ = a.db.UpsertDeepReadParseCache(&DeepReadParseCache{
		PaperID:        paper.ID,
		PDFPath:        targetPath,
		Status:         "idle",
		ErrorMessage:   "",
		Sections:       []DeepReadSection{},
		LastPreparedAt: time.Now(),
	})
	return paper, nil
}

func (a *App) RetryFolderPendingDownloads(folderID string) (int, error) {
	if err := a.ensureReady(); err != nil {
		return 0, err
	}

	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return 0, fmt.Errorf("folder id cannot be empty")
	}
	papers, err := a.db.GetPapers(folderID)
	if err != nil {
		return 0, err
	}

	folderPath := folderID
	if folder, folderErr := a.db.GetFolderByID(folderID); folderErr == nil {
		folderPath = normalizeFolderPath(folder.Path)
	}

	queued := 0
	for _, paper := range papers {
		status := strings.ToLower(strings.TrimSpace(paper.DownloadStatus))
		if status == "downloaded" && deepReadPDFAvailable(paper.PDFPath) {
			continue
		}
		if status == "downloaded" && !deepReadPDFAvailable(paper.PDFPath) {
			// Treat a stale downloaded marker as recoverable download work.
		} else if status != "queued" && status != "downloading" && status != "failed" && strings.TrimSpace(paper.PDFPath) != "" {
			continue
		}
		if err := a.db.UpdatePaperDownloadState(paper.ID, "queued", "", ""); err != nil {
			return queued, err
		}
		a.enqueuePaperDownload(paperDownloadJob{
			PaperID:       paper.ID,
			FolderID:      paper.FolderID,
			FolderPath:    folderPath,
			SourceURL:     strings.TrimSpace(paper.URL),
			SourcePaperID: strings.TrimSpace(paper.SourcePaperID),
		})
		queued++
	}

	return queued, nil
}

func copyLocalPDF(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
		return err
	}

	tmpPath := targetPath + ".tmp"
	target, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := target.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, targetPath)
}
