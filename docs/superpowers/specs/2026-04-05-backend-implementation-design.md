# DiveEnd Backend Implementation Design (Phase 5 & 6)

> **Document Version**: v1.0
> **Date**: 2026-04-05
> **Status**: Ready for Implementation

---

## 1. Overview

本文档描述 DiveEnd 项目中 **Phase 5: Screening Pipeline** 和 **Phase 6: Baidu Cloud Sync** 的后端实现设计。

前端 UI 已完成（`frontend/src/pages/Screening.tsx` 和 `Sync.tsx`），需要实现对应的后端 Go 方法和数据库支持。

---

## 2. Phase 5: Screening Pipeline 后端设计

### 2.1 数据模型

```go
// ScreeningPaper 论文筛选过程中的论文
type ScreeningPaper struct {
    ID           string    `json:"id"`
    SessionID    string    `json:"sessionId"`
    FileName     string    `json:"fileName"`
    FilePath     string    `json:"filePath"`
    FileSize     int64     `json:"fileSize"`
    Status       string    `json:"status"` // "pending", "extracting", "extracted", "screening", "selected", "rejected"
    
    // 提取的内容
    Title        string    `json:"title,omitempty"`
    Authors      string    `json:"authors,omitempty"`
    Abstract     string    `json:"abstract,omitempty"`
    FullText     string    `json:"fullText,omitempty"`
    SectionsJSON string    `json:"sectionsJson,omitempty"` // JSON string of sections
    
    // 筛选结果
    Selection    string    `json:"selection,omitempty"` // "selected" or "rejected"
    Reason       string    `json:"reason,omitempty"`
    TargetFolderID string    `json:"targetFolderId,omitempty"`
    
    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
}

// ScreeningSession 论文筛选会话
type ScreeningSession struct {
    ID           string    `json:"id"`
    Title         string    `json:"title"`
    Status        string    `json:"status"` // "upload", "extract", "screen", "complete"
    TotalPapers  int       `json:"totalPapers"`
    
    // 当前决策树状态
    CurrentNodeJSON string   `json:"currentNodeJson,omitempty"`
    SelectedOptionsJSON string `json:"selectedOptionsJson,omitempty"` // JSON array
    PathHistoryJSON    string `json:"pathHistoryJson,omitempty"` // JSON array of {dimension, choice}
    
    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
}

// ScreeningDecisionNode 决策树节点
type ScreeningDecisionNode struct {
    ID               string    `json:"id"`
    Message          string    `json:"message"`
    Dimension        string    `json:"dimension"`
    OptionsJSON     string    `json:"optionsJson"` // JSON array of {key, label, paperIds, count}
    AllowMultiSelect  bool      `json:"allowMultiSelect"`
    AllowSkip       bool      `json:"allowSkip"`
    RemainingPaperIDsJSON string `json:"remainingPaperIdsJson,omitempty"`
}
```

### 2.2 数据库表

```sql
CREATE TABLE IF NOT EXISTS screening_sessions (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'upload',
    total_papers INTEGER DEFAULT 0,
    current_node_json TEXT,
    selected_options_json TEXT NOT NULL DEFAULT '[]',
    path_history_json TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS screening_papers (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT,
    file_size INTEGER,
    status TEXT NOT NULL DEFAULT 'pending',
    title TEXT,
    authors TEXT,
    abstract TEXT,
    full_text TEXT,
    sections_json TEXT,
    selection TEXT,
    reason TEXT,
    target_folder_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(session_id) REFERENCES screening_sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_screening_papers_session_id ON screening_papers(session_id);
CREATE INDEX IF NOT EXISTS idx_screening_papers_status ON screening_papers(status);
```

### 2.3 Wails 方法签名

```go
// App 方法
type App struct {
    // ... existing fields
    pdfService *PDFServiceClient  // PDF 服务客户端
}

// 文件上传（接收文件路径）
func (a *App) UploadScreeningFiles(sessionID string, filePaths []string) (*ScreeningSession, error)

// 提取论文内容（调用 Marker 服务）
func (a *App) ExtractPaperContent(sessionID string) (*ExtractProgress, error)

// 获取提取进度
func (a *App) GetExtractProgress(sessionID string) (*ExtractProgress, error)

// 分析论文（生成决策树）
func (a *App) AnalyzePapers(sessionID string) (*ScreeningDecisionNode, error)

// 应用用户选择
func (a *App) ApplyScreeningChoice(sessionID string, selectedOptions []string) (*ScreeningDecisionNode, error)

// 完成筛选（保存选中的论文到 papers 表）
func (a *App) CompleteScreening(sessionID string, targetFolderID string) ([]Paper, error)

// 获取会话列表
func (a *App) ListScreeningSessions() ([]ScreeningSession, error)

// 获取会话详情
func (a *App) GetScreeningSession(sessionID string) (*ScreeningSessionDetail, error)

// 取消会话
func (a *App) CancelScreening(sessionID string) error
```

### 2.4 辅助结构

```go
type ExtractProgress struct {
    SessionID      string    `json:"sessionId"`
    Total          int       `json:"total"`
    Completed      int       `json:"completed"`
    Current        string    `json:"current"`
    Status         string    `json:"status"` // "processing", "completed", "error"
}

type ScreeningSessionDetail struct {
    Session       ScreeningSession        `json:"session"`
    Papers        []ScreeningPaper       `json:"papers"`
    CurrentNode   *ScreeningDecisionNode `json:"currentNode,omitempty"`
    PathHistory    []PathHistoryItem      `json:"pathHistory"`
}

type PathHistoryItem struct {
    Dimension string   `json:"dimension"`
    Choice   string   `json:"choice"`
}
```

---

## 3. Phase 6: Baidu Cloud Sync 后端设计

### 3.1 设计原则

**云端版本优先策略**：
- 软件启动时，先从云端同步内容到本地
- 任何设备启动软件，看到的内容一致
- 使用时间戳比较解决冲突（总是保留最新的）

### 3.2 数据模型

```go
// SyncRecord 同步记录
type SyncRecord struct {
    ID           string    `json:"id"`
    Type         string    `json:"type"` // "upload", "download", "conflict"
    FileName     string    `json:"fileName"`
    FileSize     int64     `json:"fileSize"`
    RemotePath   string    `json:"remotePath"`
    LocalPath    string    `json:"localPath"`
    Status       string    `json:"status"` // "pending", "success", "failed"
    ErrorMessage string    `json:"errorMessage,omitempty"`
    CreatedAt    time.Time `json:"createdAt"`
    CompletedAt time.Time `json:"completedAt,omitempty"`
}

// SyncConflict 同步冲突
type SyncConflict struct {
    ID           string    `json:"id"`
    FileName     string    `json:"fileName"`
    LocalPath    string    `json:"localPath"`
    LocalTime    time.Time `json:"localTime"`
    RemotePath   string    `json:"remotePath"`
    RemoteTime   time.Time `json:"remoteTime"`
    Resolution   string    `json:"resolution"` // "local", "remote", "skipped"
    ResolvedAt   time.Time `json:"resolvedAt,omitempty"`
    CreatedAt    time.Time `json:"createdAt"`
}

// SyncStatus 同步状态
type SyncStatus struct {
    Enabled         bool       `json:"enabled"`
    Provider        string     `json:"provider"`
    LastSync        *time.Time  `json:"lastSync,omitempty"`
    SyncInProgress  bool       `json:"syncInProgress"`
    PendingFiles   int        `json:"pendingFiles"`
    Conflicts      int        `json:"conflicts"`
    TotalSynced    int        `json:"totalSynced"`
    TotalFailed    int        `json:"totalFailed"`
}

// SyncSettings 同步设置
type SyncSettings struct {
    AutoSync         bool   `json:"autoSync"`
    SyncOnStartup   bool   `json:"syncOnStartup"`
    SyncInterval    int    `json:"syncInterval"` // minutes
    ConflictResolution string `json:"conflictResolution"` // "timestamp" (always use newest)
}
```

### 3.3 数据库表

```sql
CREATE TABLE IF NOT EXISTS sync_records (
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
);

CREATE TABLE IF NOT EXISTS sync_conflicts (
    id TEXT PRIMARY KEY,
    file_name TEXT NOT NULL,
    local_path TEXT NOT NULL,
    local_time DATETIME NOT NULL,
    remote_path TEXT NOT NULL,
    remote_time DATETIME NOT NULL,
    resolution TEXT,
    resolved_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sync_records_status ON sync_records(status);
CREATE INDEX IF NOT EXISTS idx_sync_records_created_at ON sync_records(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sync_conflicts_created_at ON sync_conflicts(created_at DESC);
```

### 3.4 百度云 API 集成

百度网盘 PCS API 主要端点：

```go
// BaiduPCSClient 百度云客户端
type BaiduPCSClient struct {
    AccessToken string
    AppKey     string
    SecretKey   string
    BaseURL    string
}

// 核心方法
func (c *BaiduPCSClient) GetAccessToken(appKey, secretKey, authCode string) (*TokenResponse, error)
func (c *BaiduPCSClient) RefreshAccessToken(refreshToken string) (*TokenResponse, error)
func (c *BaiduPCSClient) ListFiles(path string) ([]FileInfo, error)
func (c *BaiduPCSClient) UploadFile(localPath, remotePath string, onProgress func(float64)) (*UploadResult, error)
func (c *BaiduPCSClient) DownloadFile(remotePath, localPath string, onProgress func(float64)) error
func (c *BaiduPCSClient) DeleteFile(path string) error
func (c *BaiduPCSClient) GetFileInfo(path string) (*FileInfo, error)
```

### 3.5 Wails 方法签名

```go
// 获取当前同步状态
func (a *App) GetSyncStatus() (*SyncStatus, error)

// 手动触发同步
func (a *App) TriggerSync() (*SyncProgress, error)

// 获取同步进度
func (a *App) GetSyncProgress() (*SyncProgress, error)

// 获取同步历史
func (a *App) GetSyncHistory(limit int) ([]SyncRecord, error)

// 获取冲突列表
func (a *App) GetConflicts() ([]SyncConflict, error)

// 解决冲突（使用时间戳策略）
funcari (a *App) ResolveConflict(conflictID string, resolution string) error

// 获取同步设置
func (a *App) GetSyncSettings() (*SyncSettings, error)

// 更新同步设置
func (a *App) UpdateSyncSettings(settings SyncSettings) error

// 启动时自动同步（在 app.startup 中调用）
func (a *App) syncOnStartup() error
```

### 3.6 辅助结构

```go
type SyncProgress struct {
    Total          int            `json:"total"`
    Completed      int            `json:"completed"`
    CurrentFile    string         `json:"currentFile"`
    Status         string         `json:"status"` // "preparing", "uploading", "downloading", "resolving", "complete", "error"
    Message        string         `json:"message,omitempty"`
}

type FileInfo struct {
    Path       string    `json:"path"`
    Size       int64     `json:"size"`
    IsDir      bool      `json:"isDir"`
    Modified   time.Time `json:"modified"`
    MD5        string    `json:"md5,omitempty"`
}

type TokenResponse struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    ExpiresIn    int       `json:"expires_in"`
    Scope       string    `json:"scope"`
}
```

---

## 4. 实现顺序

### Phase 5 实现步骤：
1. 扩展 database.go - 添加 screening_sessions 和 screening_papers 表
2. 扩展 models.go - 添加 ScreeningPaper, ScreeningSession, ScreeningDecisionNode 结构
3. 创建 screening.go - 实现 screening 相关的数据库操作方法
4. 创建 pdf_service_client.go - 实现 PDF 服务 HTTP 客户端
5. 扩展 app.go - 添加 screening 相关的 Wails 方法
6. 集成到 App 结构和 startup/shutdown

### Phase 6 实现步骤：
1. 扩展 database.go - 添加 sync_records 和 sync_conflicts 表
2. 扩展 models.go - 添加 SyncRecord, SyncConflict, SyncStatus 结构
3. 创建 baidu_pcs.go - 实现百度云 API 客户端
4. 创建 sync.go - 实现同步逻辑和数据库操作
5. 扩展 app.go - 添加 sync 相关的 Wails 方法
6. 在 App.startup 中集成启动时同步

---

## 5. 技术细节

### PDF 服务调用
使用现有的 PDF Service (`http://localhost:50051`)：
- `POST /parse/upload` - 上传文件并解析
- `POST /extract/` - LLM 提取结构化数据

### 百度云 API
使用 OAuth 2.0 授权流程：
1. 获取授权 URL，用户在浏览器中授权
2. 获取 code，换取 access_token
3. 使用 refresh_token 自动刷新过期 token
4. 所有 API 调用携带 Bearer access_token

### 错误处理
- 所有数据库操作使用事务
- 网络请求超时 30 秒
- LLM 调用失败返回友好错误消息
- 文件操作检查权限

---

## 6. 测试计划

### Phase 5 测试：
1. 单元测试 - screening.go 的数据库操作
2. 集成测试 - 文件上传 → 提取 → 分析完整流程
3. 前端集成测试 - 通过 Wails 调用各方法

### Phase 6 测试：
1. 单元测试 - baidu_pcs.go 的 API 调用
2. 单元测试 - sync.go 的同步逻辑
3. 手动测试 - 配置百度云授权，测试上传/下载

---

**End of Design Document**
