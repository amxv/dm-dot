# dm-dot

**A direct line to your ChatGPT dot.**

Send messages from another ChatGPT conversation, any MCP-capable agent, or an application using an authenticated REST API. One small bridge, one subscribed dot, no extra model runtime.

**Website & ZueDocs:** [dm-dot.ashray.xyz](https://dm-dot.ashray.xyz) · **MCP & REST API:** [dm-dot.vercel.app](https://dm-dot.vercel.app) · **License:** [Apache-2.0](LICENSE)

> **Source-release note:** The GitHub repository is intentionally **private for now**. The code and documentation are being prepared for a future open-source release. No public repository access or unauthenticated message sending is implied.

## What it does

```text
 ChatGPT chat ──┐
 External agent ├──→ dm-dot ── signed MCP event ──→ your dot
 Your backend ──┘         ↳ one active subscription
```

- `send_to_dot(message)`: the only exposed **MCP tool**.
- `POST /v1/messages`: authenticated **REST API** for any backend or script.
- `message.created`: webhook event that the dot explicitly subscribes to.
- Go, official MCP SDK, stateless Vercel functions, and Upstash Redis for subscription persistence.

A successful send confirms ChatGPT **accepted the webhook**, not that the dot has completed processing. This version has no reply channel, durable queue, or message history.

## Quick start

1. Deploy a private `dm-dot` instance to Vercel with Upstash Redis and secret environment variables. [Deployment guide](docs/src/content/docs/deployment.md)
2. Connect the authenticated MCP endpoint to ChatGPT and rescan for `send_to_dot` and `message.created`.
3. Ask your dot to subscribe to `message.created`.
4. Send a message through the MCP tool or REST API.

**REST example** (run from a trusted server; keep your bearer key out of client-side code):

```bash
curl -X POST 'https://YOUR-VERCEL-HOST/v1/messages' \
  -H "Authorization: Bearer ${DM_DOT_REST_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"message":"Keep me posted when the deployment finishes."}'
```

The REST API returns HTTP `202` with a generated `event_id` when the webhook is acknowledged.

## Reference

The technical details live in [ZueDocs](https://dm-dot.ashray.xyz/docs), restored from the original [`go-mcp-template`](https://github.com/amxv/go-mcp-template). The canonical Markdown is in [`docs/src/content/docs/`](docs/src/content/docs/), rendered by Astro + ZueDocs with its built-in syntax highlighting, code-copy buttons, Mermaid diagrams, search, and navigation.

| Guide | What you'll find |
| --- | --- |
| [Getting started](docs/src/content/docs/getting-started.md) | First connection and subscription |
| [Deployment](docs/src/content/docs/deployment.md) | Vercel, Redis, secrets, and configuration |
| [REST API](docs/src/content/docs/rest-api.md) | Auth, request schema, examples, errors |
| [MCP & Events](docs/src/content/docs/protocol.md) | RPC methods, discovery, webhook payloads |
| [Architecture](docs/src/content/docs/architecture.md) | Message flow, storage, and trade-offs |
| [Security](docs/src/content/docs/security.md) | Trust boundaries and credential handling |
| [Troubleshooting](docs/src/content/docs/troubleshooting.md) | Event-source discovery and delivery failures |
| [Latency](docs/src/content/docs/latency.md) | Measuring sender-to-dot execution |
| [Development](docs/src/content/docs/development.md) | Tests and project structure |

## Development

Requires **Go 1.26+**, **Node.js 22+**, and **Bun 1.4+** for the ZueDocs app.

```bash
make docs-install
make check
```

The landing page is [`docs/src/pages/index.astro`](docs/src/pages/index.astro); all reference pages use ZueDocs layouts and enhancements. Two Vercel projects deploy independently: `dm-dot-docs` (root `docs/`) for the website and `dm-dot` (repository root) for the Go MCP/REST functions. Docs-only changes don't redeploy the API. See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).

Built from [amxv/go-mcp-template](https://github.com/amxv/go-mcp-template). Independent project, not affiliated with OpenAI.
