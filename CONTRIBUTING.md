# Contributing to dm-dot

Thanks for your interest in improving this project.

## Project principles

- Keep the core small and auditable.
- Maintain exactly one target dot in v1 and one public MCP tool, `send_to_dot`.
- Keep sending semantics consistent between MCP and REST.
- Treat event payloads as untrusted data and preserve existing authentication, HTTPS callback validation, and webhook signing.
- Prefer clear docs and tests over dependencies for their own sake.

## Work locally

Install Go 1.26+, Node.js 22+, and Bun 1.4+, then run:

```bash
go test ./...
go vet ./...
make docs-install
make check
```

The landing page is `docs/src/pages/index.astro` and the docs are in `docs/src/content/docs/`. The original Astro + ZueDocs app builds the website into `docs/dist/` for a **separate** Vercel project. The repository-root project deploys only the Go API functions. Modify ZueDocs source, not generated output.

Before submitting a change, run `make check`, ensure the diff is focused, and update the relevant reference docs. Do not include API tokens, private URLs, personal data or production subscriptions.

## Issues and security

Use issues or pull requests for ordinary bugs and improvements. For sensitive vulnerabilities, **do not** post a public issue; see [SECURITY.md](SECURITY.md).
