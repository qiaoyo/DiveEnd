package main

import (
	"fmt"
	"strings"
	"time"
)

func (a *App) GetSyncStatus() (*SyncStatus, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	status := &SyncStatus{
		Enabled:  syncConfigured(a.config),
		Provider: "baidu_cloud",
	}

	records, err := a.db.GetSyncRecords(200)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.Status == "success" {
			status.TotalSynced++
			if status.LastSync == nil {
				lastSync := record.CreatedAt
				if !record.CompletedAt.IsZero() {
					lastSync = record.CompletedAt
				}
				status.LastSync = &lastSync
			}
		}
		if record.Status == "failed" {
			status.TotalFailed++
		}
	}

	if manager := a.ensureSyncManager(); manager != nil {
		progress := manager.GetSyncProgress()
		status.SyncInProgress = progress != nil && progress.Status != "" && progress.Status != "idle" && progress.Status != "complete" && progress.Status != "error"
		if syncConfigured(a.config) {
			localFiles, err := manager.getLocalDataFiles()
			if err == nil {
				status.PendingFiles = len(localFiles)
			}
		}
	}

	conflicts, err := a.refreshSyncConflictsIfPossible()
	if err != nil {
		return nil, err
	}
	for _, conflict := range conflicts {
		if strings.TrimSpace(conflict.Resolution) == "" {
			status.Conflicts++
		}
	}

	return status, nil
}

func (a *App) TriggerSync() (*SyncProgress, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if !syncConfigured(a.config) {
		return nil, fmt.Errorf("baidu sync is not configured")
	}

	manager := a.ensureSyncManager()
	if manager == nil {
		return nil, fmt.Errorf("sync manager is not initialized")
	}

	if err := manager.SyncToCloud(); err != nil {
		return cloneSyncProgress(manager.GetSyncProgress()), err
	}
	if _, err := a.refreshSyncConflictsIfPossible(); err != nil {
		return cloneSyncProgress(manager.GetSyncProgress()), err
	}

	return cloneSyncProgress(manager.GetSyncProgress()), nil
}

func (a *App) GetSyncProgress() (*SyncProgress, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	manager := a.ensureSyncManager()
	if manager == nil {
		return &SyncProgress{Status: "idle"}, nil
	}
	return cloneSyncProgress(manager.GetSyncProgress()), nil
}

func (a *App) GetSyncConflicts() ([]SyncConflict, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.refreshSyncConflictsIfPossible()
}

func (a *App) GetSyncRecords(limit int) ([]SyncRecord, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.db.GetSyncRecords(limit)
}

func (a *App) ResolveSyncConflict(conflictID, resolution string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}

	resolution = strings.TrimSpace(resolution)
	switch resolution {
	case "local", "remote", "skipped", "timestamp":
	default:
		return fmt.Errorf("unsupported sync conflict resolution: %s", resolution)
	}

	return a.db.ResolveSyncConflict(conflictID, resolution)
}

func (a *App) ensureSyncManager() *SyncManager {
	if a.db == nil {
		return nil
	}
	if a.syncManager == nil {
		a.syncManager = NewSyncManager(a.db, a.config)
	}
	return a.syncManager
}

func (a *App) refreshSyncConflictsIfPossible() ([]SyncConflict, error) {
	manager := a.ensureSyncManager()
	if manager == nil || !syncConfigured(a.config) {
		return a.db.GetSyncConflicts(200)
	}

	conflicts, err := manager.DetectConflicts()
	if err != nil {
		return nil, err
	}

	if err := a.db.ClearSyncConflicts(); err != nil {
		return nil, err
	}
	for i := range conflicts {
		if conflicts[i].ID == "" {
			conflicts[i].ID = fmt.Sprintf("conflict-%d", time.Now().UnixNano()+int64(i))
		}
		if conflicts[i].CreatedAt.IsZero() {
			conflicts[i].CreatedAt = time.Now()
		}
		if err := a.db.SaveSyncConflict(&conflicts[i]); err != nil {
			return nil, err
		}
	}

	return a.db.GetSyncConflicts(200)
}

func syncConfigured(config AppConfig) bool {
	return config.BaiduCloud.Enabled && strings.TrimSpace(config.BaiduCloud.Token) != ""
}

func cloneSyncProgress(progress *SyncProgress) *SyncProgress {
	if progress == nil {
		return &SyncProgress{Status: "idle"}
	}
	cloned := *progress
	return &cloned
}
