# Deployment & configuration

`dm-dot` is designed for **Vercel Go Functions** backed by **Upstash Redis**. It uses stateless Streamable HTTP; a Redis record retains the one webhook subscription between requests and redeployments.

## Prerequisites

- A Vercel account/project with access to the repository (including private-repository permissions, when applicable).
- An Upstash Redis database with its HTTPS REST endpoint and REST token. Prefer a **dedicated** database because it stores a webhook signing secret.
- A long, random API key stored in your password manager or secret store.
- Go 1.26+ for local development and testing; the Vercel Go builder installs its own toolchain.

The baseline architecture does not require Docker, a VPS, an OAuth identity provider, or an OpenAI Platform API key.

## 1. Provision Redis

Create an Upstash Redis database through the [Upstash console](https://console.upstash.com/) (or its official API). For a small personal deployment, select a supported free tier if available. Prefer **TLS enabled**, **eviction disabled**, and a region near your Vercel function.

Find the database's REST URL and token. **Do not put the REST token in a tracked `.env` file or documentation.** Pricing, limits, and free-tier availability are determined by Upstash, not this repository.

`dm-dot` uses the Upstash REST API and does not need a Redis TCP connection string. The application owns one key:

```text
dm-dot:subscription:v1
```

This key holds the subscription callback URL, HMAC signing secret, ID, and expiration. Its Redis TTL follows the granted subscription duration. **It does not store message bodies.**

## 2. Create the Vercel project

Import the GitHub repository in Vercel, or use the CLI from the repository root:

```bash
vercel project create dm-dot
vercel link --yes --project dm-dot
```

Configure:

| Setting | Value |
| --- | --- |
| Root directory | `.` (repository root) |
| Framework preset | **Other** |
| Source branch | `main` |
| Function entry point | `api/mcp.go` |
| Public MCP route | `/mcp` (rewritten to `/api/mcp`) |
| Max function duration | 60 seconds (configured in `vercel.json`) |

When linking a private repository, grant the Vercel GitHub integration access to **this repository**, not unnecessarily to every private repository.

The checked-in `vercel.json` enables automatic Git deployments for `main` and disables automatic previews on other branches using the `**` branch pattern. Manual deployments remain possible. There is **no public documentation deployment** in this project.

## 3. Set Production secrets

| Variable | Required | Description |
| --- | --- | --- |
| `MCP_API_KEY` | Yes | Shared bearer-equivalent secret passed in `?key=` on the MCP URL |
| `DM_DOT_REST_API_KEY` | Yes, for REST | Separate bearer token authorizing `POST /v1/messages` |
| `UPSTASH_REDIS_REST_URL` | Yes | HTTPS endpoint for the dedicated Redis REST API |
| `UPSTASH_REDIS_REST_TOKEN` | Yes | REST token to read/write the subscription key |

Generate a random `MCP_API_KEY` of at least 32 bytes using a secure generator, and save it in a trusted secret manager. Configure each value as a **sensitive Production secret** in Vercel. For example, the Vercel CLI prompts for each value:

```bash
vercel env add MCP_API_KEY --environment production --sensitive
vercel env add DM_DOT_REST_API_KEY --environment production --sensitive
vercel env add UPSTASH_REDIS_REST_URL --environment production --sensitive
vercel env add UPSTASH_REDIS_REST_TOKEN --environment production --sensitive
```

The CLI also supports piping values through standard input for automation. Avoid passing tokens in command-line arguments, where they may end up in shell history or process listings.

**Important:** The API key is needed again to connect an MCP client. Keep a recoverable copy in your password manager or OS keychain. Do not rely on retrieving a sensitive Vercel environment variable later.

## 4. Deploy and verify

```bash
make check
vercel deploy --prod --yes
```

The public route follows this pattern:

```text
https://YOUR-VERCEL-HOST/mcp?key=YOUR-MCP-API-KEY
```

Expected outcomes:

1. Without `?key=`, the MCP endpoint responds with **HTTP 401** (when `MCP_API_KEY` is configured).
2. If `MCP_API_KEY` is missing, it responds with **HTTP 503** and refuses requests.
3. With a valid key, MCP `initialize` and `server/discover` advertise `events`, `tools/list` exposes `send_to_dot`, and `events/list` exposes `message.created`.
4. Before a dot subscribes, calling `send_to_dot` returns a **tool error** saying there is no active subscription. That is expected; it also confirms the application can query Redis.
5. `POST /v1/messages` requires `Authorization: Bearer <DM_DOT_REST_API_KEY>`; with no subscription it returns `409`, and after delivery succeeds it returns `202`.

Use [Getting started](getting-started.md) to connect ChatGPT and subscribe the dot. Follow [Troubleshooting](troubleshooting.md) if the platform's event source list lags behind the plugin UI.

## Updating or recovering

- **Changing secrets:** Update the Vercel Production secret(s) and **redeploy**. Redeploying is required for the running function to receive new values.
- **Rotating `MCP_API_KEY`:** Every connected MCP client must update its authenticated URL. Because the subscription ID incorporates this key, reconnect/re-subscribe the dot after rotation.
- **Replacing Redis:** Recreate the variables and have the dot subscribe again. Existing subscriptions are not automatically migrated.
- **Refreshing a subscription:** The client calls `events/subscribe` again before `refreshBefore`. No background cron refresh is provided by `dm-dot`.
- **Separating environments:** Use independent Redis databases and different MCP API keys for production and staging. Never point both environments at the same single-subscription key.

### Deployment protection

Vercel project-wide authentication, an upstream access gateway, or private network protection can prevent ChatGPT from reaching `/mcp`. The application-level `?key=` check **does not bypass** those layers. Verify that external MCP clients can actually access the endpoint while the application still enforces its own secret. Do not disable account or workspace security controls without evaluating the consequences.

Next: [Security](security.md) · [Troubleshooting](troubleshooting.md).
