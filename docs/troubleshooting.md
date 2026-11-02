# Troubleshooting

Identify which step failed: **plugin discovery**, **event-source indexing**, **subscription registration**, **Redis**, **webhook acknowledgment**, or **the dot's later execution**. These are separate systems.

## Tools appear, but no webhook events are listed

`send_to_dot` is registered through the official MCP Go SDK. The event catalog is implemented separately. Make sure the deployed server advertises `events: {}` in **both** legacy `initialize` and MCP 2.0 `server/discover`, and that `events/list` returns `message.created` with webhook delivery.

MCP 2.0 (`2026-07-28`) requires its own method header and protocol metadata. A handcrafted probe missing `Mcp-Method: server/discover` will return HTTP `400` even on a correct server. Follow the [MCP reference](protocol.md#event-discovery) instead of treating every failed manual probe as a server defect.

Rescan the MCP connection after a deployment that changes advertised capabilities.

## Events show in the plugin UI, but the dot says there are no event sources

This happened during development: `message.created` appeared in ChatGPT's plugin UI while `list_event_sources` omitted `dm-dot` and `discover_webhook_schema` returned **“This connector has no available webhook events.”** After the plugin's event catalog had time to propagate, subscription became available without changing the server again.

First confirm the deployed MCP 2.0 discovery response is correct, rescan the plugin, and retry the dot subscription after the platform updates its event catalog. Do **not** delete the subscription or rotate keys merely because the UI and the event-source index temporarily disagree. If it persists, report the connector ID, UTC time, and non-secret response details to the platform provider; never share an authenticated MCP URL.

## Cannot subscribe: callback challenge failed

`events/subscribe` requires a valid public HTTPS callback, Standard Webhooks signing secret, and exact challenge echo. Callback verification returns a protocol error if the remote callback doesn't accept and echo the challenge correctly.

Verify external HTTPS reachability and ChatGPT's event support for the selected dot. Do not disable signing, TLS verification, or private-IP protections to force a subscription through.

## A different dot is already subscribed

Only **one callback** is active per deployment. A new callback cannot replace it until the existing one unsubscribes or expires. Ask the former dot to unsubscribe, or carefully clear the single Redis key if recovering a subscription you can no longer reach. Do not clear unrelated Redis data.

## `send_to_dot` or REST says no active subscription

The plugin may be connected without the dot having subscribed. Ask the dot to subscribe to `message.created` and confirm that registration succeeded. Subscriptions are finite (maximum seven days per grant); ChatGPT needs to refresh them before expiry. There is no queue for messages sent while the dot is unsubscribed.

## REST returns 401, 409, or 503

| Error | Likely cause | Check |
| --- | --- | --- |
| `401 unauthorized` | Wrong REST bearer key | Use `DM_DOT_REST_API_KEY`, not `MCP_API_KEY` |
| `409 no_subscription` | Dot not subscribed/expired | Subscribe or refresh in the dot |
| `503 not_configured` | Missing REST API key | Configure Vercel Production secret, redeploy |
| `503 storage_unavailable` | Incorrect Redis REST credentials or unreachable Upstash | Inspect Vercel env and Upstash health |
| `502 delivery_failed` | Webhook failed or returned non-success | Check remote callback and transient errors |

See [REST API](rest-api.md) for the complete status mapping.

## Webhook accepted, but the dot has not done anything

An HTTP `202` from the REST API, or an event ID from the MCP tool, confirms **ChatGPT webhook acceptance**. The dot works asynchronously and may be subject to platform scheduling, permissions, and its own tool calls. There is no built-in completion receipt. Test with a unique message ID and ask the dot to acknowledge via an already-authorized channel.

See [Measuring latency](latency.md) for an accurate timing method.

## Deployment doesn't reflect source changes

Verify the production alias points at the expected deployment and Git commit. Changes to sensitive environment variables require a **new deployment**. The project intentionally skips Git-created preview deployments on feature branches; manual previews are still possible.

## Security and duplicates

The authenticated MCP URL includes a query-string secret and should not appear in issue reports. Keep REST keys out of browser code. Transient retries can create duplicate webhook deliveries; never assume exactly-once execution.
