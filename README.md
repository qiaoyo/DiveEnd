# DiveEnd

DiveEnd 是一个本地优先的论文研究工作台，用 Wails 桌面壳把 React/TypeScript 前端、Go 后端、SQLite 文库和 Python PDF 微服务串成一条研究工作流。

核心目标不是只保存 PDF，而是覆盖从“发现方向”到“筛选论文”再到“沉浸阅读和同步备份”的完整闭环。

## 当前状态

更新时间：2026-07-30

项目已完成 Phase 1-6 的功能骨架，并完成一轮 P0-P3 code review remediation。当前阶段是可靠性、真实环境验收、发布安全和体验打磨。

已落地的主流程：

- **DeepStart**：自然语言输入研究方向，聚合 Semantic Scholar 和 arXiv，执行最多 3 个重写 query 后统一按标题/摘要相关性和年份排序，默认收敛到约 20 篇高相关候选；每篇保留可核查的标题/摘要命中解释，只为排名最前的 4 篇预取和结构抽取，其余候选按需处理。
- **Screening**：批量选择本地 PDF，复制到 DiveEnd managed data 目录，调用 PDF service 解析，再用 LLM 决策树逐轮筛选并导入选中论文；抽取、首次分析和后续决策均可非破坏性停止，节点、路径和论文状态以 SQLite 事务一次提交，取消或模型失败不会留下半完成选择。分析页列出最近的本地会话，可恢复提取、筛选或已入库状态且不会自动重跑模型。
- **DeepRead**：按文库论文打开阅读区，加载 managed PDF，解析章节，保存翻译、摘要和笔记；低延迟问答优先走弱模型，全文总结走强模型，二者都只保留能在已解析章节中逐字验证的依据。长论文上下文按问题相关性和核心章节公平分配，AI 请求可主动取消。PDF 优先通过 Wails asset server 同源 URL 加载，大文件避免全量 base64。
- **Sync**：百度云同步本地 SQLite 快照和 managed PDF。数据库同步使用 staging、manifest、稳定 remote keys、冲突检测和恢复向导，而不是直接上传 live DB；同步工作台使用统一中文操作语义，集中呈现预检、进度、历史、冲突和自动化设置。
- **Library**：右侧论文库支持文件夹树、创建、删除、重命名、移动、单篇移动、批量移动，并同步维护 managed PDF 路径和 DeepRead cache。
- **安全与可靠性**：配置和 token 脱敏、用户可见错误脱敏、context cancellation、覆盖 Go 与 Python 抽取链路的每日共享 LLM token 硬预算、PDF/URL 边界校验、symlink 防护、原子文件写入、同步进度事件和大量回归测试已落地。
- **当前信息架构**：一级任务收敛为“发现 / 阅读 / 分析”，研究记录、同步和设置作为工具入口；检索页不再展示伪造预览数据，阅读页默认提供可收起论文库的专注三栏工作区。

## 架构

```text
React/TypeScript UI
  |
  | Wails bridge + events
  v
Go app backend
  |-- config_store.go        runtime config, secret merge, seed loading
  |-- database.go            SQLite migration and core persistence
  |-- deepstart*.go          search, AI analysis, preprocessing, background tasks
  |-- deepread*.go           PDF preparation, cache, notes, asset-server URL
  |-- screening*.go          batch upload, extraction, decision tree, import
  |-- sync*.go               Baidu Cloud sync, progress, conflicts, restore
  |-- baidu_pcs.go           Baidu PCS client
  |-- secure_file.go         atomic and symlink-safe file operations
  |
  v
SQLite data path
  |-- diveend.db
  |-- papers/
  |-- screening/
  |-- deepstart_cache/
  |-- .sync-staging/
  |-- .sync-restore/

Python PDF service
  |-- FastAPI
  |-- PyMuPDF4LLM/PyMuPDF PDF parsing
  |-- OpenAI/Anthropic-compatible extraction
  |-- Desktop-managed startup, readiness, and shutdown
```

Important correction for older docs: the current PDF service uses **PyMuPDF4LLM/PyMuPDF**, not Marker. The Go persistence layer uses `database/sql` with `github.com/mattn/go-sqlite3`, not GORM.

## Read This First

- [AGENTS.md](AGENTS.md): concise agent entry point and current implementation status.
- [docs/PROJECT_MAP.md](docs/PROJECT_MAP.md): map from product workflows to files and tests.
- [docs/AUTONOMOUS_DEVELOPMENT_SETUP.md](docs/AUTONOMOUS_DEVELOPMENT_SETUP.md): responsibility boundaries, design decisions, account prerequisites, and the long-running autonomous development loop.
- [docs/superpowers/plans/2026-06-10-code-review-remediation.md](docs/superpowers/plans/2026-06-10-code-review-remediation.md): detailed remediation record.
- [docs/superpowers/plans/2026-06-11-secret-history-remediation.md](docs/superpowers/plans/2026-06-11-secret-history-remediation.md): Git history secret cleanup plan.
- `docs/superpowers/specs/*` and older `docs/superpowers/plans/*`: historical design and implementation plans. Treat them as background unless they conflict with `README.md`, `AGENTS.md`, or `docs/PROJECT_MAP.md`.

## Current Gaps

- Git history still contains previously committed local files and at least one historical OpenAI-style key pattern. Current tracking is guarded, but public release requires history rewrite and credential rotation.
- Wails desktop event bridge needs manual verification in the packaged desktop app for `deepstart-progress`, `extract-progress`, and `sync-progress`.
- Real Baidu Cloud sync passed an authorized upload/list/download/cleanup E2E on 2026-07-30, including automatic refresh and secure persistence of an expired access token.
- The configured Semantic Scholar API key returned `403 Forbidden` on 2026-07-30, while the unauthenticated shared endpoint returned `429`; discovery must retain arXiv/cache degradation until the key is replaced.
- GitHub SSH read/write access is available, but the local `gh` CLI is not authenticated, so automatic PR creation through `gh` remains blocked.
- DeepRead can still be improved with finer page-level cache/prefetch, PDF 页码级证据定位、跨论文对比和更好的长文档导航。
- High-DPI polish remains useful; library, sync, and settings have completed the current shared-token cleanup, and the main workflows pass narrow-window smoke coverage.

## Ignored Local Files

The following are intentionally local-only and must not be committed:

- `.DS_Store`
- `auth.json`
- `config.toml`
- `test.txt`
- `downloaded.txt`
- `baidupan_test.py`
- `baidu_pcs.go.tmpl`
- `baiduyun_token.json`
- `config/weak_llm.json`
- `config/strong_llm.json`
- `config/stroing_llm.json`
- `config/semantic_scholar.json`
- `frontend/node_modules/`
- `frontend/dist/`
- `build/bin/`
- `.pytest_cache/`
- `__pycache__/`

Use the `.example` files under `config/` and `baiduyun_token.json.example` as templates.

## Development

Install frontend dependencies:

```bash
cd frontend
npm install
```

Run frontend tests:

```bash
cd frontend
npm test -- --run
```

Build frontend:

```bash
cd frontend
npm run build
```

Run Go tests:

```bash
go test ./...
```

Run Go vet:

```bash
go vet ./...
```

Run current-tree secret scan:

```bash
bash scripts/secret_scan.sh
```

Run PDF service tests:

```bash
python -m pytest services/pdf_service/tests
```

Create the local managed PDF service environment once:

```bash
bash scripts/setup_pdf_service.sh
```

The desktop app then starts the service from `services/pdf_service/.venv`, waits for `/health/ready`, and stops the owned process on exit. Set `DIVEEND_PDF_SERVICE_URL` to use an externally managed service instead.

Run Wails development app:

```bash
wails dev
```

Build Wails app:

```bash
wails build
```

## Continuous Integration

`.github/workflows/ci.yml` runs the credential-free macOS baseline on pull requests and protected development branches:

- tracked-file secret scan;
- Go test, vet, build, and race detector;
- frontend tests and production build;
- critical-level production dependency audit;
- PDF service tests on Python 3.13;
- packaged Wails desktop build.

Real LLM, Semantic Scholar, and Baidu E2E remain explicit local checks and are never supplied with personal credentials in routine CI.

## Runtime Configuration

Runtime config is stored in the user config directory, not in tracked project files:

- Config: `$XDG_CONFIG_HOME/DiveEnd/config.json` or the platform default user config directory.
- Data path: `config.dataPath`.
- SQLite DB: `<dataPath>/diveend.db`.

Optional local seed files are read on first startup but remain ignored:

- `config/strong_llm.json`
- `config/weak_llm.json`
- `config/semantic_scholar.json`
- `baiduyun_token.json`

## Verification Baseline

Last local baseline: 2026-07-30.

Passed:

- `bash scripts/secret_scan.sh`
- `go test ./...`
- `go vet ./...`
- `go build ./...`
- `go test -race ./...`
- `cd frontend && npm test -- --run`
- `cd frontend && npm run build`
- `services/pdf_service/.venv/bin/python -m pytest services/pdf_service/tests`
- `wails build`
- Real strong and weak LLM chat-completion probes.
- Opt-in strong/weak LLM budget E2E (`DIVEEND_REAL_LLM_E2E=1`) verifies provider usage accounting and restart persistence.
- Opt-in retrieval E2E (`DIVEEND_REAL_SEARCH_E2E=1`) verifies all rewritten queries, permanent Semantic Scholar failure degradation, arXiv results, and top-five topic relevance.
- Opt-in PDF extraction E2E (`DIVEEND_REAL_PDF_EXTRACTION_E2E=1`) verifies managed service startup, real parsing/extraction, provider usage reporting, and exact shared-budget settlement.
- Real Baidu upload/list/download/cleanup E2E with automatic OAuth refresh persistence.
- Browser and real-backend E2E through search degradation, focused 20-paper discovery, top-4 PDF preprocessing, AI map generation, DeepRead PDF display, Screening decisions, and sync workflows.
- Browser UI smoke covers 24 visible states across discovery, reading, screening, and sync at 1440, 900, and 720 px widths with no console errors; the narrow runs assert zero page-level horizontal overflow before every screenshot.
- Packaged Wails build and managed PDF-service startup/shutdown lifecycle. Direct packaged-window click and Wails event-delivery verification remains pending because the macOS session was locked during the 2026-07-30 run.

Notes:

- `npm audit --omit=dev` reports the React Router RSC-mode advisory against `react-router@7.18.2`. DiveEnd uses a client-only `HashRouter` and does not use RSC; the currently published `react-router-dom` line has no version that clears this advisory without a React 19/Router 8 migration. Keep this scoped exception under review.
- The configured Semantic Scholar key returned `403 Forbidden`; arXiv fallback and permanent-error fast failure were verified.
- Computer Use could not inspect the packaged window while macOS was locked; do not infer packaged UI event verification from browser smoke coverage.
- Workspace pages now load by route: the common entry bundle is about 236 KiB before gzip, while the roughly 407 KiB PDF reader chunk loads only when DeepRead is opened.
