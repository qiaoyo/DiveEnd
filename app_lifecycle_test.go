package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitForSyncCompletionReturnsAfterComplete(t *testing.T) {
	manager := &SyncManager{progress: &SyncProgress{Status: "uploading", Message: "上传中"}}
	result := make(chan error, 1)

	go func() {
		result <- waitForSyncCompletion(context.Background(), manager)
	}()

	time.Sleep(25 * time.Millisecond)
	manager.updateProgress(func(progress *SyncProgress) {
		progress.Status = "complete"
		progress.Message = "同步完成"
	})

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("waitForSyncCompletion() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waitForSyncCompletion() did not return after terminal progress")
	}
}

func TestWaitForSyncCompletionReturnsProgressError(t *testing.T) {
	manager := &SyncManager{progress: &SyncProgress{
		Status:  "error",
		Message: "baidu token preflight failed",
	}}

	err := waitForSyncCompletion(context.Background(), manager)
	if err == nil || !strings.Contains(err.Error(), "baidu token preflight failed") {
		t.Fatalf("expected progress error message, got %v", err)
	}
}

func TestWaitForSyncCompletionHonorsContextCancellation(t *testing.T) {
	manager := &SyncManager{progress: &SyncProgress{Status: "uploading"}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForSyncCompletion(ctx, manager)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestShouldPromptBeforeCloseRequiresConfiguredExitSyncAndDatabase(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"

	app.config = config
	if app.shouldPromptBeforeClose() {
		t.Fatal("shouldPromptBeforeClose() should be false before database initialization")
	}

	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		if app.db != nil {
			_ = app.db.Close()
		}
	})

	if app.shouldPromptBeforeClose() {
		t.Fatal("shouldPromptBeforeClose() should be false when exit sync is disabled")
	}

	app.config.Sync.AutoSync = true
	if app.shouldPromptBeforeClose() {
		t.Fatal("shouldPromptBeforeClose() should stay false when only periodic auto sync is enabled")
	}

	app.config.Sync.AutoSync = false
	app.config.Sync.SyncBeforeExit = true
	if !app.shouldPromptBeforeClose() {
		t.Fatal("shouldPromptBeforeClose() should be true when sync is configured and exit sync is enabled")
	}

	app.config.BaiduCloud.Token = ""
	if app.shouldPromptBeforeClose() {
		t.Fatal("shouldPromptBeforeClose() should be false when Baidu token is missing")
	}
}

func TestShutdownSyncSkipsWhenNoLocalUploadsArePending(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	config.Sync.SyncBeforeExit = true

	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}

	dbInfo, err := os.Stat(filepath.Join(config.DataPath, "diveend.db"))
	if err != nil {
		t.Fatalf("Stat database error = %v", err)
	}
	uploadedAt := dbInfo.ModTime().Add(time.Hour)
	if err := app.db.SaveSyncRecord(&SyncRecord{
		Type:        "upload",
		FileName:    syncDatabaseKey,
		RemotePath:  syncRemotePathForKey(syncDatabaseKey),
		Status:      "success",
		CreatedAt:   uploadedAt,
		CompletedAt: uploadedAt,
	}); err != nil {
		t.Fatalf("SaveSyncRecord() error = %v", err)
	}

	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected configured sync manager")
	}

	var baiduRequests atomic.Int32
	manager.baiduClient.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			baiduRequests.Add(1)
			return nil, errors.New("unexpected baidu request during shutdown")
		}),
	}

	if app.shouldRunShutdownSync() {
		t.Fatal("shouldRunShutdownSync() should be false when all local sync candidates are already uploaded")
	}

	app.shutdown(context.Background())

	if got := baiduRequests.Load(); got != 0 {
		t.Fatalf("expected shutdown to skip Baidu requests when no uploads are pending, got %d", got)
	}
}

func TestSaveSyncSettingsStartsAndStopsPeriodicSyncLoop(t *testing.T) {
	useTestConfigPath(t)

	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	app.periodicSyncInterval = time.Hour

	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		app.stopPeriodicSyncLoop()
		if app.db != nil {
			_ = app.db.Close()
		}
	})

	if app.periodicSyncLoopRunning() {
		t.Fatal("periodic sync loop should not run before auto sync is enabled")
	}

	if _, err := app.SaveSyncSettings(SyncSettings{
		AutoSync:           true,
		SyncOnStartup:      false,
		SyncInterval:       5,
		ConflictResolution: "timestamp",
	}); err != nil {
		t.Fatalf("SaveSyncSettings(enable) error = %v", err)
	}
	if !app.periodicSyncLoopRunning() {
		t.Fatal("periodic sync loop should start after enabling auto sync")
	}

	if _, err := app.SaveSyncSettings(SyncSettings{
		AutoSync:           false,
		SyncOnStartup:      false,
		SyncInterval:       5,
		ConflictResolution: "timestamp",
	}); err != nil {
		t.Fatalf("SaveSyncSettings(disable) error = %v", err)
	}
	if app.periodicSyncLoopRunning() {
		t.Fatal("periodic sync loop should stop after disabling auto sync")
	}
}

func TestSaveSyncSettingsDoesNotWaitForStoppingPeriodicSyncWorker(t *testing.T) {
	useTestConfigPath(t)

	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		app.stopPeriodicSyncLoop()
		if app.db != nil {
			_ = app.db.Close()
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	releaseWorker := make(chan struct{})
	auditDone := make(chan struct{})

	app.periodicSyncMu.Lock()
	app.periodicSyncCancel = cancel
	app.periodicSyncWG.Add(1)
	app.periodicSyncMu.Unlock()
	go func() {
		defer app.periodicSyncWG.Done()
		defer close(auditDone)
		<-ctx.Done()
		<-releaseWorker
	}()

	saveDone := make(chan error, 1)
	go func() {
		_, err := app.SaveSyncSettings(SyncSettings{
			AutoSync:           false,
			SyncOnStartup:      false,
			SyncInterval:       5,
			ConflictResolution: "timestamp",
		})
		saveDone <- err
	}()

	select {
	case err := <-saveDone:
		if err != nil {
			close(releaseWorker)
			t.Fatalf("SaveSyncSettings() error = %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		close(releaseWorker)
		<-auditDone
		t.Fatal("SaveSyncSettings() waited for the stopping periodic sync worker")
	}

	if app.periodicSyncLoopRunning() {
		close(releaseWorker)
		t.Fatal("periodic sync schedule should be stopped after saving disabled auto sync")
	}
	close(releaseWorker)
	<-auditDone
}

func TestPeriodicSyncStopAndReconfigureAreSerialized(t *testing.T) {
	useTestConfigPath(t)

	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	config.Sync.AutoSync = true
	app.periodicSyncInterval = time.Hour

	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		app.stopPeriodicSyncLoop()
		if app.db != nil {
			_ = app.db.Close()
		}
	})

	if !app.periodicSyncLoopRunning() {
		t.Fatal("expected periodic sync loop to be running")
	}

	stopStarted := make(chan struct{})
	stopDone := make(chan struct{})
	saveDone := make(chan error, 1)

	go func() {
		close(stopStarted)
		app.stopPeriodicSyncLoop()
		close(stopDone)
	}()
	<-stopStarted

	go func() {
		_, err := app.SaveSyncSettings(SyncSettings{
			AutoSync:           true,
			SyncOnStartup:      false,
			SyncInterval:       15,
			ConflictResolution: "timestamp",
		})
		saveDone <- err
	}()

	select {
	case err := <-saveDone:
		if err != nil {
			t.Fatalf("SaveSyncSettings() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SaveSyncSettings() deadlocked while periodic stop was in progress")
	}
	<-stopDone
}

func TestPeriodicSyncDurationUsesConfiguredInterval(t *testing.T) {
	app := NewApp()
	app.config.Sync.SyncInterval = 7
	if got := app.periodicSyncDuration(); got != 7*time.Minute {
		t.Fatalf("expected 7 minute periodic sync duration, got %s", got)
	}

	app.config.Sync.SyncInterval = 0
	if got := app.periodicSyncDuration(); got != 30*time.Minute {
		t.Fatalf("expected default periodic sync duration, got %s", got)
	}

	app.periodicSyncInterval = 25 * time.Millisecond
	if got := app.periodicSyncDuration(); got != 25*time.Millisecond {
		t.Fatalf("expected test override duration, got %s", got)
	}
}

func TestSyncLocalFileModifiedAfterUploadIgnoresDatabaseRecordWriteGrace(t *testing.T) {
	uploadedAt := time.Now()
	if syncLocalFileModifiedAfterUpload(syncDatabaseKey, uploadedAt.Add(500*time.Millisecond), uploadedAt) {
		t.Fatal("database mtime inside sync-record write grace should not count as pending")
	}
	if !syncLocalFileModifiedAfterUpload(syncDatabaseKey, uploadedAt.Add(3*time.Second), uploadedAt) {
		t.Fatal("database mtime outside sync-record write grace should count as pending")
	}
	if !syncLocalFileModifiedAfterUpload("papers/paper-1/paper.pdf", uploadedAt.Add(500*time.Millisecond), uploadedAt) {
		t.Fatal("paper mtime after upload should count as pending without database grace")
	}
}
