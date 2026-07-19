package main

import (
	"errors"
	"fmt"
	"log"
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

	syncInProgress := false
	if manager := a.ensureSyncManager(); manager != nil {
		progress := manager.GetSyncProgress()
		syncInProgress = syncProgressActive(progress)
		status.SyncInProgress = syncInProgress
		if syncConfigured(a.config) {
			pendingCount, err := manager.pendingLocalUploadCount()
			if err == nil {
				status.PendingFiles = pendingCount
			}
		}
	}

	var conflicts []SyncConflict
	if syncInProgress {
		conflicts, err = a.db.GetSyncConflicts(200)
		if err != nil {
			return nil, err
		}
	} else {
		conflicts, err = a.refreshSyncConflictsIfPossible()
		if err != nil {
			conflicts, err = a.db.GetSyncConflicts(200)
			if err != nil {
				return nil, err
			}
		}
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

	progress := manager.GetSyncProgress()
	if syncProgressActive(progress) {
		return cloneSyncProgress(progress), nil
	}

	initialProgress, err := manager.StartSyncToCloudAsync(func(err error) {
		if err != nil {
			manager.failProgress(err.Error())
			return
		}
		_, _ = a.refreshSyncConflictsIfPossible()
	})
	if errors.Is(err, errSyncAlreadyInProgress) {
		return cloneSyncProgress(manager.GetSyncProgress()), nil
	}
	if err != nil {
		return nil, err
	}

	return cloneSyncProgress(initialProgress), nil
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

func (a *App) RefreshBaiduToken() (*BaiduTokenRefreshStatus, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	manager := a.ensureSyncManager()
	if manager == nil {
		return &BaiduTokenRefreshStatus{Enabled: false, TokenFile: defaultBaiduTokenPath(), CheckedAt: time.Now(), Message: "百度云同步未配置"}, fmt.Errorf("baidu client not initialized")
	}
	return manager.RefreshBaiduToken()
}

func (a *App) GetSyncPreview() (*SyncPreview, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	manager := a.ensureSyncManager()
	if manager == nil {
		return &SyncPreview{Enabled: false, TokenFile: defaultBaiduTokenPath(), CheckedAt: time.Now(), Warning: "百度云同步未配置"}, fmt.Errorf("baidu client not initialized")
	}
	return manager.BuildSyncPreview()
}

func (a *App) GetSyncConflicts() ([]SyncConflict, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if manager := a.ensureSyncManager(); manager != nil && syncProgressActive(manager.GetSyncProgress()) {
		conflicts, err := a.db.GetSyncConflicts(200)
		if err != nil {
			return nil, err
		}
		return unresolvedSyncConflicts(conflicts), nil
	}
	conflicts, err := a.refreshSyncConflictsIfPossible()
	if err != nil {
		conflicts, fallbackErr := a.db.GetSyncConflicts(200)
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		return unresolvedSyncConflicts(conflicts), nil
	}
	return conflicts, nil
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

	conflict, err := a.db.GetSyncConflict(strings.TrimSpace(conflictID))
	if err != nil {
		return err
	}

	if resolution != "skipped" {
		manager := a.ensureSyncManager()
		if manager == nil {
			return fmt.Errorf("sync manager is not initialized")
		}
		if err := manager.ResolveConflict(conflict, resolution); err != nil {
			return err
		}
	}

	return a.db.ResolveSyncConflict(conflictID, resolution)
}

func (a *App) GetSyncSettings() (SyncSettings, error) {
	if err := a.ensureReady(); err != nil {
		return SyncSettings{}, err
	}
	return normalizeSyncSettings(a.config.Sync), nil
}

func (a *App) SaveSyncSettings(settings SyncSettings) (SyncSettings, error) {
	if err := a.ensureReady(); err != nil {
		return SyncSettings{}, err
	}
	normalized := normalizeSyncSettings(settings)
	persistedConfig := a.config
	if a.pendingRestartConfig != nil {
		persistedConfig = *a.pendingRestartConfig
	}
	persistedConfig.Sync = normalized
	if err := SaveAppConfig(persistedConfig); err != nil {
		return SyncSettings{}, err
	}

	a.config.Sync = normalized
	if a.pendingRestartConfig != nil {
		pendingConfig := persistedConfig
		a.pendingRestartConfig = &pendingConfig
	}
	a.configurePeriodicSyncLoopWithoutWait()
	return normalized, nil
}

func (a *App) GetPendingDatabaseRestore() (*DatabaseRestoreStatus, error) {
	config := normalizeAppConfig(a.config)
	return getPendingDatabaseRestore(config.DataPath)
}

func (a *App) ApplyPendingDatabaseRestore() (*DatabaseRestoreStatus, error) {
	config := normalizeAppConfig(a.config)
	a.stopDownloadWorkers()
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}

	status, err := applyPendingDatabaseRestore(config.DataPath)
	if err != nil {
		_ = a.applyConfig(config, true)
		return nil, err
	}
	if err := a.applyConfig(config, true); err != nil {
		return nil, err
	}
	return status, nil
}

func (a *App) CancelPendingDatabaseRestore() error {
	config := normalizeAppConfig(a.config)
	return cancelPendingDatabaseRestore(config.DataPath)
}

func (a *App) ensureSyncManager() *SyncManager {
	if a.db == nil {
		return nil
	}
	if a.syncManager == nil {
		a.syncManager = NewSyncManager(a.db, a.config)
		a.configureSyncManager()
	}
	return a.syncManager
}

func (a *App) refreshSyncConflictsIfPossible() ([]SyncConflict, error) {
	manager := a.ensureSyncManager()
	if manager == nil || !syncConfigured(a.config) {
		conflicts, err := a.db.GetSyncConflicts(200)
		if err != nil {
			return nil, err
		}
		return unresolvedSyncConflicts(conflicts), nil
	}

	conflicts, err := manager.DetectConflicts()
	if err != nil {
		return nil, err
	}

	conflicts = a.applyConfiguredConflictResolution(manager, conflicts)
	existingConflicts, err := a.db.GetSyncConflicts(200)
	if err != nil {
		return nil, err
	}
	if syncConflictsEquivalent(existingConflicts, conflicts) {
		return unresolvedSyncConflicts(existingConflicts), nil
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

func syncConflictsEquivalent(existing []SyncConflict, detected []SyncConflict) bool {
	existing = unresolvedSyncConflicts(existing)
	if len(existing) != len(detected) {
		return false
	}
	existingKeys := make(map[string]int, len(existing))
	for _, conflict := range existing {
		existingKeys[syncConflictIdentity(conflict)]++
	}
	for _, conflict := range detected {
		key := syncConflictIdentity(conflict)
		if existingKeys[key] == 0 {
			return false
		}
		existingKeys[key]--
	}
	return true
}

func unresolvedSyncConflicts(conflicts []SyncConflict) []SyncConflict {
	unresolved := make([]SyncConflict, 0, len(conflicts))
	for _, conflict := range conflicts {
		if strings.TrimSpace(conflict.Resolution) == "" {
			unresolved = append(unresolved, conflict)
		}
	}
	return unresolved
}

func syncConflictIdentity(conflict SyncConflict) string {
	localTime := conflict.LocalTime.UTC().Format(time.RFC3339Nano)
	if key, ok := syncDownloadKeyForRemotePath(conflict.RemotePath); ok && key == syncDatabaseKey {
		localTime = ""
	}
	return strings.Join([]string{
		strings.TrimSpace(conflict.FileName),
		strings.TrimSpace(conflict.LocalPath),
		localTime,
		strings.TrimSpace(conflict.RemotePath),
		conflict.RemoteTime.UTC().Format(time.RFC3339Nano),
	}, "\x00")
}

func (a *App) applyConfiguredConflictResolution(manager *SyncManager, conflicts []SyncConflict) []SyncConflict {
	if manager == nil || len(conflicts) == 0 {
		return conflicts
	}
	strategy := normalizeSyncSettings(a.config.Sync).ConflictResolution
	if strategy == "manual" {
		return conflicts
	}

	unresolved := make([]SyncConflict, 0, len(conflicts))
	for i := range conflicts {
		conflict := conflicts[i]
		if a.conflictAlreadyHandledByPendingRestore(manager, conflict, strategy) {
			continue
		}
		if err := manager.ResolveConflict(&conflict, strategy); err != nil {
			log.Printf("[Sync] auto conflict resolution failed for %s using %s: %s", conflict.FileName, strategy, redactErrorText(err))
			unresolved = append(unresolved, conflict)
			continue
		}
	}
	return unresolved
}

func (a *App) conflictAlreadyHandledByPendingRestore(manager *SyncManager, conflict SyncConflict, strategy string) bool {
	if manager == nil {
		return false
	}
	resolution := strings.TrimSpace(strategy)
	if resolution == "timestamp" {
		if conflict.RemoteTime.After(conflict.LocalTime) {
			resolution = "remote"
		} else {
			resolution = "local"
		}
	}
	if resolution != "remote" || !manager.isLiveDatabasePath(conflict.LocalPath) {
		return false
	}
	status, err := getPendingDatabaseRestore(manager.config.DataPath)
	return err == nil && status != nil && status.Pending && status.RemotePath == conflict.RemotePath
}

func syncConfigured(config AppConfig) bool {
	return config.BaiduCloud.Enabled && strings.TrimSpace(config.BaiduCloud.Token) != ""
}

func syncManagerConfigChanged(previous, next AppConfig) bool {
	return previous.DataPath != next.DataPath ||
		previous.BaiduCloud.Enabled != next.BaiduCloud.Enabled ||
		previous.BaiduCloud.Token != next.BaiduCloud.Token ||
		previous.BaiduCloud.RefreshToken != next.BaiduCloud.RefreshToken ||
		previous.BaiduCloud.ClientID != next.BaiduCloud.ClientID ||
		previous.BaiduCloud.ClientSecret != next.BaiduCloud.ClientSecret
}

func cloneSyncProgress(progress *SyncProgress) *SyncProgress {
	if progress == nil {
		return &SyncProgress{Status: "idle"}
	}
	cloned := *progress
	return &cloned
}

func syncProgressActive(progress *SyncProgress) bool {
	if progress == nil {
		return false
	}
	switch strings.TrimSpace(progress.Status) {
	case "", "idle", "complete", "completed", "error":
		return false
	default:
		return true
	}
}
