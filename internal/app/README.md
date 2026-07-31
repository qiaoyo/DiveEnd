# Application Package

`internal/app` is the only Wails-bound backend package. It owns the exported
`App` facade and currently contains the existing workflow implementations while
their private dependencies are being extracted into domain packages.

## File ownership

- `app*.go`, `wails_*.go`: lifecycle, Wails facade, and generated-model aliases.
- `clients*.go`, `llm_*.go`: LLM and search client compatibility layer.
- `database*.go`: SQLite connection, migrations, and persistence methods.
- `deepstart*.go`: discovery workflow and background processing.
- `deepread*.go`: reader state, PDF paths, and reader AI.
- `screening*.go`: screening sessions, extraction, and decision tree.
- `folders*.go`, `folder_utils.go`, `paper_*.go`, `local_file_actions*.go`: library assets.
- `sync*.go`, `baidu_pcs*.go`: cloud sync and Baidu PCS.
- `secure_file.go`, `platform_compat.go`, `redaction_test.go`: shared hardening during migration.

The package is intentionally a transitional aggregate, not the final domain
architecture. New cross-domain behavior belongs in an explicit service port;
new provider code should be placed in the target package described in
[`docs/GO_STRUCTURE.md`](../../docs/GO_STRUCTURE.md).
