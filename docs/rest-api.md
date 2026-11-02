# REST API

Send a message to the **same subscribed ChatGPT dot** using ordinary HTTPS. The REST endpoint is useful for other AI agents, backend services, scripts, CI workflows, or applications that cannot speak MCP.

**Endpoint:** `POST https://YOUR-VERCEL-HOST/v1/messages`

## Authentication

Every request needs a dedicated REST bearer token in the `Authorization` header:

```http
Authorization: Bearer <DM_DOT_REST_API_KEY>
Content-Type: application/json
```

Set `DM_DOT_REST_API_KEY` as a strong, random **Production secret in Vercel**. It is separate from the MCP connection's `MCP_API_KEY` and cannot be used interchangeably.

Do not embed the REST token in a browser bundle, mobile app binary, public GitHub Actions log, or webpage. For untrusted clients, call the API through your own authenticated backend. The REST endpoint intentionally does **not** enable permissive browser CORS.

## Request body

```json
{
  "message": "Please review the latest deployment and flag any issues."
}
```

`message` is a required string, trimmed of leading/trailing whitespace, containing between **1 and 16,384 UTF-8 bytes**. The JSON request body has a 32 KiB limit. Unknown JSON fields are rejected; there is no `dot_id` or `recipient` field because v1 supports exactly one active subscription.

## Example: curl

Use your own deployment hostname and export the API key **securely in your shell** (for example, from your OS keychain), rather than typing it into terminal history.

```bash
curl --fail-with-body \
  -X POST 'https://YOUR-VERCEL-HOST/v1/messages' \
  -H "Authorization: Bearer ${DM_DOT_REST_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"message":"A new build is ready for review."}'
```

## Example: JavaScript / TypeScript (server-side)

```ts
const response = await fetch('https://YOUR-VERCEL-HOST/v1/messages', {
  method: 'POST',
  headers: {
    Authorization: `Bearer ${process.env.DM_DOT_REST_API_KEY}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({ message: 'A new build is ready for review.' }),
});

const result = await response.json();
if (!response.ok) {
  throw new Error(`dm-dot HTTP ${response.status}: ${result.error?.code}`);
}
console.log(result.event_id);
```

## Example: Go

```go
payload := strings.NewReader(`{"message":"A new build is ready for review."}`)
req, err := http.NewRequest(http.MethodPost, "https://YOUR-VERCEL-HOST/v1/messages", payload)
if err != nil { log.Fatal(err) }
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Authorization", "Bearer "+os.Getenv("DM_DOT_REST_API_KEY"))

client := &http.Client{Timeout: 15 * time.Second}
resp, err := client.Do(req)
if err != nil { log.Fatal(err) }
defer resp.Body.Close()
fmt.Println(resp.Status)
```

## Response

When ChatGPT's webhook returns HTTP `2xx`, the REST API returns **HTTP 202 Accepted**:

```json
{
  "status": "accepted",
  "event_id": "evt_0123456789abcdef"
}
```

This is **not a receipt from the dot**. The event ID identifies the webhook delivery. ChatGPT may process the event asynchronously; there is no status polling endpoint, completion callback, reply tool, or message history in this version.

### Error responses

Errors are JSON objects in this form:

```json
{
  "error": {
    "code": "no_subscription",
    "message": "No active dot subscription"
  }
}
```

| HTTP status | Error code | Meaning |
| --- | --- | --- |
| `400` | `invalid_json`, `invalid_body`, `invalid_message` | Malformed JSON or invalid message |
| `401` | `unauthorized` | Missing or incorrect REST bearer token |
| `405` | `method_not_allowed` | Only POST is supported |
| `409` | `no_subscription` | No dot has an active webhook subscription |
| `413` | `payload_too_large` | Request exceeds the JSON body limit |
| `415` | `unsupported_media_type` | Expected `Content-Type: application/json` |
| `502` | `delivery_failed` | ChatGPT did not acknowledge webhook delivery |
| `503` | `not_configured`, `storage_unavailable` | REST secret missing or Redis unavailable |

Responses include `Cache-Control: no-store`. Incorrect credentials never appear in the response.

## Delivery and retries

The REST endpoint uses the **same** `SendMessage` delivery logic as `send_to_dot`. A callback may be attempted up to three times for transient network failures, HTTP 429, or HTTP 5xx. Those attempts share one event ID. There is **no durable outbox**, so after ambiguous failures the caller may not know whether ChatGPT accepted the event.

Do not assume retries are exactly-once. For application-level deduplication, include a unique request identifier inside your message and have your receiving workflow recognize repeats. An `Idempotency-Key` HTTP header is **not implemented** in v1.

## Security considerations

- Keep the bearer token **server-side** and use HTTPS.
- Do not turn the REST endpoint into a publicly callable browser proxy without your own authentication and rate limiting.
- Anyone with the REST key can wake the subscribed dot; share it only with trusted applications.
- Messages should be treated as **untrusted inputs** by the dot. Sending a message does not authorize unrelated actions.

See [Security](security.md) and [MCP & Events reference](protocol.md) for the underlying protocol.
