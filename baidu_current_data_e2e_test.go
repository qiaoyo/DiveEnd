package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealBaiduSyncCurrentDataE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DIVEEND_REAL_BAIDU_E2E")) != "1" {
		t.Skip("set DIVEEND_REAL_BAIDU_E2E=1 to sync the configured local DiveEnd library to the real Baidu Cloud API")
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}
	if !config.BaiduCloud.Enabled {
		t.Fatal("baidu cloud sync is disabled in loaded config")
	}
	if strings.TrimSpace(config.DataPath) == "" {
		t.Fatal("configured dataPath is empty")
	}

	token, err := LoadBaiduToken(defaultBaiduTokenPath())
	if err != nil {
		t.Fatalf("LoadBaiduToken(%q) error = %v", defaultBaiduTokenPath(), err)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		t.Fatal("canonical baiduyun_token.json does not contain access_token")
	}

	db, err := NewDB(config.DataPath)
	if err != nil {
		t.Fatalf("NewDB(%q) error = %v", config.DataPath, err)
	}
	t.Cleanup(func() { _ = db.Close() })

	manager := NewSyncManager(db, config)
	if manager.baiduClient == nil {
		t.Fatal("expected Baidu client from loaded config/token file")
	}
	manager.baiduClient.tokenFilePath = defaultBaiduTokenPath()
	if _, err := manager.baiduClient.GetAccessToken(); err != nil {
		t.Fatalf("Baidu token preflight failed before current-data sync: %v", err)
	}

	localFiles, cleanup, err := manager.prepareSyncSnapshot()
	if err != nil {
		t.Fatalf("prepareSyncSnapshot() error = %v", err)
	}
	defer cleanup()
	if len(localFiles) == 0 {
		t.Fatal("expected at least the SQLite database snapshot to be synced")
	}
	localKeys := map[string]bool{}
	var localBytes int64
	for _, file := range localFiles {
		localKeys[file.Key] = true
		localBytes += file.Size
	}
	if !localKeys[syncDatabaseKey] {
		t.Fatalf("expected local sync snapshot to include %q, got %#v", syncDatabaseKey, localKeys)
	}
	t.Logf("current-data sync input: dataPath=%s files=%d bytes=%d", config.DataPath, len(localFiles), localBytes)
	if strings.TrimSpace(os.Getenv("DIVEEND_CURRENT_DATA_SYNC_DRY_RUN")) == "1" {
		return
	}

	if err := manager.SyncToCloud(); err != nil {
		t.Fatalf("SyncToCloud() current-data real Baidu error = %v", err)
	}
	progress := manager.GetSyncProgress()
	if progress.Status != "complete" {
		t.Fatalf("expected complete sync progress, got %+v", progress)
	}

	remoteRoot := syncRemoteRootPath()
	remoteFiles, err := manager.baiduClient.ListFilesRecursive(remoteRoot)
	if err != nil {
		t.Fatalf("ListFilesRecursive(%q) error = %v", remoteRoot, err)
	}
	remotePaths := map[string]bool{}
	for _, file := range remoteFiles {
		remotePaths[file.Path] = true
	}
	for _, expected := range []string{
		syncRemotePathForKey(syncDatabaseKey),
		syncRemotePathForKey(syncManifestFileName),
	} {
		if !remotePaths[expected] {
			t.Fatalf("expected remote path %q after current-data sync, got %#v", expected, remotePaths)
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
	for key := range localKeys {
		if !manifestKeys[key] {
			t.Fatalf("expected manifest to include local key %q", key)
		}
	}
	t.Logf("current-data sync complete: remoteRoot=%s manifestFiles=%d remoteObjects=%d", remoteRoot, len(manifest.Files), len(remoteFiles))
}
