# dm-dot

**Send a message to your ChatGPT dot from anywhere.**

Your work happens across ChatGPT conversations, coding agents, and automation scripts. Your dot lives in ChatGPT. **dm-dot connects them**, so another assistant or app can pass your dot a note or delegate a task without running a second AI agent.

For example, a coding agent finishes a deployment and sends: *"The new version is live. Check for errors and tell me if anything needs attention."* Your subscribed dot can pick up that message and act using its existing tools and permissions.

[Website](https://dm-dot.ashray.xyz) · [Documentation](https://dm-dot.ashray.xyz/docs) · [Self-hosting guide](https://dm-dot.ashray.xyz/docs/deployment)

## How it works

```text
Another ChatGPT chat ──┐
Coding agent (MCP) ────┼──→ dm-dot ── signed event ──→ Your ChatGPT dot
Script or app (REST) ──┘
```

- **From an AI assistant:** Call the `send_to_dot` MCP tool with a message.
- **From an app or script:** Call the authenticated `POST /v1/messages` REST endpoint.
- **In ChatGPT:** Your dot subscribes to the `message.created` event and handles incoming messages through its normal workflow.

It's a small, self-hosted bridge, not another model runtime. You don't need an OpenAI API key. Each deployment supports **one subscribed dot**, and messages are delivered through signed webhooks.

## Get started

1. [Deploy your own API](https://dm-dot.ashray.xyz/docs/deployment) with Vercel, Upstash Redis, and private credentials.
2. [Connect the MCP endpoint to ChatGPT](https://dm-dot.ashray.xyz/docs/getting-started), then ask your dot to subscribe to `message.created`.
3. Send a message from another ChatGPT conversation, an MCP agent, or a backend script.

For a script or backend, send a message like this:

```bash
curl -X POST 'https://YOUR-API-HOST/v1/messages' \
  -H "Authorization: Bearer ${DM_DOT_REST_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"message":"The deployment is complete. Please review the results."}'
```

Keep your API key on the server, never in browser code. See the [REST API reference](https://dm-dot.ashray.xyz/docs/rest-api) for authentication and responses.

## What to expect

**This is one-way messaging.** dm-dot does not provide a reply channel, message history, or a durable queue. A successful send means ChatGPT accepted the webhook, **not** that your dot has finished (or even started) processing it. Your dot must explicitly subscribe, and your ChatGPT account must support MCP Events subscriptions.

For more detail, see:

- [Getting started](https://dm-dot.ashray.xyz/docs/getting-started) and [self-hosting](https://dm-dot.ashray.xyz/docs/deployment)
- [MCP tools & Events](https://dm-dot.ashray.xyz/docs/protocol) and [REST API](https://dm-dot.ashray.xyz/docs/rest-api)
- [Architecture](https://dm-dot.ashray.xyz/docs/architecture), [security](https://dm-dot.ashray.xyz/docs/security), and [troubleshooting](https://dm-dot.ashray.xyz/docs/troubleshooting)

## Development

Requires Go 1.26+, Node.js 22+, and Bun 1.4+ to run the Go checks and build the documentation site:

```bash
make docs-install
make check
```

See the [development guide](https://dm-dot.ashray.xyz/docs/development), [contribution guidelines](CONTRIBUTING.md), and [security policy](SECURITY.md).

Licensed under [Apache-2.0](LICENSE). Based on [amxv/go-mcp-template](https://github.com/amxv/go-mcp-template). Independent project, not affiliated with OpenAI.
