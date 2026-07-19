package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
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
	pdfDownloadMaxRedirects  = 5
)

var arxivIDPattern = regexp.MustCompile(`^\d{4}\.\d{4,5}(v\d+)?$`)
var pdfDownloadMaxBytes int64 = 100 * 1024 * 1024

// Tests use httptest loopback servers for deterministic PDF downloads. Keep the
// production default fail-closed for local/private hosts.
var allowPrivatePDFDownloadHostsForTest bool
var pdfDownloadHTTPTransport http.RoundTripper = http.DefaultTransport

type paperDownloadJob struct {
	PaperID       string
	FolderID      string
	FolderPath    string
	DataPath      string
	SourceURL     string
	SourcePaperID string
	ExternalIDs   map[string]string
	PDFCandidates []string
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

	resolvedFolderID, err := a.resolveFolderID(folderID)
	if err != nil {
		return nil, err
	}
	folderID = resolvedFolderID

	result := &ImportPapersWithAssetsResult{
		Imported: make([]Paper, 0, len(papers)),
		Skipped:  make([]ImportSkippedPaper, 0),
	}

	folder, err := a.db.GetFolderByID(folderID)
	if err != nil {
		return nil, err
	}
	targetFolderPath := normalizeFolderPath(folder.Path)

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

		a.enqueuePaperDownload(paperDownloadJob{
			PaperID:       paper.ID,
			FolderID:      paper.FolderID,
			FolderPath:    targetFolderPath,
			DataPath:      a.config.DataPath,
			SourceURL:     strings.TrimSpace(searchPaper.URL),
			SourcePaperID: sourcePaperID,
			ExternalIDs:   normalizeExternalIDMap(searchPaper.ExternalIDs),
			PDFCandidates: compactStrings(searchPaper.PDFCandidates, 8),
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
	if strings.TrimSpace(job.DataPath) == "" {
		job.DataPath = a.config.DataPath
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

	if strings.TrimSpace(job.DataPath) == "" {
		job.DataPath = a.config.DataPath
	}
	targetFolderPath := a.resolvePaperDownloadFolderPath(job.FolderID, job.FolderPath)
	papersRoot := filepath.Join(job.DataPath, "papers")
	targetPath := filepath.Join(
		papersRoot,
		filepath.FromSlash(targetFolderPath),
		safePaperPDFFileName(job.PaperID),
	)
	managedTargetPath, ok := managedPathInsideRoot(papersRoot, targetPath)
	if !ok {
		_ = a.db.UpdatePaperDownloadState(job.PaperID, "failed", "", "download target is outside managed papers directory")
		return
	}
	targetPath = managedTargetPath
	targetPath, err := ensureManagedFileParent(papersRoot, targetPath)
	if err != nil {
		_ = a.db.UpdatePaperDownloadState(job.PaperID, "failed", "", err.Error())
		return
	}
	candidates := buildPDFCandidateURLs(job.SourceURL, job.SourcePaperID, job.ExternalIDs, job.PDFCandidates)
	if len(candidates) == 0 {
		_ = a.db.UpdatePaperDownloadState(job.PaperID, "failed", "", "no downloadable pdf url")
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

func (a *App) resolvePaperDownloadFolderPath(folderID, folderPath string) string {
	if normalized := normalizeFolderPath(folderPath); normalized != "" {
		return normalized
	}
	if a != nil && a.db != nil {
		if folder, err := a.db.GetFolderByID(folderID); err == nil {
			if normalized := normalizeFolderPath(folder.Path); normalized != "" {
				return normalized
			}
			if normalized := normalizeFolderPath(folder.Name); normalized != "" {
				return normalized
			}
			return safeSyncSegment(folder.ID)
		}
	}
	return safeSyncSegment(folderID)
}

func downloadPDFToFile(ctx context.Context, rawURL, targetPath string) error {
	parsedURL, err := validatePDFDownloadURL(ctx, rawURL)
	if err != nil {
		return err
	}

	targetDir := filepath.Dir(targetPath)
	if err := ensurePlainDirectory(targetDir); err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, pdfDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, parsedURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create pdf download request: %s", redactURLQueryValuesInText(err.Error()))
	}
	req.Header.Set("User-Agent", "DiveEnd/1.0 (+https://github.com)")
	req.Header.Set("Accept", "application/pdf,*/*;q=0.8")

	client := &http.Client{
		Transport: pdfDownloadHTTPTransport,
		CheckRedirect: func(redirectReq *http.Request, via []*http.Request) error {
			if len(via) >= pdfDownloadMaxRedirects {
				return fmt.Errorf("too many pdf url redirects")
			}
			validatedRedirectURL, err := validatePDFDownloadURL(redirectReq.Context(), redirectReq.URL.String())
			if err != nil {
				return err
			}
			parsedRedirectURL, err := url.Parse(validatedRedirectURL)
			if err != nil {
				return err
			}
			redirectReq.URL = parsedRedirectURL
			redirectReq.Header.Set("User-Agent", "DiveEnd/1.0 (+https://github.com)")
			redirectReq.Header.Set("Accept", "application/pdf,*/*;q=0.8")
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download pdf: %s", redactURLQueryValuesInText(err.Error()))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if resp.ContentLength > pdfDownloadMaxBytes {
		return fmt.Errorf("pdf is too large: %d bytes exceeds %d bytes limit", resp.ContentLength, pdfDownloadMaxBytes)
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	return writeFileAtomicWithWriter(targetPath, 0600, func(file *os.File) error {
		limitedBody := io.LimitReader(resp.Body, pdfDownloadMaxBytes+1)
		firstChunk := make([]byte, 1024)
		n, readErr := limitedBody.Read(firstChunk)
		totalWritten := int64(n)
		if n > 0 {
			if _, err := file.Write(firstChunk[:n]); err != nil {
				return err
			}
		}
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		if !looksLikePDF(contentType, firstChunk[:n]) {
			return fmt.Errorf("response is not a valid pdf")
		}

		copied, err := io.Copy(file, limitedBody)
		totalWritten += copied
		if err != nil {
			return err
		}
		if totalWritten > pdfDownloadMaxBytes {
			return fmt.Errorf("pdf is too large: exceeds %d bytes limit", pdfDownloadMaxBytes)
		}
		return nil
	}, func(file *os.File, _ string) error {
		return validateOpenLocalPDFFile(file)
	})
}

func validatePDFDownloadURL(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty url")
	}

	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed == nil {
		return "", fmt.Errorf("pdf url must be a valid http/https url")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("pdf url must be a valid http/https url")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", fmt.Errorf("pdf url must include a host")
	}
	if err := rejectPrivateDownloadHost(ctx, host); err != nil {
		return "", err
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func rejectPrivateDownloadHost(ctx context.Context, host string) error {
	host = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(host), "."))
	if host == "" {
		return fmt.Errorf("pdf url must include a host")
	}
	if allowPrivatePDFDownloadHostsForTest {
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("pdf url host is not allowed: %s", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedDownloadIP(ip) {
			return fmt.Errorf("pdf url host resolves to a private or local address: %s", host)
		}
		return nil
	}

	lookupCtx, cancel := context.WithTimeout(appContext(ctx), 2*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return fmt.Errorf("failed to resolve pdf url host %s: %w", host, err)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("failed to resolve pdf url host %s", host)
	}
	for _, address := range addresses {
		if isBlockedDownloadIP(address.IP) {
			return fmt.Errorf("pdf url host resolves to a private or local address: %s", host)
		}
	}
	return nil
}

func isBlockedDownloadIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

func looksLikePDF(contentType string, prefix []byte) bool {
	if strings.Contains(contentType, "application/pdf") {
		return true
	}
	trimmed := bytes.TrimSpace(prefix)
	return bytes.HasPrefix(trimmed, []byte("%PDF-"))
}

func buildPDFCandidateURLs(sourceURL, sourcePaperID string, externalIDs map[string]string, extraCandidates []string) []string {
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

	for _, candidate := range extraCandidates {
		appendCandidate(candidate)
	}

	normalizedSourceURL := strings.TrimSpace(sourceURL)
	if normalizedSourceURL != "" {
		if strings.Contains(strings.ToLower(normalizedSourceURL), "/abs/") && strings.Contains(strings.ToLower(normalizedSourceURL), "arxiv.org") {
			appendCandidate(arxivAbsToPDFURL(normalizedSourceURL))
		}
		appendCandidate(normalizedSourceURL)
	}

	if arxivID := extractArxivID(externalIDValue(externalIDs, "ArXiv")); arxivID != "" {
		appendCandidate(fmt.Sprintf("https://arxiv.org/pdf/%s.pdf", arxivID))
	}
	if doi := strings.TrimSpace(externalIDValue(externalIDs, "DOI")); doi != "" {
		doi = strings.TrimPrefix(strings.TrimPrefix(doi, "doi:"), "DOI:")
		appendCandidate("https://doi.org/" + doi)
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
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".pdf") {
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if err := validateLocalPDFFile(filepath.Join(folderPath, entry.Name())); err != nil {
				continue
			}
			count++
		}
	}
	return count
}

func (a *App) RetryPaperDownload(paperID string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}

	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return fmt.Errorf("paper id cannot be empty")
	}

	paper, err := a.db.GetPaperByID(paperID)
	if err != nil {
		return err
	}

	folderPath := ""
	if folder, folderErr := a.db.GetFolderByID(paper.FolderID); folderErr == nil {
		folderPath = normalizeFolderPath(folder.Path)
	}

	if err := a.db.UpdatePaperDownloadState(paper.ID, "queued", "", ""); err != nil {
		return err
	}

	a.enqueuePaperDownload(paperDownloadJob{
		PaperID:       paper.ID,
		FolderID:      paper.FolderID,
		FolderPath:    folderPath,
		DataPath:      a.config.DataPath,
		SourceURL:     strings.TrimSpace(paper.URL),
		SourcePaperID: strings.TrimSpace(paper.SourcePaperID),
	})
	return nil
}

func validateManualDownloadURL(raw string) (string, error) {
	return validatePDFDownloadURL(context.Background(), raw)
}

func (a *App) RetryPaperDownloadWithURL(paperID, manualURL string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}

	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return fmt.Errorf("paper id cannot be empty")
	}

	validatedURL, err := validateManualDownloadURL(manualURL)
	if err != nil {
		return err
	}

	paper, err := a.db.GetPaperByID(paperID)
	if err != nil {
		return err
	}

	folderPath := ""
	if folder, folderErr := a.db.GetFolderByID(paper.FolderID); folderErr == nil {
		folderPath = normalizeFolderPath(folder.Path)
	}

	paper.URL = validatedURL
	paper.UpdatedAt = time.Now()
	if err := a.db.UpsertPaper(paper); err != nil {
		return err
	}

	if err := a.db.UpdatePaperDownloadState(paper.ID, "queued", "", ""); err != nil {
		return err
	}

	a.enqueuePaperDownload(paperDownloadJob{
		PaperID:       paper.ID,
		FolderID:      paper.FolderID,
		FolderPath:    folderPath,
		DataPath:      a.config.DataPath,
		SourceURL:     validatedURL,
		SourcePaperID: strings.TrimSpace(paper.SourcePaperID),
	})
	return nil
}
