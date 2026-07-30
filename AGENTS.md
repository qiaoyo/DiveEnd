# DiveEnd Agent Guide

This file is the canonical entry point for LLM agents working in this repository.

更新时间：2026-07-30

## Project Summary

DiveEnd is a local-first desktop research tool for academic paper discovery, screening, reading, translation, notes, and cloud backup.

The current implementation is:

- Frontend: React 18, TypeScript, Vite, Tailwind CSS, Radix UI, Framer Motion, Zustand, react-pdf.
- Desktop bridge: Wails v2.
- Backend: Go, `database/sql`, SQLite via `github.com/mattn/go-sqlite3`.
- PDF microservice: Python FastAPI, PyMuPDF4LLM/PyMuPDF, OpenAI/Anthropic-compatible extraction.
- Sync: Baidu Cloud PCS.

Important correction for older docs: do not assume Marker or GORM are current implementation details. They appear in historical docs but the code has moved on.

## Current Status

Phase 1-6 feature skeletons are implemented and wired to real backend paths. The project is now in reliability, release readiness, and UX polish.

Implemented:

- Config persistence, secret merging, local seed loading, redacted display values, and a shared persistent daily LLM token budget (default 100,000,000) covering both Go calls and Python Screening extraction.
- SQLite migrations for folders, papers, translations, DeepStart, DeepRead, Screening, Sync records, and conflicts.
- DeepStart search over Semantic Scholar and arXiv, all-query retrieval across up to three rewrites, relevance/recency ranking with persisted title/abstract match explanations, focused default retrieval of about 20 candidates, eager PDF/weak-model preprocessing for the top 4, on-demand handling for the rest, user-triggered supplemental expansion, enrichment, AI analysis, progress events, cancellation.
- DeepRead paper state, PDF parsing cache, notes, translation history, weak-model low-latency questions, strong-model summaries, relevance-aware long-paper context, verbatim-grounded section evidence, cancellable AI requests, Wails asset-server PDF URL, bounded base64 fallback.
- Screening sessions, recent-session resume without automatic model reruns, managed PDF upload storage, extraction progress, session-scoped cancellation across extraction and AI decisions, provider token usage reporting, shared-budget settlement, atomic node/path/paper decision persistence, LLM decision tree, final import.
- Library folder tree, folder create/delete/rename/move, paper delete, single-paper move, batch move, managed PDF/cache path maintenance.
- Baidu sync preview, background manual sync, startup sync, periodic sync, exit sync prompt, conflict detection, conflict resolution, database restore staging/apply/cancel.
- Security hardening around local secret files, symlink/path traversal, PDF validation, SSRF, atomic file writes, bounded HTTP reads, and user-visible error redaction.
- Frontend route/page tests, Go regression tests, Python PDF route tests, browser smoke script.
- Frontend primary navigation is organized around discovery, reading, and analysis; workspace routes are lazy-loaded, DeepStart/DeepRead/Screening/Sync use the shared restrained research-tool tokens, and the browser fallback and desktop bridge share the same backend wrapper contract.

Remaining risks:

- Git history still contains previously committed local secret/scratch files and at least one historical OpenAI-style key pattern. Current tree tracking is fixed, but public release requires history rewrite and token rotation.
- Wails desktop event delivery still needs manual verification in the packaged desktop app.
- Real Baidu Cloud sync passed an authorized upload/list/download/cleanup E2E on 2026-07-30, including automatic token refresh and secure persistence.
- The configured Semantic Scholar key returned `403 Forbidden` on 2026-07-30 and the shared unauthenticated endpoint returned `429`; use arXiv/cache degradation until the key is replaced.
- GitHub SSH read/write works, but `gh` CLI authentication is still required for automatic PR creation through `gh`.
- DeepRead page-level evidence navigation, cross-paper analysis, and legacy library UI polish remain future work; the settings route no longer retains the obsolete collapsible-sidebar interaction.

## Reading Order

1. `README.md` for product status and development commands.
2. `AGENTS.md` for this current agent-facing summary.
3. `docs/PROJECT_MAP.md` for the exact mapping from workflow to files and tests.
4. `docs/AUTONOMOUS_DEVELOPMENT_SETUP.md` for responsibility boundaries, design decisions, external account prerequisites, and autonomous execution rules.
5. `docs/superpowers/plans/2026-06-10-code-review-remediation.md` for the large reliability/security remediation history.
6. `docs/superpowers/plans/2026-06-11-secret-history-remediation.md` before any public push or release.
7. Historical specs in `docs/superpowers/specs/` only after reading the current docs above.

If a historical document conflicts with current code or this file, prefer current code, `README.md`, `AGENTS.md`, and `docs/PROJECT_MAP.md`.

## Code Map

Root Go files are the backend package. The legacy `src/config`, `src/database`, and `src/llm` packages have been removed from the active implementation.

- `main.go`: Wails app setup and asset server registration.
- `app.go`: app lifecycle, config application, Wails methods, translation, paper import/move/delete entry points.
- `models.go`: shared Go models exported to frontend bindings.
- `database.go`: SQLite connection, migrations, core folder/paper/translation/DeepStart/DeepRead persistence.
- `config_store.go`: runtime config loading, saving, sanitization, secret prefill, local seed loading.
- `clients.go`: LLM clients and paper search clients.
- `deepstart*.go`: DeepStart session workflow, search rewrite, enrichment, preprocessing, background work, cancellation.
- `deepread*.go`: DeepRead state, parse cache, notes, managed PDF path checks, Wails asset server.
- `screening.go`, `screening_workflow.go`: Screening persistence and Wails workflow.
- `sync.go`, `sync_api.go`, `baidu_pcs.go`, `database_restore.go`: Baidu sync, progress, conflicts, restore flow.
- `paper_import_assets.go`, `local_file_actions.go`, `folders_api.go`, `folder_utils.go`: library asset management and user file actions.
- `secure_file.go`, `redaction.go`, `http_response.go`, `llm_context.go`: shared hardening utilities.

Frontend:

- `frontend/src/App.tsx`: initial hydration and theme setup.
- `frontend/src/components/layout/`: router, global nav, app layout.
- `frontend/src/components/deepstart/`: DeepStart page and session detail.
- `frontend/src/components/deepread/`: DeepRead reader.
- `frontend/src/pages/Screening.tsx`: Screening workflow page.
- `frontend/src/pages/Sync.tsx`: Sync dashboard and conflict UI.
- `frontend/src/components/paperlist/`: right-side library management.
- `frontend/src/components/settings/`: settings and credential UI.
- `frontend/src/lib/backend.ts`: frontend wrapper over Wails bindings and browser preview mocks.
- `frontend/src/lib/errors.ts`: user-visible error redaction.
- `frontend/src/types/index.ts`: frontend domain types.
- `frontend/wailsjs/`: generated Wails bindings.

PDF service:

- `services/pdf_service/app/main.py`: FastAPI app and CORS/error handling.
- `services/pdf_service/app/routes/parse.py`: upload/URL PDF parsing and SSRF/size/PDF validation.
- `services/pdf_service/app/routes/extract.py`: LLM structured extraction.
- `services/pdf_service/core/config.py`: service settings.
- `services/pdf_service/core/llm_client.py`: OpenAI/Anthropic-compatible client.
- `services/pdf_service/core/redaction.py`: service-side secret redaction.
- `pdf_service_process.go`: desktop-managed local PDF service discovery, Python verification, readiness wait, and shutdown.
- `scripts/setup_pdf_service.sh`: one-time ignored `.venv` bootstrap for local desktop use.

## Git Hygiene

Never stage local secrets or scratch files. The following must remain ignored:

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
- `services/pdf_service/.venv/`

Before staging or pushing, run:

```bash
bash scripts/secret_scan.sh
git status --short --ignored
```

The history audit is expected to fail until an intentional destructive history rewrite is done:

```bash
bash scripts/history_secret_audit.sh
```

Do not rewrite Git history unless the user explicitly asks for that destructive operation and understands collaborator impact.

## Verification Commands

Minimum local baseline:

```bash
go test ./...
go vet ./...
cd frontend && npm test -- --run
cd frontend && npm run build
python -m pytest services/pdf_service/tests
bash scripts/secret_scan.sh
```

Last full run on 2026-07-30: passed. The managed PDF service environment is `services/pdf_service/.venv` and remains ignored.

Known audit exception: `npm audit --omit=dev` reports a React Router RSC-mode advisory against `react-router@7.18.2`. DiveEnd uses client-only `HashRouter`, not RSC. The published `react-router-dom` line currently has no clean upgrade path without a React 19/Router 8 migration; do not describe the production audit as zero-vulnerability until this is resolved upstream or migrated.

Extended baseline before release:

```bash
go build ./...
go test -race ./...
wails build
cd frontend && npm audit --omit=dev --json
node frontend/scripts/ui-smoke.mjs
```

Real Baidu sync tests are opt-in and require valid local credentials. Do not assume they can run in a clean environment.

Routine remote verification is defined in `.github/workflows/ci.yml`. It must remain credential-free: use mocks and fixtures in CI, and keep real LLM, Semantic Scholar, and Baidu checks as explicit local E2E.
