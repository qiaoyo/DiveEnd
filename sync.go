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
	syncApp = "diveend"
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
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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
func (db *) GetSyncRecords(limit int) ([]SyncRecord, error) {
	query := `
		SELECT id, type, file_name, file_size, remote_path, local_path,
		       status, error_message, created_at, completed_at
		FROM sync_records
		ORDER BY created_at DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

.rows, err := db.conn.Query(query)
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

// SyncManager 同步管理器
type SyncManager struct {
	db          *DB
	baiduClient *BaiduPCSClient
	config      AppConfig
}

// NewSyncManager 创建同步管理器
func NewSyncManager(db *DB, config AppConfig) *SyncManager {
	var baiduClient *BaiduPCSClient
	if config.BaiduCloud.Enabled && config.BaiduCloud.Token != "" {
		token := &BaiduToken{
			AccessToken:  config.BaiduCloud.Token,
			RefreshToken: config.BaiduCloud.Token,
			ClientID:     "",
			ClientSecret:  "",
		}
		baiduClient = NewBaiduPCSClient(token)
	}

	return &SyncManager{
		db:          db,
		baiduClient: baiduClient,
		config:      config,
	}
}

// SyncOnStartup 启动时同步（云端版本优先策略）
func (sm *SyncManager) SyncOnStartup() error {
	if sm.baiduClient == nil {
		return fmt.Errorf("baidu client not initialized")
	}

	fmt.Println("[Sync] 开始启动时同步...")

	// 获取云端文件列表
	remotePath := fmt.Sprintf("/apps/%s", syncApp)
	remoteFiles, err := sm.baiduClient.ListFiles(remotePath)
	if err != nil {
		return fmt.Errorf("failed to list remote files: %w", err)
	}

	// 获取本地数据库中的文件
	localFiles, err := sm.getLocalDataFiles()
	if err != nil {
		return fmt.Errorf("failed to get local files: %w", err)
	}

	// 比较并同步
	for _, remoteFile := range remoteFiles {
		if remoteFile.IsDir {
			continue
		}

		_, exists := localFiles[remoteFile.Path]
		if !exists {
			// 云端有，本地没有 → 下载
			fmt.Printf("[Sync] 下载新文件: %s\n", remoteFile.Path)
			sm.downloadFile(remoteFile.Path)
		}
	}

	fmt.Println("[Sync] 启动时同步完成")
	return nil
}

// SyncToCloud 手动同步到云端
func (sm *SyncManager) SyncToCloud() error {
	if sm.baiduClient == nil {
		return fmt.Errorf("baidu client not initialized")
	}

	fmt.Println("[Sync] 开始同步到云端...")

	localFiles, err := sm.getLocalDataFiles()
	if err != nil {
		return fmt.Errorf("failed to get local files: %w", err)
	}

	for localPath := range localFiles {
		fmt.Printf("[Sync] 上传文件: %s\n", localPath)
		sm.baiduClient.UploadFile(localPath, "")
	}

	fmt.Println("[Sync] 同步到云端完成")
	return nil
}

// getLocalDataFiles 获取本地数据文件列表
func (sm *SyncManager) getLocalDataFiles() (map[string]FileInfo, error) {
	files := make(map[string]FileInfo)

	papers, err := sm.db.GetPapers("")
	if err != nil {
		return nil, err
	}

	for _, paper := range papers {
		if paper.PDFPath != "" {
			if info, err := os.Stat(paper.PDFPath); err == nil {
				files[paper.PDFPath] = FileInfo{
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
