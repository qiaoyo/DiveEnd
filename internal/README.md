# Internal Packages

This directory contains the application package, shared contracts, and
provider-neutral platform helpers. Packages here must not import the root
`main` package.

Current packages:

- `app`: Wails facade, lifecycle adapters, and the current transitional
  backend aggregate. Its files are grouped by workflow prefix until each
  workflow has a stable service boundary.
- `domain`: JSON/Wails-facing data contracts and workflow request/response types.
- `contracts`: narrow service ports used by application orchestration.
- `platform/httpx`: bounded HTTP body and JSON decoding safeguards.
- `platform/redaction`: provider-neutral credential and error redaction.

The next package extractions are tracked in
[docs/GO_STRUCTURE.md](../docs/GO_STRUCTURE.md). Do not add new business
logic to the repository root or create a second `main` package under `internal`.
