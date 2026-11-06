---
title: "Security"
description: "Credentials, callback verification, and trust boundaries."
order: 6
category: Reference
---

`dm-dot` is a **single-deployment, single-subscriber** integration. It is not designed as a public, anonymous endpoint or as a multi-tenant service. Anyone who holds the authenticated MCP URL can call the message tool **and** the subscription protocol methods.

## Trust boundaries

| Boundary | What is trusted | What is not automatically trusted |
| --- | --- | --- |
| MCP client → `dm-dot` | Holder of the shared `MCP_API_KEY` | The user's intent, any text in `message`, the identity of the calling agent |
| REST caller → `dm-dot` | Holder of `DM_DOT_REST_API_KEY` | The user's intent or identity of the application |
| `dm-dot` → Upstash | Dedicated Redis REST credentials | Redis as a substitute for independent sender authorization |
| `dm-dot` → ChatGPT webhook | Callback verified during subscription, signed payload | The dot completing the requested task |
| Event → dot | Event metadata and source context | Instructions embedded in user-generated message content |

**Important:** The event may request actions like sending mail, editing files, or deploying code. The dot should still apply its existing trust, approval, and safety requirements. The event payload is **untrusted input**, not an elevated system instruction.

## MCP endpoint authentication

The server checks `?key=` against `MCP_API_KEY` with a constant-time digest comparison. Requests fail closed if no key is configured, and both tool calls and event-subscription RPCs pass through the same check.

Limitations:

- This is **one shared secret**, not OAuth or per-user authentication. Different agents using the same URL are indistinguishable to `dm-dot`.
- Query-string credentials can be captured in browser history, proxy/access logs, screenshots, or copied error reports. Treat the complete URL as a password.
- The service has no built-in rate limiting, abuse quotas, audit log, per-sender permissions, or session revocation list.
- The SDK tool's `readOnlyHint` metadata is **not a security boundary**. `send_to_dot` performs an external side effect by delivering an event.

Recommended precautions: use a high-entropy key, store it in a secure credential manager, rotate it if exposed, keep client access limited, and enable reasonable platform monitoring and throttling around the endpoint. Do not post a real authenticated URL in GitHub issues, chat screenshots, or a public README.

The REST API uses a **separate `DM_DOT_REST_API_KEY` bearer token**. It does not accept the MCP query parameter as authentication, and it does not advertise permissive browser CORS. Both token types can initiate dot work, so neither belongs in client-side JavaScript or public environment variables. See [REST API](/docs/rest-api).

The native CLI uses this same REST token. In v0.2+, `dm-dot configure --token-stdin` stores the token in **macOS Keychain**, **Windows Credential Manager**, or **Linux Secret Service**. Its JSON config file (`dm-dot/config.json` under the OS user configuration directory) holds only the deployment URL, with mode `0600` on Unix. Existing v0.1 plaintext credentials are migrated into the native store and verified before the file is replaced; migration fails closed without deleting the original if the native store is unavailable. Linux headless environments without Secret Service must use paired `DM_DOT_API_URL` and `DM_DOT_REST_API_KEY` environment variables; there is no plaintext fallback.

**Origin binding:** A configured REST credential is bound to its normalized HTTPS origin. A different `--url` or `DM_DOT_API_URL` (even when an environment key is also set) is rejected before reading the native key or sending HTTP. An environment-only client needs both explicit URL and token variables. Changing the configured destination requires `dm-dot configure` with the new key, or clearing the configuration first.

OS-native secret storage is not a sandbox: agents running under the same user and able to execute `dm-dot` can still send messages. Never pass tokens as CLI arguments, and grant CLI access only to trusted local agents.

## Callback security

Before accepting a subscription, the server:

1. Requires webhook delivery, `message.created`, empty filters, and a signing secret prefixed `whsec_` that decodes to 24–64 bytes.
2. Requires a public HTTPS callback URL (port 443) without embedded URL credentials or fragments.
3. Resolves destination IPs at connection time, rejects private/local and several reserved address ranges, connects to the validated address, and preserves the hostname for TLS verification.
4. Disables outbound proxy routing and HTTP redirects for webhook requests.
5. Posts a signed one-time verification challenge and requires the exact challenge echoed in an HTTP `2xx` response.

These protections help mitigate SSRF and callback impersonation. They are **not a guarantee against every possible DNS or cloud networking configuration**. Review the implementation before exposing it beyond your own trusted clients.

For event delivery, the server generates unique event IDs and sends Standard Webhooks signatures (`webhook-id`, `webhook-timestamp`, `webhook-signature`, and `X-MCP-Subscription-Id`). The receiver verifies these signatures with the key supplied at subscription time.

## Data retention and secrets

**Persisted in Upstash Redis:** one callback URL, subscription ID, signing secret, and expiration, under `dm-dot:subscription:v1`. Redis retains the record until its TTL expires or it is unsubscribed.

**Not persisted by the application:** message bodies, message history, client identities, a receipt history, webhook delivery outcomes, or dot responses.

**Encryption caveat:** `dm-dot` stores the webhook secret as a JSON string in Redis; there is **no application-level encryption at rest**. Use an appropriately secured, dedicated Redis database and enforce least privilege on its credentials. Vercel and Upstash may have their own retention, backup, and logging policies; this project does not control them.

Do not place real credentials in `.env.example`, docs, tests, commits, CI output, issue reports, or PR descriptions. Local `.env*` and `.vercel/` files are ignored by Git; still inspect `git diff --cached` before pushing.

## Operational limitations

- **No durable queue:** an acknowledged HTTP `2xx` is the last observable boundary; dot processing is not confirmed.
- **Possible duplicates:** transient retries reuse the same event ID; callbacks may see repeat delivery after an ambiguous failure.
- **Race conditions:** the single Redis key uses read-then-write subscription handling, not a transactionally enforced one-subscriber lock. Do not treat it as a concurrent, hostile multi-tenant reservation service.
- **Rotation:** changing the MCP API key changes the deterministic subscription identity. Clear or expire the old subscription before re-subscribing with a new key.
- **External tool permissions:** granting an agent the MCP URL effectively gives it the ability to wake the dot. Limit distribution accordingly.

## Before a wider release

Before advertising a hosted instance to other users or converting the system into a multi-user service, consider OAuth/per-sender identities, per-recipient ownership checks, atomic subscription updates, durable delivery/observability, rate limits, encryption of stored signing secrets, abuse controls, and an independent security assessment.

Source code visibility does **not** require making the current hosted MCP endpoint public to unauthenticated callers. The deployment must continue enforcing its API key.

For reporting vulnerabilities, see [SECURITY.md](https://github.com/amxv/dm-dot/blob/main/SECURITY.md).
