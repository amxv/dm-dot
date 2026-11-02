# dm-dot

One-way messages to a single ChatGPT dot. A private Go MCP server built from [`amxv/go-mcp-template`](https://github.com/amxv/go-mcp-template).

**One MCP tool:** `send_to_dot(message)`.

The receiver is a dot that explicitly subscribes to `message.created` using [OpenAI MCP Events](https://developers.openai.com/plugins/build/mcp-events). Calling `send_to_dot` sends a signed webhook to that subscription. There are no other tools, no routing arguments, and no reply mechanism.

## How it works

1. Add the deployed MCP endpoint as a private ChatGPT plugin and rescan its tools/events.
2. In your dot, ask it to subscribe to `message.created` on `dm-dot` and process incoming messages as context.
3. From a regular ChatGPT conversation or another MCP client, call `send_to_dot` with a message.
4. `dm-dot` POSTs the message as a signed `message.created` webhook. ChatGPT acknowledges receipt asynchronously.

The tool confirms **webhook acceptance**, not completion by the dot. An event's message is user-generated data, not privileged instructions. MCP Events must be enabled for your account and dot.

## Deploy

Requires **Go 1.26+**, Vercel (Go Functions), and a dedicated **Upstash Redis** database for the persistent subscription record. Vercel functions are stateless; Redis is needed so a subscription survives instance restarts and redeploys.

Set these *Production* secrets on the Vercel project:

| Variable | Purpose |
| --- | --- |
| `MCP_API_KEY` | Long random key securing the MCP URL |
| `UPSTASH_REDIS_REST_URL` | HTTPS REST endpoint of the dedicated Redis DB |
| `UPSTASH_REDIS_REST_TOKEN` | Token for that Redis DB |

The endpoint is `https://<vercel-host>/mcp?key=<MCP_API_KEY>`.
Don't commit or paste the authenticated URL in public places: query parameters can appear in access logs. Without the API key the endpoint fails closed; without Redis no subscription can be registered or sent to.

```sh
make check
vercel link --yes --project dm-dot
vercel --prod --yes
```

`vercel.json` permits Git auto-deployments on `main`, not preview branches. No public docs site or CLI is bundled.

## Wire up your dot

Connect `dm-dot` in ChatGPT Settings → Plugins (exact settings screens depend on your plan/workspace). Verify the plugin discovers one tool and the `message.created` event. From your dot, request:

> Subscribe to `dm-dot`'s `message.created` event. When one arrives, treat its `message` as an incoming note from my other agents, and incorporate it into your ongoing work. Don't send a reply unless I separately request one.

The dot provides a unique HTTPS callback URL and signing secret through `events/subscribe`. The server validates the callback with a signed challenge and stores **one** subscription. A different callback cannot replace the dot until it unsubscribes or the old subscription expires. ChatGPT refreshes subscriptions before expiration (maximum 7 days per grant). To disconnect, unsubscribe in the dot.

## Delivery and limitations

- One registered dot; no `dot_id` input, inbox history, reply tool, or message retrieval.
- Events use Standard Webhooks HMAC signatures (`whsec_` secret). Callbacks must be public HTTPS; redirects, private IPs, and proxy routing are blocked.
- Each `send_to_dot` call delivers once with up to three bounded synchronous attempts for transient failures. A `2xx` means accepted by ChatGPT, *not* processed by the dot.
- If webhook delivery fails, the tool returns an error. **There is no durable delivery queue or offline replay in v1.** After ambiguous failures, manually retrying can duplicate a delivery.
- Messages up to 16 KiB. No plaintext message history is stored; Redis stores the callback URL, signing secret, and subscription expiry.

Licensed under Apache-2.0.
