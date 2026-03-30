package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/qiaoyo/DiveEnd/internal/config"
)

// BaiduClient handles Baidu Cloud (Pan) sync
type BaiduClient struct {
	ak         string
	sk         string
	accessToken string
	httpClient *http.Client
}

// NewBaiduClient creates a new Baidu Cloud client
func NewBaiduClient(cfg config.BaiduConfig) *BaiduClient {
	return &BaiduClient{
		ak:         cfg.AK,
		sk:         cfg.SK,
		httpClient: &http.Client{Timeout: 10 * time.Minute},
	}
}

// StartBackgroundSync starts background sync
func StartBackgroundSync(cfg config.BaiduConfig, dataPath string) {
	client := NewBaiduClient(cfg)
	log.Println("Starting Baidu Cloud background sync...")
	// Sync on startup
	if err := client.SyncDirectory(dataPath, cfg.RemoteDir); err != nil {
		log.Printf("Initial sync failed: %v", err)
	} else {
		log.Println("Initial sync completed successfully")
	}
}

// SyncDirectory sync local directory to Baidu Cloud
func (b *BaiduClient) SyncDirectory(localDir, remoteDir string) error {
	// First get access token
	if err := b.getAccessToken(); err != nil {
		return fmt.Errorf("failed to get access token: %v", err)
	}

	// Walk directory and upload all files
	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Create directory on Baidu
			relPath, _ := filepath.Rel(localDir, path)
			remotePath := filepath.Join(remoteDir, relPath)
			if err := b.createDir(remotePath); err != nil {
				log.Printf("Warning: failed to create directory %s: %v", remotePath, err)
			}
			return nil
		}

		// Upload file
		relPath, _ := filepath.Rel(localDir, path)
		remotePath := filepath.Join(remoteDir, relPath)
		if err := b.uploadFile(path, remotePath); err != nil {
			log.Printf("Warning: failed to upload %s: %v", path, err)
		}
		return nil
	})
}

func (b *BaiduClient) getAccessToken() error {
	reqURL := fmt.Sprintf("https://openapi.baidu.com/oauth/2.0/token?grant_type=client_credentials&client_id=%s&client_secret=%s", b.ak, b.sk)

	resp, err := b.httpClient.Get(reqURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error        string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Error != "" {
		return fmt.Errorf("%s: %s", result.Error, result.ErrorDescription)
	}

	b.accessToken = result.AccessToken
	return nil
}

func (b *BaiduClient) createDir(remotePath string) error {
	url := "https://pan.baidu.com/rest/2.0/xpan/file?method=mkdir"

	reqBody := map[string]interface{}{
		"path": remotePath,
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.accessToken)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response
	var result struct {
		ErrCode int    `json:"errno"`
		ErrMsg  string `json:"errmsg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	// ErrCode 3 means directory already exists, which is OK
	if result.ErrCode != 0 && result.ErrCode != 3 {
		return fmt.Errorf("mkdir failed: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

func (b *BaiduClient) uploadFile(localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// First get upload URL
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	// Baidu upload has two steps: pre-create and then upload
	remoteDir := filepath.Dir(remotePath)
	if err := b.createDir(remoteDir); err != nil {
		log.Printf("Warning: creating remote dir %s: %v", remoteDir, err)
	}

	// Pre-create
	uploadURL, err := b.getUploadURL(remotePath, info.Size())
	if err != nil {
		return err
	}

	// Upload content
	req, err := http.NewRequest("PUT", uploadURL, file)
	if err != nil {
		return err
	}
	req.ContentLength = info.Size()

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (b *BaiduClient) getUploadURL(remotePath string, size int64) (string, error) {
	reqURL := fmt.Sprintf("https://pan.baidu.com/rest/2.0/xpan/file?method=precreate&access_token=%s", b.accessToken)

	reqBody := map[string]interface{}{
		"path":   remotePath,
		"size":   size,
		"isdir":  0,
		"autoinit": 1,
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errno"`
		UploadID string `json:"uploadid"`
		BlockList []int `json:"block_list"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("precreate failed: %d", result.ErrCode)
	}

	// For small files (< 1GB), single block upload is fine
	return fmt.Sprintf("https://d.pcs.baidu.com/rest/2.0/pcs/file?method=upload&access_token=%s&path=%s&uploadid=%s&partseq=0",
		b.accessToken, url.QueryEscape(remotePath), result.UploadID), nil
}
