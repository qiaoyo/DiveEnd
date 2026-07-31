package app

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	baiduXPAN  = "https://pan.baidu.com/rest/2.0/xpan"
	baiduPCS   = "https://d.pcs.baidu.com/rest/2.0/pcs/superfile2"
	baiduAuth  = "https://openapi.baidu.com/oauth/2.0/token"
	baiduQuota = "https://pan.baidu.com/api/quota"
	blockSize  = 4 * 1024 * 1024 // 4MB

	baiduHTTPTimeout          = 30 * time.Second
	baiduDownloadMaxRedirects = 5
)

var baiduDownloadMaxBytes int64 = 512 * 1024 * 1024

// BaiduPCSClient 百度网盘客户端
type BaiduPCSClient struct {
	token         *BaiduToken
	tokenFilePath string
	httpClient    *http.Client
	tokenMu       sync.Mutex
}

// NewBaiduPCSClient 创建百度网盘客户端
func NewBaiduPCSClient(token *BaiduToken) *BaiduPCSClient {
	return NewBaiduPCSClientWithTokenPath(token, "")
}

func NewBaiduPCSClientWithTokenPath(token *BaiduToken, tokenFilePath string) *BaiduPCSClient {
	return &BaiduPCSClient{
		token:         token,
		tokenFilePath: strings.TrimSpace(tokenFilePath),
		httpClient:    defaultBaiduHTTPClient(),
	}
}

func defaultBaiduHTTPClient() *http.Client {
	return &http.Client{Timeout: baiduHTTPTimeout}
}

// TokenResponse token 响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	ExpiresIn    int    `json:"expires_in"`
}

// LoadBaiduToken 从文件加载 token
func LoadBaiduToken(path string) (*BaiduToken, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("token file path cannot be empty")
	}

	data, err := readLimitedFile(path, seedConfigFileLimitBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token BaiduToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// SaveBaiduToken 保存 token 到文件
func SaveBaiduToken(path string, token *BaiduToken) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := writeFileAtomic(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// RefreshToken 刷新 access_token
func RefreshToken(token *BaiduToken) (*BaiduToken, error) {
	return refreshTokenWithClient(defaultBaiduHTTPClient(), token)
}

func refreshTokenWithClient(client *http.Client, token *BaiduToken) (*BaiduToken, error) {
	if client == nil {
		client = defaultBaiduHTTPClient()
	}
	params := url.Values{}
	params.Set("grant_type", "refresh_token")
	params.Set("refresh_token", token.RefreshToken)
	params.Set("client_id", token.ClientID)
	params.Set("client_secret", token.ClientSecret)

	reqURL := fmt.Sprintf("%s?%s", baiduAuth, params.Encode())
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token request: %s", redactErrorText(err))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %s", redactErrorText(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("refresh token failed: http status %d", resp.StatusCode)
	}

	var result TokenResponse
	if err := decodeExternalJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("refresh token failed: no access_token in response")
	}

	updatedToken := &BaiduToken{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ClientID:     token.ClientID,
		ClientSecret: token.ClientSecret,
	}
	if strings.TrimSpace(updatedToken.RefreshToken) == "" {
		updatedToken.RefreshToken = token.RefreshToken
	}

	return updatedToken, nil
}

// GetAccessToken 获取有效的 access_token，过期自动刷新
func (c *BaiduPCSClient) GetAccessToken() (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token == nil {
		return "", fmt.Errorf("token is nil")
	}
	if strings.TrimSpace(c.token.AccessToken) == "" {
		return "", fmt.Errorf("access token is empty")
	}

	// 检查 token 是否有效
	params := url.Values{}
	params.Set("access_token", c.token.AccessToken)
	params.Set("checkexpire", "1")

	reqURL := fmt.Sprintf("%s?%s", baiduQuota, params.Encode())
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return "", fmt.Errorf("failed to check token: %s", redactErrorText(err))
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := decodeExternalJSON(resp.Body, &result); err != nil {
		return "", fmt.Errorf("failed to decode quota response: %w", err)
	}

	// Mirrors the proven Python flow: token is valid when errno == 0 and
	// `expire` is absent/empty/false/0.
	if errno, ok := result["errno"].(float64); ok && errno == 0 {
		if tokenStillValid(result["expire"]) {
			return c.token.AccessToken, nil
		}
	}

	// token 过期，刷新
	log.Println("[Token] 已过期，正在刷新...")
	if strings.TrimSpace(c.token.RefreshToken) == "" || strings.TrimSpace(c.token.ClientID) == "" || strings.TrimSpace(c.token.ClientSecret) == "" {
		return "", fmt.Errorf("baidu access token expired and refresh_token/client_id/client_secret are not configured")
	}
	updatedToken, err := refreshTokenWithClient(c.httpClient, c.token)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %s", redactErrorText(err))
	}

	c.token = updatedToken
	if c.tokenFilePath != "" {
		if err := SaveBaiduToken(c.tokenFilePath, updatedToken); err != nil {
			return "", fmt.Errorf("token refreshed but save failed: %w", err)
		}
	}
	log.Println("[Token] 刷新成功")
	return updatedToken.AccessToken, nil
}

// RefreshAccessToken forces an OAuth refresh using refresh_token and persists
// the rotated token to the canonical token file when configured.
func (c *BaiduPCSClient) RefreshAccessToken() (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token == nil {
		return "", fmt.Errorf("token is nil")
	}
	if strings.TrimSpace(c.token.RefreshToken) == "" || strings.TrimSpace(c.token.ClientID) == "" || strings.TrimSpace(c.token.ClientSecret) == "" {
		return "", fmt.Errorf("refresh_token/client_id/client_secret are not configured")
	}

	log.Println("[Token] 手动刷新 access token...")
	updatedToken, err := refreshTokenWithClient(c.httpClient, c.token)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %s", redactErrorText(err))
	}

	c.token = updatedToken
	if c.tokenFilePath != "" {
		if err := SaveBaiduToken(c.tokenFilePath, updatedToken); err != nil {
			return "", fmt.Errorf("token refreshed but save failed: %w", err)
		}
	}
	log.Println("[Token] 手动刷新成功")
	return updatedToken.AccessToken, nil
}

func tokenStillValid(expire any) bool {
	if expire == nil {
		return true
	}
	switch value := expire.(type) {
	case bool:
		return !value
	case float64:
		return value == 0
	case string:
		value = strings.TrimSpace(value)
		return value == "" || value == "0" || strings.EqualFold(value, "false")
	default:
		return false
	}
}

// UploadFile 上传文件到百度网盘
func (c *BaiduPCSClient) UploadFile(localPath, remoteName string) error {
	if remoteName == "" {
		remoteName = filepath.Base(localPath)
	}
	remotePath := fmt.Sprintf("/apps/%s/%s", syncApp, remoteName)
	return c.UploadFileToPath(localPath, remotePath)
}

func (c *BaiduPCSClient) UploadFileToPath(localPath, remotePath string) error {
	normalizedRemotePath, err := normalizeBaiduAppRemoteFilePath(remotePath)
	if err != nil {
		return err
	}
	remotePath = normalizedRemotePath

	uploadFile, fileInfo, err := openBaiduUploadSource(localPath)
	if err != nil {
		return err
	}
	defer uploadFile.Close()
	fileSize := fileInfo.Size()

	if err := c.EnsureRemoteDirectories(path.Dir(remotePath)); err != nil {
		return err
	}

	blockList, err := fileBlockMD5s(uploadFile)
	if err != nil {
		return err
	}
	if _, err := uploadFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to reset upload file: %w", err)
	}

	log.Printf("[上传] file=%s size=%d bytes chunks=%d", filepath.Base(localPath), fileSize, len(blockList))

	accessToken, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	blockListJSON, err := json.Marshal(blockList)
	if err != nil {
		return fmt.Errorf("failed to marshal block list: %w", err)
	}

	// 1. 预上传
	precreateForm := url.Values{}
	precreateForm.Set("path", remotePath)
	precreateForm.Set("size", fmt.Sprintf("%d", fileSize))
	precreateForm.Set("isdir", "0")
	precreateForm.Set("autoinit", "1")
	precreateForm.Set("block_list", string(blockListJSON))
	precreateForm.Set("rtype", "3")

	reqURL := fmt.Sprintf("%s/file?method=precreate&access_token=%s", baiduXPAN, accessToken)
	resp, err := c.httpClient.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(precreateForm.Encode()))
	if err != nil {
		return fmt.Errorf("failed to precreate: %s", redactErrorText(err))
	}
	defer resp.Body.Close()

	var precreateResult map[string]interface{}
	if err := decodeExternalJSON(resp.Body, &precreateResult); err != nil {
		return fmt.Errorf("failed to decode precreate response: %w", err)
	}

	errno, ok := precreateResult["errno"].(float64)
	if !ok || errno != 0 {
		return fmt.Errorf("precreate failed: %s", redactSensitiveValue(precreateResult))
	}

	uploadID, ok := precreateResult["uploadid"].(string)
	if !ok {
		return fmt.Errorf("no uploadid in response")
	}
	log.Printf("[预上传] OK chunks=%d", len(blockList))

	// 2. 分片上传
	buf := make([]byte, blockSize)
	for i := range blockList {
		n, readErr := io.ReadFull(uploadFile, buf)
		if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
			return fmt.Errorf("failed to read chunk %d: %w", i, readErr)
		}
		if n == 0 {
			return fmt.Errorf("unexpected empty chunk %d", i)
		}
		chunk := buf[:n]
		chunkHash := md5HexString(chunk)
		if chunkHash != blockList[i] {
			return fmt.Errorf("local upload chunk %d changed while uploading", i)
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", fmt.Sprintf("chunk-%d", i))
		if err != nil {
			return fmt.Errorf("failed to create upload chunk body: %w", err)
		}
		if _, err := part.Write(chunk); err != nil {
			return fmt.Errorf("failed to write upload chunk: %w", err)
		}
		if err := writer.Close(); err != nil {
			return fmt.Errorf("failed to finalize upload chunk body: %w", err)
		}

		reqURL := fmt.Sprintf(
			"%s?method=upload&access_token=%s&type=tmpfile&path=%s&uploadid=%s&partseq=%d",
			baiduPCS,
			accessToken,
			url.PathEscape(remotePath),
			uploadID,
			i,
		)

		req, err := http.NewRequest("POST", reqURL, body)
		if err != nil {
			return fmt.Errorf("failed to create upload request: %s", redactErrorText(err))
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to upload chunk %d: %s", i, redactErrorText(err))
		}

		var chunkResult map[string]interface{}
		if err := decodeExternalJSON(resp.Body, &chunkResult); err != nil {
			_ = resp.Body.Close()
			return fmt.Errorf("failed to decode chunk response: %w", err)
		}
		_ = resp.Body.Close()

		if remoteMD5, ok := chunkResult["md5"].(string); ok && remoteMD5 != blockList[i] {
			return fmt.Errorf("baidu upload chunk %d md5 mismatch", i)
		}
	}
	extra := make([]byte, 1)
	n, readErr := uploadFile.Read(extra)
	if readErr != nil && readErr != io.EOF {
		return fmt.Errorf("failed to verify upload source size: %w", readErr)
	}
	if n > 0 {
		return fmt.Errorf("upload source changed while uploading")
	}
	currentInfo, err := uploadFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat upload source after chunks: %w", err)
	}
	if currentInfo.Size() != fileSize {
		return fmt.Errorf("upload source changed while uploading")
	}

	// 3. 合并创建文件
	createForm := url.Values{}
	createForm.Set("path", remotePath)
	createForm.Set("size", fmt.Sprintf("%d", fileSize))
	createForm.Set("isdir", "0")
	createForm.Set("uploadid", uploadID)
	createForm.Set("block_list", string(blockListJSON))
	createForm.Set("rtype", "3")

	reqURL = fmt.Sprintf("%s/file?method=create&access_token=%s", baiduXPAN, accessToken)
	resp, err = c.httpClient.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(createForm.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create file: %s", redactErrorText(err))
	}
	defer resp.Body.Close()

	var createResult map[string]interface{}
	if err := decodeExternalJSON(resp.Body, &createResult); err != nil {
		return fmt.Errorf("failed to decode create response: %w", err)
	}

	if fsID := fmt.Sprint(createResult["fs_id"]); fsID != "" && fsID != "<nil>" {
		log.Printf("[上传成功] file=%s", filepath.Base(localPath))
	} else {
		log.Printf("[上传失败] errno=%v, msg=%s", createResult["errno"], redactSensitiveValue(createResult["show_msg"]))
		return fmt.Errorf("create file failed: %s", redactSensitiveValue(createResult))
	}

	return nil
}

func normalizeBaiduAppRemoteFilePath(remotePath string) (string, error) {
	cleaned, err := normalizeBaiduAppRemotePath(remotePath)
	if err != nil {
		return "", err
	}
	root := path.Join("/apps", syncApp)
	if cleaned == root {
		return "", fmt.Errorf("remote path must be a file inside the Baidu app directory")
	}
	return cleaned, nil
}

func normalizeBaiduAppRemoteDirPath(remotePath string) (string, error) {
	return normalizeBaiduAppRemotePath(remotePath)
}

func normalizeBaiduAppRemoteDeletePath(remotePath string) (string, error) {
	cleaned, err := normalizeBaiduAppRemotePath(remotePath)
	if err != nil {
		return "", err
	}
	root := path.Join("/apps", syncApp)
	if cleaned == root {
		return "", fmt.Errorf("refusing to delete Baidu app root directory")
	}
	return cleaned, nil
}

func normalizeBaiduAppRemotePath(remotePath string) (string, error) {
	remotePath = strings.TrimSpace(filepath.ToSlash(remotePath))
	if remotePath == "" {
		return "", fmt.Errorf("remote path cannot be empty")
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(remotePath, "/"))
	root := path.Join("/apps", syncApp)
	if cleaned == "/" {
		return "", fmt.Errorf("remote path must be inside the Baidu app directory")
	}
	if cleaned != root && !strings.HasPrefix(cleaned, root+"/") {
		return "", fmt.Errorf("remote path escapes Baidu app directory")
	}
	return cleaned, nil
}

func openBaiduUploadSource(localPath string) (*os.File, os.FileInfo, error) {
	return openBaiduUploadSourceWithOpen(localPath, os.Open)
}

func openBaiduUploadSourceWithOpen(localPath string, openFile func(string) (*os.File, error)) (*os.File, os.FileInfo, error) {
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return nil, nil, fmt.Errorf("upload source path cannot be empty")
	}
	if openFile == nil {
		return nil, nil, fmt.Errorf("upload source open function cannot be nil")
	}

	linkInfo, err := os.Lstat(localPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat upload source: %w", err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("upload source is a symbolic link")
	}
	if !linkInfo.Mode().IsRegular() || linkInfo.Size() == 0 {
		return nil, nil, fmt.Errorf("upload source is empty or invalid")
	}

	file, err := openFile(localPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open upload source: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("failed to stat opened upload source: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		_ = file.Close()
		return nil, nil, fmt.Errorf("upload source is empty or invalid")
	}
	if !os.SameFile(linkInfo, info) {
		_ = file.Close()
		return nil, nil, fmt.Errorf("upload source changed while opening")
	}
	return file, info, nil
}

func fileBlockMD5s(file *os.File) ([]string, error) {
	blockList := []string{}
	buf := make([]byte, blockSize)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		if n == 0 {
			break
		}
		hasher := md5.New()
		if _, err := hasher.Write(buf[:n]); err != nil {
			return nil, fmt.Errorf("failed to hash block: %w", err)
		}
		blockList = append(blockList, hex.EncodeToString(hasher.Sum(nil)))
	}
	return blockList, nil
}

func md5HexString(content []byte) string {
	sum := md5.Sum(content)
	return hex.EncodeToString(sum[:])
}

func (c *BaiduPCSClient) EnsureRemoteDirectories(remoteDir string) error {
	normalizedRemoteDir, err := normalizeBaiduAppRemoteDirPath(remoteDir)
	if err != nil {
		return err
	}
	remoteDir = normalizedRemoteDir
	root := path.Join("/apps", syncApp)
	if remoteDir == root {
		return nil
	}

	relative := strings.Trim(strings.TrimPrefix(remoteDir, root), "/")
	current := root
	for _, part := range strings.Split(relative, "/") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		current = path.Join(current, part)
		if err := c.CreateDirectory(current); err != nil {
			return err
		}
	}
	return nil
}

func (c *BaiduPCSClient) CreateDirectory(remotePath string) error {
	normalizedRemotePath, err := normalizeBaiduAppRemoteDirPath(remotePath)
	if err != nil {
		return err
	}
	remotePath = normalizedRemotePath

	accessToken, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("path", remotePath)
	form.Set("isdir", "1")

	reqURL := fmt.Sprintf("%s/file?method=create&access_token=%s", baiduXPAN, accessToken)
	resp, err := c.httpClient.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create remote directory %s: %s", remotePath, redactErrorText(err))
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := decodeExternalJSON(resp.Body, &result); err != nil {
		return fmt.Errorf("failed to decode create directory response: %w", err)
	}
	if fsID := fmt.Sprint(result["fs_id"]); fsID != "" && fsID != "<nil>" {
		return nil
	}
	if errno, ok := result["errno"].(float64); ok && (errno == 0 || errno == -8) {
		return nil
	}
	return fmt.Errorf("create remote directory failed: %s", redactSensitiveValue(result))
}

// DownloadFile 从百度网盘下载文件
func (c *BaiduPCSClient) DownloadFile(remotePath, localPath string) error {
	normalizedRemotePath, err := normalizeBaiduAppRemoteFilePath(remotePath)
	if err != nil {
		return err
	}
	remotePath = normalizedRemotePath

	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}
	if err := ensurePlainDirectory(filepath.Dir(localPath)); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	accessToken, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	// 1. 列目录获取 fs_id
	dirPath := filepath.Dir(remotePath)
	listURL := fmt.Sprintf("%s/file?method=list&access_token=%s&dir=%s&limit=1000", baiduXPAN, accessToken, url.QueryEscape(dirPath))
	resp, err := c.httpClient.Get(listURL)
	if err != nil {
		return fmt.Errorf("failed to list files: %s", redactErrorText(err))
	}
	defer resp.Body.Close()

	var listResult map[string]interface{}
	if err := decodeBaiduJSON(resp.Body, &listResult); err != nil {
		return fmt.Errorf("failed to decode list response: %w", err)
	}

	fileList, ok := listResult["list"].([]interface{})
	if !ok {
		return fmt.Errorf("no file list in response")
	}

	var fsID string
	expectedSize := int64(-1)
	for _, item := range fileList {
		if fileMap, ok := item.(map[string]interface{}); ok {
			if path, ok := fileMap["path"].(string); ok && path == remotePath {
				fsID = baiduFSIDString(fileMap["fs_id"])
				expectedSize = baiduInt64(fileMap["size"])
				break
			}
		}
	}

	if fsID == "" {
		return fmt.Errorf("file not found: %s", remotePath)
	}
	if expectedSize > baiduDownloadMaxBytes {
		return fmt.Errorf("remote file exceeds download limit: %d bytes > %d bytes", expectedSize, baiduDownloadMaxBytes)
	}

	// 2. 获取下载链接
	fsidsJSON := []byte("[" + fsID + "]")
	metaURL := fmt.Sprintf("%s/multimedia?method=filemetas&access_token=%s&fsids=%s&dlink=1", baiduXPAN, accessToken, url.QueryEscape(string(fsidsJSON)))
	resp, err = c.httpClient.Get(metaURL)
	if err != nil {
		return fmt.Errorf("failed to get file meta: %s", redactErrorText(err))
	}
	defer resp.Body.Close()

	var metaResult map[string]interface{}
	if err := decodeBaiduJSON(resp.Body, &metaResult); err != nil {
		return fmt.Errorf("failed to decode meta response: %w", err)
	}

	metaList, ok := metaResult["list"].([]interface{})
	if !ok || len(metaList) == 0 {
		return fmt.Errorf("no meta list in response")
	}

	firstMeta, ok := metaList[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid meta format")
	}

	dlink, ok := firstMeta["dlink"].(string)
	if !ok {
		return fmt.Errorf("no download link in meta")
	}
	downloadURL, err := baiduDownloadURLWithAccessToken(dlink, accessToken)
	if err != nil {
		return err
	}

	// 3. 下载文件（必须带 User-Agent）
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %s", redactErrorText(err))
	}
	req.Header.Set("User-Agent", "pan.baidu.com")

	resp, err = c.downloadHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %s", redactErrorText(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: http status %d", resp.StatusCode)
	}

	var written int64
	copyLimit := baiduDownloadMaxBytes + 1
	if expectedSize >= 0 && expectedSize < baiduDownloadMaxBytes {
		copyLimit = expectedSize + 1
	}
	if err := writeFileAtomicWithWriter(localPath, 0600, func(tmp *os.File) error {
		var err error
		written, err = io.Copy(tmp, io.LimitReader(resp.Body, copyLimit))
		if err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		if written > baiduDownloadMaxBytes {
			return fmt.Errorf("download exceeds limit: %d bytes > %d bytes", written, baiduDownloadMaxBytes)
		}
		if expectedSize >= 0 && written != expectedSize {
			return fmt.Errorf("downloaded size mismatch: got %d bytes, expected %d bytes", written, expectedSize)
		}
		return nil
	}, nil); err != nil {
		return fmt.Errorf("failed to replace output file: %w", err)
	}

	log.Printf("[下载完成] file=%s size=%d bytes", filepath.Base(localPath), written)
	return nil
}

func (c *BaiduPCSClient) downloadHTTPClient() *http.Client {
	base := c.httpClient
	if base == nil {
		base = defaultBaiduHTTPClient()
	}
	cloned := *base
	cloned.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= baiduDownloadMaxRedirects {
			return fmt.Errorf("too many baidu download redirects")
		}
		validated, err := validateBaiduDownloadURL(req.URL.String())
		if err != nil {
			return err
		}
		parsed, err := url.Parse(validated)
		if err != nil {
			return err
		}
		req.URL = parsed
		req.Header.Set("User-Agent", "pan.baidu.com")
		return nil
	}
	return &cloned
}

func baiduDownloadURLWithAccessToken(dlink, accessToken string) (string, error) {
	validated, err := validateBaiduDownloadURL(dlink)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(validated)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("access_token", accessToken)
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validateBaiduDownloadURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("baidu download link is empty")
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed == nil {
		return "", fmt.Errorf("baidu download link must be a valid http/https url")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("baidu download link must be a valid http/https url")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("baidu download link must not include userinfo")
	}
	host := strings.TrimSpace(strings.TrimSuffix(strings.ToLower(parsed.Hostname()), "."))
	if host == "" {
		return "", fmt.Errorf("baidu download link must include a host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedDownloadIP(ip) {
			return "", fmt.Errorf("baidu download link host is not allowed: %s", host)
		}
		return "", fmt.Errorf("baidu download link host is not a Baidu host: %s", host)
	}
	if !isAllowedBaiduDownloadHost(host) {
		return "", fmt.Errorf("baidu download link host is not a Baidu host: %s", host)
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func isAllowedBaiduDownloadHost(host string) bool {
	for _, suffix := range []string{"baidu.com", "baidupcs.com"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

func decodeBaiduJSON(reader io.Reader, target interface{}) error {
	return decodeExternalJSONUseNumber(reader, target)
}

func baiduFSIDString(value interface{}) string {
	switch typed := value.(type) {
	case json.Number:
		return strings.TrimSpace(typed.String())
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func baiduInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return parsed
		}
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
			return parsed
		}
	}
	return -1
}

// ListFiles 列出指定目录的文件
func (c *BaiduPCSClient) ListFiles(dir string) ([]FileInfo, error) {
	normalizedDir, err := normalizeBaiduAppRemoteDirPath(dir)
	if err != nil {
		return nil, err
	}
	dir = normalizedDir

	accessToken, err := c.GetAccessToken()
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/file?method=list&access_token=%s&dir=%s&limit=1000", baiduXPAN, accessToken, url.QueryEscape(dir))
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %s", redactErrorText(err))
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := decodeExternalJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode list response: %w", err)
	}
	if errno, ok := result["errno"].(float64); ok && errno != 0 {
		return nil, fmt.Errorf("list files failed: %s", redactSensitiveValue(result))
	}

	fileList, ok := result["list"].([]interface{})
	if !ok {
		return []FileInfo{}, nil
	}

	files := make([]FileInfo, 0, len(fileList))
	for _, item := range fileList {
		if fileMap, ok := item.(map[string]interface{}); ok {
			file := FileInfo{
				IsDir: false,
			}

			if remotePath, ok := fileMap["path"].(string); ok {
				file.Path = strings.TrimSpace(filepath.ToSlash(remotePath))
			}
			if size := baiduInt64(fileMap["size"]); size >= 0 {
				file.Size = size
			}
			file.IsDir = baiduInt64(fileMap["isdir"]) != 0
			normalizedPath, err := normalizeBaiduListEntryPath(file.Path, file.IsDir)
			if err != nil || normalizedPath != file.Path {
				continue
			}
			file.Path = normalizedPath
			if md5, ok := fileMap["md5"].(string); ok {
				file.MD5 = md5
			}
			if modified := baiduInt64(fileMap["server_mtime"]); modified > 0 {
				file.Modified = time.Unix(modified, 0)
			}

			files = append(files, file)
		}
	}

	return files, nil
}

func normalizeBaiduListEntryPath(remotePath string, isDir bool) (string, error) {
	if isDir {
		return normalizeBaiduAppRemoteDirPath(remotePath)
	}
	return normalizeBaiduAppRemoteFilePath(remotePath)
}

func (c *BaiduPCSClient) ListFilesRecursive(dir string) ([]FileInfo, error) {
	files, err := c.ListFiles(dir)
	if err != nil {
		return nil, err
	}

	result := make([]FileInfo, 0, len(files))
	for _, file := range files {
		if file.IsDir {
			children, err := c.ListFilesRecursive(file.Path)
			if err != nil {
				return nil, err
			}
			result = append(result, children...)
			continue
		}
		result = append(result, file)
	}
	return result, nil
}

// GetFileInfo 获取文件信息
func (c *BaiduPCSClient) GetFileInfo(path string) (*FileInfo, error) {
	normalizedPath, err := normalizeBaiduAppRemoteFilePath(path)
	if err != nil {
		return nil, err
	}
	path = normalizedPath

	files, err := c.ListFiles(filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.Path == path {
			return &file, nil
		}
	}

	return nil, fmt.Errorf("file not found: %s", path)
}

// DeleteFile 删除文件
func (c *BaiduPCSClient) DeleteFile(path string) error {
	normalizedPath, err := normalizeBaiduAppRemoteDeletePath(path)
	if err != nil {
		return err
	}
	path = normalizedPath

	accessToken, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	pathsJSON, err := json.Marshal([]string{path})
	if err != nil {
		return fmt.Errorf("failed to marshal paths: %w", err)
	}

	form := url.Values{}
	form.Set("filelist", string(pathsJSON))
	reqURL := fmt.Sprintf("%s/file?method=filemanager&opera=delete&access_token=%s", baiduXPAN, accessToken)
	req, err := http.NewRequest("POST", reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create delete request: %s", redactErrorText(err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete file: %s", redactErrorText(err))
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := decodeExternalJSON(resp.Body, &result); err != nil {
		return fmt.Errorf("failed to decode delete response: %w", err)
	}

	if errno, ok := result["errno"].(float64); !ok || errno != 0 {
		return fmt.Errorf("delete failed: %s", redactSensitiveValue(result))
	}

	return nil
}
