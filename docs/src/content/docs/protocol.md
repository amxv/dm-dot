---
title: "MCP & Events"
description: "Tool and event methods, schemas, signed webhook delivery."
order: 4
category: Reference
---

All MCP JSON-RPC methods share one authenticated Streamable HTTP endpoint:

```text
POST https://YOUR-VERCEL-HOST/mcp?key=YOUR-MCP-API-KEY
```

`MCP_API_KEY` is a **shared connection credential**. Do not paste its actual value in examples, issue reports, terminal history, or logs. Without a valid key, requests are rejected before reaching the MCP server.

This page covers MCP Events. Applications using normal HTTP can call the **separate authenticated** [REST API](/docs/rest-api) at `POST /v1/messages`; local agents can use the [native CLI](/docs/cli), which calls that REST endpoint.

## Public MCP tool

Only one tool is registered in `tools/list`:

### `send_to_dot`

| Input | Type | Required | Description |
| --- | --- | --- | --- |
| `message` | string | Yes | The message to pass to the subscribed dot; 1–16,384 UTF-8 bytes after trimming surrounding whitespace |

Example `tools/call` parameters:

```json
{
  "name": "send_to_dot",
  "arguments": {
    "message": "Please review the deployment checklist and report any blockers."
  }
}
```

On successful webhook delivery, the tool returns text of the form:

```text
ChatGPT accepted event evt_<generated-id> for asynchronous processing by your dot.
```

This means a webhook returned HTTP `2xx`. It **does not** mean the dot finished or even began reasoning about the message. If Redis is unavailable, there is no active subscription, input is invalid, or the callback fails, the tool returns a normal MCP tool result with `isError: true` and an explanatory text message.

The implementation sets the tool's `readOnlyHint` client annotation as a compatibility convention. **This is not a pure read:** calling it transmits a message and may trigger downstream actions by a dot.

## Event discovery

The same endpoint exposes **protocol methods** (not additional tools):

| JSON-RPC method | Function |
| --- | --- |
| `events/list` | Advertise the `message.created` event |
| `events/subscribe` | Register or refresh a single webhook subscription |
| `events/unsubscribe` | Remove the matching subscription |

The server advertises `"events": {}` in capabilities from both MCP legacy `initialize` and MCP 2.0 `server/discover`. MCP 2.0's `server/discover` uses the `2026-07-28` protocol and requires the appropriate `Mcp-Method` header and per-request metadata for valid discovery probes. The official Go SDK owns protocol negotiation; the event capability is added by `pkg/server/server.go`.

### `events/list`

The server offers one unfiltered event. The relevant result shape is:

```json
{
  "events": [
    {
      "name": "message.created",
      "description": "A new message addressed to the single configured dot.",
      "delivery": ["webhook"],
      "inputSchema": {
        "type": "object",
        "properties": {},
        "additionalProperties": false
      },
      "payloadSchema": {
        "type": "object",
        "properties": {
          "message": { "type": "string" }
        },
        "required": ["message"],
        "additionalProperties": false
      }
    }
  ],
  "nextCursor": null
}
```

No event filters, second event type, polling mode, or stream delivery mode are supported.

### `events/subscribe`

ChatGPT calls this method when a dot explicitly requests a subscription. Example parameters (illustrative placeholders only):

```json
{
  "name": "message.created",
  "arguments": {},
  "delivery": {
    "mode": "webhook",
    "url": "https://receiver.example.com/unique-callback",
    "secret": "whsec_<base64-signing-key>"
  },
  "ttlMs": 3600000,
  "cursor": null
}
```

Processing includes validation of the event name, empty arguments, HTTPS callback URL, and signing secret; a signed callback challenge must pass before the subscription becomes active. The `whsec_` value must decode to a **24–64-byte key**.

On success, the result includes:

```json
{
  "id": "sub_<deterministic-id>",
  "refreshBefore": "2026-11-03T00:00:00Z",
  "cursor": null,
  "truncated": false
}
```

The example expiration above is illustrative, not a live server value. The actual expiration is a UTC timestamp.

- **Default TTL:** 7 days when `ttlMs` is omitted.
- **Requested TTL:** At least 60 seconds; capped at 7 days. `ttlMs: null` also results in a finite 7-day subscription in this implementation.
- **Refresh:** Calling subscribe again for the same callback updates the existing subscription, including a replacement signing secret. The result returns a new `refreshBefore`.
- **Single subscriber:** Another callback is rejected while an active different callback exists.
- **Storage:** Redis key `dm-dot:subscription:v1`, with an expiry matching the subscription lifetime.

An idempotent subscription ID is derived from the shared authenticated principal (`MCP_API_KEY`), callback URL, event name, and empty event arguments. As a result, changing the MCP API key affects subscription identity; see [Deployment](/docs/deployment#updating-or-recovering).

### `events/unsubscribe`

Use the same `name`, `arguments`, and `delivery.mode`/`delivery.url` that identify the subscription. The signing secret can be omitted. Removing a matching subscription returns `{}`; repeated unsubscription is harmless. A different callback does not clear the active subscription.

## Webhook payload

`send_to_dot` generates one signed event object (example values):

```json
{
  "eventId": "evt_0123456789abcdef",
  "name": "message.created",
  "timestamp": "2026-11-03T10:30:00Z",
  "data": {
    "message": "Please review the deployment checklist."
  },
  "cursor": null
}
```

The event's `timestamp` is set by `dm-dot` when it builds the event body, not when the dot begins processing it. It is an ISO 8601 UTC timestamp; the actual implementation uses nanosecond precision when available.

The server signs the **exact serialized body bytes** using Standard Webhooks and sends:

| Header | Meaning |
| --- | --- |
| `Content-Type: application/json` | JSON body |
| `webhook-id` | Matches the `eventId` |
| `webhook-timestamp` | Unix seconds at signing |
| `webhook-signature` | `v1,<base64-HMAC-SHA256>` |
| `X-MCP-Subscription-Id` | Subscription ID from `events/subscribe` |

For transient failures, the server retries up to **three total attempts**, preserving the event ID and serialized body. It does not store undelivered events for future retries.

## Errors and edge cases

| Failure | Response |
| --- | --- |
| Missing `MCP_API_KEY` environment variable | HTTP `503` |
| Wrong or missing query-string key | HTTP `401` |
| Invalid event, filter, callback, signing secret, or TTL | JSON-RPC `-32602` |
| Existing subscription for a different callback | JSON-RPC `-32000` |
| Callback verification failure | JSON-RPC `-32015` (`CallbackEndpointError`) |
| Subscription storage unavailable | JSON-RPC `-32603` |
| No subscription or failed message delivery | MCP tool result with `isError: true` |

See [Security](/docs/security) for callback restrictions, [Architecture](/docs/architecture) for the life cycle, and the [OpenAI MCP Events documentation](https://developers.openai.com/plugins/build/mcp-events) for the broader event contract.
