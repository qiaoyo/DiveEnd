package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConfigureSyncManagerEmitsRuntimeSyncProgress(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	manager := &SyncManager{progress: &SyncProgress{Status: "idle"}}
	app.syncManager = manager

	type emittedEvent struct {
		name string
		data []interface{}
	}
	events := make(chan emittedEvent, 1)
	app.emitRuntimeEvent = func(ctx context.Context, eventName string, optionalData ...interface{}) {
		if ctx == nil {
			t.Fatal("expected non-nil runtime context")
		}
		events <- emittedEvent{name: eventName, data: append([]interface{}(nil), optionalData...)}
	}

	app.configureSyncManager()
	manager.updateProgress(func(progress *SyncProgress) {
		progress.Total = 2
		progress.Completed = 1
		progress.CurrentFile = "data/diveend.db"
		progress.Status = "uploading"
		progress.Message = "上传数据库快照"
	})

	select {
	case event := <-events:
		if event.name != "sync-progress" {
			t.Fatalf("expected sync-progress event, got %q", event.name)
		}
		if len(event.data) != 1 {
			t.Fatalf("expected one event payload, got %d", len(event.data))
		}
		progress, ok := event.data[0].(SyncProgress)
		if !ok {
			t.Fatalf("expected SyncProgress payload, got %T", event.data[0])
		}
		if progress.Status != "uploading" || progress.CurrentFile != "data/diveend.db" || progress.Completed != 1 {
			t.Fatalf("unexpected sync progress payload: %+v", progress)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sync-progress event")
	}
}

func TestSyncProgressActiveTreatsLegacyCompletedAsTerminal(t *testing.T) {
	for _, status := range []string{"", "idle", "complete", "completed", "error"} {
		if syncProgressActive(&SyncProgress{Status: status}) {
			t.Fatalf("status %q should not be active", status)
		}
	}
	for _, status := range []string{"preparing", "uploading", "downloading", "resolving"} {
		if !syncProgressActive(&SyncProgress{Status: status}) {
			t.Fatalf("status %q should be active", status)
		}
	}
	if syncProgressActive(nil) {
		t.Fatal("nil progress should not be active")
	}
}

func TestPendingLocalUploadCountUsesLatestSuccessfulUploadTimes(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected default folder")
	}

	pdfPath := filepath.Join(config.DataPath, "papers", "pending-count", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0700); err != nil {
		t.Fatalf("MkdirAll paper dir error = %v", err)
	}
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\npending count\n"), 0600); err != nil {
		t.Fatalf("WriteFile paper error = %v", err)
	}
	now := time.Now()
	paper := &Paper{
		ID:             "pending-count",
		SourcePaperID:  "pending-count",
		Title:          "Pending Count",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		URL:            "https://example.org/pending-count",
		PDFPath:        pdfPath,
		DownloadStatus: "downloaded",
		FolderID:       folders[0].ID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	manager := app.ensureSyncManager()
	if manager == nil {
		t.Fatal("expected sync manager")
	}
	pending, err := manager.pendingLocalUploadCount()
	if err != nil {
		t.Fatalf("pendingLocalUploadCount() error = %v", err)
	}
	if pending != 2 {
		t.Fatalf("expected database and paper to be pending before upload records, got %d", pending)
	}

	uploadedAt := time.Now().Add(time.Hour)
	for _, key := range []string{syncDatabaseKey, syncKeyForPaper(*paper)} {
		if err := app.db.SaveSyncRecord(&SyncRecord{
			Type:        "upload",
			FileName:    key,
			RemotePath:  syncRemotePathForKey(key),
			Status:      "success",
			CreatedAt:   uploadedAt,
			CompletedAt: uploadedAt,
		}); err != nil {
			t.Fatalf("SaveSyncRecord(%s) error = %v", key, err)
		}
	}

	pending, err = manager.pendingLocalUploadCount()
	if err != nil {
		t.Fatalf("pendingLocalUploadCount() after upload records error = %v", err)
	}
	if pending != 0 {
		t.Fatalf("expected no pending files after successful upload records, got %d", pending)
	}
	manager.updateProgress(func(progress *SyncProgress) {
		progress.Status = "uploading"
	})
	status, err := app.GetSyncStatus()
	if err != nil {
		t.Fatalf("GetSyncStatus() error = %v", err)
	}
	if status.PendingFiles != 0 {
		t.Fatalf("expected SyncStatus pending files to be 0 after successful upload records, got %+v", status)
	}

	modifiedAfterUpload := uploadedAt.Add(time.Hour)
	if err := os.Chtimes(pdfPath, modifiedAfterUpload, modifiedAfterUpload); err != nil {
		t.Fatalf("Chtimes paper error = %v", err)
	}
	pending, err = manager.pendingLocalUploadCount()
	if err != nil {
		t.Fatalf("pendingLocalUploadCount() after paper modification error = %v", err)
	}
	if pending != 1 {
		t.Fatalf("expected only modified paper to be pending, got %d", pending)
	}
	status, err = app.GetSyncStatus()
	if err != nil {
		t.Fatalf("GetSyncStatus() after paper modification error = %v", err)
	}
	if status.PendingFiles != 1 {
		t.Fatalf("expected SyncStatus pending files to be 1 after paper modification, got %+v", status)
	}
}

func TestTriggerSyncPublishesProgressSnapshots(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	remoteObjects := map[string]baiduRemoteObject{}
	uploadChunks := map[string]map[int][]byte{}
	var remoteMu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			handleGenericBaiduFileContract(t, w, r, &remoteMu, remoteObjects, uploadChunks)
		case "/rest/2.0/pcs/superfile2":
			handleGenericBaiduUploadContract(t, w, r, &remoteMu, uploadChunks)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	progressCh := make(chan SyncProgress, 32)
	manager.SetProgressReporter(func(progress SyncProgress) {
		progressCh <- progress
	})

	initial, err := app.TriggerSync()
	if err != nil {
		t.Fatalf("TriggerSync() error = %v", err)
	}
	if initial == nil {
		t.Fatal("expected initial progress")
	}

	events := collectSyncProgressUntil(t, progressCh, "complete", 5*time.Second)
	if !syncProgressContainsStatus(events, "preparing") {
		t.Fatalf("expected preparing progress event, got %+v", events)
	}
	if !syncProgressContainsStatus(events, "uploading") {
		t.Fatalf("expected uploading progress event, got %+v", events)
	}
	final := events[len(events)-1]
	if final.Status != "complete" || final.Completed != final.Total || final.Total < 2 {
		t.Fatalf("unexpected final sync progress: %+v", final)
	}

	remoteMu.Lock()
	_, hasDB := remoteObjects[syncRemotePathForKey(syncDatabaseKey)]
	_, hasManifest := remoteObjects[syncRemotePathForKey(syncManifestFileName)]
	remoteMu.Unlock()
	if !hasDB || !hasManifest {
		t.Fatalf("expected remote DB and manifest uploads, hasDB=%v hasManifest=%v", hasDB, hasManifest)
	}

	records, err := app.db.GetSyncRecords(10)
	if err != nil {
		t.Fatalf("GetSyncRecords() error = %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected sync records for DB and manifest uploads, got %+v", records)
	}
	for _, record := range records {
		if record.Status != "success" {
			t.Fatalf("expected successful sync records, got %+v", records)
		}
	}

	polled, err := app.GetSyncProgress()
	if err != nil {
		t.Fatalf("GetSyncProgress() error = %v", err)
	}
	if polled.Status != "complete" {
		t.Fatalf("expected polled progress to be complete, got %+v", polled)
	}
}

func TestSyncToCloudFinalizesDatabaseUploadRecordAfterBatch(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected default folder")
	}
	pdfPath := filepath.Join(config.DataPath, "papers", "batch-finalize", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0700); err != nil {
		t.Fatalf("MkdirAll paper dir error = %v", err)
	}
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\nbatch finalize\n"), 0600); err != nil {
		t.Fatalf("WriteFile paper error = %v", err)
	}
	now := time.Now()
	paper := &Paper{
		ID:             "batch-finalize",
		SourcePaperID:  "batch-finalize",
		Title:          "Batch Finalize",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		URL:            "https://example.org/batch-finalize",
		PDFPath:        pdfPath,
		DownloadStatus: "downloaded",
		FolderID:       folders[0].ID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	remoteObjects := map[string]baiduRemoteObject{}
	uploadChunks := map[string]map[int][]byte{}
	var remoteMu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			handleGenericBaiduFileContract(t, w, r, &remoteMu, remoteObjects, uploadChunks)
		case "/rest/2.0/pcs/superfile2":
			handleGenericBaiduUploadContract(t, w, r, &remoteMu, uploadChunks)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	if err := manager.SyncToCloud(); err != nil {
		t.Fatalf("SyncToCloud() error = %v", err)
	}

	records, err := app.db.GetSyncRecords(20)
	if err != nil {
		t.Fatalf("GetSyncRecords() error = %v", err)
	}
	var dbRecord *SyncRecord
	latestOtherCompletedAt := time.Time{}
	for i := range records {
		record := records[i]
		if record.Status != "success" {
			continue
		}
		if record.RemotePath == syncRemotePathForKey(syncDatabaseKey) {
			dbRecord = &record
			continue
		}
		if record.CompletedAt.After(latestOtherCompletedAt) {
			latestOtherCompletedAt = record.CompletedAt
		}
	}
	if dbRecord == nil {
		t.Fatalf("expected database upload record, got %+v", records)
	}
	if dbRecord.CompletedAt.Before(latestOtherCompletedAt) {
		t.Fatalf("expected database upload record to be finalized after batch records, db=%s latestOther=%s records=%+v", dbRecord.CompletedAt, latestOtherCompletedAt, records)
	}
	pending, err := manager.pendingLocalUploadCount()
	if err != nil {
		t.Fatalf("pendingLocalUploadCount() error = %v", err)
	}
	if pending != 0 {
		t.Fatalf("expected finalized database upload baseline to avoid pending uploads, got %d", pending)
	}
}

func TestStartSyncToCloudAsyncReservesSyncSlotImmediately(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	remoteObjects := map[string]baiduRemoteObject{}
	uploadChunks := map[string]map[int][]byte{}
	var remoteMu sync.Mutex
	precreateStarted := make(chan struct{})
	allowPrecreate := make(chan struct{})
	var oncePrecreate sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") == "precreate" {
				oncePrecreate.Do(func() { close(precreateStarted) })
				<-allowPrecreate
			}
			handleGenericBaiduFileContract(t, w, r, &remoteMu, remoteObjects, uploadChunks)
		case "/rest/2.0/pcs/superfile2":
			handleGenericBaiduUploadContract(t, w, r, &remoteMu, uploadChunks)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	done := make(chan error, 1)
	initial, err := manager.StartSyncToCloudAsync(func(err error) {
		done <- err
	})
	if err != nil {
		t.Fatalf("StartSyncToCloudAsync() error = %v", err)
	}
	if initial == nil || initial.Status != "preparing" {
		t.Fatalf("expected preparing initial progress, got %+v", initial)
	}

	second, err := manager.StartSyncToCloudAsync(nil)
	if !errors.Is(err, errSyncAlreadyInProgress) {
		t.Fatalf("expected second start to return errSyncAlreadyInProgress, progress=%+v err=%v", second, err)
	}

	select {
	case <-precreateStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first sync to reach precreate")
	}
	close(allowPrecreate)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("first async sync error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for first async sync to complete")
	}
}

func TestSyncToCloudReturnsErrorWhenUploadsFail(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			switch r.URL.Query().Get("method") {
			case "create":
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm create error = %v", err)
				}
				if r.Form.Get("isdir") == "1" {
					_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "fs_id": 1, "path": r.Form.Get("path")})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"errno": 313, "show_msg": "create rejected"})
			case "precreate":
				_ = json.NewEncoder(w).Encode(map[string]any{"errno": 313, "show_msg": "precreate rejected"})
			default:
				t.Fatalf("unexpected xpan method %q", r.URL.Query().Get("method"))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	err = manager.SyncToCloud()
	if err == nil {
		t.Fatal("expected SyncToCloud to report upload failures")
	}
	progress := manager.GetSyncProgress()
	if progress.Status != "error" {
		t.Fatalf("expected error progress after upload failures, got %+v", progress)
	}
	if !strings.Contains(progress.Message, "文件失败") {
		t.Fatalf("expected failure count in progress message, got %+v", progress)
	}

	records, err := app.db.GetSyncRecords(10)
	if err != nil {
		t.Fatalf("GetSyncRecords() error = %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected failed sync records")
	}
	for _, record := range records {
		if record.Status != "failed" {
			t.Fatalf("expected only failed records, got %+v", records)
		}
	}
}

func TestSyncToCloudIfIdleDoesNotQueueBehindActiveSync(t *testing.T) {
	manager := &SyncManager{progress: &SyncProgress{Status: "uploading"}}
	manager.syncMu.Lock()
	defer manager.syncMu.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- manager.SyncToCloudIfIdle()
	}()

	select {
	case err := <-done:
		if !errors.Is(err, errSyncAlreadyInProgress) {
			t.Fatalf("expected errSyncAlreadyInProgress, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("SyncToCloudIfIdle blocked behind active sync")
	}
}

func TestPrepareSyncSnapshotWritesSecureAtomicFiles(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		app.stopDownloadWorkers()
		_ = app.db.Close()
	})

	manager := NewSyncManager(app.db, app.config)
	files, cleanup, err := manager.prepareSyncSnapshot()
	if err != nil {
		t.Fatalf("prepareSyncSnapshot() error = %v", err)
	}
	defer cleanup()

	byKey := map[string]syncLocalFile{}
	for _, file := range files {
		byKey[file.Key] = file
	}
	for _, key := range []string{syncDatabaseKey, syncManifestFileName} {
		file, ok := byKey[key]
		if !ok {
			t.Fatalf("expected snapshot file %q in %+v", key, files)
		}
		info, err := os.Stat(file.Path)
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", file.Path, err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("expected %s permissions 0600, got %o", key, info.Mode().Perm())
		}
	}

	stagingDir := filepath.Dir(byKey[syncManifestFileName].Path)
	tempMatches, err := filepath.Glob(filepath.Join(stagingDir, ".*.tmp-*"))
	if err != nil {
		t.Fatalf("Glob staging temp files error = %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected no leftover staging temp files, got %v", tempMatches)
	}
}

func TestPrepareSyncSnapshotRejectsSymlinkedStagingRoot(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		app.stopDownloadWorkers()
		_ = app.db.Close()
	})

	outsideDir := t.TempDir()
	stagingRoot := filepath.Join(config.DataPath, ".sync-staging")
	if err := os.Symlink(outsideDir, stagingRoot); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	manager := NewSyncManager(app.db, app.config)
	files, cleanup, err := manager.prepareSyncSnapshot()
	if cleanup != nil {
		cleanup()
	}
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked staging root error, got files=%+v err=%v", files, err)
	}
	entries, readErr := os.ReadDir(outsideDir)
	if readErr != nil {
		t.Fatalf("ReadDir outside staging target error = %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no sync snapshot files outside DataPath, got %v", entries)
	}
}

func TestPrepareSyncSnapshotSkipsExternalPaperPDFPaths(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() {
		app.stopDownloadWorkers()
		_ = app.db.Close()
	})

	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected default folder")
	}

	managedPDF := filepath.Join(app.config.DataPath, "papers", "sync", "managed.pdf")
	if err := os.MkdirAll(filepath.Dir(managedPDF), 0700); err != nil {
		t.Fatalf("MkdirAll managed pdf dir error = %v", err)
	}
	if err := os.WriteFile(managedPDF, []byte("%PDF-1.4\nmanaged\n"), 0600); err != nil {
		t.Fatalf("WriteFile managed pdf error = %v", err)
	}
	resolvedManagedPDF, err := filepath.EvalSymlinks(managedPDF)
	if err != nil {
		t.Fatalf("EvalSymlinks(managedPDF) error = %v", err)
	}
	externalPDF := filepath.Join(t.TempDir(), "external.pdf")
	if err := os.WriteFile(externalPDF, []byte("%PDF-1.4\nexternal\n"), 0600); err != nil {
		t.Fatalf("WriteFile external pdf error = %v", err)
	}

	now := time.Now()
	managedPaper := &Paper{
		ID:             "sync-managed-paper",
		SourcePaperID:  "sync-managed-paper",
		Title:          "Managed Sync Paper",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		URL:            "https://example.org/managed",
		PDFPath:        managedPDF,
		DownloadStatus: "downloaded",
		FolderID:       folders[0].ID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	externalPaper := &Paper{
		ID:             "sync-external-paper",
		SourcePaperID:  "sync-external-paper",
		Title:          "External Sync Paper",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		URL:            "https://example.org/external",
		PDFPath:        externalPDF,
		DownloadStatus: "downloaded",
		FolderID:       folders[0].ID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	if err := app.db.UpsertPaper(managedPaper); err != nil {
		t.Fatalf("UpsertPaper(managed) error = %v", err)
	}
	if err := app.db.UpsertPaper(externalPaper); err != nil {
		t.Fatalf("UpsertPaper(external) error = %v", err)
	}

	manager := NewSyncManager(app.db, app.config)
	files, cleanup, err := manager.prepareSyncSnapshot()
	if err != nil {
		t.Fatalf("prepareSyncSnapshot() error = %v", err)
	}
	defer cleanup()

	managedKey := syncKeyForPaper(*managedPaper)
	externalKey := syncKeyForPaper(*externalPaper)
	seen := map[string]syncLocalFile{}
	for _, file := range files {
		seen[file.Key] = file
	}
	if file, ok := seen[managedKey]; !ok {
		t.Fatalf("expected managed pdf key %q in snapshot files %+v", managedKey, files)
	} else if file.Path != resolvedManagedPDF {
		t.Fatalf("expected managed pdf path %q, got %q", resolvedManagedPDF, file.Path)
	}
	if _, ok := seen[externalKey]; ok {
		t.Fatalf("external pdf key %q should not be included in sync snapshot: %+v", externalKey, files)
	}

	localFiles, err := manager.getLocalDataFiles()
	if err != nil {
		t.Fatalf("getLocalDataFiles() error = %v", err)
	}
	if _, ok := localFiles[managedKey]; !ok {
		t.Fatalf("expected managed pdf key %q in local files %+v", managedKey, localFiles)
	}
	if _, ok := localFiles[externalKey]; ok {
		t.Fatalf("external pdf key %q should not be included in local files: %+v", externalKey, localFiles)
	}
}

func TestSyncKeyForPaperSanitizesFallbackPaperFileName(t *testing.T) {
	paper := Paper{ID: "../bad/paper", PDFPath: ""}
	key := syncKeyForPaper(paper)
	if path.Base(key) != safePaperPDFFileName(paper.ID) {
		t.Fatalf("expected sanitized fallback filename %q, got key %q", safePaperPDFFileName(paper.ID), key)
	}
	if strings.Contains(key, "../") || strings.Contains(key, "..\\") || strings.Contains(key, "bad/paper.pdf") {
		t.Fatalf("sync key retained unsafe paper id path fragments: %q", key)
	}
	if !strings.HasPrefix(key, "papers/"+safeSyncSegment(paper.ID)+"/") {
		t.Fatalf("expected sanitized paper directory in key, got %q", key)
	}
}

func TestWaitForSyncCompletionReturnsAfterActiveSyncCompletes(t *testing.T) {
	manager := &SyncManager{progress: &SyncProgress{Status: "uploading", Message: "上传中"}}
	done := make(chan error, 1)
	go func() {
		done <- waitForSyncCompletion(context.Background(), manager)
	}()

	select {
	case err := <-done:
		t.Fatalf("waitForSyncCompletion returned before sync completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	manager.updateProgress(func(progress *SyncProgress) {
		progress.Status = "complete"
		progress.Message = "同步完成"
	})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waitForSyncCompletion() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sync completion")
	}
}

func TestSyncOnStartupReturnsErrorWhenDownloadsFail(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	remotePath := syncRemotePathForKey("papers/remote-only/remote.pdf")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				t.Fatalf("unexpected xpan method %q", r.URL.Query().Get("method"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{
						"path":         remotePath,
						"size":         42,
						"isdir":        0,
						"fs_id":        123456789,
						"server_mtime": time.Now().Unix(),
					},
				},
			})
		case "/rest/2.0/xpan/multimedia":
			if r.URL.Query().Get("method") != "filemetas" {
				t.Fatalf("unexpected multimedia method %q", r.URL.Query().Get("method"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"fs_id": 123456789, "dlink": "https://d.pcs.baidu.com/download/remote.pdf"},
				},
			})
		case "/download/remote.pdf":
			http.Error(w, "download rejected", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	err = manager.SyncOnStartup()
	if err == nil {
		t.Fatal("expected SyncOnStartup to report download failures")
	}
	progress := manager.GetSyncProgress()
	if progress.Status != "error" {
		t.Fatalf("expected error progress after download failures, got %+v", progress)
	}
	if !strings.Contains(progress.Message, "下载失败") {
		t.Fatalf("expected download failure count in progress message, got %+v", progress)
	}

	records, err := app.db.GetSyncRecords(10)
	if err != nil {
		t.Fatalf("GetSyncRecords() error = %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected failed sync record")
	}
	if records[0].Status != "failed" || records[0].Type != "download" {
		t.Fatalf("expected failed download record, got %+v", records)
	}
}

func TestSyncDownloadKeyForRemotePathRejectsUnsupportedKeys(t *testing.T) {
	validPath := syncRemotePathForKey("papers/paper-1/paper.pdf")
	key, ok := syncDownloadKeyForRemotePath(validPath)
	if !ok || key != "papers/paper-1/paper.pdf" {
		t.Fatalf("expected valid paper key, got key=%q ok=%v", key, ok)
	}

	for _, remotePath := range []string{
		path.Join(syncRemoteRootPath(), "diveend.db"),
		syncRemotePathForKey(syncManifestFileName),
		syncRemoteRootPath() + "/papers/paper-1/../evil.pdf",
		syncRemotePathForKey("papers/paper-1/notes.txt"),
		path.Join("/apps", syncApp, "other-root", "papers", "paper-1", "paper.pdf"),
	} {
		if key, ok := syncDownloadKeyForRemotePath(remotePath); ok {
			t.Fatalf("expected %q to be rejected, got key %q", remotePath, key)
		}
	}
}

func TestSyncOnStartupSkipsUnsupportedRemoteRootFiles(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	dbPath := filepath.Join(config.DataPath, "diveend.db")
	originalDB, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("ReadFile original DB error = %v", err)
	}

	remoteRootFile := path.Join(syncRemoteRootPath(), "diveend.db")
	downloadRequested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				t.Fatalf("unexpected xpan method %q", r.URL.Query().Get("method"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{
						"path":         remoteRootFile,
						"size":         len("malicious-db"),
						"isdir":        0,
						"fs_id":        987654321,
						"server_mtime": time.Now().Unix(),
					},
				},
			})
		case "/rest/2.0/xpan/multimedia":
			downloadRequested = true
			t.Fatalf("unsupported root file should not request download metadata")
		default:
			if strings.HasPrefix(r.URL.Path, "/download/") {
				downloadRequested = true
				t.Fatalf("unsupported root file should not be downloaded")
			}
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	if err := manager.SyncOnStartup(); err != nil {
		t.Fatalf("SyncOnStartup() error = %v", err)
	}
	progress := manager.GetSyncProgress()
	if progress.Status != "complete" || progress.Total != 0 || progress.Completed != 0 {
		t.Fatalf("expected unsupported remote files to be excluded from progress totals, got %+v", progress)
	}
	if downloadRequested {
		t.Fatal("unsupported root file unexpectedly triggered a download")
	}
	currentDB, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("ReadFile current DB error = %v", err)
	}
	if string(currentDB) != string(originalDB) {
		t.Fatal("unsupported remote root file changed the local database")
	}
}

func TestSyncOnStartupRejectsInvalidRemotePaperPDF(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	remotePath := syncRemotePathForKey("papers/remote-only/remote.pdf")
	server := newInvalidBaiduPaperDownloadServer(t, remotePath, []byte("not a pdf"))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	err = manager.SyncOnStartup()
	if err == nil || !strings.Contains(err.Error(), "downloaded paper pdf validation failed") {
		t.Fatalf("expected invalid paper pdf error, got %v", err)
	}

	targetPath := filepath.Join(config.DataPath, "papers", "remote-only", "remote.pdf")
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("invalid remote paper should not be kept at target, stat err=%v", err)
	}
	assertNoSyncDownloadTemps(t, filepath.Dir(targetPath), "remote.pdf")

	records, err := app.db.GetSyncRecords(10)
	if err != nil {
		t.Fatalf("GetSyncRecords() error = %v", err)
	}
	if len(records) == 0 || records[0].Status != "failed" || records[0].Type != "download" {
		t.Fatalf("expected failed download record, got %+v", records)
	}
}

func TestSyncDownloadManagedPaperPDFRejectsSymlinkedTargetDirectory(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	outsideDir := t.TempDir()
	targetDir := filepath.Join(config.DataPath, "papers", "paper-1")
	if err := os.MkdirAll(filepath.Dir(targetDir), 0700); err != nil {
		t.Fatalf("MkdirAll papers root error = %v", err)
	}
	if err := os.Symlink(outsideDir, targetDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	networkCalled := false
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		networkCalled = true
		return nil, fmt.Errorf("network should not be called")
	})}

	targetPath := filepath.Join(targetDir, "paper.pdf")
	err := manager.downloadManagedPaperPDFToPath(syncRemotePathForKey("papers/paper-1/paper.pdf"), targetPath)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked target directory error, got %v", err)
	}
	if networkCalled {
		t.Fatal("Baidu API should not be called when the managed paper target directory is a symlink")
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "paper.pdf")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no pdf to be written through symlinked target directory, stat err=%v", statErr)
	}
}

func TestResolveConflictRejectsUnsupportedRemotePath(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}

	conflict := &SyncConflict{
		FileName:   "diveend.db",
		LocalPath:  filepath.Join(config.DataPath, "diveend.db"),
		LocalTime:  time.Now(),
		RemotePath: path.Join(syncRemoteRootPath(), "diveend.db"),
		RemoteTime: time.Now().Add(time.Minute),
	}
	err := manager.ResolveConflict(conflict, "remote")
	if err == nil || !strings.Contains(err.Error(), "unsupported sync conflict remote path") {
		t.Fatalf("expected unsupported remote path error, got %v", err)
	}
}

func TestResolveConflictRemoteDatabaseCreatesRestoreDirectoryAndPendingMarker(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	if err := os.RemoveAll(databaseRestoreDir(config.DataPath)); err != nil {
		t.Fatalf("RemoveAll restore dir error = %v", err)
	}
	remoteDB := createSQLiteDBBytesForTest(t, "cloud_conflict_marker")
	remotePath := syncRemotePathForKey(syncDatabaseKey)
	server := newBaiduDatabaseDownloadServer(t, remotePath, remoteDB)
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	now := time.Now()
	conflict := &SyncConflict{
		FileName:   "diveend.db",
		LocalPath:  filepath.Join(config.DataPath, "diveend.db"),
		LocalTime:  now,
		RemotePath: remotePath,
		RemoteTime: now.Add(time.Minute),
	}
	if err := manager.ResolveConflict(conflict, "remote"); err != nil {
		t.Fatalf("ResolveConflict(remote database) error = %v", err)
	}

	status, err := getPendingDatabaseRestore(config.DataPath)
	if err != nil {
		t.Fatalf("getPendingDatabaseRestore() error = %v", err)
	}
	if !status.Pending {
		t.Fatalf("expected pending database restore, got %+v", status)
	}
	if _, ok := managedPathInsideRoot(databaseRestoreDir(config.DataPath), status.StagedPath); !ok {
		t.Fatalf("expected staged database inside restore dir, got %q", status.StagedPath)
	}
	if err := validateSQLiteDatabase(status.StagedPath); err != nil {
		t.Fatalf("staged database should be valid: %v", err)
	}
	info, err := os.Stat(databaseRestoreDir(config.DataPath))
	if err != nil {
		t.Fatalf("restore directory should exist: %v", err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatalf("unexpected restore directory mode: %v", info.Mode())
	}

	records, err := app.db.GetSyncRecords(10)
	if err != nil {
		t.Fatalf("GetSyncRecords() error = %v", err)
	}
	if len(records) == 0 || records[0].Type != "download" || records[0].Status != "success" || records[0].RemotePath != remotePath {
		t.Fatalf("expected successful download record for remote conflict resolution, got %+v", records)
	}
	if !strings.Contains(records[0].Message, "Conflict resolved by keeping cloud version") || !strings.Contains(records[0].Message, ".sync-conflicts") {
		t.Fatalf("expected cloud conflict resolution message, got %+v", records[0])
	}
}

func TestResolveConflictLocalDatabaseRecordsUploadAndClearsPending(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	remoteObjects := map[string]baiduRemoteObject{}
	uploadChunks := map[string]map[int][]byte{}
	var remoteMu sync.Mutex
	remotePath := syncRemotePathForKey(syncDatabaseKey)
	remoteDB := createSQLiteDBBytesForTest(t, "cloud_preserved_before_local_upload")
	remoteObjects[remotePath] = baiduRemoteObject{
		content: remoteDB,
		fsID:    "remote-db-fs-id",
		md5:     md5Hex(remoteDB),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			handleGenericBaiduFileContract(t, w, r, &remoteMu, remoteObjects, uploadChunks)
		case "/rest/2.0/pcs/superfile2":
			handleGenericBaiduUploadContract(t, w, r, &remoteMu, uploadChunks)
		case "/rest/2.0/xpan/multimedia":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{{
					"fs_id": "remote-db-fs-id",
					"dlink": "https://d.pcs.baidu.com/download/remote-db-fs-id",
				}},
			})
		default:
			if r.URL.Path == "/download/remote-db-fs-id" {
				if got := r.Header.Get("User-Agent"); got != "pan.baidu.com" {
					t.Fatalf("expected Baidu download User-Agent, got %q", got)
				}
				_, _ = w.Write(remoteDB)
				return
			}
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	now := time.Now()
	conflict := &SyncConflict{
		FileName:   "diveend.db",
		LocalPath:  filepath.Join(config.DataPath, "diveend.db"),
		LocalTime:  now,
		RemotePath: remotePath,
		RemoteTime: now.Add(-time.Minute),
	}
	if err := manager.ResolveConflict(conflict, "local"); err != nil {
		t.Fatalf("ResolveConflict(local database) error = %v", err)
	}

	remoteMu.Lock()
	_, uploaded := remoteObjects[remotePath]
	remoteMu.Unlock()
	if !uploaded {
		t.Fatalf("expected local database to be uploaded to %s", remotePath)
	}

	records, err := app.db.GetSyncRecords(10)
	if err != nil {
		t.Fatalf("GetSyncRecords() error = %v", err)
	}
	if len(records) == 0 || records[0].Type != "upload" || records[0].Status != "success" || records[0].RemotePath != remotePath {
		t.Fatalf("expected successful upload record for local conflict resolution, got %+v", records)
	}
	if !strings.Contains(records[0].Message, "Conflict resolved by keeping local version") || !strings.Contains(records[0].Message, ".sync-conflicts") {
		t.Fatalf("expected local conflict resolution message, got %+v", records[0])
	}
	archivePath := strings.TrimSpace(strings.TrimPrefix(records[0].Message, "Conflict resolved by keeping local version；被覆盖版本已保留："))
	if archivePath == records[0].Message || archivePath == "" {
		t.Fatalf("expected archive path in message, got %q", records[0].Message)
	}
	if err := validateSQLiteDatabase(archivePath); err != nil {
		t.Fatalf("expected archived remote DB to be valid, got %v", err)
	}

	pending, err := manager.pendingLocalUploadCount()
	if err != nil {
		t.Fatalf("pendingLocalUploadCount() error = %v", err)
	}
	if pending != 0 {
		t.Fatalf("expected conflict local upload record to clear pending uploads, got %d", pending)
	}
}

func TestRefreshSyncConflictsManualStrategyStoresUnresolvedConflict(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	config.Sync.ConflictResolution = "manual"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	localDBPath := filepath.Join(config.DataPath, "diveend.db")
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(localDBPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes local DB error = %v", err)
	}

	remotePath := syncRemotePathForKey(syncDatabaseKey)
	server := newBaiduDatabaseDownloadServer(t, remotePath, createSQLiteDBBytesForTest(t, "manual_conflict_marker"))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	conflicts, err := app.refreshSyncConflictsIfPossible()
	if err != nil {
		t.Fatalf("refreshSyncConflictsIfPossible() error = %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected one unresolved conflict in manual mode, got %+v", conflicts)
	}
	if conflicts[0].RemotePath != remotePath || strings.TrimSpace(conflicts[0].Resolution) != "" {
		t.Fatalf("unexpected manual conflict: %+v", conflicts[0])
	}
}

func TestRefreshSyncConflictsManualStrategyPreservesStableConflictID(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	config.Sync.ConflictResolution = "manual"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	localDBPath := filepath.Join(config.DataPath, "diveend.db")
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(localDBPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes local DB error = %v", err)
	}

	remotePath := syncRemotePathForKey(syncDatabaseKey)
	remoteMTime := time.Now().Add(time.Hour).Truncate(time.Second)
	server := newBaiduFixedListServer(t, []map[string]any{
		{
			"path":         remotePath,
			"size":         1024,
			"isdir":        0,
			"fs_id":        24680,
			"server_mtime": remoteMTime.Unix(),
		},
	})
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	first, err := app.refreshSyncConflictsIfPossible()
	if err != nil {
		t.Fatalf("first refreshSyncConflictsIfPossible() error = %v", err)
	}
	if len(first) != 1 || strings.TrimSpace(first[0].ID) == "" {
		t.Fatalf("expected one persisted conflict with ID, got %+v", first)
	}
	second, err := app.refreshSyncConflictsIfPossible()
	if err != nil {
		t.Fatalf("second refreshSyncConflictsIfPossible() error = %v", err)
	}
	if len(second) != 1 || second[0].ID != first[0].ID || !second[0].CreatedAt.Equal(first[0].CreatedAt) {
		t.Fatalf("expected stable conflict identity across idempotent refresh, first=%+v second=%+v", first, second)
	}
}

func TestGetSyncConflictsHidesResolvedCachedConflicts(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	config.Sync.ConflictResolution = "manual"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	resolvedAt := time.Now()
	if err := app.db.SaveSyncConflict(&SyncConflict{
		ID:         "resolved-conflict",
		FileName:   "diveend.db",
		LocalPath:  filepath.Join(config.DataPath, "diveend.db"),
		LocalTime:  resolvedAt.Add(-time.Hour),
		RemotePath: syncRemotePathForKey(syncDatabaseKey),
		RemoteTime: resolvedAt,
		Resolution: "local",
		ResolvedAt: resolvedAt,
		CreatedAt:  resolvedAt,
	}); err != nil {
		t.Fatalf("SaveSyncConflict() error = %v", err)
	}

	server := newBaiduFixedListServer(t, []map[string]any{})
	defer server.Close()
	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	conflicts, err := app.GetSyncConflicts()
	if err != nil {
		t.Fatalf("GetSyncConflicts() error = %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("expected resolved cached conflict to be hidden, got %+v", conflicts)
	}

	status, err := app.GetSyncStatus()
	if err != nil {
		t.Fatalf("GetSyncStatus() error = %v", err)
	}
	if status.Conflicts != 0 {
		t.Fatalf("expected resolved cached conflict not to count in status, got %+v", status)
	}
}

func TestRefreshSyncConflictsTimestampStrategyStagesRemoteDatabase(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	config.Sync.ConflictResolution = "timestamp"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	localDBPath := filepath.Join(config.DataPath, "diveend.db")
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(localDBPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes local DB error = %v", err)
	}

	remotePath := syncRemotePathForKey(syncDatabaseKey)
	server := newBaiduDatabaseDownloadServer(t, remotePath, createSQLiteDBBytesForTest(t, "timestamp_conflict_marker"))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	conflicts, err := app.refreshSyncConflictsIfPossible()
	if err != nil {
		t.Fatalf("refreshSyncConflictsIfPossible() error = %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("expected timestamp strategy to auto-resolve conflict, got %+v", conflicts)
	}

	status, err := getPendingDatabaseRestore(config.DataPath)
	if err != nil {
		t.Fatalf("getPendingDatabaseRestore() error = %v", err)
	}
	if !status.Pending || status.RemotePath != remotePath {
		t.Fatalf("expected pending restore for remote DB conflict, got %+v", status)
	}
	if err := validateSQLiteDatabase(status.StagedPath); err != nil {
		t.Fatalf("staged remote database should be valid: %v", err)
	}
}

func TestResolveConflictRemoteDatabaseRejectsSymlinkedRestoreDirectoryBeforeDownload(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	outsideDir := t.TempDir()
	restoreDir := databaseRestoreDir(config.DataPath)
	if err := os.Symlink(outsideDir, restoreDir); err != nil {
		t.Fatalf("Symlink restore dir error = %v", err)
	}

	networkCalled := false
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		networkCalled = true
		return nil, fmt.Errorf("network should not be called")
	})}

	now := time.Now()
	conflict := &SyncConflict{
		FileName:   "diveend.db",
		LocalPath:  filepath.Join(config.DataPath, "diveend.db"),
		LocalTime:  now,
		RemotePath: syncRemotePathForKey(syncDatabaseKey),
		RemoteTime: now.Add(time.Minute),
	}
	err := manager.ResolveConflict(conflict, "remote")
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked restore directory error, got %v", err)
	}
	if networkCalled {
		t.Fatal("Baidu API should not be called when restore directory is a symlink")
	}
}

func TestResolveConflictRejectsPaperTargetOutsideManagedPapers(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}

	conflict := &SyncConflict{
		FileName:   "paper.pdf",
		LocalPath:  filepath.Join(t.TempDir(), "outside.pdf"),
		LocalTime:  time.Now(),
		RemotePath: syncRemotePathForKey("papers/paper-1/paper.pdf"),
		RemoteTime: time.Now().Add(time.Minute),
	}
	err := manager.ResolveConflict(conflict, "remote")
	if err == nil || !strings.Contains(err.Error(), "managed papers directory") {
		t.Fatalf("expected managed papers directory error, got %v", err)
	}
}

func TestResolveConflictRejectsPaperTargetWithoutMatchingMetadata(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}

	conflict := &SyncConflict{
		FileName:   "paper.pdf",
		LocalPath:  filepath.Join(config.DataPath, "papers", "other-paper", "paper.pdf"),
		LocalTime:  time.Now(),
		RemotePath: syncRemotePathForKey("papers/paper-1/paper.pdf"),
		RemoteTime: time.Now().Add(time.Minute),
	}
	err := manager.ResolveConflict(conflict, "remote")
	if err == nil || !strings.Contains(err.Error(), "managed paper metadata") {
		t.Fatalf("expected managed paper metadata error, got %v", err)
	}
}

func TestResolveConflictRemotePreservesExistingPaperWhenDownloadedPDFInvalid(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	folders, err := app.db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected default folder")
	}

	pdfPath := filepath.Join(config.DataPath, "papers", "paper-1", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0700); err != nil {
		t.Fatalf("MkdirAll paper dir error = %v", err)
	}
	originalPDF := []byte("%PDF-1.4\nlocal paper\n")
	if err := os.WriteFile(pdfPath, originalPDF, 0600); err != nil {
		t.Fatalf("WriteFile local paper error = %v", err)
	}

	now := time.Now()
	paper := &Paper{
		ID:             "paper-1",
		SourcePaperID:  "paper-1",
		Title:          "Paper Conflict",
		Authors:        "Tester",
		Abstract:       "test",
		Year:           2026,
		URL:            "https://example.org/paper",
		PDFPath:        pdfPath,
		DownloadStatus: "downloaded",
		FolderID:       folders[0].ID,
		AddedAt:        now,
		UpdatedAt:      now,
	}
	if err := app.db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	remotePath := syncRemotePathForKey(syncKeyForPaper(*paper))
	server := newInvalidBaiduPaperDownloadServer(t, remotePath, []byte("definitely not a pdf"))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	conflict := &SyncConflict{
		FileName:   filepath.Base(pdfPath),
		LocalPath:  pdfPath,
		LocalTime:  now,
		RemotePath: remotePath,
		RemoteTime: now.Add(time.Minute),
	}
	err = manager.ResolveConflict(conflict, "remote")
	if err == nil || !strings.Contains(err.Error(), "downloaded paper pdf validation failed") {
		t.Fatalf("expected invalid paper pdf error, got %v", err)
	}

	currentPDF, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("ReadFile local paper error = %v", err)
	}
	if string(currentPDF) != string(originalPDF) {
		t.Fatal("invalid remote paper replaced the existing local PDF")
	}
	assertNoSyncDownloadTemps(t, filepath.Dir(pdfPath), filepath.Base(pdfPath))
}

func TestDetectConflictsWaitsForInProgressSync(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	precreateStarted := make(chan struct{})
	allowPrecreate := make(chan struct{})
	listHit := make(chan struct{})
	var oncePrecreate sync.Once
	var onceList sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			switch r.URL.Query().Get("method") {
			case "create":
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm create error = %v", err)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "fs_id": fmt.Sprint(time.Now().UnixNano()), "path": r.Form.Get("path")})
			case "precreate":
				oncePrecreate.Do(func() { close(precreateStarted) })
				<-allowPrecreate
				_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "uploadid": "sync-lock-test"})
			case "list":
				onceList.Do(func() { close(listHit) })
				_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "list": []any{}})
			default:
				t.Fatalf("unexpected xpan method %q", r.URL.Query().Get("method"))
			}
		case "/rest/2.0/pcs/superfile2":
			reader, err := r.MultipartReader()
			if err != nil {
				t.Fatalf("MultipartReader error = %v", err)
			}
			var payload []byte
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("NextPart error = %v", err)
				}
				if part.FormName() != "file" {
					continue
				}
				payload, err = io.ReadAll(part)
				if err != nil {
					t.Fatalf("ReadAll part error = %v", err)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"md5": md5Hex(payload)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	syncDone := make(chan error, 1)
	go func() {
		syncDone <- manager.SyncToCloud()
	}()

	select {
	case <-precreateStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sync upload to start")
	}

	conflictsDone := make(chan error, 1)
	go func() {
		_, err := manager.DetectConflicts()
		conflictsDone <- err
	}()

	select {
	case <-listHit:
		t.Fatal("DetectConflicts reached remote list while SyncToCloud was still in progress")
	case <-time.After(100 * time.Millisecond):
	}

	close(allowPrecreate)

	if err := <-syncDone; err != nil {
		t.Fatalf("SyncToCloud() error = %v", err)
	}
	if err := <-conflictsDone; err != nil {
		t.Fatalf("DetectConflicts() error = %v", err)
	}
	select {
	case <-listHit:
	default:
		t.Fatal("expected DetectConflicts to list remote files after sync completed")
	}
}

func TestSyncAPIsReturnCachedConflictsDuringInProgressSync(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	cachedConflict := &SyncConflict{
		ID:         "cached-conflict",
		FileName:   "diveend.db",
		LocalPath:  filepath.Join(config.DataPath, "diveend.db"),
		LocalTime:  time.Now(),
		RemotePath: syncRemotePathForKey(syncDatabaseKey),
		RemoteTime: time.Now().Add(time.Minute),
		CreatedAt:  time.Now(),
	}
	if err := app.db.SaveSyncConflict(cachedConflict); err != nil {
		t.Fatalf("SaveSyncConflict() error = %v", err)
	}

	precreateStarted := make(chan struct{})
	allowPrecreate := make(chan struct{})
	listHit := make(chan struct{})
	var oncePrecreate sync.Once
	var onceList sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			switch r.URL.Query().Get("method") {
			case "create":
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm create error = %v", err)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "fs_id": fmt.Sprint(time.Now().UnixNano()), "path": r.Form.Get("path")})
			case "precreate":
				oncePrecreate.Do(func() { close(precreateStarted) })
				<-allowPrecreate
				_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "uploadid": "sync-api-cache-test"})
			case "list":
				onceList.Do(func() { close(listHit) })
				_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "list": []any{}})
			default:
				t.Fatalf("unexpected xpan method %q", r.URL.Query().Get("method"))
			}
		case "/rest/2.0/pcs/superfile2":
			reader, err := r.MultipartReader()
			if err != nil {
				t.Fatalf("MultipartReader error = %v", err)
			}
			var payload []byte
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("NextPart error = %v", err)
				}
				if part.FormName() != "file" {
					continue
				}
				payload, err = io.ReadAll(part)
				if err != nil {
					t.Fatalf("ReadAll part error = %v", err)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"md5": md5Hex(payload)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	manager := app.ensureSyncManager()
	if manager == nil || manager.baiduClient == nil {
		t.Fatal("expected sync manager with Baidu client")
	}
	manager.baiduClient.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	syncDone := make(chan error, 1)
	go func() {
		syncDone <- manager.SyncToCloud()
	}()

	select {
	case <-precreateStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sync upload to start")
	}

	statusDone := make(chan *SyncStatus, 1)
	statusErr := make(chan error, 1)
	go func() {
		status, err := app.GetSyncStatus()
		if err != nil {
			statusErr <- err
			return
		}
		statusDone <- status
	}()

	select {
	case err := <-statusErr:
		t.Fatalf("GetSyncStatus() error = %v", err)
	case status := <-statusDone:
		if !status.SyncInProgress {
			t.Fatalf("expected in-progress status, got %+v", status)
		}
		if status.Conflicts != 1 {
			t.Fatalf("expected cached conflict count 1, got %+v", status)
		}
	case <-time.After(200 * time.Millisecond):
		close(allowPrecreate)
		t.Fatal("GetSyncStatus blocked on cloud conflict refresh during in-progress sync")
	}

	conflictsDone := make(chan []SyncConflict, 1)
	conflictsErr := make(chan error, 1)
	go func() {
		conflicts, err := app.GetSyncConflicts()
		if err != nil {
			conflictsErr <- err
			return
		}
		conflictsDone <- conflicts
	}()

	select {
	case err := <-conflictsErr:
		t.Fatalf("GetSyncConflicts() error = %v", err)
	case conflicts := <-conflictsDone:
		if len(conflicts) != 1 || conflicts[0].ID != cachedConflict.ID {
			t.Fatalf("expected cached conflicts, got %+v", conflicts)
		}
	case <-time.After(200 * time.Millisecond):
		close(allowPrecreate)
		t.Fatal("GetSyncConflicts blocked on cloud conflict refresh during in-progress sync")
	}

	select {
	case <-listHit:
		close(allowPrecreate)
		t.Fatal("sync APIs hit remote conflict list while sync was in progress")
	default:
	}

	close(allowPrecreate)
	if err := <-syncDone; err != nil {
		t.Fatalf("SyncToCloud() error = %v", err)
	}
}

func newInvalidBaiduPaperDownloadServer(t *testing.T, remotePath string, payload []byte) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				t.Fatalf("unexpected xpan method %q", r.URL.Query().Get("method"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{
						"path":         remotePath,
						"size":         len(payload),
						"isdir":        0,
						"fs_id":        246813579,
						"server_mtime": time.Now().Unix(),
					},
				},
			})
		case "/rest/2.0/xpan/multimedia":
			if r.URL.Query().Get("method") != "filemetas" {
				t.Fatalf("unexpected multimedia method %q", r.URL.Query().Get("method"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"fs_id": 246813579, "dlink": "https://d.pcs.baidu.com/download/invalid-paper.pdf"},
				},
			})
		case "/download/invalid-paper.pdf":
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func newBaiduDatabaseDownloadServer(t *testing.T, remotePath string, payload []byte) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				t.Fatalf("unexpected xpan method %q", r.URL.Query().Get("method"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{
						"path":         remotePath,
						"size":         len(payload),
						"isdir":        0,
						"fs_id":        135792468,
						"server_mtime": time.Now().Unix(),
					},
				},
			})
		case "/rest/2.0/xpan/multimedia":
			if r.URL.Query().Get("method") != "filemetas" {
				t.Fatalf("unexpected multimedia method %q", r.URL.Query().Get("method"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"fs_id": 135792468, "dlink": "https://d.pcs.baidu.com/download/remote-diveend.db"},
				},
			})
		case "/download/remote-diveend.db":
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func newBaiduFixedListServer(t *testing.T, entries []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				t.Fatalf("unexpected xpan method %q", r.URL.Query().Get("method"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "list": entries})
		default:
			http.NotFound(w, r)
		}
	}))
}

func createSQLiteDBBytesForTest(t *testing.T, markerTable string) []byte {
	t.Helper()
	dbDir := t.TempDir()
	db, err := NewDB(dbDir)
	if err != nil {
		t.Fatalf("NewDB(remote fixture) error = %v", err)
	}
	if _, err := db.conn.Exec(fmt.Sprintf("CREATE TABLE %s (id TEXT PRIMARY KEY)", markerTable)); err != nil {
		_ = db.Close()
		t.Fatalf("create remote fixture marker error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(remote fixture DB) error = %v", err)
	}
	payload, err := os.ReadFile(filepath.Join(dbDir, "diveend.db"))
	if err != nil {
		t.Fatalf("ReadFile(remote fixture DB) error = %v", err)
	}
	return payload
}

func assertNoSyncDownloadTemps(t *testing.T, dir, baseName string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "."+baseName+".sync-download-*"))
	if err != nil {
		t.Fatalf("Glob sync download temps error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no sync download temp files, got %v", matches)
	}
}

func handleGenericBaiduFileContract(
	t *testing.T,
	w http.ResponseWriter,
	r *http.Request,
	remoteMu *sync.Mutex,
	remoteObjects map[string]baiduRemoteObject,
	uploadChunks map[string]map[int][]byte,
) {
	t.Helper()
	switch r.URL.Query().Get("method") {
	case "create":
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm create error = %v", err)
		}
		if r.Form.Get("isdir") == "1" {
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "fs_id": fmt.Sprint(time.Now().UnixNano()), "path": r.Form.Get("path")})
			return
		}
		remotePath := r.Form.Get("path")
		uploadID := r.Form.Get("uploadid")
		blockList := decodeBlockList(t, r.Form.Get("block_list"))
		remoteMu.Lock()
		content := joinChunks(uploadChunks[uploadID], len(blockList))
		remoteObjects[remotePath] = baiduRemoteObject{
			content: append([]byte(nil), content...),
			fsID:    fmt.Sprint(len(remoteObjects) + 1),
			md5:     md5Hex(content),
		}
		remoteMu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "fs_id": fmt.Sprint(time.Now().UnixNano()), "path": remotePath})
	case "precreate":
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm precreate error = %v", err)
		}
		remotePath := r.Form.Get("path")
		uploadID := "sync-progress-" + strings.ReplaceAll(remotePath, "/", "_")
		remoteMu.Lock()
		uploadChunks[uploadID] = map[int][]byte{}
		remoteMu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "uploadid": uploadID})
	case "list":
		dir := r.URL.Query().Get("dir")
		remoteMu.Lock()
		list := listRemoteObjects(dir, remoteObjects)
		remoteMu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "list": list})
	default:
		t.Fatalf("unexpected xpan file method %q", r.URL.Query().Get("method"))
	}
}

func handleGenericBaiduUploadContract(
	t *testing.T,
	w http.ResponseWriter,
	r *http.Request,
	remoteMu *sync.Mutex,
	uploadChunks map[string]map[int][]byte,
) {
	t.Helper()
	uploadID := r.URL.Query().Get("uploadid")
	partSeq, err := strconv.Atoi(r.URL.Query().Get("partseq"))
	if err != nil {
		t.Fatalf("bad partseq: %v", err)
	}
	reader, err := r.MultipartReader()
	if err != nil {
		t.Fatalf("MultipartReader error = %v", err)
	}
	var payload []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart error = %v", err)
		}
		if part.FormName() != "file" {
			continue
		}
		payload, err = io.ReadAll(part)
		if err != nil {
			t.Fatalf("ReadAll part error = %v", err)
		}
	}
	if len(payload) == 0 {
		t.Fatal("expected upload payload")
	}
	remoteMu.Lock()
	if uploadChunks[uploadID] == nil {
		uploadChunks[uploadID] = map[int][]byte{}
	}
	uploadChunks[uploadID][partSeq] = append([]byte(nil), payload...)
	remoteMu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]any{"md5": md5Hex(payload)})
}

func collectSyncProgressUntil(t *testing.T, ch <-chan SyncProgress, targetStatus string, timeout time.Duration) []SyncProgress {
	t.Helper()
	deadline := time.After(timeout)
	events := []SyncProgress{}
	for {
		select {
		case event := <-ch:
			events = append(events, event)
			if event.Status == targetStatus {
				return events
			}
		case <-deadline:
			t.Fatalf("timed out waiting for sync progress %q, got %+v", targetStatus, events)
		}
	}
}

func syncProgressContainsStatus(events []SyncProgress, status string) bool {
	for _, event := range events {
		if event.Status == status {
			return true
		}
	}
	return false
}

func TestSyncTriggerRejectsConcurrentRuns(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = true
	config.BaiduCloud.Token = "valid-token"
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	manager := app.ensureSyncManager()
	manager.updateProgress(func(progress *SyncProgress) {
		progress.Status = "uploading"
		progress.Total = 2
		progress.Completed = 1
		progress.CurrentFile = "data/diveend.db"
	})

	progress, err := app.TriggerSync()
	if err != nil {
		t.Fatalf("TriggerSync() should return in-progress state instead of error: %v", err)
	}
	if progress.Status != "uploading" || progress.CurrentFile != "data/diveend.db" {
		t.Fatalf("expected current in-progress snapshot, got %+v", progress)
	}
}

func TestSyncProgressCloneIsImmutable(t *testing.T) {
	manager := &SyncManager{progress: &SyncProgress{Status: "uploading", CurrentFile: "data/diveend.db"}}
	first := manager.GetSyncProgress()
	first.Status = "mutated"
	second := manager.GetSyncProgress()
	if second.Status != "uploading" {
		t.Fatalf("expected progress clone to be immutable, got %+v", second)
	}
}
