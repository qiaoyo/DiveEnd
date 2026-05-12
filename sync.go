package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const (
	// The OAuth app name must match the Baidu app folder name used by the proven
	// token/upload script in this repo.
	syncApp = "pcstest_oauth"
)

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
			error_message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS sync_conflicts (
			id TEXT PRIMARY KEY,
			file_name TEXT NOT NULL,
			local_path TEXT NOT NULL,
			local_time DATETIME NOT NULL,
			remote_path TEXT NOT NULL,
			remote_time DATETIME NOT NULL,
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

	return nil
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
				status, error_message, created_at, completed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
		record.ID,
		record.Type,
		record.FileName,
		record.FileSize,
		nullIfBlank(record.RemotePath),
		nullIfBlank(record.LocalPath),
		record.Status,
		nullIfBlank(record.ErrorMessage),
		record.CreatedAt,
		nullIfTime(record.CompletedAt),
	)

	return err
}

// GetSyncRecords 获取同步记录
func (db *DB) GetSyncRecords(limit int) ([]SyncRecord, error) {
	query := `
			SELECT id, type, file_name, file_size, remote_path, local_path,
			       status, error_message, created_at, completed_at
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
		var remotePath, localPath, errorMessage sql.NullString

		if err := rows.Scan(
			&record.ID,
			&record.Type,
			&record.FileName,
			&record.FileSize,
			&remotePath,
			&localPath,
			&record.Status,
			&errorMessage,
			&record.CreatedAt,
			&completedAt,
		); err != nil {
			return nil, err
		}

		record.RemotePath = remotePath.String
		record.LocalPath = localPath.String
		record.ErrorMessage = errorMessage.String
		record.CompletedAt = completedAt.Time

		records = append(records, record)
	}

	return records, rows.Err()
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
				id, file_name, local_path, local_time, remote_path,
				remote_time, resolution, resolved_at, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
		conflict.ID,
		conflict.FileName,
		conflict.LocalPath,
		conflict.LocalTime,
		conflict.RemotePath,
		conflict.RemoteTime,
		nullIfBlank(conflict.Resolution),
		nullIfTime(conflict.ResolvedAt),
		conflict.CreatedAt,
	)

	return err
}

// GetSyncConflicts 获取冲突列表
func (db *DB) GetSyncConflicts(limit int) ([]SyncConflict, error) {
	query := `
			SELECT id, file_name, local_path, local_time, remote_path,
			       remote_time, resolution, resolved_at, created_at
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
		var resolution sql.NullString
		var resolvedAt sql.NullTime

		if err := rows.Scan(
			&conflict.ID,
			&conflict.FileName,
			&conflict.LocalPath,
			&conflict.LocalTime,
			&conflict.RemotePath,
			&conflict.RemoteTime,
			&resolution,
			&resolvedAt,
			&conflict.CreatedAt,
		); err != nil {
			return nil, err
		}

		conflict.Resolution = resolution.String
		conflict.ResolvedAt = resolvedAt.Time

		conflicts = append(conflicts, conflict)
	}

	return conflicts, rows.Err()
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
	progress    *SyncProgress
}

// NewSyncManager 创建同步管理器
func NewSyncManager(db *DB, config AppConfig) *SyncManager {
	var baiduClient *BaiduPCSClient
	if config.BaiduCloud.Enabled && config.BaiduCloud.Token != "" {
		token := &BaiduToken{
			AccessToken:  config.BaiduCloud.Token,
			RefreshToken: config.BaiduCloud.RefreshToken,
			ClientID:     config.BaiduCloud.ClientID,
			ClientSecret: config.BaiduCloud.ClientSecret,
		}
		baiduClient = NewBaiduPCSClientWithTokenPath(token, defaultBaiduTokenPath())
	}

	return &SyncManager{
		db:          db,
		baiduClient: baiduClient,
		config:      config,
		progress:    &SyncProgress{Status: "idle"},
	}
}

// SyncOnStartup 启动时同步（云端版本优先策略）
func (sm *SyncManager) SyncOnStartup() error {
	if sm.baiduClient == nil {
		return fmt.Errorf("baidu client not initialized")
	}

	sm.progress.Status = "preparing"
	sm.progress.Message = "准备启动时同步..."

	fmt.Println("[Sync] 开始启动时同步...")

	// 获取云端文件列表
	remotePath := fmt.Sprintf("/apps/%s", syncApp)
	remoteFiles, err := sm.baiduClient.ListFiles(remotePath)
	if err != nil {
		sm.progress.Status = "error"
		sm.progress.Message = fmt.Sprintf("获取云端文件失败: %v", err)
		return fmt.Errorf("failed to list remote files: %w", err)
	}

	// 获取本地数据库中的文件
	localFiles, err := sm.getLocalDataFiles()
	if err != nil {
		sm.progress.Status = "error"
		sm.progress.Message = fmt.Sprintf("获取本地文件失败: %v", err)
		return fmt.Errorf("failed to get local files: %w", err)
	}

	sm.progress.Total = len(localFiles)
	sm.progress.Completed = 0

	// 比较并同步
	for _, remoteFile := range remoteFiles {
		if remoteFile.IsDir {
			continue
		}

		_, exists := localFiles[localSyncKeyForRemote(remoteFile.Path)]
		if !exists {
			// 云端有，本地没有 → 下载
			sm.progress.CurrentFile = remoteFile.Path
			sm.progress.Status = "downloading"
			fmt.Printf("[Sync] 下载新文件: %s\n", remoteFile.Path)

			if err := sm.downloadFile(remoteFile.Path); err != nil {
				sm.progress.Message = fmt.Sprintf("下载失败: %v", err)
				continue
			}

			// 保存同步记录
			record := &SyncRecord{
				Type:        "download",
				FileName:    filepath.Base(remoteFile.Path),
				FileSize:    remoteFile.Size,
				RemotePath:  remoteFile.Path,
				LocalPath:   filepath.Join(sm.config.DataPath, filepath.Base(remoteFile.Path)),
				Status:      "success",
				CompletedAt: time.Now(),
			}
			sm.db.SaveSyncRecord(record)
		}
		sm.progress.Completed++
	}

	sm.progress.Status = "complete"
	sm.progress.Message = "启动时同步完成"
	fmt.Println("[Sync] 启动时同步完成")
	return nil
}

// SyncToCloud 手动同步到云端
func (sm *SyncManager) SyncToCloud() error {
	if sm.baiduClient == nil {
		return fmt.Errorf("baidu client not initialized")
	}

	sm.progress.Status = "preparing"
	sm.progress.Message = "准备同步到云端..."

	fmt.Println("[Sync] 开始同步到云端...")

	localFiles, err := sm.getLocalDataFiles()
	if err != nil {
		sm.progress.Status = "error"
		sm.progress.Message = fmt.Sprintf("获取本地文件失败: %v", err)
		return fmt.Errorf("failed to get local files: %w", err)
	}

	sm.progress.Total = len(localFiles)
	sm.progress.Completed = 0

	for _, localFile := range localFiles {
		localPath := localFile.Path
		sm.progress.CurrentFile = localPath
		sm.progress.Status = "uploading"
		fmt.Printf("[Sync] 上传文件: %s\n", localPath)

		if err := sm.baiduClient.UploadFile(localPath, ""); err != nil {
			sm.progress.Status = "error"
			sm.progress.Message = fmt.Sprintf("上传失败: %v", err)

			// 保存失败的同步记录
			record := &SyncRecord{
				Type:         "upload",
				FileName:     filepath.Base(localPath),
				FileSize:     localFile.Size,
				LocalPath:    localPath,
				RemotePath:   fmt.Sprintf("/apps/%s/%s", syncApp, filepath.Base(localPath)),
				Status:       "failed",
				ErrorMessage: err.Error(),
			}
			sm.db.SaveSyncRecord(record)
			continue
		}

		// 保存成功的同步记录
		record := &SyncRecord{
			Type:        "upload",
			FileName:    filepath.Base(localPath),
			FileSize:    localFile.Size,
			LocalPath:   localPath,
			RemotePath:  fmt.Sprintf("/apps/%s/%s", syncApp, filepath.Base(localPath)),
			Status:      "success",
			CompletedAt: time.Now(),
		}
		sm.db.SaveSyncRecord(record)

		sm.progress.Completed++
	}

	sm.progress.Status = "complete"
	sm.progress.Message = "同步到云端完成"
	fmt.Println("[Sync] 同步到云端完成")
	return nil
}

// GetSyncProgress 获取同步进度
func (sm *SyncManager) GetSyncProgress() *SyncProgress {
	return sm.progress
}

// getLocalDataFiles 获取本地数据文件列表
func (sm *SyncManager) getLocalDataFiles() (map[string]FileInfo, error) {
	files := make(map[string]FileInfo)

	dbPath := filepath.Join(sm.config.DataPath, "diveend.db")
	if info, err := os.Stat(dbPath); err == nil && !info.IsDir() {
		files[localSyncKeyForLocal(dbPath)] = FileInfo{
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
		if paper.PDFPath != "" {
			if info, err := os.Stat(paper.PDFPath); err == nil {
				files[localSyncKeyForLocal(paper.PDFPath)] = FileInfo{
					Path:     paper.PDFPath,
					Size:     info.Size(),
					Modified: info.ModTime(),
					IsDir:    false,
				}
			}
		}
	}

	return files, nil
}

// downloadFile 下载文件
func (sm *SyncManager) downloadFile(remotePath string) error {
	localPath := filepath.Join(sm.config.DataPath, filepath.Base(remotePath))
	return sm.baiduClient.DownloadFile(remotePath, localPath)
}

// DetectConflicts 检测冲突
func (sm *SyncManager) DetectConflicts() ([]SyncConflict, error) {
	if sm.baiduClient == nil {
		return nil, fmt.Errorf("baidu client not initialized")
	}

	remotePath := fmt.Sprintf("/apps/%s", syncApp)
	remoteFiles, err := sm.baiduClient.ListFiles(remotePath)
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

		localFile, exists := localFiles[localSyncKeyForRemote(remoteFile.Path)]
		if exists {
			// 两边都有文件，比较修改时间
			if remoteFile.Modified.After(localFile.Modified) {
				// 云端更新
				conflicts = append(conflicts, SyncConflict{
					FileName:   filepath.Base(remoteFile.Path),
					LocalPath:  localFile.Path,
					LocalTime:  localFile.Modified,
					RemotePath: remoteFile.Path,
					RemoteTime: remoteFile.Modified,
					Resolution: "", // 待解决
				})
			}
		}
	}

	return conflicts, nil
}

func localSyncKeyForLocal(path string) string {
	return filepath.Base(path)
}

func localSyncKeyForRemote(path string) string {
	return filepath.Base(path)
}
