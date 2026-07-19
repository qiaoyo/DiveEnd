# DiveEnd

DiveEnd 是一个本地优先的论文研究工作台，用 Wails 桌面壳把 React/TypeScript 前端、Go 后端、SQLite 文库和 Python PDF 微服务串成一条研究工作流。

核心目标不是只保存 PDF，而是覆盖从“发现方向”到“筛选论文”再到“沉浸阅读和同步备份”的完整闭环。

## 当前状态

更新时间：2026-07-19

项目已完成 Phase 1-6 的功能骨架，并完成一轮 P0-P3 code review remediation。当前阶段是可靠性、真实环境验收、发布安全和体验打磨。

已落地的主流程：

- **DeepStart**：自然语言输入研究方向，聚合 Semantic Scholar 和 arXiv，生成 AI 分类、论文摘要、推荐路径，并导入本地文库。
- **Screening**：批量选择本地 PDF，复制到 DiveEnd managed data 目录，调用 PDF service 解析，再用 LLM 决策树逐轮筛选并导入选中论文。
- **DeepRead**：按文库论文打开阅读区，加载 managed PDF，解析章节，保存翻译、摘要和笔记。PDF 优先通过 Wails asset server 同源 URL 加载，大文件避免全量 base64。
- **Sync**：百度云同步本地 SQLite 快照和 managed PDF。数据库同步使用 staging、manifest、稳定 remote keys、冲突检测和恢复向导，而不是直接上传 live DB。
- **Library**：右侧论文库支持文件夹树、创建、删除、重命名、移动、单篇移动、批量移动，并同步维护 managed PDF 路径和 DeepRead cache。
- **安全与可靠性**：配置和 token 脱敏、用户可见错误脱敏、context cancellation、PDF/URL 边界校验、symlink 防护、原子文件写入、同步进度事件和大量回归测试已落地。

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
```

Important correction for older docs: the current PDF service uses **PyMuPDF4LLM/PyMuPDF**, not Marker. The Go persistence layer uses `database/sql` with `github.com/mattn/go-sqlite3`, not GORM.

## Read This First

- [AGENTS.md](AGENTS.md): concise agent entry point and current implementation status.
- [docs/PROJECT_MAP.md](docs/PROJECT_MAP.md): map from product workflows to files and tests.
- [docs/superpowers/plans/2026-06-10-code-review-remediation.md](docs/superpowers/plans/2026-06-10-code-review-remediation.md): detailed remediation record.
- [docs/superpowers/plans/2026-06-11-secret-history-remediation.md](docs/superpowers/plans/2026-06-11-secret-history-remediation.md): Git history secret cleanup plan.
- `docs/superpowers/specs/*` and older `docs/superpowers/plans/*`: historical design and implementation plans. Treat them as background unless they conflict with `README.md`, `AGENTS.md`, or `docs/PROJECT_MAP.md`.

## Current Gaps

- Git history still contains previously committed local files and at least one historical OpenAI-style key pattern. Current tracking is guarded, but public release requires history rewrite and credential rotation.
- Wails desktop event bridge needs manual verification in the packaged desktop app for `deepstart-progress`, `extract-progress`, and `sync-progress`.
- Real Baidu Cloud end-to-end sync still needs an authorized account pass.
- DeepRead can still be improved with finer page-level cache/prefetch and better long-PDF ergonomics.
- UI polish remains useful for dense library workflows, narrow windows, and high-DPI layouts.

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

Run Wails development app:

```bash
wails dev
```

Build Wails app:

```bash
wails build
```

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

Last local baseline: 2026-07-19.

Passed:

- `bash scripts/secret_scan.sh`
- `go test ./...`
- `go vet ./...`
- `go build ./...`
- `go test -race ./...`
- `cd frontend && npm test -- --run`
- `cd frontend && npm run build`
- `cd frontend && npm audit --omit=dev --json` with 0 production vulnerabilities
- `/private/tmp/diveend-pdf-test-venv/bin/python -m pytest services/pdf_service/tests`
- `wails build`

Notes:

- PDF service tests were run through a temporary venv under `/private/tmp` because the available `python` and `python3` commands did not already have `pytest`.
- Real Baidu Cloud E2E was not run because it requires valid local credentials.
- Wails desktop event bridge still needs manual app-level verification.
