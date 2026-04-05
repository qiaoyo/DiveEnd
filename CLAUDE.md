# DiveEnd - 项目进度概览

> **项目名称**: DiveEnd - 沉浸式论文收集与AI翻译阅读工具
> **最后更新**: 2026-04-06
> **当前阶段**: Phase 5-6 后端开发正在进行中

---

## 项目架构总览

```
┌─────────────────────────────────────────────────────────────────────┐
│                           DiveEnd - Unified Architecture                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐     ┌──────────────────┐     ┌─────────────────────────┐ │
│  │   Frontend    │────▶│    Go Backend    │────▶│   Python Microservice   │ │
│  │  React/TS    │     │  (Wails v2)      │     │  (PDF + LLM Pipeline)  │ │
│  └──────────────┘     └──────────────────┘     └─────────────────────────┘ │
│         │                      │                           │               │
│         ▼                      ▼                           ▼               │
│   ┌──────────┐          ┌──────────────────┐              ┌──────────────┐         │
│   │   3-Column │          │  SQLite   │              │  Marker PDF  │         │
│   │   Layout   │          │  Database │              │  Parser      │         │
│   └──────────┘          └──────────────────┘              └──────────────┘         │
│                                                          │                  │
│                                                          ▼                  │
│                                                   ┌──────────────┐         │
│                                                   │  LLM Clients │         │
│                                                   │  - Weak      │         │
│                                                   │  - Strong    │         │
│                                                   └──────────────┘         │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 开发阶段进度

| 阶段 | 名称 | 状态 | 关键交付物 | 完成度 |
|------|------|------|------------|--------|
| **Phase 1** | Foundation (基础架构) | ✅ 已完成 | 配置系统、LLM Client、数据库基础 | 100% |
| **Phase 2** | PDF Service (PDF服务) | ✅ 已完成 | Python微服务、Marker集成、FastAPI | 100% |
| **Phase 3** | DeepRead (沉浸式阅读) | ✅ 已完成 | 分屏阅读UI、翻译视图、PDF查看器 | 100% |
| **Phase 4** | DeepStart (领域探索) | ✅ 已完成 | 搜索集成、AI引导选择、导入流程 | 100% |
| **Phase 5** | Screening Pipeline (论文筛选) | 🟡 后端进行中 | 三阶段Pipeline、决策树UI | 60% |
| **Phase 6** | Sync & Polish (同步与优化) | 🟡 后端进行中 | 百度云同步、冲突解决UI | 40% |

---

## 详细任务状态

### Phase 1: Foundation ✅ 已完成

- [x] 配置系统 (Config system)
- [x] LLM Client (支持OpenAI/Anthropic)
- [x] 数据库基础 (SQLite + GORM)
- [x] 项目结构初始化

### Phase 2: PDF Service ✅ 已完成

- [x] Python微服务架构
- [x] Marker PDF解析集成
- [x] FastAPI服务实现
- [x] Docker Compose配置

### Phase 3: DeepRead ✅ 已完成

- [x] 分屏阅读布局 (SplitScreenLayout)
- [x] PDF查看器组件 (PDFViewer)
- [x] 翻译面板 (TranslationPanel)
- [x] DeepRead页面集成

### Phase 4: DeepStart ✅ 已完成

**已完成:**
- [x] DeepStart主页面 (`frontend/src/components/deepstart/DeepStartPanel.tsx`)
- [x] SearchPanel组件 - 搜索输入面板
- [x] SearchResults组件 - 搜索结果展示
- [x] CategoryTree组件 - AI分类树形展示
- [x] SelectionPanel组件 - 多轮选择面板
- [x] 分步骤工作流程 (Search → Categories → Select → Import)
- [x] 后端搜索集成 (Semantic Scholar + arXiv)
- [x] AI分类算法实现
- [x] 论文导入数据库逻辑
- [x] 路由系统配置 (react-router-dom)
- [x] 类型系统统一 (types/index.ts)

### Phase 5: Screening Pipeline 🟡 前端进行中

**已完成:**
- [x] Screening主页面 (`frontend/src/pages/Screening.tsx`)
- [x] PDF上传界面
- [x] 内容提取进度模拟
- [x] 决策树交互界面
- [x] 筛选结果展示
- [x] 四阶段流程 (Upload → Extract → Screen → Results)
- [x] 数据模型定义 (`models.go`): ScreeningPaper, ScreeningSession, ScreeningDecisionNode
- [x] 数据库迁移 (`database.go`): screening_sessions, screening_papers 表
- [x] 数据库操作 (`screening.go`): 完整CRUD方法
- [x] Wails方法 (`app.go`): CreateScreeningSession, UploadScreeningFiles, ListScreeningSessions, GetScreeningSession, CancelScreening
- [x] PDF服务客户端 (`pdf_service_client.go`): HTTP客户端封装

**待完成:**
- [ ] Marker PDF解析服务集成
- [ ] 后端LLM分析集成
- [ ] 批量论文存储逻辑
- [ ] 决策树生成算法
- [ ] 提取进度实时更新

### Phase 6: Sync & Polish 🟡 前端进行中

**已完成:**
- [x] Sync主页面 (`frontend/src/pages/Sync.tsx`)
- [x] 同步状态仪表板
- [x] 同步历史记录展示
- [x] 冲突检测UI
- [x] 同步设置模态框
- [x] 自动同步配置选项
- [x] 数据模型定义 (`models.go`): SyncRecord, SyncConflict, SyncStatus, SyncSettings
- [x] 数据库迁移 (`database.go`): sync_records, sync_conflicts 表
- [x] 数据库操作 (`sync.go`): SyncManager基础实现
- [x] 百度云API客户端 (`baidu_pcs.go`): 基础API实现（Upload/Download/ListFiles）
- [x] Token管理: 自动刷新过期token

**待完成:**
- [ ] 后端Wails方法集成 (GetSyncStatus, TriggerSync, GetSyncProgress, GetConflicts, ResolveConflict)
- [ ] App结构集成SyncManager
- [ ] 启动时自动同步
- [ ] 关闭时未同步提醒
- [ ] 冲突解决算法
- [ ] 同步进度实时推送

---

## 文件结构现状

```
DiveEnd/
├── backend (Go + Wails)
│   ├── app.go                    # 主应用逻辑 ✅
│   ├── deepstart.go              # DeepStart后端 ✅
│   ├── clients.go                # LLM/Search客户端 ✅
│   ├── database.go               # 数据库操作 ✅
│   ├── models.go                 # 数据模型 ✅
│   ├── screening.go              # Screening数据库操作 ✅ (新增)
│   ├── sync.go                   # Sync数据库操作 ✅ (新增)
│   ├── baidu_pcs.go             # 百度云API客户端 ✅ (新增)
│   ├── pdf_service_client.go     # PDF服务客户端 ✅ (新增)
│   └── ...
│
├── frontend (React + TypeScript)
│   ├── src/
│   │   ├── pages/
│   │   │   ├── Screening.tsx     # Screening页面 ✅
│   │   │   └── Sync.tsx          # Sync页面 ✅
│   │   ├── components/
│   │   │   ├── deepstart/        # DeepStart组件 ✅
│   │   │   ├── deepread/         # DeepRead组件 ✅
│   │   │   ├── layout/
│   │   │   │   └── Router.tsx   # 路由配置 ✅ (新增)
│   │   │   └── ...
│   │   │   └── types/index.ts   # 统一类型定义 ✅ (更新)
│   │   └── ...
│   │
├── services/
│   └── pdf_service/              # Python PDF服务 ✅
```

---

## 下一步行动建议

### 立即进行
1. **Phase 5后端开发** - 测试Screening基础功能
   - 测试文件上传接口
   - 测试数据库CRUD操作
   - 验证会话创建和管理

2. **Phase 6后端开发** - 完成同步基础功能
   - 测试百度云API连接
   - 测试文件上传/下载
   - 验证Token刷新机制

### 后续工作
3. **Marker服务集成** - 连接PDF解析服务
4. **LLM分析集成** - 实现AI驱动的决策树生成
5. **前端状态管理** - 完善Zustand或React Query

---

## 最近提交记录

```
(配置react-router-dom路由)
feat(frontend): add react-router-dom and configure routing
- Install react-router-dom dependency
- Create Router component with routes for /deepstart, /deepread, /screening, /sync
- Update AppLayout to useLocation hook for active panel detection
- Move type definitions from pages/DeepStart.tsx to types/index.ts
- Fix component imports to use types instead of pages
- Remove unused pages/DeepStart.tsx file

(添加Screening数据库支持)
feat(database): add screening_sessions and screening_papers tables
- Add screening_sessions table with decision tree state
- Add screening_papers table with extraction status
- Add indexes for session_id and status
- Integrate screening migration into DB.migrate()

(实现Screening数据库操作)
feat(database): implement screening database operations
- Add CRUD methods for ScreeningSession
- Add batch methods for ScreeningPaper
- Add update methods for status, content, path history
- Implement GetScreeningSessionDetail with path history parsing

(添加Screening Wails方法)
feat(app): add screening Wails methods
- Add CreateScreeningSession for new session creation
- Add UploadScreeningFiles for file upload
- Add ListScreeningSessions for listing sessions
- Add GetScreeningSession for session details
- Add CancelScreening for session deletion
- Add placeholder methods for extraction and analysis

(添加Sync数据库支持)
feat(database): add sync_records and sync_conflicts tables
- Add sync_records table for upload/download history
- Add sync_conflicts table for conflict tracking
- Add indexes for status and created_at
- Integrate sync migration into DB.migrate()

(实现Sync基础功能)
feat(database): implement sync database operations
- Add SaveSyncRecord for recording sync operations
- Add GetSyncRecords for history retrieval
- Add SaveSyncConflict for conflict tracking
- Add GetSyncConflicts for listing conflicts
- Add ResolveSyncConflict for conflict resolution

(实现百度云API客户端)
feat(baidu): implement Baidu PCS API client
- Implement token management with auto-refresh
- Implement file upload with chunking and MD5
- Implement file download with progress tracking
- Implement file listing and deletion
- Implement BaiduToken, FileInfo structures

(添加Sync数据模型)
feat(models): add sync data models
- Add BaiduToken for token management
- Add SyncRecord for operation history
- Add SyncConflict for conflict tracking
- Add SyncStatus and SyncSettings for configuration
- Add FileInfo and SyncProgress for API operations

(创建PDF服务客户端)
feat(pdf): create PDF service HTTP client
- Implement UploadFile method
- Implement ExtractContent method
- Implement response structures
```

---

*文档更新: 2026-04-06*
*作者: Claude Code*
*版本: v1.2*
