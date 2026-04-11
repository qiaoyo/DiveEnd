package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	pdfDownloadWorkerCount   = 3
	pdfDownloadTimeout       = 25 * time.Second
	pdfDownloadMaxRetry      = 3
	pdfDownloadQueueCapacity = 512
)

var arxivIDPattern = regexp.MustCompile(`^\d{4}\.\d{4,5}(v\d+)?$`)

type paperDownloadJob struct {
	PaperID       string
	FolderID      string
	FolderPath    string
	SourceURL     string
	SourcePaperID string
}

func normalizeSourcePaperID(paper SearchPaper) string {
	if id := strings.TrimSpace(paper.ID); id != "" {
		return id
	}

	if normalizedURL := strings.TrimSpace(paper.URL); normalizedURL != "" {
		return normalizedURL
	}

	titleKey := normalizedDedupeToken(paper.Title)
	if titleKey == "" {
		return uuid.NewString()
	}
	if paper.Year > 0 {
		return fmt.Sprintf("%s|%d", titleKey, paper.Year)
	}
	return titleKey
}

func (a *App) ImportPapersWithAssets(folderID string, papers []SearchPaper) (*ImportPapersWithAssetsResult, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		folders, err := a.db.GetFolders()
		if err != nil {
			return nil, err
		}
		if len(folders) == 0 {
			return nil, fmt.Errorf("no folder available")
		}
		folderID = folders[0].ID
	}

	result := &ImportPapersWithAssetsResult{
		Imported: make([]Paper, 0, len(papers)),
		Skipped:  make([]ImportSkippedPaper, 0),
	}

	targetFolderPath := ""
	if folder, err := a.db.GetFolderByID(folderID); err == nil {
		targetFolderPath = normalizeFolderPath(folder.Path)
	}

	for _, searchPaper := range papers {
		sourcePaperID := normalizeSourcePaperID(searchPaper)
		if existing, err := a.db.GetPaperByFolderAndSource(folderID, sourcePaperID); err == nil && existing != nil {
			result.Skipped = append(result.Skipped, ImportSkippedPaper{
				SourcePaperID: sourcePaperID,
				Title:         strings.TrimSpace(searchPaper.Title),
				Reason:        "already_exists_in_folder",
			})
			continue
		} else if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		now := time.Now()
		paper := Paper{
			ID:             uuid.NewString(),
			SourcePaperID:  sourcePaperID,
			Title:          strings.TrimSpace(searchPaper.Title),
			Authors:        strings.TrimSpace(searchPaper.Authors),
			Abstract:       strings.TrimSpace(searchPaper.Abstract),
			Year:           searchPaper.Year,
			Journal:        strings.TrimSpace(searchPaper.Journal),
			URL:            strings.TrimSpace(searchPaper.URL),
			PDFPath:        "",
			DownloadStatus: "queued",
			DownloadError:  "",
			FolderID:       folderID,
			Category:       strings.TrimSpace(searchPaper.Category),
			Tags:           uniqueStrings(searchPaper.Tags),
			AddedAt:        now,
			UpdatedAt:      now,
		}

		if err := a.db.UpsertPaper(&paper); err != nil {
			return nil, err
		}
		result.Imported = append(result.Imported, paper)
	}

	for _, paper := range result.Imported {
		a.enqueuePaperDownload(paperDownloadJob{
			PaperID:       paper.ID,
			FolderID:      paper.FolderID,
			FolderPath:    targetFolderPath,
			SourceURL:     paper.URL,
			SourcePaperID: paper.SourcePaperID,
		})
	}

	result.Queued = len(result.Imported)
	if result.Queued == 0 && len(result.Skipped) > 0 {
		result.Message = "选中的论文已经存在于当前文件夹。"
	} else if result.Queued > 0 && len(result.Skipped) > 0 {
		result.Message = fmt.Sprintf("已导入 %d 篇，跳过 %d 篇重复论文。", result.Queued, len(result.Skipped))
	} else if result.Queued > 0 {
		result.Message = fmt.Sprintf("已导入 %d 篇论文，后台正在下载 PDF。", result.Queued)
	}

	return result, nil
}

func (a *App) GetLocalStorageOverview() (*LocalStorageOverview, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	folders, err := a.db.GetFolders()
	if err != nil {
		return nil, err
	}

	rootPath := filepath.Join(a.config.DataPath, "papers")
	overview := &LocalStorageOverview{
		RootPath:     rootPath,
		TotalFolders: len(folders),
		Folders:      make([]LocalStorageFolderOverview, 0, len(folders)),
		GeneratedAt:  time.Now(),
	}

	totalFiles := 0
	for _, folder := range folders {
		stats, err := a.db.GetFolderPaperStats(folder.ID)
		if err != nil {
			return nil, err
		}

		normalizedPath := normalizeFolderPath(folder.Path)
		if normalizedPath == "" {
			normalizedPath = normalizeFolderPath(folder.Name)
		}
		folderPath := filepath.Join(rootPath, filepath.FromSlash(normalizedPath))
		fileCount := countStoredPDFFiles(folderPath)
		totalFiles += fileCount

		item := LocalStorageFolderOverview{
			FolderID:        folder.ID,
			FolderName:      folder.Name,
			FolderPath:      folderPath,
			TotalPapers:     stats.Total,
			Queued:          stats.Queued,
			Downloading:     stats.Downloading,
			Downloaded:      stats.Downloaded,
			Failed:          stats.Failed,
			StoredFileCount: fileCount,
		}
		overview.Folders = append(overview.Folders, item)

		overview.Queued += item.Queued
		overview.Downloading += item.Downloading
		overview.Downloaded += item.Downloaded
		overview.Failed += item.Failed
	}

	overview.TotalFiles = totalFiles
	return overview, nil
}

func (a *App) startDownloadWorkers() {
	if a.db == nil {
		return
	}

	a.downloadMu.Lock()
	defer a.downloadMu.Unlock()

	if a.downloadQueue != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.downloadCancel = cancel
	queue := make(chan paperDownloadJob, pdfDownloadQueueCapacity)
	a.downloadQueue = queue

	for i := 0; i < pdfDownloadWorkerCount; i++ {
		a.downloadWG.Add(1)
		go a.runDownloadWorker(ctx, queue)
	}
}

func (a *App) stopDownloadWorkers() {
	a.downloadMu.Lock()
	cancel := a.downloadCancel
	a.downloadCancel = nil
	a.downloadQueue = nil
	a.downloadMu.Unlock()

	if cancel != nil {
		cancel()
	}
	a.downloadWG.Wait()
}

func (a *App) enqueuePaperDownload(job paperDownloadJob) {
	job.PaperID = strings.TrimSpace(job.PaperID)
	if job.PaperID == "" {
		return
	}

	a.downloadMu.Lock()
	queue := a.downloadQueue
	a.downloadMu.Unlock()

	if queue == nil {
		return
	}

	select {
	case queue <- job:
	default:
		go func() {
			select {
			case queue <- job:
			case <-time.After(2 * time.Second):
				_ = a.db.UpdatePaperDownloadState(job.PaperID, "failed", "", "download queue is busy")
			}
		}()
	}
}

func (a *App) runDownloadWorker(ctx context.Context, queue <-chan paperDownloadJob) {
	defer a.downloadWG.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job := <-queue:
			if strings.TrimSpace(job.PaperID) == "" {
				continue
			}
			a.processDownloadJob(ctx, job)
		}
	}
}

func (a *App) processDownloadJob(ctx context.Context, job paperDownloadJob) {
	if err := a.db.UpdatePaperDownloadState(job.PaperID, "downloading", "", ""); err != nil {
		return
	}

	targetFolderPath := normalizeFolderPath(job.FolderPath)
	if targetFolderPath == "" {
		if folder, err := a.db.GetFolderByID(job.FolderID); err == nil {
			targetFolderPath = normalizeFolderPath(folder.Path)
		}
	}
	if targetFolderPath == "" {
		targetFolderPath = job.FolderID
	}

	targetPath := filepath.Join(
		a.config.DataPath,
		"papers",
		filepath.FromSlash(targetFolderPath),
		job.PaperID+".pdf",
	)
	candidates := buildPDFCandidateURLs(job.SourceURL, job.SourcePaperID)
	if len(candidates) == 0 {
		_ = a.db.UpdatePaperDownloadState(job.PaperID, "failed", "", "no downloadable pdf url")
		return
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
		_ = a.db.UpdatePaperDownloadState(job.PaperID, "failed", "", err.Error())
		return
	}

	var lastErr error
	for attempt := 1; attempt <= pdfDownloadMaxRetry; attempt++ {
		for _, candidate := range candidates {
			err := downloadPDFToFile(ctx, candidate, targetPath)
			if err == nil {
				_ = a.db.UpdatePaperDownloadState(job.PaperID, "downloaded", targetPath, "")
				return
			}
			lastErr = err
		}

		if attempt < pdfDownloadMaxRetry {
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown download error")
	}
	_ = a.db.UpdatePaperDownloadState(job.PaperID, "failed", "", lastErr.Error())
}

func downloadPDFToFile(ctx context.Context, rawURL, targetPath string) error {
	parsedURL := strings.TrimSpace(rawURL)
	if parsedURL == "" {
		return fmt.Errorf("empty url")
	}

	reqCtx, cancel := context.WithTimeout(ctx, pdfDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, parsedURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "DiveEnd/1.0 (+https://github.com)")
	req.Header.Set("Accept", "application/pdf,*/*;q=0.8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	tmpPath := targetPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	firstChunk := make([]byte, 1024)
	n, readErr := resp.Body.Read(firstChunk)
	if n > 0 {
		if _, err := file.Write(firstChunk[:n]); err != nil {
			_ = file.Close()
			_ = os.Remove(tmpPath)
			return err
		}
	}
	if readErr != nil && readErr != io.EOF {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return readErr
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if !looksLikePDF(contentType, firstChunk[:n]) {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("response is not a valid pdf")
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func looksLikePDF(contentType string, prefix []byte) bool {
	if strings.Contains(contentType, "application/pdf") {
		return true
	}
	trimmed := bytes.TrimSpace(prefix)
	return bytes.HasPrefix(trimmed, []byte("%PDF-"))
}

func buildPDFCandidateURLs(sourceURL, sourcePaperID string) []string {
	candidates := make([]string, 0, 4)
	appendCandidate := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		for _, existing := range candidates {
			if existing == raw {
				return
			}
		}
		candidates = append(candidates, raw)
	}

	normalizedSourceURL := strings.TrimSpace(sourceURL)
	if normalizedSourceURL != "" {
		if strings.Contains(strings.ToLower(normalizedSourceURL), "/abs/") && strings.Contains(strings.ToLower(normalizedSourceURL), "arxiv.org") {
			appendCandidate(arxivAbsToPDFURL(normalizedSourceURL))
		}
		appendCandidate(normalizedSourceURL)
	}

	if arxivID := extractArxivID(sourcePaperID); arxivID != "" {
		appendCandidate(fmt.Sprintf("https://arxiv.org/pdf/%s.pdf", arxivID))
	}
	if arxivID := extractArxivID(sourceURL); arxivID != "" {
		appendCandidate(fmt.Sprintf("https://arxiv.org/pdf/%s.pdf", arxivID))
	}

	return candidates
}

func arxivAbsToPDFURL(absURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(absURL))
	if err != nil {
		return absURL
	}
	path := strings.TrimSpace(parsed.Path)
	if !strings.Contains(path, "/abs/") {
		return absURL
	}
	pdfPath := strings.Replace(path, "/abs/", "/pdf/", 1)
	if !strings.HasSuffix(pdfPath, ".pdf") {
		pdfPath += ".pdf"
	}
	parsed.Path = pdfPath
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func extractArxivID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		path := strings.Trim(parsed.Path, "/")
		if strings.HasPrefix(path, "abs/") {
			path = strings.TrimPrefix(path, "abs/")
		}
		if strings.HasPrefix(path, "pdf/") {
			path = strings.TrimPrefix(path, "pdf/")
		}
		path = strings.TrimSuffix(path, ".pdf")
		if arxivIDPattern.MatchString(path) {
			return path
		}
		return ""
	}

	raw = strings.TrimPrefix(raw, "arXiv:")
	if arxivIDPattern.MatchString(raw) {
		return raw
	}
	return ""
}

func countStoredPDFFiles(folderPath string) int {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".pdf") {
			count++
		}
	}
	return count
}
