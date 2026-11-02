# Contributing to dm-dot

Thanks for your interest in improving this project.

The repository is currently private. This document prepares for a future source release; it is **not** an invitation to make the repository public or change access settings.

## Project principles

- Keep the core small and auditable.
- Maintain exactly one target dot in v1 and one public MCP tool, `send_to_dot`.
- Keep sending semantics consistent between MCP and REST.
- Treat event payloads as untrusted data and preserve existing authentication, HTTPS callback validation, and webhook signing.
- Prefer clear docs and tests over dependencies for their own sake.

## Work locally

Install Go 1.26+, Node.js 22+ and npm, then run:

```bash
go test ./...
go vet ./...
npm ci
npm run check
```

The website is generated from `web/` and `docs/` into ignored `public/` files. Modify the source and rebuild rather than editing the generated output.

Before submitting a change, run `make check`, ensure the diff is focused, and update the relevant reference docs. Do not include API tokens, private URLs, personal data or production subscriptions.

## Issues and security

Use issues or pull requests for ordinary bugs and improvements once the repository is available. For sensitive vulnerabilities, **do not** post a public issue; see [SECURITY.md](SECURITY.md).
