# Internal Packages

This directory contains code that is shared by the Wails composition layer and
the feature implementations. Packages here must not import the root `main`
package.

Current packages:

- `domain`: JSON/Wails-facing data contracts and workflow request/response types.
- `contracts`: narrow service ports used by application orchestration.
- `platform/httpx`: bounded HTTP body and JSON decoding safeguards.
- `platform/redaction`: provider-neutral credential and error redaction.

Planned packages are tracked in [docs/GO_STRUCTURE.md](../docs/GO_STRUCTURE.md).
