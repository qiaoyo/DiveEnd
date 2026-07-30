# DiveEnd Project Map

更新时间：2026-07-30

This document maps product workflows to the current code structure. Use it after reading `README.md` and `AGENTS.md`.

## Source Of Truth

Current code and these files are authoritative:

- `README.md`
- `AGENTS.md`
- `docs/PROJECT_MAP.md`
- `docs/superpowers/plans/2026-06-10-code-review-remediation.md`
- `docs/superpowers/plans/2026-06-11-secret-history-remediation.md`

Historical specs are useful context but not always current:

- `docs/superpowers/specs/2026-04-02-diveend-unified-design.md`
- `docs/superpowers/specs/2026-04-05-backend-implementation-design.md`
- `docs/superpowers/plans/2026-04-02-diveend-implementation-plan.md`
- `docs/superpowers/plans/2026-04-05-phase5-screening-backend.md`

Known historical drift:

- Older docs mention Marker; current PDF parser backend is PyMuPDF4LLM/PyMuPDF.
- Older docs mention GORM; current Go persistence uses `database/sql` with sqlite3.
- Older docs list startup sync, exit sync, and batch move as pending; current code implements them.

## Product Workflows

### 1. App Startup And Configuration

Purpose:

- Load persisted config.
- Apply pending cloud DB restore if needed.
- Open or migrate SQLite DB.
- Configure LLM, search, PDF service, sync manager, and download workers.
- Start the managed local PDF service asynchronously, wait for readiness only when a PDF workflow first needs it, and stop the owned process on app shutdown.
- Hydrate frontend initial state.

Backend files:

- `main.go`
- `app.go`
- `config_store.go`
- `database.go`
- `database_restore.go`
- `models.go`
- `pdf_service_process.go`

Frontend files:

- `frontend/src/App.tsx`
- `frontend/src/stores/appStore.ts`
- `frontend/src/lib/backend.ts`
- `frontend/src/components/settings/SettingsPanel.tsx`

Tests:

- `app_test.go`
- `app_lifecycle_test.go`
- `config_store_test.go`
- `database_test.go`
- `database_restore_test.go`
- `frontend/src/App.test.tsx`
- `frontend/src/components/settings/SettingsPanel.test.tsx`
- `frontend/src/stores/appStore.test.ts`

### 2. DeepStart Discovery

Purpose:

- Start an AI-assisted research exploration from a natural language prompt.
- Search Semantic Scholar and arXiv.
- Rewrite and enrich queries.
- Persist a concise explanation of which retrieval phrase or title/abstract terms caused each paper to rank.
- Preprocess PDF/markdown caches when possible.
- Generate analysis directions and recommended papers.
- Persist session history and selections.

Backend files:

- `deepstart.go`
- `deepstart_task.go`
- `deepstart_search_rewrite.go`
- `deepstart_enrichment.go`
- `deepstart_pipeline.go`
- `deepstart_background.go`
- `clients.go`
- `llm_budget.go`
- `llm_context.go`
- `paper_import_assets.go`

Frontend files:

- `frontend/src/components/deepstart/DeepStartPanel.tsx`
- `frontend/src/components/deepstart/SessionDetailPanel.tsx`
- `frontend/src/components/deepstart/CategoryTree.tsx`
- `frontend/src/components/deepstart/SearchResults.tsx`
- `frontend/src/components/deepstart/SearchPanel.tsx`
- `frontend/src/lib/backend.ts`
- `frontend/src/components/settings/SettingsPanel.tsx` (daily budget and usage)

Tests:

- `app_test.go`
- `clients_test.go`
- `llm_budget_test.go`
- `llm_real_e2e_test.go` (opt-in real providers)
- `search_real_e2e_test.go` (opt-in weak rewrite + live search)
- `deepstart_enrichment_test.go`
- `deepstart_pipeline_test.go`
- `frontend/src/components/deepstart/SessionDetailPanel.test.tsx`

### 3. DeepRead Reading

Purpose:

- Load a library paper into a reading workspace.
- Attach or download managed PDF files.
- Prepare parsed markdown and sections through the PDF service.
- Default DeepStart discovery keeps about 20 focused candidates while eagerly parsing/extracting only the top 4; remaining candidates stay available for on-demand processing.
- Route low-latency questions to the weak model and full-paper summaries to the strong model, with fallback when only one assistant is configured.
- Build bounded long-paper context from the selected section, question-relevant sections, and core sections without allowing one large abstract to consume the entire context.
- Return an answer, concise takeaway, verbatim-validated section evidence, and explicit limitations; reject invented section IDs and paraphrased evidence.
- Let evidence citations switch the active parsed section; exact PDF-page navigation remains unavailable until extraction stores page anchors.
- Cancel an in-flight AI reading request from the reader or during application shutdown.
- Show a range-friendly Wails asset URL for PDFs, with bounded base64 fallback.
- Save translations and notes.

Backend files:

- `deepread.go`
- `deepread_task.go`
- `clients.go`
- `deepread_pdf_paths.go`
- `deepread_asset_server.go`
- `pdf_service_client.go`
- `pdf_service_process.go`
- `pdf_extraction_real_e2e_test.go` (opt-in managed service + provider usage)
- `local_file_actions.go`
- `paper_import_assets.go`
- `secure_file.go`

Frontend files:

- `frontend/src/components/deepread/DeepReadPanel.tsx`
- `frontend/src/lib/backend.ts`
- `frontend/src/lib/errors.ts`

Tests:

- `deepread_pdf_paths_test.go`
- `deepread_ai_test.go`
- `deepread_task_test.go`
- `deepread_real_e2e_test.go` (opt-in strong/weak provider grounding)
- `pdf_service_client_test.go`
- `local_file_actions_test.go`
- `paper_import_assets_test.go`
- `frontend/src/components/deepread/DeepReadPanel.test.tsx`

### 4. Screening Pipeline

Purpose:

- Create a screening session.
- Select PDFs through native picker or path resolution.
- Copy PDFs into `DataPath/screening/<session>/<paper>/`.
- Extract content through the PDF service.
- Stop extraction, initial AI analysis, or later decision generation without deleting the session; completed papers and the last committed decision remain available.
- Build and traverse an LLM decision tree; initial and subsequent decisions atomically persist the node, path, session state, and affected paper states.
- Resume recent local sessions at their persisted extraction, decision, or imported state without silently issuing another LLM request.
- Import selected papers into the library.

Backend files:

- `screening.go`
- `screening_workflow.go`
- `screening_task.go`
- `pdf_service_client.go`
- `local_file_actions.go`
- `models.go`

Frontend files:

- `frontend/src/pages/Screening.tsx`
- `frontend/src/lib/backend.ts`
- `frontend/src/types/index.ts`

Tests:

- `screening_sync_test.go`
- `screening_task_test.go`
- `pdf_service_client_test.go`
- `frontend/src/pages/Screening.test.tsx`

### 5. Library And Managed Files

Purpose:

- Maintain folder tree and paper records.
- Create, rename, move, and delete folders.
- Delete papers.
- Move single or multiple papers across folders.
- Keep managed PDF paths and DeepRead parse cache paths consistent.

Backend files:

- `folders_api.go`
- `folder_utils.go`
- `paper_import_assets.go`
- `local_file_actions.go`
- `app.go`
- `secure_file.go`

Frontend files:

- `frontend/src/components/paperlist/PaperListPanel.tsx`
- `frontend/src/components/layout/AppLayout.tsx`
- `frontend/src/stores/appStore.ts`

Tests:

- `folders_api_test.go`
- `paper_delete_test.go`
- `paper_move_test.go`
- `paper_import_assets_test.go`
- `local_file_actions_test.go`
- `frontend/src/components/paperlist/PaperListPanel.test.tsx`

### 6. Sync And Cloud Restore

Purpose:

- Preview local files before upload.
- Upload SQLite snapshot, manifest, and managed PDFs to Baidu Cloud.
- Download missing remote files on startup.
- Detect local/remote conflicts.
- Resolve file conflicts.
- Stage remote database restore safely and apply/cancel through user confirmation.
- Emit `sync-progress` events while preserving polling fallback.
- Present preflight, live progress, history, conflicts, restore confirmation, and automation settings in one restrained Chinese-language workspace.

Backend files:

- `sync.go`
- `sync_api.go`
- `baidu_pcs.go`
- `database_restore.go`
- `secure_file.go`
- `redaction.go`
- `http_response.go`
- `app_lifecycle.go`

Frontend files:

- `frontend/src/pages/Sync.tsx`
- `frontend/src/lib/backend.ts`
- `frontend/src/types/index.ts`

Tests:

- `sync_progress_test.go`
- `baidu_pcs_test.go`
- `baidu_real_e2e_test.go`
- `baidu_current_data_e2e_test.go`
- `app_lifecycle_test.go`
- `screening_sync_test.go`
- `frontend/src/pages/Sync.test.tsx`

### 7. PDF Microservice

Purpose:

- Parse uploaded or URL PDFs.
- Reject oversized, non-PDF, private-network, redirect-to-private, or unsafe inputs.
- Extract structured paper metadata/metrics/baselines through LLMs.
- Redact sensitive error output.

Files:

- `services/pdf_service/app/main.py`
- `services/pdf_service/app/routes/health.py`
- `services/pdf_service/app/routes/parse.py`
- `services/pdf_service/app/routes/extract.py`
- `services/pdf_service/core/config.py`
- `services/pdf_service/core/llm_client.py`
- `services/pdf_service/core/redaction.py`
- `services/pdf_service/docker-compose.yml`
- `services/pdf_service/requirements.txt`

Tests:

- `services/pdf_service/tests/test_routes.py`

## Shared Hardening Utilities

- `secure_file.go`: symlink-safe directory creation, atomic writes, fd-relative rename/remove helpers.
- `redaction.go`: Go-side secret redaction.
- `http_response.go`: bounded HTTP response reads.
- `llm_context.go`: context-aware LLM helper paths.
- `services/pdf_service/core/redaction.py`: Python-side redaction.
- `frontend/src/lib/errors.ts`: frontend user-visible error redaction.

## Generated Or Derived Files

Tracked generated files:

- `frontend/wailsjs/go/main/App.d.ts`
- `frontend/wailsjs/go/main/App.js`
- `frontend/wailsjs/go/models.ts`

Ignored generated/build/dependency files:

- `frontend/dist/`
- `frontend/node_modules/`
- `build/bin/`
- Python `__pycache__/`
- `.pytest_cache/`

## Verification Matrix

Use this table to pick tests for a change:

| Change Area | Minimum Checks |
| --- | --- |
| Config/startup | `go test ./... -run 'Test(LoadAppConfig|AppSaveConfig|Startup|DatabaseRestore)'` |
| DeepStart | `go test ./... -run 'Test(AppDeepStart|SearchClient|PreprocessDeepStart|DeepStartEnricher)'` |
| Real retrieval | `DIVEEND_REAL_SEARCH_E2E=1 go test ./... -run TestRealDeepStartRetrievalE2E -v` |
| Real LLM budget | `DIVEEND_REAL_LLM_E2E=1 go test ./... -run TestRealStrongAndWeakLLMBudgetE2E -v` |
| Real PDF extraction budget | `DIVEEND_REAL_PDF_EXTRACTION_E2E=1 DIVEEND_REAL_PDF_PATH=/absolute/paper.pdf go test ./... -run TestRealPDFExtractionBudgetE2E -v` |
| DeepRead/PDF | `go test ./... -run 'Test(DeepRead|PDFServiceClient|AttachLocalPDF|DownloadPDF)'` |
| Screening | `go test ./... -run 'Test(AppScreening|UploadScreening|CancelScreening)'` |
| Library moves/deletes | `go test ./... -run 'Test(MovePaper|MovePapers|DeletePaper|Folder)'` |
| Sync/Baidu | `go test ./... -run 'Test(Sync|Baidu|DatabaseRestore)'` |
| Frontend | `cd frontend && npm test -- --run` |
| Browser UI | `node frontend/scripts/ui-smoke.mjs` (set `UI_SMOKE_WIDTH` and `UI_SMOKE_HEIGHT` for narrow-window checks) |
| PDF service | `python -m pytest services/pdf_service/tests` |
| Remote CI | `.github/workflows/ci.yml` |
| Security scan | `bash scripts/secret_scan.sh` |

Full release baseline:

```bash
go test ./...
go vet ./...
go build ./...
go test -race ./...
cd frontend && npm test -- --run
cd frontend && npm run build
cd frontend && npm audit --omit=dev --json
python -m pytest services/pdf_service/tests
bash scripts/secret_scan.sh
wails build
node frontend/scripts/ui-smoke.mjs
UI_SMOKE_WIDTH=900 UI_SMOKE_HEIGHT=900 node frontend/scripts/ui-smoke.mjs
UI_SMOKE_WIDTH=720 UI_SMOKE_HEIGHT=900 node frontend/scripts/ui-smoke.mjs
```

## Release Notes For Agents

- Do not commit ignored local config or token files.
- Do not run destructive history rewrite unless explicitly requested.
- Do not assume real Baidu tests can run without local credentials.
- Keep `README.md`, `AGENTS.md`, and this file synchronized when changing project structure or status.
