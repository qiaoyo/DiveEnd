# Platform

Platform packages contain provider-neutral safeguards and operating-system
integration. They must not import the root `main` package or feature domains.

- `httpx`: bounded HTTP body reads and strict single-value JSON decoding.
- `redaction`: credential/error redaction for logs and user-visible messages.

The root `platform_compat.go` wrappers exist only for legacy package-main code
and tests. New code should import these packages directly.
