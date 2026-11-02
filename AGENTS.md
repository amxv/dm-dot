# dm-dot

- This is a single-dot, one-way MCP Events bridge based on `go-mcp-template`.
- Keep the public tool surface at exactly `send_to_dot`. The events protocol additionally implements `events/list`, `events/subscribe`, and `events/unsubscribe` on the same MCP endpoint.
- Use official Go MCP SDK and stateless Streamable HTTP. Persist subscriptions in a dedicated Upstash Redis REST database.
- Vercel's `api/mcp.go` must import `pkg/server` (not a Go `internal/` package).
- Preserve fail-closed `MCP_API_KEY` authentication, callback verification, private-IP/SSRF protection, and Standard Webhooks HMAC signing.
- Follow the local MCP tool annotation convention: `ReadOnlyHint: true`, omit `OpenWorldHint`.
- Keep all credentials in Vercel environment variables, never in Git.
- Run `make check` after changes. Production branch is `main`; previews are disabled.
