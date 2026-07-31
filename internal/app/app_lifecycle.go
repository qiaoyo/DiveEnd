package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if !a.shouldPromptBeforeClose() {
		return false
	}

	a.closeMu.Lock()
	if a.closeBypass {
		a.closeMu.Unlock()
		return false
	}
	if a.closeSyncing {
		a.closeMu.Unlock()
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "同步正在进行",
			Message: "DiveEnd 正在执行退出前同步。完成后会自动退出；如果失败，请根据错误提示重试或直接退出。",
			Buttons: []string{"知道了"},
		})
		return true
	}
	a.closeMu.Unlock()

	pendingCount, err := a.localSyncCandidateCount()
	if err != nil || pendingCount == 0 {
		return false
	}

	choice, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "退出前同步",
		Message:       fmt.Sprintf("检测到 %d 个本地同步候选文件。退出前是否先同步到百度网盘？", pendingCount),
		Buttons:       []string{"同步后退出", "直接退出", "取消"},
		DefaultButton: "同步后退出",
		CancelButton:  "取消",
	})
	if err != nil {
		return false
	}

	switch strings.TrimSpace(choice) {
	case "直接退出":
		a.closeMu.Lock()
		a.closeBypass = true
		a.closeMu.Unlock()
		return false
	case "同步后退出":
		a.startCloseSync(ctx)
		return true
	default:
		return true
	}
}

func (a *App) shouldPromptBeforeClose() bool {
	if !a.config.Sync.SyncBeforeExit || !syncConfigured(a.config) {
		return false
	}
	if a.db == nil {
		return false
	}
	return true
}

func (a *App) shouldRunShutdownSync() bool {
	if !a.shouldPromptBeforeClose() {
		return false
	}
	pendingCount, err := a.localSyncCandidateCount()
	return err == nil && pendingCount > 0
}

func (a *App) shouldRunLocalUploadSync() bool {
	if !a.config.Sync.AutoSync || !syncConfigured(a.config) || a.db == nil {
		return false
	}
	pendingCount, err := a.localSyncCandidateCount()
	return err == nil && pendingCount > 0
}

func (a *App) configurePeriodicSyncLoop() {
	a.configurePeriodicSyncLoopWithWait(true)
}

func (a *App) configurePeriodicSyncLoopWithoutWait() {
	a.configurePeriodicSyncLoopWithWait(false)
}

func (a *App) configurePeriodicSyncLoopWithWait(wait bool) {
	a.periodicSyncLifeMu.Lock()
	defer a.periodicSyncLifeMu.Unlock()

	a.stopPeriodicSyncLoopLocked(wait)
	if !a.periodicSyncEnabled() {
		return
	}

	interval := a.periodicSyncDuration()
	ctx, cancel := context.WithCancel(context.Background())

	a.periodicSyncMu.Lock()
	a.periodicSyncCancel = cancel
	a.periodicSyncWG.Add(1)
	a.periodicSyncMu.Unlock()

	go func() {
		defer a.periodicSyncWG.Done()
		timer := time.NewTimer(interval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				a.runPeriodicSyncOnce()
				timer.Reset(interval)
			}
		}
	}()
}

func (a *App) stopPeriodicSyncLoop() {
	a.stopPeriodicSyncLoopWithWait(true)
}

func (a *App) stopPeriodicSyncLoopWithWait(wait bool) {
	a.periodicSyncLifeMu.Lock()
	defer a.periodicSyncLifeMu.Unlock()
	a.stopPeriodicSyncLoopLocked(wait)
}

func (a *App) stopPeriodicSyncLoopLocked(wait bool) {
	a.periodicSyncMu.Lock()
	cancel := a.periodicSyncCancel
	a.periodicSyncCancel = nil
	a.periodicSyncMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if wait {
		a.periodicSyncWG.Wait()
	}
}

func (a *App) periodicSyncEnabled() bool {
	return a.config.Sync.AutoSync && syncConfigured(a.config) && a.db != nil
}

func (a *App) periodicSyncDuration() time.Duration {
	if a.periodicSyncInterval > 0 {
		return a.periodicSyncInterval
	}
	minutes := a.config.Sync.SyncInterval
	if minutes <= 0 {
		minutes = defaultSyncSettings().SyncInterval
	}
	return time.Duration(minutes) * time.Minute
}

func (a *App) periodicSyncLoopRunning() bool {
	a.periodicSyncMu.Lock()
	defer a.periodicSyncMu.Unlock()
	return a.periodicSyncCancel != nil
}

func (a *App) runPeriodicSyncOnce() {
	if !a.shouldRunLocalUploadSync() {
		return
	}
	manager := a.ensureSyncManager()
	if manager == nil || syncProgressActive(manager.GetSyncProgress()) {
		return
	}
	if err := manager.SyncToCloudIfIdle(); err != nil && !errors.Is(err, errSyncAlreadyInProgress) {
		log.Printf("[Sync] periodic auto-sync failed: %s", redactErrorText(err))
	}
}

func (a *App) localSyncCandidateCount() (int, error) {
	manager := a.ensureSyncManager()
	if manager == nil {
		return 0, nil
	}
	return manager.pendingLocalUploadCount()
}

func (sm *SyncManager) pendingLocalUploadCount() (int, error) {
	localFiles, err := sm.getLocalDataFiles()
	if err != nil {
		return 0, err
	}
	latestUploads, err := sm.db.GetLatestSuccessfulUploadTimes()
	if err != nil {
		return 0, err
	}
	pending := 0
	for key, file := range localFiles {
		remotePath := syncRemotePathForKey(key)
		uploadedAt, ok := latestUploads[remotePath]
		if !ok || syncLocalFileModifiedAfterUpload(key, file.Modified, uploadedAt) {
			pending++
		}
	}
	return pending, nil
}

const syncDatabaseRecordWriteGrace = 2 * time.Second

func syncLocalFileModifiedAfterUpload(key string, modifiedAt, uploadedAt time.Time) bool {
	if !modifiedAt.After(uploadedAt) {
		return false
	}
	if key == syncDatabaseKey && modifiedAt.Sub(uploadedAt) <= syncDatabaseRecordWriteGrace {
		return false
	}
	return true
}

func (a *App) startCloseSync(ctx context.Context) {
	a.closeMu.Lock()
	if a.closeSyncing {
		a.closeMu.Unlock()
		return
	}
	a.closeSyncing = true
	a.closeMu.Unlock()

	go func() {
		manager := a.ensureSyncManager()
		var syncErr error
		if manager == nil {
			syncErr = fmt.Errorf("sync manager is not initialized")
		} else if syncProgressActive(manager.GetSyncProgress()) {
			syncErr = waitForSyncCompletion(ctx, manager)
		} else {
			syncErr = manager.SyncToCloudIfIdle()
			if errors.Is(syncErr, errSyncAlreadyInProgress) {
				syncErr = waitForSyncCompletion(ctx, manager)
			}
		}

		a.closeMu.Lock()
		a.closeSyncing = false
		if syncErr == nil {
			a.closeBypass = true
		}
		shouldQuit := syncErr == nil
		a.closeMu.Unlock()

		if syncErr != nil {
			if a.ctx != nil {
				_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
					Type:    runtime.ErrorDialog,
					Title:   "退出前同步失败",
					Message: fmt.Sprintf("同步失败，应用将保持打开。你可以修复配置后重试，或再次关闭并选择直接退出。\n\n%v", syncErr),
					Buttons: []string{"知道了"},
				})
			}
			return
		}

		if shouldQuit && a.ctx != nil {
			runtime.Quit(a.ctx)
		} else if shouldQuit {
			runtime.Quit(ctx)
		}
	}()
}

func waitForSyncCompletion(ctx context.Context, manager *SyncManager) error {
	if manager == nil {
		return fmt.Errorf("sync manager is not initialized")
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		progress := manager.GetSyncProgress()
		if !syncProgressActive(progress) {
			if progress != nil && progress.Status == "error" {
				message := strings.TrimSpace(progress.Message)
				if message == "" {
					message = "cloud sync failed"
				}
				return errors.New(message)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
