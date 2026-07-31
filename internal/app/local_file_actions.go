package app

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
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, fmt.Errorf("selected PDF is empty or invalid")
	}
	if info.Size() > pdfDownloadMaxBytes {
		return nil, fmt.Errorf("selected PDF is too large: %d bytes exceeds %d bytes limit", info.Size(), pdfDownloadMaxBytes)
	}

	folder, err := a.db.GetFolderByID(paper.FolderID)
	if err != nil {
		return nil, fmt.Errorf("paper folder not found: %w", err)
	}
	folderPath := normalizeFolderPath(folder.Path)
	papersRoot := filepath.Join(a.config.DataPath, "papers")
	targetPath := filepath.Join(papersRoot, filepath.FromSlash(folderPath), safePaperPDFFileName(paper.ID))
	managedTargetPath, err := ensureManagedFileParent(papersRoot, targetPath)
	if err != nil {
		return nil, fmt.Errorf("managed pdf target is not safe: %w", err)
	}
	targetPath = managedTargetPath
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

	folder, err := a.db.GetFolderByID(folderID)
	if err != nil {
		return 0, err
	}
	folderPath := normalizeFolderPath(folder.Path)

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
			DataPath:      a.config.DataPath,
			SourceURL:     strings.TrimSpace(paper.URL),
			SourcePaperID: strings.TrimSpace(paper.SourcePaperID),
		})
		queued++
	}

	return queued, nil
}

func copyLocalPDF(sourcePath, targetPath string) error {
	return copyLocalPDFWithOpen(sourcePath, targetPath, os.Open)
}

func copyLocalPDFWithOpen(sourcePath, targetPath string, openFile func(string) (*os.File, error)) error {
	return copyLocalPDFWithOpenAndValidator(sourcePath, targetPath, openFile, validateLocalPDFFile)
}

func copyLocalPDFWithOpenAndValidator(sourcePath, targetPath string, openFile func(string) (*os.File, error), validateCopiedFile func(string) error) error {
	sourcePath = strings.TrimSpace(sourcePath)
	targetPath = strings.TrimSpace(targetPath)
	if !strings.EqualFold(filepath.Ext(sourcePath), ".pdf") {
		return fmt.Errorf("source file is not a PDF")
	}
	if !strings.EqualFold(filepath.Ext(targetPath), ".pdf") {
		return fmt.Errorf("target file is not a PDF")
	}
	if validateCopiedFile == nil {
		return fmt.Errorf("copied pdf validation function cannot be nil")
	}

	source, _, err := openValidatedLocalPDFFileWithOpen(sourcePath, openFile)
	if err != nil {
		return err
	}
	defer source.Close()

	return copyOpenedPDFToTarget(source, targetPath, validateCopiedFile)
}

func copyOpenedPDFToTarget(source *os.File, targetPath string, validateCopiedFile func(string) error) error {
	targetPath = strings.TrimSpace(targetPath)
	if source == nil {
		return fmt.Errorf("source pdf file cannot be nil")
	}
	if !strings.EqualFold(filepath.Ext(targetPath), ".pdf") {
		return fmt.Errorf("target file is not a PDF")
	}
	if validateCopiedFile == nil {
		return fmt.Errorf("copied pdf validation function cannot be nil")
	}

	return writeFileAtomicWithWriter(targetPath, 0600, func(tmp *os.File) error {
		written, err := io.Copy(tmp, io.LimitReader(source, pdfDownloadMaxBytes+1))
		if err != nil {
			return err
		}
		if written > pdfDownloadMaxBytes {
			return fmt.Errorf("local pdf is too large: exceeds %d bytes limit", pdfDownloadMaxBytes)
		}
		return nil
	}, func(tmp *os.File, tmpPath string) error {
		if err := validateOpenLocalPDFFile(tmp); err != nil {
			return fmt.Errorf("copied local pdf validation failed: %w", err)
		}
		if err := validateCopiedFile(tmpPath); err != nil {
			return fmt.Errorf("copied local pdf validation failed: %w", err)
		}
		return nil
	})
}
