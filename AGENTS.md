# DiveEnd Agent Guide

This file is the canonical entry point for LLM agents working in this repository.

更新时间：2026-08-05

## Project Summary

DiveEnd is a local-first desktop research tool for academic paper discovery, screening, reading, translation, notes, and cloud backup.

The current implementation is:

- Frontend: React 18, TypeScript, Vite, Tailwind CSS, Radix UI, Framer Motion, Zustand, react-pdf.
- Desktop bridge: Wails v2.
- Backend: Go, `database/sql`, SQLite via `github.com/mattn/go-sqlite3`.
- Go toolchain: minimum `1.25.12`; older 1.25 patch releases contain reachable standard-library vulnerabilities.
- PDF microservice: Python FastAPI, PyMuPDF4LLM/PyMuPDF, OpenAI/Anthropic-compatible extraction.
- Sync: Google Drive primary via OAuth/Drive API, with Baidu Cloud PCS fallback.

Important correction for older docs: do not assume Marker or GORM are current implementation details. They appear in historical docs but the code has moved on.

## Current Status

Phase 1-6 feature skeletons are implemented and wired to real backend paths. The project is now in reliability, release readiness, and UX polish.

Implemented:

- Config persistence, secret merging, local seed loading, redacted display values, and a shared persistent daily LLM token budget (default 100,000,000) covering both Go calls and Python Screening extraction.
- SQLite migrations for folders, papers, translations, DeepStart, DeepRead, Screening, Sync records, and conflicts.
- DeepStart uses one topic-independent retrieval pipeline over OpenAlex, arXiv, OpenReview, and DBLP for every original/rewritten query. It merges versions by DOI, arXiv ID, OpenReview forum, and normalized title; applies conservative relevance filtering and unified ranking; preserves source provenance; retries with bounded backoff; and degrades each source independently. Focused retrieval still returns about 20 candidates, eagerly preprocesses the top 4, and handles the rest on demand.
- DeepRead paper state, PDF parsing cache, notes, translation history, weak-model low-latency questions, strong-model summaries, relevance-aware long-paper context, verbatim-grounded section evidence, cancellable AI requests, Wails asset-server PDF URL, bounded base64 fallback.
- Screening sessions, recent-session resume without automatic model reruns, managed PDF upload storage, extraction progress, session-scoped cancellation across extraction and AI decisions, provider token usage reporting, shared-budget settlement, atomic node/path/paper decision persistence, LLM decision tree, final import.
- Library folder tree, folder create/delete/rename/move, paper delete, single-paper move, batch move, managed PDF/cache path maintenance.
- Provider-neutral sync preview, background manual sync, startup sync, periodic sync, exit sync prompt, conflict detection, conflict resolution, database restore staging/apply/cancel, and explicit partial-probe failure reporting without discarding available state. Google Drive is the primary provider with resumable uploads, atomic downloads, OAuth loopback authorization, refresh-token persistence, and Baidu fallback before a run starts.
- Security hardening around local secret files, symlink/path traversal, PDF validation, SSRF, atomic file writes, bounded HTTP reads, and user-visible error redaction.
- Frontend route/page tests, Go regression tests, Python PDF route tests, browser smoke script.
- Frontend primary navigation is organized around discovery, reading, and analysis; workspace routes are lazy-loaded, DeepStart/DeepRead/Screening/Library/Sync/Settings use the shared restrained research-tool tokens, and the browser fallback and desktop bridge share the same backend wrapper contract.

Remaining risks:

- Git history still contains previously committed local secret/scratch files and at least one historical OpenAI-style key pattern. Current tree tracking is fixed, but public release requires history rewrite and token rotation.
- Packaged-window Computer Use passed on 2026-07-31: a real DeepStart run received live Wails progress events and completed a 20-paper research map; DeepRead loaded a 10-page PDF and returned a grounded weak-model answer with three verified excerpts; Sync and Settings loaded real backend state.
- Real Baidu Cloud sync passed an authorized upload/list/download/cleanup E2E on 2026-07-30, including automatic token refresh and secure persistence.
- Google Drive implementation and mocked fallback tests are complete; real Google Drive E2E awaits the user's Desktop OAuth client JSON and browser authorization. Setup is documented in `docs/GOOGLE_DRIVE_SYNC_SETUP.md`.
- The OpenAlex key passed a real rate-limit probe and multi-query product E2E on 2026-07-31. A Semantic Scholar key is now configured locally; a 2026-08-07 smoke test returned `200` directly, and the project client recovered from one `429` on its second attempt. Semantic Scholar is included in the default source set; an explicit configuration can still disable it.
- GitHub SSH read/write and `gh` CLI API access work for `qiaoyo/DiveEnd`; the authenticated account has `ADMIN` repository permission and `repo` scope.
- DeepRead page-level evidence navigation, cross-paper analysis, and high-DPI polish remain future work; library, sync, and settings have completed the current shared-token cleanup, and the main workflows pass 900 px and 720 px smoke coverage without page-level horizontal overflow.

## Reading Order

1. `README.md` for product status and development commands.
2. `AGENTS.md` for this current agent-facing summary.
3. `docs/PROJECT_MAP.md` for the exact mapping from workflow to files and tests.
4. `docs/GO_STRUCTURE.md` for Go package boundaries and feature PR ownership.
5. `docs/PAPER_SEARCH_SOURCE_EVALUATION.md` for tested search sources, agent tools, integration roles, and account prerequisites.
6. `docs/GOOGLE_DRIVE_SYNC_SETUP.md` for Google Cloud OAuth setup, local authorization, provider fallback behavior, and credential hygiene.
7. `docs/AUTONOMOUS_DEVELOPMENT_SETUP.md` for responsibility boundaries, design decisions, external account prerequisites, and autonomous execution rules.
8. `docs/USER_ACCEPTANCE_MANUAL.md` for the full real-user workflow, failure recovery, and developer verification checklist.
9. `docs/superpowers/plans/2026-06-10-code-review-remediation.md` for the large reliability/security remediation history.
10. `docs/superpowers/plans/2026-06-11-secret-history-remediation.md` before any public push or release.
11. Historical specs in `docs/superpowers/specs/` only after reading the current docs above.

If a historical document conflicts with current code or this file, prefer current code, `README.md`, `AGENTS.md`, and `docs/PROJECT_MAP.md`.

## Code Map

The repository root contains only the Wails desktop entrypoint. The backend
application and all backend tests live under `internal/app`; the legacy
`src/config`, `src/database`, and `src/llm` packages have been removed from the
active implementation.

- `main.go`: Wails app setup, embedded frontend assets, dependency composition, and lifecycle callback wiring.
- `internal/app/app.go`: application state, config application, Wails methods, translation, and paper workflow entry points.
- `internal/app/clients.go`: LLM clients and the provider-neutral search orchestrator.
- `internal/app/search_sources.go`: OpenAlex, OpenReview, and DBLP clients plus cross-source merge, filtering, retry metadata, and provenance helpers.
- `internal/app/database.go`: SQLite connection, migrations, and core persistence.
- `internal/app/deepstart*.go`: DeepStart session workflow, search rewrite, enrichment, preprocessing, background work, and cancellation.
- `internal/app/deepread*.go`: DeepRead state, parse cache, notes, managed PDF paths, and asset server.
- `internal/app/screening*.go`: Screening persistence, extraction, decision tree, and import workflow.
- `internal/app/sync*.go`, `internal/app/google_drive.go`, `internal/app/sync_provider.go`, `internal/app/baidu_pcs.go`: provider-neutral sync, Google Drive primary provider, Baidu fallback, progress, conflicts, restore flow, and PCS client.
- `internal/app/paper_import_assets.go`, `internal/app/local_file_actions.go`, `internal/app/folders_api.go`: library asset management and user file actions.
- `internal/app/wails_lifecycle.go`: package-level Wails lifecycle adapters; these are deliberately not bound as frontend methods.
- `internal/domain/`: shared Go models and workflow request/response contracts.
- `internal/contracts/`: service ports for strong/weak LLM, paper search, query rewrite, and DeepRead assistance.
- `internal/platform/`: provider-neutral bounded HTTP, JSON decoding, and secret/error redaction helpers.

Frontend:

- `frontend/src/App.tsx`: initial hydration and theme setup.
- `frontend/src/components/layout/`: router, global nav, app layout.
- `frontend/src/components/home/`: product home / research desk with quick entry points into discovery, reading, analysis, and history.
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
- `internal/app/pdf_service_process.go`: desktop-managed local PDF service discovery, Python verification, readiness wait, and shutdown.
- `scripts/setup_pdf_service.sh`: one-time ignored `.venv` bootstrap for local desktop use.

## Git Hygiene

The canonical default branch is `main`. The pre-governance unrelated `master` history is preserved read-only as `archive/legacy-master-2026-07-31`; do not use it as a development base.

Never stage local secrets or scratch files. The following must remain ignored:

- `.DS_Store`
- `auth.json`
- `config.toml`
- `test.txt`
- `downloaded.txt`
- `baidupan_test.py`
- `baidu_pcs.go.tmpl`
- `baiduyun_token.json`
- `config/google_drive_client.json`
- `google_drive_token.json`
- `config/weak_llm.json`
- `config/strong_llm.json`
- `config/stroing_llm.json`
- `config/semantic_scholar.json`
- `config/openalex.json`
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

Last full run on 2026-07-30: Go test/vet/race, all 71 frontend tests, 18 PDF service tests, frontend build, Wails build, secret scan, and the 24-state browser UI smoke at 1440, 900, and 720 px widths passed. On 2026-07-31, packaged-window DeepStart progress/completion, DeepRead PDF/grounded Q&A, Sync status, and Settings state were additionally verified with Computer Use. Google Drive provider tests and the frontend's 71 tests pass after the provider integration; real Google Drive E2E remains credential-gated. The managed PDF service environment is `services/pdf_service/.venv` and remains ignored.

On 2026-08-10, the current tree passed `go test ./...`, `go vet ./...`, all 74 frontend tests, the frontend production build, all 18 PDF service tests, `git diff --check`, and the secret scan. The browser smoke now covers 25 states, including the home dashboard, and passed at desktop and 720 px widths without page-level horizontal overflow.

Known audit exception: `npm audit --omit=dev` reports a React Router RSC-mode advisory against `react-router@7.18.2`. DiveEnd uses client-only `HashRouter`, not RSC. The published `react-router-dom` line currently has no clean upgrade path without a React 19/Router 8 migration; do not describe the production audit as zero-vulnerability until this is resolved upstream or migrated.

Extended baseline before release:

```bash
go build ./...
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
wails build
cd frontend && npm audit --omit=dev --json
node frontend/scripts/ui-smoke.mjs
```

Real Baidu and Google Drive sync tests are opt-in and require valid local credentials. Do not assume they can run in a clean environment. Follow `docs/GOOGLE_DRIVE_SYNC_SETUP.md` before attempting Google authorization.

Routine remote verification is defined in `.github/workflows/ci.yml`. It must remain credential-free: use mocks and fixtures in CI, and keep real LLM, Semantic Scholar, and Baidu checks as explicit local E2E.
