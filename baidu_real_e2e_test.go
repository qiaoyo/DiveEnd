package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealBaiduSyncE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DIVEEND_REAL_BAIDU_E2E")) != "1" {
		t.Skip("set DIVEEND_REAL_BAIDU_E2E=1 to run against the real Baidu Cloud API")
	}

	tokenPath := strings.TrimSpace(os.Getenv("DIVEEND_BAIDU_TOKEN_FILE"))
	if tokenPath == "" {
		tokenPath = defaultBaiduTokenPath()
	}
	token, err := LoadBaiduToken(tokenPath)
	if err != nil {
		t.Fatalf("LoadBaiduToken(%q) error = %v", tokenPath, err)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		t.Fatal("baidu token file does not contain access_token")
	}

	oldRoot := syncDataRootName
	syncDataRootName = "diveend-e2e-" + time.Now().UTC().Format("20060102-150405")
	remoteRoot := syncRemoteRootPath()
	t.Cleanup(func() {
		syncDataRootName = oldRoot
	})

	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = token.AccessToken
	config.BaiduCloud.RefreshToken = token.RefreshToken
	config.BaiduCloud.ClientID = token.ClientID
	config.BaiduCloud.ClientSecret = token.ClientSecret

	db, err := NewDB(config.DataPath)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	pdfPath := filepath.Join(config.DataPath, "papers", "e2e-paper.pdf")
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\n%DiveEnd real Baidu E2E smoke file\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	folders, err := db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected default folder")
	}

	paper := Paper{
		ID:             "baidu-e2e-paper",
		SourcePaperID:  "baidu-e2e-paper",
		Title:          "DiveEnd Baidu E2E Smoke Paper",
		Authors:        "DiveEnd",
		Year:           2026,
		PDFPath:        pdfPath,
		DownloadStatus: "downloaded",
		FolderID:       folders[0].ID,
		Tags:           []string{"e2e"},
	}
	if err := db.UpsertPaper(&paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	manager := NewSyncManager(db, config)
	if manager.baiduClient == nil {
		t.Fatal("expected Baidu client")
	}
	// Baidu refresh tokens can be rotated by a successful refresh. Persist
	// updates to the configured token file so a real E2E run does not consume a
	// refresh token and then discard its replacement.
	manager.baiduClient.tokenFilePath = tokenPath
	if _, err := manager.baiduClient.GetAccessToken(); err != nil {
		t.Fatalf("Baidu token preflight failed before real E2E sync: %v", err)
	}
	t.Cleanup(func() {
		cleanupRealBaiduE2ERoot(t, manager.baiduClient, remoteRoot)
	})

	if err := manager.SyncToCloud(); err != nil {
		t.Fatalf("SyncToCloud() real Baidu error = %v", err)
	}
	progress := manager.GetSyncProgress()
	if progress.Status != "complete" {
		t.Fatalf("expected complete sync progress, got %+v", progress)
	}

	files, err := manager.baiduClient.ListFilesRecursive(remoteRoot)
	if err != nil {
		t.Fatalf("ListFilesRecursive(%q) error = %v", remoteRoot, err)
	}
	remotePaths := map[string]bool{}
	for _, file := range files {
		remotePaths[file.Path] = true
	}

	expectedPDFPath := syncRemotePathForKey(syncKeyForPaper(paper))
	for _, expected := range []string{
		syncRemotePathForKey(syncDatabaseKey),
		syncRemotePathForKey(syncManifestFileName),
		expectedPDFPath,
	} {
		if !remotePaths[expected] {
			t.Fatalf("expected remote path %q in real Baidu listing, got %#v", expected, remotePaths)
		}
	}

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	if err := manager.baiduClient.DownloadFile(syncRemotePathForKey(syncManifestFileName), manifestPath); err != nil {
		t.Fatalf("DownloadFile(manifest) error = %v", err)
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest) error = %v", err)
	}
	var manifest syncManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest) error = %v", err)
	}
	if manifest.Version != 1 || manifest.App != "DiveEnd" {
		t.Fatalf("unexpected manifest identity: %+v", manifest)
	}
	manifestKeys := map[string]bool{}
	for _, file := range manifest.Files {
		manifestKeys[file.Key] = true
	}
	for _, expected := range []string{syncDatabaseKey, syncKeyForPaper(paper)} {
		if !manifestKeys[expected] {
			t.Fatalf("expected manifest key %q, got %#v", expected, manifestKeys)
		}
	}
}

func cleanupRealBaiduE2ERoot(t *testing.T, client *BaiduPCSClient, remoteRoot string) {
	t.Helper()
	files, err := client.ListFilesRecursive(remoteRoot)
	if err != nil {
		t.Logf("cleanup list remote test root %q failed: %v", remoteRoot, err)
		return
	}
	for _, file := range files {
		if file.IsDir {
			continue
		}
		if err := client.DeleteFile(file.Path); err != nil {
			t.Logf("cleanup remote test file %q failed: %v", file.Path, err)
		}
	}
	if err := client.DeleteFile(remoteRoot); err != nil {
		t.Logf("cleanup remote test root %q after deleting files failed: %v", remoteRoot, err)
	}
}
