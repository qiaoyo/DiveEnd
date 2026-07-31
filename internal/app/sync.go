package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	sqlite3 "github.com/mattn/go-sqlite3"
)

var errSyncAlreadyInProgress = errors.New("cloud sync already in progress")

const (
	// The OAuth app name must match the Baidu app folder name used by the proven
	// token/upload script in this repo.
	syncApp = "pcstest_oauth"

	defaultSyncDataRootName = "diveend-v1"
	syncManifestFileName    = "manifest.json"
	syncDatabaseKey         = "data/diveend.db"
	syncConflictArchiveName = ".sync-conflicts"
)

var syncDataRootName = defaultSyncDataRootName

type syncLocalFile struct {
	Key      string
	Path     string
	Size     int64
	Modified time.Time
	SHA256   string
}

type syncManifest struct {
	Version   int                `json:"version"`
	App       string             `json:"app"`
	CreatedAt time.Time          `json:"createdAt"`
	Files     []syncManifestFile `json:"files"`
}

type syncManifestFile struct {
	Key      string    `json:"key"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	SHA256   string    `json:"sha256"`
}

// migrateSync 创建同步相关的数据库表
func (db *DB) migrateSync() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS sync_records (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			file_name TEXT NOT NULL,
			file_size INTEGER,
			remote_path TEXT,
			local_path TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			message TEXT,
			error_message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS sync_conflicts (
			id TEXT PRIMARY KEY,
			file_name TEXT NOT NULL,
			file_kind TEXT,
			local_path TEXT NOT NULL,
			local_size INTEGER NOT NULL DEFAULT 0,
			local_time DATETIME NOT NULL,
			remote_path TEXT NOT NULL,
			remote_size INTEGER NOT NULL DEFAULT 0,
			remote_time DATETIME NOT NULL,
			newer_side TEXT,
			resolution TEXT,
			resolved_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_records_status ON sync_records(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_records_created_at ON sync_records(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_conflicts_created_at ON sync_conflicts(created_at DESC)`,
	}

	for _, stmt := range statements {
		if _, err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute statement: %w", err)
		}
	}
	if err := db.ensureSyncRecordColumn("message", "TEXT"); err != nil {
		return err
	}
	for _, column := range []struct {
		name string
		typ  string
	}{
		{name: "file_kind", typ: "TEXT"},
		{name: "local_size", typ: "INTEGER NOT NULL DEFAULT 0"},
		{name: "remote_size", typ: "INTEGER NOT NULL DEFAULT 0"},
		{name: "newer_side", typ: "TEXT"},
	} {
		if err := db.ensureSyncConflictColumn(column.name, column.typ); err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) ensureSyncRecordColumn(columnName, columnType string) error {
	rows, err := db.conn.Query(`PRAGMA table_info(sync_records)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			return nil
		}
	}

	_, err = db.conn.Exec(fmt.Sprintf(`ALTER TABLE sync_records ADD COLUMN %s %s`, columnName, columnType))
	return err
}

func (db *DB) ensureSyncConflictColumn(columnName, columnType string) error {
	rows, err := db.conn.Query(`PRAGMA table_info(sync_conflicts)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.conn.Exec(fmt.Sprintf(`ALTER TABLE sync_conflicts ADD COLUMN %s %s`, columnName, columnType))
	return err
}

// SaveSyncRecord 保存同步记录
func (db *DB) SaveSyncRecord(record *SyncRecord) error {
	now := time.Now()
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}

	_, err := db.conn.Exec(`
			INSERT INTO sync_records (
				id, type, file_name, file_size, remote_path, local_path,
				status, message, error_message, created_at, completed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
		record.ID,
		record.Type,
		record.FileName,
		record.FileSize,
		nullIfBlank(record.RemotePath),
		nullIfBlank(record.LocalPath),
		record.Status,
		nullIfBlank(record.Message),
		nullIfBlank(record.ErrorMessage),
		record.CreatedAt,
		nullIfTime(record.CompletedAt),
	)

	return err
}

func (db *DB) UpdateSyncRecordCompletedAt(recordID string, completedAt time.Time) error {
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return fmt.Errorf("sync record ID cannot be empty")
	}
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	_, err := db.conn.Exec(`
			UPDATE sync_records
			SET completed_at = ?
			WHERE id = ?
		`, completedAt, recordID)
	return err
}

// GetSyncRecords 获取同步记录
func (db *DB) GetSyncRecords(limit int) ([]SyncRecord, error) {
	query := `
			SELECT id, type, file_name, file_size, remote_path, local_path,
			       status, message, error_message, created_at, completed_at
			FROM sync_records
			ORDER BY created_at DESC
		`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []SyncRecord
	for rows.Next() {
		var record SyncRecord
		var completedAt sql.NullTime
		var remotePath, localPath, message, errorMessage sql.NullString

		if err := rows.Scan(
			&record.ID,
			&record.Type,
			&record.FileName,
			&record.FileSize,
			&remotePath,
			&localPath,
			&record.Status,
			&message,
			&errorMessage,
			&record.CreatedAt,
			&completedAt,
		); err != nil {
			return nil, err
		}

		record.RemotePath = remotePath.String
		record.LocalPath = localPath.String
		record.Message = message.String
		record.ErrorMessage = errorMessage.String
		record.CompletedAt = completedAt.Time

		records = append(records, record)
	}

	return records, rows.Err()
}

func (db *DB) GetLatestSuccessfulUploadTimes() (map[string]time.Time, error) {
	rows, err := db.conn.Query(`
			SELECT remote_path, created_at, completed_at
			FROM sync_records
			WHERE type = 'upload'
			  AND status = 'success'
			  AND remote_path IS NOT NULL
			  AND remote_path != ''
			ORDER BY remote_path ASC, COALESCE(completed_at, created_at) DESC
		`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	uploadTimes := make(map[string]time.Time)
	for rows.Next() {
		var remotePath string
		var createdAt time.Time
		var completedAt sql.NullTime
		if err := rows.Scan(&remotePath, &createdAt, &completedAt); err != nil {
			return nil, err
		}
		if _, exists := uploadTimes[remotePath]; exists {
			continue
		}
		uploadedAt := createdAt
		if completedAt.Valid {
			uploadedAt = completedAt.Time
		}
		uploadTimes[remotePath] = uploadedAt
	}
	return uploadTimes, rows.Err()
}

// SaveSyncConflict 保存冲突记录
func (db *DB) SaveSyncConflict(conflict *SyncConflict) error {
	now := time.Now()
	if conflict.ID == "" {
		conflict.ID = uuid.NewString()
	}
	if conflict.CreatedAt.IsZero() {
		conflict.CreatedAt = now
	}

	_, err := db.conn.Exec(`
			INSERT INTO sync_conflicts (
				id, file_name, file_kind, local_path, local_size, local_time,
				remote_path, remote_size, remote_time, newer_side, resolution, resolved_at, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
		conflict.ID,
		conflict.FileName,
		nullIfBlank(conflict.FileKind),
		conflict.LocalPath,
		conflict.LocalSize,
		conflict.LocalTime,
		conflict.RemotePath,
		conflict.RemoteSize,
		conflict.RemoteTime,
		nullIfBlank(conflict.NewerSide),
		nullIfBlank(conflict.Resolution),
		nullIfTime(conflict.ResolvedAt),
		conflict.CreatedAt,
	)

	return err
}

// GetSyncConflicts 获取冲突列表
func (db *DB) GetSyncConflicts(limit int) ([]SyncConflict, error) {
	query := `
			SELECT id, file_name, file_kind, local_path, local_size, local_time,
			       remote_path, remote_size, remote_time, newer_side,
			       resolution, resolved_at, created_at
			FROM sync_conflicts
			ORDER BY created_at DESC
		`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conflicts []SyncConflict
	for rows.Next() {
		var conflict SyncConflict
		var fileKind, newerSide, resolution sql.NullString
		var resolvedAt sql.NullTime

		if err := rows.Scan(
			&conflict.ID,
			&conflict.FileName,
			&fileKind,
			&conflict.LocalPath,
			&conflict.LocalSize,
			&conflict.LocalTime,
			&conflict.RemotePath,
			&conflict.RemoteSize,
			&conflict.RemoteTime,
			&newerSide,
			&resolution,
			&resolvedAt,
			&conflict.CreatedAt,
		); err != nil {
			return nil, err
		}

		conflict.FileKind = fileKind.String
		conflict.NewerSide = newerSide.String
		conflict.Resolution = resolution.String
		conflict.ResolvedAt = resolvedAt.Time

		conflicts = append(conflicts, conflict)
	}

	return conflicts, rows.Err()
}

func (db *DB) GetSyncConflict(conflictID string) (*SyncConflict, error) {
	row := db.conn.QueryRow(`
			SELECT id, file_name, file_kind, local_path, local_size, local_time,
			       remote_path, remote_size, remote_time, newer_side,
			       resolution, resolved_at, created_at
			FROM sync_conflicts
			WHERE id = ?
		`, conflictID)

	var conflict SyncConflict
	var fileKind, newerSide, resolution sql.NullString
	var resolvedAt sql.NullTime
	if err := row.Scan(
		&conflict.ID,
		&conflict.FileName,
		&fileKind,
		&conflict.LocalPath,
		&conflict.LocalSize,
		&conflict.LocalTime,
		&conflict.RemotePath,
		&conflict.RemoteSize,
		&conflict.RemoteTime,
		&newerSide,
		&resolution,
		&resolvedAt,
		&conflict.CreatedAt,
	); err != nil {
		return nil, err
	}

	conflict.FileKind = fileKind.String
	conflict.NewerSide = newerSide.String
	conflict.Resolution = resolution.String
	conflict.ResolvedAt = resolvedAt.Time
	return &conflict, nil
}

// ResolveSyncConflict 解决冲突
func (db *DB) ResolveSyncConflict(conflictID, resolution string) error {
	now := time.Now()
	_, err := db.conn.Exec(`
			UPDATE sync_conflicts
			SET resolution = ?, resolved_at = ?
			WHERE id = ?
		`, nullIfBlank(resolution), nullIfTime(now), conflictID)

	return err
}

func (db *DB) ClearSyncConflicts() error {
	_, err := db.conn.Exec(`DELETE FROM sync_conflicts`)
	return err
}

// SyncManager 同步管理器
type SyncManager struct {
	db          *DB
	baiduClient *BaiduPCSClient
	config      AppConfig
	syncMu      sync.Mutex
	progressMu  sync.RWMutex
	progress    *SyncProgress
	progressCb  func(SyncProgress)
}

// NewSyncManager 创建同步管理器
func NewSyncManager(db *DB, config AppConfig) *SyncManager {
	var baiduClient *BaiduPCSClient
	if config.BaiduCloud.Enabled {
		if token := baiduTokenForSync(config); token != nil {
			baiduClient = NewBaiduPCSClientWithTokenPath(token, defaultBaiduTokenPath())
		}
	}

	return &SyncManager{
		db:          db,
		baiduClient: baiduClient,
		config:      config,
		progress:    &SyncProgress{Status: "idle"},
	}
}

func baiduTokenForSync(config AppConfig) *BaiduToken {
	if strings.TrimSpace(config.BaiduCloud.Token) != "" {
		return &BaiduToken{
			AccessToken:  strings.TrimSpace(config.BaiduCloud.Token),
			RefreshToken: strings.TrimSpace(config.BaiduCloud.RefreshToken),
			ClientID:     strings.TrimSpace(config.BaiduCloud.ClientID),
			ClientSecret: strings.TrimSpace(config.BaiduCloud.ClientSecret),
		}
	}

	if token, err := LoadBaiduToken(defaultBaiduTokenPath()); err == nil && strings.TrimSpace(token.AccessToken) != "" {
		return token
	}
	return nil
}

func (sm *SyncManager) BuildSyncPreview() (*SyncPreview, error) {
	preview := &SyncPreview{
		Enabled:    sm != nil && sm.baiduClient != nil,
		DataPath:   "",
		RemoteRoot: syncRemoteRootPath(),
		TokenFile:  defaultBaiduTokenPath(),
		CheckedAt:  time.Now(),
	}
	if sm == nil {
		preview.Warning = "同步管理器未初始化"
		return preview, fmt.Errorf("sync manager is nil")
	}
	preview.DataPath = sm.config.DataPath
	if sm.baiduClient == nil {
		preview.Warning = "百度云同步未配置"
		return preview, fmt.Errorf("baidu client not initialized")
	}

	localFiles, cleanup, err := sm.prepareSyncSnapshot()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		preview.Warning = fmt.Sprintf("准备本地同步快照失败: %v", err)
		return preview, err
	}

	preview.Files = make([]SyncPreviewFile, 0, len(localFiles))
	for _, file := range localFiles {
		kind := "other"
		switch {
		case file.Key == syncDatabaseKey:
			kind = "database"
			preview.DatabaseBytes += file.Size
		case strings.HasPrefix(file.Key, "papers/"):
			kind = "paper_pdf"
			preview.PaperPDFCount++
			preview.PaperPDFBytes += file.Size
		default:
			preview.OtherFiles++
		}
		preview.TotalFiles++
		preview.TotalBytes += file.Size
		preview.Files = append(preview.Files, SyncPreviewFile{
			Key:        file.Key,
			FileName:   filepath.Base(file.Key),
			Kind:       kind,
			Size:       file.Size,
			RemotePath: syncRemotePathForKey(file.Key),
		})
	}
	if preview.TotalBytes >= 1024*1024*1024 {
		preview.Warning = "本次同步超过 1 GiB，建议确认网络稳定并保持应用打开。"
	}
	return preview, nil
}

func (sm *SyncManager) RefreshBaiduToken() (*BaiduTokenRefreshStatus, error) {
	if sm == nil || sm.baiduClient == nil {
		return &BaiduTokenRefreshStatus{Enabled: false, TokenFile: defaultBaiduTokenPath(), CheckedAt: time.Now(), Message: "百度云同步未配置"}, fmt.Errorf("baidu client not initialized")
	}
	if _, err := sm.baiduClient.RefreshAccessToken(); err != nil {
		status := sm.baiduTokenStatus(false, "百度 access token 刷新失败")
		return status, err
	}
	return sm.baiduTokenStatus(true, "百度 access token 已通过 refresh token 更新"), nil
}

func (sm *SyncManager) baiduTokenStatus(refreshed bool, message string) *BaiduTokenRefreshStatus {
	status := &BaiduTokenRefreshStatus{
		Enabled:   sm != nil && sm.baiduClient != nil,
		TokenFile: defaultBaiduTokenPath(),
		Refreshed: refreshed,
		CheckedAt: time.Now(),
		Message:   message,
	}
	if sm == nil || sm.baiduClient == nil {
		return status
	}
	sm.baiduClient.tokenMu.Lock()
	defer sm.baiduClient.tokenMu.Unlock()
	if sm.baiduClient.token == nil {
		return status
	}
	status.HasAccessToken = strings.TrimSpace(sm.baiduClient.token.AccessToken) != ""
	status.HasRefreshToken = strings.TrimSpace(sm.baiduClient.token.RefreshToken) != ""
	status.HasClientID = strings.TrimSpace(sm.baiduClient.token.ClientID) != ""
	status.HasClientSecret = strings.TrimSpace(sm.baiduClient.token.ClientSecret) != ""
	return status
}

// SyncOnStartup 启动时同步（云端版本优先策略）
func (sm *SyncManager) SyncOnStartup() error {
	sm.syncMu.Lock()
	defer sm.syncMu.Unlock()

	if sm.baiduClient == nil {
		return fmt.Errorf("baidu client not initialized")
	}

	sm.updateProgress(func(progress *SyncProgress) {
		progress.Status = "preparing"
		progress.Message = "准备启动时同步..."
		progress.CurrentFile = ""
		progress.Total = 0
		progress.Completed = 0
	})

	log.Println("[Sync] 开始启动时同步...")

	// 获取云端文件列表
	remoteFiles, err := sm.listRemoteSyncFiles()
	if err != nil {
		sm.failProgress(fmt.Sprintf("获取云端文件失败: %v", err))
		return fmt.Errorf("failed to list remote files: %w", err)
	}

	// 获取本地数据库中的文件
	localFiles, err := sm.getLocalDataFiles()
	if err != nil {
		sm.failProgress(fmt.Sprintf("获取本地文件失败: %v", err))
		return fmt.Errorf("failed to get local files: %w", err)
	}

	// 比较并同步
	type syncRemoteDownloadCandidate struct {
		file FileInfo
		key  string
	}
	downloadCandidates := make([]syncRemoteDownloadCandidate, 0, len(remoteFiles))
	for _, remoteFile := range remoteFiles {
		if remoteFile.IsDir {
			continue
		}
		remoteKey, ok := syncDownloadKeyForRemotePath(remoteFile.Path)
		if !ok {
			log.Printf("[Sync] 跳过未知远端文件")
			continue
		}
		downloadCandidates = append(downloadCandidates, syncRemoteDownloadCandidate{
			file: remoteFile,
			key:  remoteKey,
		})
	}

	sm.updateProgress(func(progress *SyncProgress) {
		progress.Total = len(downloadCandidates)
		progress.Completed = 0
	})

	failedDownloads := 0
	firstDownloadErr := ""
	for _, candidate := range downloadCandidates {
		remoteFile := candidate.file
		remoteKey := candidate.key

		_, exists := localFiles[remoteKey]
		if !exists {
			// 云端有，本地没有 → 下载
			sm.updateProgress(func(progress *SyncProgress) {
				progress.CurrentFile = remoteKey
				progress.Status = "downloading"
			})
			log.Printf("[Sync] 下载新文件: %s", remoteKey)

			localPath, err := sm.downloadFile(remoteFile.Path)
			if err != nil {
				failedDownloads++
				if firstDownloadErr == "" {
					firstDownloadErr = fmt.Sprintf("%s: %v", remoteKey, err)
				}
				sm.updateProgress(func(progress *SyncProgress) {
					progress.Message = fmt.Sprintf("下载失败: %v", err)
				})
				record := &SyncRecord{
					Type:         "download",
					FileName:     filepath.Base(remoteFile.Path),
					FileSize:     remoteFile.Size,
					RemotePath:   remoteFile.Path,
					Status:       "failed",
					Message:      "Download from cloud failed",
					ErrorMessage: err.Error(),
				}
				sm.db.SaveSyncRecord(record)
				continue
			}

			// 保存同步记录
			record := &SyncRecord{
				Type:        "download",
				FileName:    filepath.Base(remoteFile.Path),
				FileSize:    remoteFile.Size,
				RemotePath:  remoteFile.Path,
				LocalPath:   localPath,
				Status:      "success",
				Message:     "Downloaded from cloud",
				CompletedAt: time.Now(),
			}
			sm.db.SaveSyncRecord(record)
		}
		sm.updateProgress(func(progress *SyncProgress) {
			progress.Completed++
		})
	}

	if failedDownloads > 0 {
		message := fmt.Sprintf("启动时同步完成，但 %d 个文件下载失败；首个失败: %s", failedDownloads, firstDownloadErr)
		sm.failProgress(message)
		return fmt.Errorf("startup sync completed with %d download failures; first failure: %s", failedDownloads, firstDownloadErr)
	}

	sm.updateProgress(func(progress *SyncProgress) {
		progress.Status = "complete"
		progress.Message = "启动时同步完成"
		progress.CurrentFile = ""
	})
	log.Println("[Sync] 启动时同步完成")
	return nil
}

// SyncToCloud 手动同步到云端
func (sm *SyncManager) SyncToCloud() error {
	sm.syncMu.Lock()
	defer sm.syncMu.Unlock()
	return sm.syncToCloudLocked()
}

func (sm *SyncManager) SyncToCloudIfIdle() error {
	if !sm.syncMu.TryLock() {
		return errSyncAlreadyInProgress
	}
	defer sm.syncMu.Unlock()
	return sm.syncToCloudLocked()
}

func (sm *SyncManager) StartSyncToCloudAsync(onComplete func(error)) (*SyncProgress, error) {
	if !sm.syncMu.TryLock() {
		return sm.GetSyncProgress(), errSyncAlreadyInProgress
	}
	if sm.baiduClient == nil {
		sm.syncMu.Unlock()
		return nil, fmt.Errorf("baidu client not initialized")
	}

	sm.updateProgress(func(progress *SyncProgress) {
		progress.Status = "preparing"
		progress.Message = "准备同步到云端..."
		progress.CurrentFile = ""
		progress.Total = 0
		progress.Completed = 0
	})
	initialProgress := sm.GetSyncProgress()

	go func() {
		err := sm.syncToCloudLocked()
		sm.syncMu.Unlock()
		if onComplete != nil {
			onComplete(err)
		}
	}()

	return initialProgress, nil
}

func (sm *SyncManager) syncToCloudLocked() error {
	if sm.baiduClient == nil {
		return fmt.Errorf("baidu client not initialized")
	}

	sm.updateProgress(func(progress *SyncProgress) {
		progress.Status = "preparing"
		progress.Message = "准备同步到云端..."
		progress.CurrentFile = ""
		progress.Total = 0
		progress.Completed = 0
	})

	log.Println("[Sync] 开始同步到云端...")

	if _, err := sm.baiduClient.GetAccessToken(); err != nil {
		message := fmt.Sprintf("百度云凭据不可用: %v", err)
		sm.failProgress(message)
		return fmt.Errorf("baidu token preflight failed: %w", err)
	}

	localFiles, cleanup, err := sm.prepareSyncSnapshot()
	if err != nil {
		sm.failProgress(fmt.Sprintf("准备本地快照失败: %v", err))
		return fmt.Errorf("failed to get local files: %w", err)
	}
	defer cleanup()

	sm.updateProgress(func(progress *SyncProgress) {
		progress.Total = len(localFiles)
		progress.Completed = 0
	})

	failedUploads := 0
	firstUploadErr := ""
	databaseUploadRecordID := ""
	for _, localFile := range localFiles {
		localPath := localFile.Path
		remotePath := syncRemotePathForKey(localFile.Key)
		sm.updateProgress(func(progress *SyncProgress) {
			progress.CurrentFile = localFile.Key
			progress.Status = "uploading"
		})
		log.Printf("[Sync] 上传文件: %s", localFile.Key)

		if err := sm.baiduClient.UploadFileToPath(localPath, remotePath); err != nil {
			failedUploads++
			if firstUploadErr == "" {
				firstUploadErr = fmt.Sprintf("%s: %v", localFile.Key, err)
			}
			sm.failProgress(fmt.Sprintf("上传失败: %v", err))

			// 保存失败的同步记录
			record := &SyncRecord{
				Type:         "upload",
				FileName:     localFile.Key,
				FileSize:     localFile.Size,
				LocalPath:    localPath,
				RemotePath:   remotePath,
				Status:       "failed",
				Message:      "Upload to cloud failed",
				ErrorMessage: err.Error(),
			}
			sm.db.SaveSyncRecord(record)
			continue
		}

		// 保存成功的同步记录
		record := &SyncRecord{
			Type:        "upload",
			FileName:    localFile.Key,
			FileSize:    localFile.Size,
			LocalPath:   localPath,
			RemotePath:  remotePath,
			Status:      "success",
			Message:     "Uploaded to cloud",
			CompletedAt: time.Now(),
		}
		sm.db.SaveSyncRecord(record)
		if localFile.Key == syncDatabaseKey {
			databaseUploadRecordID = record.ID
		}

		sm.updateProgress(func(progress *SyncProgress) {
			progress.Completed++
		})
	}

	if failedUploads > 0 {
		message := fmt.Sprintf("同步到云端完成，但 %d 个文件失败；首个失败: %s", failedUploads, firstUploadErr)
		sm.failProgress(message)
		return fmt.Errorf("sync to cloud completed with %d upload failures; first failure: %s", failedUploads, firstUploadErr)
	}
	if databaseUploadRecordID != "" {
		if err := sm.db.UpdateSyncRecordCompletedAt(databaseUploadRecordID, time.Now()); err != nil {
			log.Printf("[Sync] failed to finalize database upload record: %s", redactErrorText(err))
		}
	}

	sm.updateProgress(func(progress *SyncProgress) {
		progress.Status = "complete"
		progress.Message = "同步到云端完成"
		progress.CurrentFile = ""
	})
	log.Println("[Sync] 同步到云端完成")
	return nil
}

// GetSyncProgress 获取同步进度
func (sm *SyncManager) GetSyncProgress() *SyncProgress {
	sm.progressMu.RLock()
	defer sm.progressMu.RUnlock()
	if sm.progress == nil {
		return &SyncProgress{Status: "idle"}
	}
	cloned := *sm.progress
	return &cloned
}

func (sm *SyncManager) SetProgressReporter(reporter func(SyncProgress)) {
	sm.progressMu.Lock()
	defer sm.progressMu.Unlock()
	sm.progressCb = reporter
}

// getLocalDataFiles 获取本地数据文件列表
func (sm *SyncManager) getLocalDataFiles() (map[string]FileInfo, error) {
	files := make(map[string]FileInfo)

	dbPath := filepath.Join(sm.config.DataPath, "diveend.db")
	if info, err := os.Stat(dbPath); err == nil && !info.IsDir() {
		files[syncDatabaseKey] = FileInfo{
			Path:     dbPath,
			Size:     info.Size(),
			Modified: info.ModTime(),
			IsDir:    false,
		}
	}

	papers, err := sm.db.GetPapers("")
	if err != nil {
		return nil, err
	}

	for _, paper := range papers {
		pdfPath, ok := syncManagedPaperPDFPath(sm.config.DataPath, paper)
		if !ok {
			continue
		}
		if info, err := os.Stat(pdfPath); err == nil {
			paper.PDFPath = pdfPath
			files[syncKeyForPaper(paper)] = FileInfo{
				Path:     pdfPath,
				Size:     info.Size(),
				Modified: info.ModTime(),
				IsDir:    false,
			}
		}
	}

	return files, nil
}

// downloadFile 下载文件
func (sm *SyncManager) downloadFile(remotePath string) (string, error) {
	key, ok := syncDownloadKeyForRemotePath(remotePath)
	if !ok {
		return "", fmt.Errorf("unsupported sync remote path: %s", remotePath)
	}
	localPath, err := sm.localPathForSyncKey(key)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(key, "papers/") {
		if _, err := os.Lstat(localPath); err == nil {
			return "", fmt.Errorf("refusing to overwrite existing untracked paper download target")
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to inspect paper download target: %w", err)
		}
	}
	if err := sm.downloadSyncKeyToPath(remotePath, key, localPath); err != nil {
		return "", err
	}
	return localPath, nil
}

func (sm *SyncManager) downloadSyncKeyToPath(remotePath, key, targetPath string) error {
	if strings.HasPrefix(key, "papers/") {
		return sm.downloadManagedPaperPDFToPath(remotePath, targetPath)
	}
	if key == syncDatabaseKey {
		if err := sm.baiduClient.DownloadFile(remotePath, targetPath); err != nil {
			return err
		}
		if err := validateSQLiteDatabase(targetPath); err != nil {
			_ = removeFileAndSyncDir(targetPath)
			return fmt.Errorf("downloaded database validation failed: %w", err)
		}
		return nil
	}
	return fmt.Errorf("unsupported sync download key: %s", key)
}

func (sm *SyncManager) downloadManagedPaperPDFToPath(remotePath, targetPath string) error {
	managedTargetPath, err := ensureManagedFileParent(filepath.Join(sm.config.DataPath, "papers"), targetPath)
	if err != nil {
		return err
	}
	targetPath = managedTargetPath
	dir := filepath.Dir(targetPath)
	stagePath := filepath.Join(dir, "."+filepath.Base(targetPath)+".sync-download-"+uuid.NewString()+".pdf")
	defer func() {
		_ = removeFileAndSyncDir(stagePath)
	}()

	if err := sm.baiduClient.DownloadFile(remotePath, stagePath); err != nil {
		return err
	}
	if err := validateLocalPDFFile(stagePath); err != nil {
		return fmt.Errorf("downloaded paper pdf validation failed: %w", err)
	}
	if err := renameFileAtomicWithinDir(stagePath, targetPath); err != nil {
		return fmt.Errorf("failed to replace paper pdf: %w", err)
	}
	return nil
}

// DetectConflicts 检测冲突
func (sm *SyncManager) DetectConflicts() ([]SyncConflict, error) {
	sm.syncMu.Lock()
	defer sm.syncMu.Unlock()

	if sm.baiduClient == nil {
		return nil, fmt.Errorf("baidu client not initialized")
	}

	remoteFiles, err := sm.listRemoteSyncFiles()
	if err != nil {
		return nil, err
	}

	localFiles, err := sm.getLocalDataFiles()
	if err != nil {
		return nil, err
	}

	var conflicts []SyncConflict

	for _, remoteFile := range remoteFiles {
		if remoteFile.IsDir {
			continue
		}

		remoteKey, ok := syncDownloadKeyForRemotePath(remoteFile.Path)
		if !ok {
			log.Printf("[Sync] 跳过未知远端冲突候选")
			continue
		}

		localFile, exists := localFiles[remoteKey]
		if exists {
			// 两边都有文件，比较修改时间
			if remoteFile.Modified.After(localFile.Modified) {
				// 云端更新
				conflicts = append(conflicts, SyncConflict{
					FileName:   filepath.Base(remoteFile.Path),
					FileKind:   syncConflictFileKind(remoteKey),
					LocalPath:  localFile.Path,
					LocalSize:  localFile.Size,
					LocalTime:  localFile.Modified,
					RemotePath: remoteFile.Path,
					RemoteSize: remoteFile.Size,
					RemoteTime: remoteFile.Modified,
					NewerSide:  syncConflictNewerSide(localFile.Modified, remoteFile.Modified),
					Resolution: "", // 待解决
				})
			}
		}
	}

	return conflicts, nil
}

func syncConflictFileKind(remoteKey string) string {
	remoteKey = strings.TrimSpace(remoteKey)
	switch {
	case remoteKey == syncDatabaseKey:
		return "database"
	case strings.HasPrefix(remoteKey, "papers/"):
		return "paper_pdf"
	default:
		return "file"
	}
}

func syncConflictNewerSide(localTime, remoteTime time.Time) string {
	switch {
	case remoteTime.After(localTime):
		return "remote"
	case localTime.After(remoteTime):
		return "local"
	default:
		return "equal"
	}
}

func (sm *SyncManager) ResolveConflict(conflict *SyncConflict, resolution string) error {
	sm.syncMu.Lock()
	defer sm.syncMu.Unlock()

	if sm.baiduClient == nil {
		return fmt.Errorf("baidu client not initialized")
	}
	if conflict == nil {
		return fmt.Errorf("sync conflict is nil")
	}

	remoteKey, ok := syncDownloadKeyForRemotePath(conflict.RemotePath)
	if !ok {
		return fmt.Errorf("unsupported sync conflict remote path: %s", conflict.RemotePath)
	}
	if err := sm.validateSyncConflictLocalPath(conflict, remoteKey); err != nil {
		return err
	}

	resolution = strings.TrimSpace(resolution)
	if resolution == "timestamp" {
		if conflict.RemoteTime.After(conflict.LocalTime) {
			resolution = "remote"
		} else {
			resolution = "local"
		}
	}

	recordType := syncRecordTypeForConflictResolution(resolution)
	recordLocalPath := conflict.LocalPath
	recordStatus := "success"
	recordError := ""
	preservedPath := ""
	switch resolution {
	case "local":
		if conflict.LocalPath == "" {
			err := fmt.Errorf("local path is empty")
			sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, "failed", err.Error(), "")
			return err
		}
		if _, err := os.Stat(conflict.LocalPath); err != nil {
			wrapped := fmt.Errorf("local conflict file is not available: %w", err)
			sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, "failed", wrapped.Error(), "")
			return wrapped
		}
		archivePath, err := sm.archiveRemoteConflictVersion(conflict, remoteKey)
		if err != nil {
			wrapped := fmt.Errorf("failed to preserve cloud conflict version before upload: %w", err)
			sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, "failed", wrapped.Error(), "")
			return wrapped
		}
		preservedPath = archivePath
		if err := sm.baiduClient.UploadFileToPath(conflict.LocalPath, conflict.RemotePath); err != nil {
			recordStatus = "failed"
			recordError = err.Error()
			sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, recordStatus, recordError, preservedPath)
			return err
		}
		sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, recordStatus, recordError, preservedPath)
		return nil
	case "remote":
		archivePath, err := sm.archiveLocalConflictVersion(conflict, remoteKey)
		if err != nil {
			wrapped := fmt.Errorf("failed to preserve local conflict version before download: %w", err)
			sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, "failed", wrapped.Error(), "")
			return wrapped
		}
		preservedPath = archivePath
		if sm.isLiveDatabasePath(conflict.LocalPath) {
			stagedPath, err := sm.stageRemoteDatabase(conflict.RemotePath)
			if err != nil {
				sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, "failed", err.Error(), preservedPath)
				return err
			}
			recordLocalPath = stagedPath
			_, err = scheduleDatabaseRestore(sm.config.DataPath, stagedPath, conflict.RemotePath)
			if err != nil {
				sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, "failed", err.Error(), preservedPath)
				return err
			}
			sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, recordStatus, recordError, preservedPath)
			return nil
		}
		if conflict.LocalPath == "" {
			err := fmt.Errorf("local path is empty")
			sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, "failed", err.Error(), preservedPath)
			return err
		}
		if err := sm.downloadSyncKeyToPath(conflict.RemotePath, remoteKey, conflict.LocalPath); err != nil {
			recordStatus = "failed"
			recordError = err.Error()
			sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, recordStatus, recordError, preservedPath)
			return err
		}
		sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, recordStatus, recordError, preservedPath)
		return nil
	case "skipped":
		sm.saveConflictResolutionRecord(conflict, recordType, recordLocalPath, recordStatus, recordError, "")
		return nil
	default:
		return fmt.Errorf("unsupported sync conflict resolution: %s", resolution)
	}
}

func syncRecordTypeForConflictResolution(resolution string) string {
	switch strings.TrimSpace(resolution) {
	case "local":
		return "upload"
	case "remote":
		return "download"
	default:
		return "conflict"
	}
}

func (sm *SyncManager) saveConflictResolutionRecord(conflict *SyncConflict, recordType, localPath, status, errorMessage, preservedPath string) {
	if sm == nil || sm.db == nil || conflict == nil {
		return
	}
	if strings.TrimSpace(recordType) == "" {
		recordType = "conflict"
	}
	if strings.TrimSpace(status) == "" {
		status = "success"
	}

	fileSize := int64(0)
	if strings.TrimSpace(localPath) != "" {
		if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
			fileSize = info.Size()
		}
	}

	record := &SyncRecord{
		Type:         recordType,
		FileName:     strings.TrimSpace(conflict.FileName),
		FileSize:     fileSize,
		RemotePath:   conflict.RemotePath,
		LocalPath:    localPath,
		Status:       status,
		Message:      syncConflictResolutionMessage(recordType, status, preservedPath),
		ErrorMessage: errorMessage,
	}
	if record.FileName == "" {
		record.FileName = filepath.Base(conflict.RemotePath)
	}
	if status == "success" {
		record.CompletedAt = time.Now()
	}
	if err := sm.db.SaveSyncRecord(record); err != nil {
		log.Printf("[Sync] failed to save conflict resolution record: %s", redactErrorText(err))
	}
}

func syncConflictResolutionMessage(recordType, status, preservedPath string) string {
	suffix := ""
	if strings.TrimSpace(preservedPath) != "" {
		suffix = "；被覆盖版本已保留：" + preservedPath
	}
	if status == "failed" {
		switch recordType {
		case "upload":
			return "Conflict resolution failed while keeping local version" + suffix
		case "download":
			return "Conflict resolution failed while keeping cloud version" + suffix
		default:
			return "Conflict resolution failed" + suffix
		}
	}
	switch recordType {
	case "upload":
		return "Conflict resolved by keeping local version" + suffix
	case "download":
		return "Conflict resolved by keeping cloud version" + suffix
	default:
		return "Conflict marked as skipped"
	}
}

func syncConflictArchiveRoot(dataPath string) string {
	dataPath = normalizeDataPath(dataPath, defaultAppConfig().DataPath)
	return filepath.Join(dataPath, syncConflictArchiveName)
}

func (sm *SyncManager) conflictArchivePath(conflict *SyncConflict, remoteKey, side string) (string, error) {
	if sm == nil {
		return "", fmt.Errorf("sync manager is nil")
	}
	side = strings.TrimSpace(side)
	if side != "local" && side != "remote" {
		return "", fmt.Errorf("unsupported conflict archive side: %s", side)
	}
	remoteKey, ok := validateDownloadSyncKey(remoteKey)
	if !ok {
		return "", fmt.Errorf("unsupported sync key: %s", remoteKey)
	}
	root := syncConflictArchiveRoot(sm.config.DataPath)
	stamp := time.Now().UTC().Format("20060102-150405")
	id := uuid.NewString()
	targetPath := filepath.Join(root, stamp+"-"+id, side, filepath.FromSlash(remoteKey))
	return ensureManagedFileParent(root, targetPath)
}

func (sm *SyncManager) archiveLocalConflictVersion(conflict *SyncConflict, remoteKey string) (string, error) {
	if conflict == nil {
		return "", fmt.Errorf("sync conflict is nil")
	}
	sourcePath := strings.TrimSpace(conflict.LocalPath)
	if sourcePath == "" {
		return "", fmt.Errorf("local path is empty")
	}
	targetPath, err := sm.conflictArchivePath(conflict, remoteKey, "local")
	if err != nil {
		return "", err
	}
	if err := copyFileAtomic(sourcePath, targetPath, 0600); err != nil {
		return "", err
	}
	if err := validateConflictArchiveFile(remoteKey, targetPath); err != nil {
		_ = removeFileAndSyncDir(targetPath)
		return "", err
	}
	return targetPath, nil
}

func (sm *SyncManager) archiveRemoteConflictVersion(conflict *SyncConflict, remoteKey string) (string, error) {
	if conflict == nil {
		return "", fmt.Errorf("sync conflict is nil")
	}
	targetPath, err := sm.conflictArchivePath(conflict, remoteKey, "remote")
	if err != nil {
		return "", err
	}
	if err := sm.baiduClient.DownloadFile(conflict.RemotePath, targetPath); err != nil {
		return "", err
	}
	if err := validateConflictArchiveFile(remoteKey, targetPath); err != nil {
		_ = removeFileAndSyncDir(targetPath)
		return "", err
	}
	return targetPath, nil
}

func validateConflictArchiveFile(remoteKey, filePath string) error {
	switch {
	case remoteKey == syncDatabaseKey:
		return validateSQLiteDatabase(filePath)
	case strings.HasPrefix(remoteKey, "papers/"):
		return validateLocalPDFFile(filePath)
	default:
		return fmt.Errorf("unsupported conflict archive key: %s", remoteKey)
	}
}

func (sm *SyncManager) prepareSyncSnapshot() ([]syncLocalFile, func(), error) {
	stagingRoot := filepath.Join(sm.config.DataPath, ".sync-staging")
	stagingDir := filepath.Join(stagingRoot, uuid.NewString())
	cleanup := func() {
		_ = removeDirectChildIfPlainRoot(stagingRoot, stagingDir)
	}

	dbSnapshotPath := filepath.Join(stagingDir, filepath.FromSlash(syncDatabaseKey))
	var err error
	dbSnapshotPath, err = ensureManagedFileParent(stagingRoot, dbSnapshotPath)
	if err != nil {
		cleanup()
		return nil, cleanup, err
	}
	if err := sm.snapshotDatabase(dbSnapshotPath); err != nil {
		cleanup()
		return nil, cleanup, err
	}

	files := []syncLocalFile{}
	if info, err := os.Stat(dbSnapshotPath); err == nil && !info.IsDir() {
		files = append(files, syncLocalFile{
			Key:      syncDatabaseKey,
			Path:     dbSnapshotPath,
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}

	papers, err := sm.db.GetPapers("")
	if err != nil {
		cleanup()
		return nil, cleanup, err
	}
	for _, paper := range papers {
		pdfPath, ok := syncManagedPaperPDFPath(sm.config.DataPath, paper)
		if !ok {
			continue
		}
		info, err := os.Stat(pdfPath)
		if err != nil || info.IsDir() {
			continue
		}
		paper.PDFPath = pdfPath
		files = append(files, syncLocalFile{
			Key:      syncKeyForPaper(paper),
			Path:     pdfPath,
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}

	manifest := syncManifest{
		Version:   1,
		App:       "DiveEnd",
		CreatedAt: time.Now(),
		Files:     make([]syncManifestFile, 0, len(files)),
	}
	for i := range files {
		hash, err := fileSHA256(files[i].Path)
		if err != nil {
			cleanup()
			return nil, cleanup, err
		}
		files[i].SHA256 = hash
		manifest.Files = append(manifest.Files, syncManifestFile{
			Key:      files[i].Key,
			Size:     files[i].Size,
			Modified: files[i].Modified,
			SHA256:   hash,
		})
	}

	manifestPath := filepath.Join(stagingDir, syncManifestFileName)
	manifestPath, err = ensureManagedFileParent(stagingRoot, manifestPath)
	if err != nil {
		cleanup()
		return nil, cleanup, err
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		cleanup()
		return nil, cleanup, err
	}
	if err := writeFileAtomic(manifestPath, manifestData, 0600); err != nil {
		cleanup()
		return nil, cleanup, err
	}
	if info, err := os.Stat(manifestPath); err == nil && !info.IsDir() {
		files = append(files, syncLocalFile{
			Key:      syncManifestFileName,
			Path:     manifestPath,
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}

	return files, cleanup, nil
}

func (sm *SyncManager) snapshotDatabase(destPath string) error {
	tmp, err := newAtomicTempFile(destPath, 0600)
	if err != nil {
		return err
	}
	defer tmp.cleanup()
	if err := tmp.file.Chmod(0600); err != nil {
		return fmt.Errorf("failed to secure sqlite snapshot temp file: %w", err)
	}
	if err := tmp.file.Close(); err != nil {
		tmp.file = nil
		return fmt.Errorf("failed to close sqlite snapshot temp file: %w", err)
	}
	tmp.file = nil

	if err := backupSQLiteDatabase(context.Background(), sm.db.conn, tmp.tmpPath); err != nil {
		return fmt.Errorf("failed to create sqlite snapshot: %w", err)
	}
	if err := tmp.syncExistingTempFile(0600); err != nil {
		return fmt.Errorf("failed to sync sqlite snapshot: %w", err)
	}
	if err := tmp.commit(); err != nil {
		return fmt.Errorf("failed to move sqlite snapshot into place: %w", err)
	}
	return nil
}

func backupSQLiteDatabase(ctx context.Context, sourceDB *sql.DB, destPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if sourceDB == nil {
		return fmt.Errorf("source database cannot be nil")
	}
	destDB, err := sql.Open("sqlite3", destPath)
	if err != nil {
		return err
	}
	defer destDB.Close()

	sourceConn, err := sourceDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer sourceConn.Close()
	destConn, err := destDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer destConn.Close()

	if err := destConn.Raw(func(destDriverConn interface{}) error {
		destSQLiteConn, ok := destDriverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("destination database is not a sqlite connection")
		}
		return sourceConn.Raw(func(sourceDriverConn interface{}) error {
			sourceSQLiteConn, ok := sourceDriverConn.(*sqlite3.SQLiteConn)
			if !ok {
				return fmt.Errorf("source database is not a sqlite connection")
			}
			backup, err := destSQLiteConn.Backup("main", sourceSQLiteConn, "main")
			if err != nil {
				return err
			}
			defer backup.Close()
			for {
				done, err := backup.Step(256)
				if err != nil {
					return err
				}
				if done {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}
		})
	}); err != nil {
		return err
	}

	var result string
	if err := destConn.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return err
	}
	if strings.TrimSpace(strings.ToLower(result)) != "ok" {
		return fmt.Errorf("quick_check returned %q", result)
	}
	return nil
}

func syncManagedPaperPDFPath(dataPath string, paper Paper) (string, bool) {
	pdfPath, err := resolveManagedDeepReadPDFPath(dataPath, paper.PDFPath)
	if err != nil {
		return "", false
	}
	return pdfPath, true
}

func (sm *SyncManager) listRemoteSyncFiles() ([]FileInfo, error) {
	files, err := sm.baiduClient.ListFilesRecursive(syncRemoteRootPath())
	if err != nil {
		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "errno:-9") ||
			strings.Contains(errText, "errno -9") ||
			strings.Contains(errText, "not exist") ||
			strings.Contains(errText, "no such") {
			return []FileInfo{}, nil
		}
		return nil, err
	}
	return files, nil
}

func (sm *SyncManager) localPathForSyncKey(key string) (string, error) {
	key, ok := validateDownloadSyncKey(key)
	if !ok {
		return "", fmt.Errorf("unsupported sync key: %s", key)
	}
	if key == syncDatabaseKey {
		name := fmt.Sprintf("diveend-remote-%s.db", time.Now().Format("20060102-150405"))
		return filepath.Join(sm.config.DataPath, ".sync-inbox", name), nil
	}
	return filepath.Join(sm.config.DataPath, filepath.FromSlash(key)), nil
}

func (sm *SyncManager) isLiveDatabasePath(localPath string) bool {
	return sameExistingLocalPath(filepath.Join(sm.config.DataPath, "diveend.db"), localPath)
}

func (sm *SyncManager) validateSyncConflictLocalPath(conflict *SyncConflict, remoteKey string) error {
	if conflict.LocalPath == "" {
		return fmt.Errorf("local path is empty")
	}
	if remoteKey == syncDatabaseKey {
		if !sm.isLiveDatabasePath(conflict.LocalPath) {
			return fmt.Errorf("database sync conflict local path must be the live database path")
		}
		return nil
	}
	if strings.HasPrefix(remoteKey, "papers/") {
		if _, ok := managedPathInsideRoot(filepath.Join(sm.config.DataPath, "papers"), conflict.LocalPath); !ok {
			return fmt.Errorf("paper sync conflict local path must be inside managed papers directory")
		}
		localFiles, err := sm.getLocalDataFiles()
		if err != nil {
			return fmt.Errorf("failed to validate paper sync conflict metadata: %w", err)
		}
		localFile, ok := localFiles[remoteKey]
		if !ok {
			return fmt.Errorf("paper sync conflict no longer matches managed paper metadata")
		}
		if !sameExistingLocalPath(localFile.Path, conflict.LocalPath) {
			return fmt.Errorf("paper sync conflict local path does not match managed paper metadata")
		}
		return nil
	}
	return fmt.Errorf("unsupported sync conflict key: %s", remoteKey)
}

func sameExistingLocalPath(leftPath, rightPath string) bool {
	left, err := filepath.Abs(filepath.Clean(leftPath))
	if err != nil {
		return false
	}
	right, err := filepath.Abs(filepath.Clean(rightPath))
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(left); err == nil {
		left = filepath.Clean(resolved)
	}
	if resolved, err := filepath.EvalSymlinks(right); err == nil {
		right = filepath.Clean(resolved)
	}
	return left == right
}

func (sm *SyncManager) stageRemoteDatabase(remotePath string) (string, error) {
	if err := ensureDatabaseRestoreDir(sm.config.DataPath); err != nil {
		return "", err
	}
	localPath := filepath.Join(databaseRestoreDir(sm.config.DataPath), "diveend-remote-"+uuid.NewString()+".db")
	if err := sm.baiduClient.DownloadFile(remotePath, localPath); err != nil {
		return "", err
	}
	return localPath, nil
}

func (sm *SyncManager) updateProgress(update func(*SyncProgress)) {
	sm.progressMu.Lock()
	if sm.progress == nil {
		sm.progress = &SyncProgress{Status: "idle"}
	}
	update(sm.progress)
	snapshot := *sm.progress
	reporter := sm.progressCb
	sm.progressMu.Unlock()

	if reporter != nil {
		reporter(snapshot)
	}
}

func (sm *SyncManager) failProgress(message string) {
	sm.updateProgress(func(progress *SyncProgress) {
		progress.Status = "error"
		progress.Message = message
	})
}

func syncRemoteRootPath() string {
	return path.Join("/apps", syncApp, syncDataRootName)
}

func syncRemotePathForKey(key string) string {
	return path.Join(syncRemoteRootPath(), cleanSyncKey(key))
}

func syncKeyForPaper(paper Paper) string {
	fileName := filepath.Base(strings.TrimSpace(paper.PDFPath))
	if fileName == "." || fileName == string(filepath.Separator) || fileName == "" {
		fileName = safePaperPDFFileName(paper.ID)
	}
	return path.Join("papers", safeSyncSegment(paper.ID), fileName)
}

func cleanSyncKey(key string) string {
	key = strings.TrimSpace(filepath.ToSlash(key))
	key = strings.TrimPrefix(path.Clean("/"+key), "/")
	if key == "." {
		return ""
	}
	return key
}

func validateSyncKey(key string) (string, bool) {
	key = cleanSyncKey(key)
	if key == syncDatabaseKey || key == syncManifestFileName {
		return key, true
	}
	parts := strings.Split(key, "/")
	if len(parts) == 3 && parts[0] == "papers" && parts[1] != "" && parts[2] != "" && strings.EqualFold(path.Ext(parts[2]), ".pdf") {
		return key, true
	}
	return "", false
}

func validateDownloadSyncKey(key string) (string, bool) {
	key, ok := validateSyncKey(key)
	if !ok || key == syncManifestFileName {
		return "", false
	}
	return key, true
}

func syncDownloadKeyForRemotePath(remotePath string) (string, bool) {
	remotePath = strings.TrimSpace(filepath.ToSlash(remotePath))
	root := syncRemoteRootPath()
	prefix := strings.TrimRight(root, "/") + "/"
	if !strings.HasPrefix(remotePath, prefix) {
		return "", false
	}
	rawKey := strings.TrimPrefix(remotePath, prefix)
	rawKey = strings.Trim(strings.TrimSpace(filepath.ToSlash(rawKey)), "/")
	cleaned := cleanSyncKey(rawKey)
	if cleaned != rawKey {
		return "", false
	}
	return validateDownloadSyncKey(cleaned)
}

func fileSHA256(localPath string) (string, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func safeSyncSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}

func safePaperPDFFileName(paperID string) string {
	return safeSyncSegment(paperID) + ".pdf"
}
