package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	baiduXPAN    = "https://pan.baidu.com/rest/2.0/xpan"
	baiduPCS     = "https://d.pcs.baidu.com/rest/2.0/pcs/superfile2"
	baiduAuth    = "https://openapi.baidu.com/oauth/2.0/token"
	baiduQuota   = "https://pan.baidu.com/api/quota"
	blockSize    = 4 * 1024 * 1024 // 4MB
)

// BaiduPCSClient 百度网盘客户端
type BaiduPCSClient struct {
	token      *BaiduToken
	httpClient *http.Client
}

// NewBaiduPCSClient 创建百度网盘客户端
func NewBaiduPCSClient(token *BaiduToken) *BaiduPCSClient {
	return &BaiduPCSClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
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
	data, err := os.ReadFile(path)
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

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// RefreshToken 刷新 access_token
func RefreshToken(token *BaiduToken) (*BaiduToken, error) {
	params := url.Values{}
	params.Set("grant_type", "refresh_token")
	params.Set("refresh_token", token.RefreshToken)
	params.Set("client_id", token.ClientID)
	params.Set("client_secret", token.ClientSecret)

	reqURL := fmt.Sprintf("%s?%s", baiduAuth, params.Encode())
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	var result TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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

	return updatedToken, nil
}

// GetAccessToken 获取有效的 access_token，过期自动刷新
func (c *BaiduPCSClient) GetAccessToken() (string, error) {
	if c.token == nil {
		return "", fmt.Errorf("token is nil")
	}

	// 检查 token 是否有效
	params := url.Values{}
	params.Set("access_token", c.token.AccessToken)
	params.Set("checkexpire", "1")

	reqURL := fmt.Sprintf("%s?%s", baiduQuota, params.Encode())
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return "", fmt.Errorf("failed to check token: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode quota response: %w", err)
	}

	// errno == 0 且 expire 为空表示 token 有效
	if errno, ok := result["errno"].(float64); ok && errno == 0 {
		if expire, ok := result["expire"]; ok && expire == nil {
			return c.token.AccessToken, nil
		}
	}

	// token 过期，刷新
	fmt.Println("[Token] 已过期，正在刷新...")
	updatedToken, err := RefreshToken(c.token)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	c.token = updatedToken
	fmt.Println("[Token] 刷新成功")
	return updatedToken.AccessToken, nil
}

// UploadFile 上传文件到百度网盘
func (c *BaiduPCSClient) UploadFile(localPath, remoteName string) error {
	if remoteName == "" {
		remoteName = filepath.Base(localPath)
	}
	remotePath := fmt.Sprintf("/apps/diveendend/%s", remoteName)

	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// 读取所有分片并计算 MD5
	chunks := [][]byte{}
	blockList := []string{}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	buf := make([]byte, blockSize)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if n == 0 {
			break
		}
		chunk := buf[:n]
		chunks = append(chunks, chunk)

		hasher := md5.New()
		hasher.Write(chunk)
		hash := hasher.Sum(nil)
		blockList = append(blockList, hex.EncodeToString(hash))
	}

	fmt.Printf("[上传] %s (%d bytes) -> %s, 分片数: %d\n", localPath, fileSize, remotePath, len(chunks))

	accessToken, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	blockListJSON, err := json.Marshal(blockList)
	if err != nil {
		return fmt.Errorf("failed to marshal block list: %w", err)
	}

	// 1. 预上传
	precreateData := map[string]interface{}{
		"path":       remotePath,
		"size":       fileSize,
		"isdir":      0,
		"autoinit":    1,
		"block_list":  string(blockListJSON),
		"rtype":       3,
	}

	precreateDataJSON, err := json.Marshal(precreateData)
	if err != nil {
		return fmt.Errorf("failed to marshal precreate data: %w", err)
	}

	reqURL := fmt.Sprintf("%s/file?method=precreate&access_token=%s", baiduXPAN, accessToken)
	resp, err := c.httpClient.Post(reqURL, "application/json", bytes.NewReader(precreateDataJSON))
	if err != nil {
		return fmt.Errorf("failed to precreate: %w", err)
	}
	defer resp.Body.Close()

	var precreateResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&precreateResult); err != nil {
		return fmt.Errorf("failed to decode precreate response: %w", err)
	}

	errno, ok := precreateResult["errno"].(float64)
	if !ok || errno != 0 {
		return fmt.Errorf("precreate failed: %v", precreateResult)
	}

	uploadID, ok := precreateResult["uploadid"].(string)
	if !ok {
		return fmt.Errorf("no uploadid in response")
	}
	fmt.Printf("[预上传] OK, uploadid=%s\n", uploadID)

	// 2. 分片上传
	for i, chunk := range chunks {
		reqURL := fmt.Sprintf(
			"%s?method=upload&access_token=%s&type=tmpfile&path=%s&uploadid=%s&partseq=%d",
			baiduPCS,
			accessToken,
			url.PathEscape(remotePath),
			uploadID,
			i,
		)

		req, err := http.NewRequest("POST", reqURL, bytes.NewReader(chunk))
		if err != nil {
			return fmt.Errorf("failed to create upload request: %w", err)
		}
		req.Header.Set("Content-Type", "multipart/form-data")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to upload chunk %d: %w", i, err)
		}
		defer resp.Body.Close()

		var chunkResult map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&chunkResult); err != nil {
			return fmt.Errorf("failed to decode chunk response: %w", err)
		}

		if md5, ok := chunkResult["md5"].(string); ok {
			match := md5 == blockList[i]
			fmt.Printf("[分片 %d] md5匹配=%v\n", i, match)
			if !match {
				fmt.Printf("  本地: %s, 服务端: %s\n", blockList[i], md5)
			}
		}
	}

	// 3. 合并创建文件
	createData := map[string]interface{}{
		"path":       remotePath,
		"size":       fileSize,
		"isdir":      0,
		"uploadid":   uploadID,
		"block_list":  string(blockListJSON),
		"rtype":       3,
	}

	createDataJSON, err := json.Marshal(createData)
	if err != nil {
		return fmt.Errorf("failed to marshal create data: %w", err)
	}

	reqURL = fmt.Sprintf("%s/file?method=create&access_token=%s", baiduXPAN, accessToken)
	resp, err = c.httpClient.Post(reqURL, "application/json", bytes.NewReader(createDataJSON))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer resp.Body.Close()

	var createResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&createResult); errerr != nil {
		return fmt.Errorf("failed to decode create response: %w", err)
	}

	if fsID, ok := createResult["fs_id"].(string); ok {
		fmt.Printf("[上传成功] path=%s, fs_id=%s\n", createResult["path"], fsID)
	} else {
		fmt.Printf("[上传失败] errno=%v, msg=%v\n", createResult["errno"], createResult["show_msg"])
	}

	return nil
}

// DownloadFile 从百度网盘下载文件
func (c *BaiduPCSClient) DownloadFile(remotePath, localPath string) error {
	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}

	accessToken, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	// 1. 列目录获取 fs_id
	dirPath := filepath.Dir(remotePath)
	listURL := fmt.Sprintf("%s/file?method=list&access_token=%s&dir=%s&limit=1000", baiduXPAN, accessToken, dirPath)
	resp, err := c.httpClient.Get(listURL)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}
	defer resp.Body.Close()

	var listResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&listResult); errerr != nil {
		return fmt.Errorf("failed to decode list response: %w", err)
	}

	fileList, ok := listResult["list"].([]interface{})
	if !ok {
		return fmt.Errorf("no file list in response")
	}

	var fsID string
	for _, item := range fileList {
		if fileMap, ok := item.(map[string]interface{}); ok {
			if path, ok := fileMap["path"].(string); ok && path == remotePath {
				if id, ok := fileMap["fs_id"].(string); ok {
					fsID = id
				}
				break
			}
		}
	}

	if fsID == "" {
		return fmt.Errorf("file not found: %s", remotePath)
	}

	// 2. 获取下载链接
	fsidsJSON, err := json.Marshal([]string{fsID})
	if err != nil {
		return fmt.Errorf("failed to marshal fsids: %w", err)
	}

	metaURL := fmt.Sprintf("%s/multimedia?method=filemetas&access_token=%s&fsids=%s&dlink=1", baiduXPAN, accessToken, fsidsJSON)
	resp, err = c.httpClient.Get(metaURL)
	if err != nil {
		return fmt.Errorf("failed to get file meta: %w", err)
	}
	defer resp.Body.Close()

	var metaResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metaResult); errerr != nil {
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

	// 3. 下载文件（必须带 User-Agent）
	downloadURL := fmt.Sprintf("%s?access_token=%s", dlink, accessToken)
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "pan.baidu.com")

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("rfailed to write file: %w", err)
	}

	fmt.Printf("[下载完成] -> %s (%d bytes)\n", localPath, written)
	return nil
}

// ListFiles 列出指定目录的文件
func (c *BaiduPCSClient) ListFiles(dir string) ([]FileInfo, error) {
	accessToken, err := c.GetAccessToken()
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/file?method=list&access_token=%s&dir=%s&limit=1000", baiduXPAN, accessToken, dir)
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); errerr != nil {
		return nil, fmt.Errorf("failed to decode list response: %w", err)
	}

	fileList, ok := result["list"].([]interface{})
	if !ok {
		return []FileInfo{}, nil
	}

	files := make([]FileInfo, 0, len(fileList))
	for _, item := range fileList {
		if fileverMap, ok := item.(map[string]interface{}); ok {
			file := FileInfo{
				IsDir: false,
			}

			if path, ok := fileMap["path"].(string); ok {
				file.Path = path
			}
			if size, ok := fileMap["size"].(float64); ok {
				file.Size = int64(size)
			}
			if isdir, ok := fileMap["isdir"].(float64); ok {
				file.IsDir = isdir != 0
			}
			if md5, ok := fileMap["md5"].(string); ok {
				file.MD5 = md5
			}

			files = append(files, file)
		}
	}

	return files, nil
}

// GetFileInfo 获取文件信息
func (c *BaiduPCSClient) GetFileInfo(path string) (*FileInfo, error) {
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
	accessToken, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	pathsJSON, err := json.Marshal([]string{path})
	if err != nil {
		return fmt.Errorf("failed to marshal paths: %w", err)
	}

	reqURL := fmt.Sprintf("%s/file?method=delete&access_token=%s", baiduXPAN, accessToken)
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(pathsJSON))
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); errerr != nil {
		return fmt.Errorf("failed to decode delete response: %w", err)
	}

	if errno, ok := result["errno"].(float64); !ok || errno != 0 {
		return fmt.Errorf("delete failed: %v", result)
	}

	return nil
}
