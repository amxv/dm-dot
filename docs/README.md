# Documentation

`dm-dot` is a one-way MCP Events bridge from any authenticated MCP client to a **single opted-in ChatGPT dot**.

**Start here:** [Getting started](getting-started.md), followed by [Deployment](deployment.md).

## Guides

| Document | Description |
| --- | --- |
| [Getting started](getting-started.md) | Connect the server, subscribe a dot, and send the first message |
| [Deployment & configuration](deployment.md) | Vercel project, Redis persistence, credentials, redeploys |
| [Architecture](architecture.md) | Components, boundaries, storage, message lifecycle |
| [REST API](rest-api.md) | Authenticated HTTP endpoint, request schema and errors |
| [MCP interface](protocol.md) | Exact tool and event methods, payloads, errors, version compatibility |
| [Security](security.md) | Authorization, trust model, callback verification and signing |
| [Troubleshooting](troubleshooting.md) | Discovery, event-source indexing, Redis, and webhook failures |
| [Development](development.md) | Tests, code layout, contribution practices, release considerations |
| [Measuring latency](latency.md) | Separate HTTP delivery time from dot execution time |

## What the project does not do

- It does **not** provide an API to initiate arbitrary regular ChatGPT chats.
- It does **not** create a dot, grant it tools, or select between dots.
- It does **not** store messages, guarantee delivery, or provide a reply path.
- It does **not** interpret received messages as commands with higher privileges than the dot's existing instructions.

### External references

- [OpenAI MCP Events](https://developers.openai.com/plugins/build/mcp-events) (the protocol integration supported by ChatGPT)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [Standard Webhooks](https://github.com/standard-webhooks/standard-webhooks)

Return to the [project README](../README.md).
