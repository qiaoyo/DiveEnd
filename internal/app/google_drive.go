package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	googleDriveFolderMimeType         = "application/vnd.google-apps.folder"
	googleDriveScope                  = drive.DriveFileScope
	googleDriveUploadChunk            = 32 * 1024 * 1024
	googleDriveDownloadMaxBytes int64 = 512 * 1024 * 1024
	googleDriveHTTPTimeout            = 5 * time.Minute
	googleDriveAuthTimeout            = 5 * time.Minute
	googleDriveRetryAttempts          = 4
)

// GoogleDriveProvider maps the provider-neutral DiveEnd paths onto one
// user-visible folder in My Drive. The logical remote path remains stable so
// manifests and conflict records are portable between providers.
type GoogleDriveProvider struct {
	service      *drive.Service
	config       GoogleDriveConfig
	tokenPath    string
	rootFolderID string
	rootFolderMu sync.Mutex
}

func NewGoogleDriveProvider(config GoogleDriveConfig) (*GoogleDriveProvider, error) {
	config = normalizeGoogleDriveConfig(config)
	clientSecretPath, err := resolveGoogleDriveClientSecretPath(config.ClientSecretPath)
	if err != nil {
		return nil, err
	}
	oauthConfig, err := loadGoogleDriveOAuthConfig(clientSecretPath)
	if err != nil {
		return nil, err
	}
	tokenPath := defaultGoogleDriveTokenPath()
	token, err := loadGoogleDriveToken(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("google drive is not authorized: %w", err)
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return nil, fmt.Errorf("google drive token does not contain a refresh token; authorize again")
	}

	ctx := context.Background()
	source := &persistingGoogleTokenSource{
		source:    oauthConfig.TokenSource(ctx, token),
		tokenPath: tokenPath,
		lastKey:   googleDriveTokenKey(token),
	}
	httpClient := oauth2.NewClient(ctx, source)
	httpClient.Timeout = googleDriveHTTPTimeout
	service, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Google Drive client: %w", err)
	}

	return &GoogleDriveProvider{
		service:   service,
		config:    config,
		tokenPath: tokenPath,
	}, nil
}

func (p *GoogleDriveProvider) Name() string {
	return "google_drive"
}

func (p *GoogleDriveProvider) RemoteRootLabel() string {
	if p == nil {
		return "Google Drive/DiveEnd Backup"
	}
	return "Google Drive/" + p.config.RootFolderName
}

func (p *GoogleDriveProvider) Check() error {
	if p == nil || p.service == nil {
		return fmt.Errorf("Google Drive client is not initialized")
	}
	rootID, err := p.ensureRootFolder()
	if err != nil {
		return err
	}
	_, err = googleDriveRetryValue(func() (*drive.File, error) {
		return p.service.Files.Get(rootID).Fields("id,name,mimeType").Do()
	})
	if err != nil {
		return fmt.Errorf("Google Drive connection check failed: %w", err)
	}
	return nil
}

func (p *GoogleDriveProvider) UploadFileToPath(localPath, remotePath string) error {
	if p == nil || p.service == nil {
		return fmt.Errorf("Google Drive client is not initialized")
	}
	localPath = filepath.Clean(strings.TrimSpace(localPath))
	if localPath == "" {
		return fmt.Errorf("local upload path cannot be empty")
	}
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to inspect upload file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("cannot upload a directory")
	}

	parts, err := googleDriveRelativeParts(remotePath)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return fmt.Errorf("Google Drive upload path must identify a file")
	}
	parentID, err := p.ensureFolderPath(parts[:len(parts)-1])
	if err != nil {
		return err
	}
	fileName := parts[len(parts)-1]
	existing, err := p.findChild(parentID, fileName, false)
	if err != nil {
		return err
	}

	operation := func() error {
		file, err := os.Open(localPath)
		if err != nil {
			return err
		}
		defer file.Close()

		metadata := &drive.File{
			Name:     fileName,
			MimeType: googleDriveMimeType(fileName),
		}
		if existing == nil {
			metadata.Parents = []string{parentID}
			_, err = p.service.Files.Create(metadata).
				Media(file, googleapi.ChunkSize(googleDriveUploadChunk)).
				Fields("id,name,size,modifiedTime,md5Checksum").
				Do()
		} else {
			_, err = p.service.Files.Update(existing.Id, metadata).
				Media(file, googleapi.ChunkSize(googleDriveUploadChunk)).
				Fields("id,name,size,modifiedTime,md5Checksum").
				Do()
		}
		return err
	}
	if err := googleDriveRetry(operation); err != nil {
		return fmt.Errorf("Google Drive upload failed for %s: %w", remotePath, err)
	}
	return nil
}

func (p *GoogleDriveProvider) DownloadFile(remotePath, localPath string) error {
	if p == nil || p.service == nil {
		return fmt.Errorf("Google Drive client is not initialized")
	}
	parts, err := googleDriveRelativeParts(remotePath)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return fmt.Errorf("Google Drive download path must identify a file")
	}
	parentID, err := p.ensureFolderPath(parts[:len(parts)-1])
	if err != nil {
		return err
	}
	fileName := parts[len(parts)-1]
	remoteFile, err := p.findChild(parentID, fileName, false)
	if err != nil {
		return err
	}
	if remoteFile == nil {
		return fmt.Errorf("Google Drive file not found: %s", remotePath)
	}
	if remoteFile.Size > googleDriveDownloadMaxBytes {
		return fmt.Errorf("Google Drive file exceeds download limit: %d bytes > %d bytes", remoteFile.Size, googleDriveDownloadMaxBytes)
	}

	localPath = filepath.Clean(strings.TrimSpace(localPath))
	if localPath == "" {
		return fmt.Errorf("local download path cannot be empty")
	}

	operation := func() error {
		response, err := p.service.Files.Get(remoteFile.Id).Download()
		if err != nil {
			return err
		}
		defer response.Body.Close()

		tmp, err := newAtomicTempFile(localPath, 0600)
		if err != nil {
			return err
		}
		defer tmp.cleanup()
		copyLimit := googleDriveDownloadMaxBytes + 1
		if remoteFile.Size >= 0 && remoteFile.Size < googleDriveDownloadMaxBytes {
			copyLimit = remoteFile.Size + 1
		}
		written, err := io.Copy(tmp.file, io.LimitReader(response.Body, copyLimit))
		if err != nil {
			return err
		}
		if written > googleDriveDownloadMaxBytes {
			return fmt.Errorf("Google Drive download exceeds limit: %d bytes > %d bytes", written, googleDriveDownloadMaxBytes)
		}
		if remoteFile.Size >= 0 && written != remoteFile.Size {
			return fmt.Errorf("Google Drive downloaded size mismatch: got %d bytes, expected %d bytes", written, remoteFile.Size)
		}
		if err := tmp.file.Chmod(0600); err != nil {
			return err
		}
		if err := tmp.file.Sync(); err != nil {
			return err
		}
		return tmp.commit()
	}
	if err := googleDriveRetry(operation); err != nil {
		return fmt.Errorf("Google Drive download failed for %s: %w", remotePath, err)
	}
	return nil
}

func (p *GoogleDriveProvider) EnsureRemoteDirectories(remoteDir string) error {
	parts, err := googleDriveRelativeParts(remoteDir)
	if err != nil {
		return err
	}
	_, err = p.ensureFolderPath(parts)
	return err
}

func (p *GoogleDriveProvider) ListFilesRecursive(remoteRoot string) ([]FileInfo, error) {
	parts, err := googleDriveRelativeParts(remoteRoot)
	if err != nil {
		return nil, err
	}
	rootID, err := p.ensureFolderPath(parts)
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, 0)
	var walk func(string, string) error
	walk = func(parentID, logicalPath string) error {
		children, err := p.listChildren(parentID)
		if err != nil {
			return err
		}
		for _, child := range children {
			childPath := path.Join(logicalPath, child.Name)
			modified, parseErr := time.Parse(time.RFC3339, child.ModifiedTime)
			if parseErr != nil {
				modified = time.Time{}
			}
			if child.MimeType == googleDriveFolderMimeType {
				files = append(files, FileInfo{Path: childPath, IsDir: true, Modified: modified})
				if err := walk(child.Id, childPath); err != nil {
					return err
				}
				continue
			}
			files = append(files, FileInfo{
				Path:     childPath,
				Size:     child.Size,
				Modified: modified,
				MD5:      child.Md5Checksum,
			})
		}
		return nil
	}

	if err := walk(rootID, remoteRoot); err != nil {
		return nil, err
	}
	return files, nil
}

func (p *GoogleDriveProvider) ensureRootFolder() (string, error) {
	p.rootFolderMu.Lock()
	defer p.rootFolderMu.Unlock()
	if p.rootFolderID != "" {
		return p.rootFolderID, nil
	}

	folder, err := p.findChild("root", p.config.RootFolderName, true)
	if err != nil {
		return "", err
	}
	if folder == nil {
		folder, err = googleDriveRetryValue(func() (*drive.File, error) {
			return p.service.Files.Create(&drive.File{
				Name:          p.config.RootFolderName,
				MimeType:      googleDriveFolderMimeType,
				Parents:       []string{"root"},
				AppProperties: map[string]string{"diveend": "backup-root"},
			}).Fields("id,name,mimeType").Do()
		})
		if err != nil {
			return "", fmt.Errorf("failed to create Google Drive root folder: %w", err)
		}
	}
	p.rootFolderID = folder.Id
	return p.rootFolderID, nil
}

func (p *GoogleDriveProvider) ensureFolderPath(parts []string) (string, error) {
	parentID, err := p.ensureRootFolder()
	if err != nil {
		return "", err
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return "", fmt.Errorf("Google Drive folder name cannot be empty")
		}
		folder, err := p.findChild(parentID, part, true)
		if err != nil {
			return "", err
		}
		if folder == nil {
			folder, err = googleDriveRetryValue(func() (*drive.File, error) {
				return p.service.Files.Create(&drive.File{
					Name:     part,
					MimeType: googleDriveFolderMimeType,
					Parents:  []string{parentID},
				}).Fields("id,name,mimeType").Do()
			})
			if err != nil {
				return "", fmt.Errorf("failed to create Google Drive folder %q: %w", part, err)
			}
		}
		parentID = folder.Id
	}
	return parentID, nil
}

func (p *GoogleDriveProvider) findChild(parentID, name string, folderOnly bool) (*drive.File, error) {
	children, err := p.listChildren(parentID)
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		if child.Name != name {
			continue
		}
		if folderOnly && child.MimeType != googleDriveFolderMimeType {
			continue
		}
		if !folderOnly && child.MimeType == googleDriveFolderMimeType {
			continue
		}
		return child, nil
	}
	return nil, nil
}

func (p *GoogleDriveProvider) listChildren(parentID string) ([]*drive.File, error) {
	files := make([]*drive.File, 0)
	pageToken := ""
	for {
		result, err := googleDriveRetryValue(func() (*drive.FileList, error) {
			call := p.service.Files.List().
				Q(fmt.Sprintf("'%s' in parents and trashed = false", googleDriveQueryEscape(parentID))).
				Spaces("drive").
				PageSize(1000).
				Fields("nextPageToken,files(id,name,mimeType,size,modifiedTime,md5Checksum)")
			if pageToken != "" {
				call.PageToken(pageToken)
			}
			return call.Do()
		})
		if err != nil {
			return nil, err
		}
		files = append(files, result.Files...)
		pageToken = result.NextPageToken
		if pageToken == "" {
			return files, nil
		}
	}
}

func googleDriveRelativeParts(remotePath string) ([]string, error) {
	normalized := path.Clean("/" + strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(remotePath)), "/"))
	root := syncRemoteRootPath()
	if normalized == root {
		return nil, nil
	}
	prefix := strings.TrimRight(root, "/") + "/"
	if !strings.HasPrefix(normalized, prefix) {
		return nil, fmt.Errorf("Google Drive path is outside the DiveEnd root: %s", remotePath)
	}
	relative := strings.TrimPrefix(normalized, prefix)
	parts := strings.Split(relative, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `/\\`) {
			return nil, fmt.Errorf("invalid Google Drive path segment")
		}
	}
	return parts, nil
}

func googleDriveQueryEscape(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "'", "\\'")
}

func googleDriveMimeType(name string) string {
	if strings.EqualFold(path.Ext(name), ".pdf") {
		return "application/pdf"
	}
	if strings.EqualFold(path.Ext(name), ".json") {
		return "application/json"
	}
	return "application/octet-stream"
}

func googleDriveRetry(operation func() error) error {
	var err error
	for attempt := 0; attempt < googleDriveRetryAttempts; attempt++ {
		err = operation()
		if err == nil || !isRetryableGoogleDriveError(err) || attempt == googleDriveRetryAttempts-1 {
			return err
		}
		time.Sleep(time.Duration(1<<attempt) * time.Second)
	}
	return err
}

func googleDriveRetryValue[T any](operation func() (T, error)) (T, error) {
	var value T
	err := googleDriveRetry(func() error {
		var operationErr error
		value, operationErr = operation()
		return operationErr
	})
	return value, err
}

func isRetryableGoogleDriveError(err error) bool {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.Code == http.StatusTooManyRequests || apiErr.Code >= 500 {
		return true
	}
	if apiErr.Code != http.StatusForbidden {
		return false
	}
	for _, item := range apiErr.Errors {
		switch item.Reason {
		case "rateLimitExceeded", "userRateLimitExceeded", "backendError":
			return true
		}
	}
	return false
}

func resolveGoogleDriveClientSecretPath(configuredPath string) (string, error) {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" {
		return "", fmt.Errorf("Google Drive OAuth client JSON path is not configured")
	}
	if filepath.IsAbs(configuredPath) {
		return filepath.Clean(configuredPath), nil
	}
	candidates := make([]string, 0, 12)
	seen := make(map[string]struct{})
	addCandidate := func(candidate string) {
		candidate = filepath.Clean(candidate)
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}
	addCandidate(configuredPath)
	if workingDir, err := os.Getwd(); err == nil && strings.TrimSpace(workingDir) != "" {
		addCandidate(filepath.Join(workingDir, configuredPath))
	}
	// Finder does not preserve the repository as the process working directory.
	// Walk up from the executable so a packaged app built inside this repository
	// can still resolve the documented relative config path.
	if executable, err := os.Executable(); err == nil {
		current := filepath.Dir(executable)
		for level := 0; level < 8; level++ {
			addCandidate(filepath.Join(current, configuredPath))
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	if configDir, err := userConfigDirFunc(); err == nil && strings.TrimSpace(configDir) != "" {
		addCandidate(filepath.Join(configDir, "DiveEnd", filepath.Base(configuredPath)))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return candidates[0], nil
}

func loadGoogleDriveOAuthConfig(clientSecretPath string) (*oauth2.Config, error) {
	data, err := readLimitedFile(clientSecretPath, seedConfigFileLimitBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read Google Drive OAuth client JSON %q: %w", clientSecretPath, err)
	}
	oauthConfig, err := google.ConfigFromJSON(data, googleDriveScope)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Google Drive OAuth client JSON: %w", err)
	}
	return oauthConfig, nil
}

func defaultGoogleDriveTokenPath() string {
	configDir, err := userConfigDirFunc()
	if err != nil || strings.TrimSpace(configDir) == "" {
		return filepath.Join(".", "google_drive_token.json")
	}
	return filepath.Join(configDir, "DiveEnd", "google_drive_token.json")
}

func loadGoogleDriveToken(tokenPath string) (*oauth2.Token, error) {
	data, err := readLimitedFile(tokenPath, seedConfigFileLimitBytes)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse Google Drive token: %w", err)
	}
	if strings.TrimSpace(token.AccessToken) == "" && strings.TrimSpace(token.RefreshToken) == "" {
		return nil, fmt.Errorf("Google Drive token is empty")
	}
	return &token, nil
}

func googleDriveTokenAvailable() bool {
	token, err := loadGoogleDriveToken(defaultGoogleDriveTokenPath())
	return err == nil && strings.TrimSpace(token.RefreshToken) != ""
}

func saveGoogleDriveToken(tokenPath string, token *oauth2.Token) error {
	if token == nil {
		return fmt.Errorf("Google Drive token cannot be nil")
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(tokenPath, data, 0600)
}

func deleteGoogleDriveToken() error {
	tokenPath := defaultGoogleDriveTokenPath()
	if _, err := os.Lstat(tokenPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return removeFileAndSyncDir(tokenPath)
}

func googleDriveAuthStatus(config GoogleDriveConfig) *GoogleDriveAuthStatus {
	config = normalizeGoogleDriveConfig(config)
	clientSecretPath, _ := resolveGoogleDriveClientSecretPath(config.ClientSecretPath)
	status := &GoogleDriveAuthStatus{
		Enabled:          config.Enabled,
		ClientSecretPath: clientSecretPath,
		TokenPath:        defaultGoogleDriveTokenPath(),
		RootFolderName:   config.RootFolderName,
		CheckedAt:        time.Now(),
	}
	if _, err := loadGoogleDriveOAuthConfig(clientSecretPath); err == nil {
		status.HasClientSecret = true
	}
	token, err := loadGoogleDriveToken(status.TokenPath)
	if err == nil && strings.TrimSpace(token.RefreshToken) != "" {
		status.Authorized = true
	}
	switch {
	case !status.HasClientSecret:
		status.Message = "Google Drive OAuth 客户端 JSON 尚未配置"
	case !status.Authorized:
		status.Message = "Google Drive 尚未完成授权"
	default:
		status.Message = "Google Drive 已授权"
	}
	return status
}

type persistingGoogleTokenSource struct {
	source    oauth2.TokenSource
	tokenPath string
	lastKey   string
	mu        sync.Mutex
}

func (s *persistingGoogleTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	key := googleDriveTokenKey(token)
	if key != s.lastKey {
		if err := saveGoogleDriveToken(s.tokenPath, token); err != nil {
			return nil, fmt.Errorf("Google Drive token refreshed but could not be saved: %w", err)
		}
		s.lastKey = key
	}
	return token, nil
}

func googleDriveTokenKey(token *oauth2.Token) string {
	if token == nil {
		return ""
	}
	return token.AccessToken + "\x00" + token.RefreshToken + "\x00" + token.Expiry.UTC().Format(time.RFC3339Nano)
}

func authorizeGoogleDrive(ctx context.Context, clientSecretPath, tokenPath string, openURL func(string) error) (*oauth2.Token, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	oauthConfig, err := loadGoogleDriveOAuthConfig(clientSecretPath)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start Google OAuth callback listener: %w", err)
	}
	defer listener.Close()
	oauthConfig.RedirectURL = "http://" + listener.Addr().String() + "/oauth2callback"
	state, err := randomGoogleOAuthState()
	if err != nil {
		return nil, err
	}
	verifier, err := randomGoogleOAuthState()
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth2callback" {
			http.NotFound(writer, request)
			return
		}
		if request.URL.Query().Get("state") != state {
			writeGoogleOAuthResponse(writer, "授权状态校验失败，请关闭页面并重试。")
			select {
			case errCh <- fmt.Errorf("Google OAuth state validation failed"):
			default:
			}
			return
		}
		if callbackErr := request.URL.Query().Get("error"); callbackErr != "" {
			writeGoogleOAuthResponse(writer, "Google Drive 授权未完成，可以关闭此页面。")
			select {
			case errCh <- fmt.Errorf("Google OAuth authorization failed: %s", callbackErr):
			default:
			}
			return
		}
		code := strings.TrimSpace(request.URL.Query().Get("code"))
		if code == "" {
			writeGoogleOAuthResponse(writer, "未收到授权码，请关闭页面并重试。")
			select {
			case errCh <- fmt.Errorf("Google OAuth callback did not contain an authorization code"):
			default:
			}
			return
		}
		writeGoogleOAuthResponse(writer, "DiveEnd 已收到 Google Drive 授权，可以关闭此页面并返回应用。")
		select {
		case codeCh <- code:
		default:
		}
	})}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			select {
			case errCh <- serveErr:
			default:
			}
		}
	}()
	defer server.Shutdown(context.Background())

	authURL := oauthConfig.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	if openURL == nil {
		return nil, fmt.Errorf("Google OAuth browser opener is not configured; open this URL manually: %s", authURL)
	}
	if err := openURL(authURL); err != nil {
		return nil, err
	}

	waitCtx, cancel := context.WithTimeout(ctx, googleDriveAuthTimeout)
	defer cancel()
	select {
	case code := <-codeCh:
		token, err := oauthConfig.Exchange(waitCtx, code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("failed to exchange Google OAuth code: %w", err)
		}
		if strings.TrimSpace(token.RefreshToken) == "" {
			return nil, fmt.Errorf("Google OAuth did not return a refresh token; revoke the existing DiveEnd grant and authorize again")
		}
		if err := saveGoogleDriveToken(tokenPath, token); err != nil {
			return nil, err
		}
		return token, nil
	case callbackErr := <-errCh:
		return nil, callbackErr
	case <-waitCtx.Done():
		return nil, fmt.Errorf("Google OAuth authorization timed out: %w", waitCtx.Err())
	}
}

func randomGoogleOAuthState() (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("failed to create OAuth state: %w", err)
	}
	return hex.EncodeToString(bytes[:]), nil
}

func writeGoogleOAuthResponse(writer http.ResponseWriter, message string) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(writer, "<!doctype html><html><body><p>%s</p></body></html>", html.EscapeString(message))
}
