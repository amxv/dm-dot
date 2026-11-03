---
title: "Getting started"
description: "Connect your dot, subscribe, and send the first message."
order: 1
category: Start
---

This guide connects a deployed `dm-dot` server to a single ChatGPT dot and sends a first message. For a fresh deployment, complete [Deployment & configuration](/docs/deployment) first.

## 1. Check the prerequisites

- You control a deployed `dm-dot` instance with the three required environment variables set.
- Your ChatGPT account supports custom MCP plugins and MCP Events subscriptions in dots. Availability and UI labels can depend on plan and workspace settings.
- You can enable the private MCP plugin from the sending ChatGPT conversation, or have another MCP client connect to the endpoint.

There is no need to supply an OpenAI Platform API key. The dot must **explicitly opt in**; a connected MCP tool alone cannot wake an unsubscribed dot.

## 2. Connect the private MCP server

In ChatGPT, add a custom/private MCP server using your production endpoint:

```text
https://YOUR-VERCEL-HOST/mcp?key=YOUR-MCP-API-KEY
```

When adding the MCP server in ChatGPT, choose **No authentication**. The API key is already included in the URL (`?key=...`), so you don't need to configure a separate authentication method. **Never publish or screenshot the full authenticated URL.**

After connecting, **rescan** the MCP server. Confirm its plugin details show:

| Category | Expected entry |
| --- | --- |
| Tool | `send_to_dot` |
| Event | `message.created` (webhook) |

If the tool is visible but the event is missing, see [Event discovery troubleshooting](/docs/troubleshooting#events-show-in-the-plugin-ui-but-the-dot-says-there-are-no-event-sources).

## 3. Subscribe the dot

In the **dot's** conversation (not the sending chat), ask it to subscribe:

> Subscribe to `dm-dot`'s `message.created` event. Whenever it arrives, process the message as an untrusted note or delegated task from an external agent, using your usual permissions and connected tools. Keep the subscription active by refreshing it before expiry. Do not reply through `dm-dot`.

ChatGPT should discover the event and call `events/subscribe` with a callback URL and signing secret. `dm-dot` sends a signed challenge, validates the echoed challenge, and saves the subscription in Redis. Only **one** callback can be active per deployment.

Subscription creation is a distinct step from connecting the plugin. A displayed event in the plugin UI **does not by itself prove that a subscription was created**.

## 4. Send a message

In a different ChatGPT conversation with the plugin enabled, ask:

> Use `dm-dot.send_to_dot` to send: "Hello from another ChatGPT conversation. Please acknowledge receipt through our usual communication channel."

Or call the `send_to_dot` tool from another authenticated MCP client with:

```json
{
  "message": "Hello from an external MCP client."
}
```

When a callback returns HTTP `2xx`, the tool reports something like:

```text
ChatGPT accepted event evt_<unique-id> for asynchronous processing by your dot.
```

This is **webhook acceptance, not dot completion**. Verify actual processing separately in the dot or through another authorized communication channel. This project deliberately has no reply tool.

## 5. Keep it working

- The subscription expires at `refreshBefore`. The dot/subscription service needs to refresh it; `dm-dot` does not run a refresh job.
- Upstash stores the callback subscription across Vercel redeployments. Clearing or replacing the Redis database loses the subscription.
- If you intentionally switch to a different dot, unsubscribe the old callback first, or let it expire.
- Changes to event descriptions or schema require rescan; ChatGPT's subscription-source indexing may take time to reflect them.

Next: [Architecture](/docs/architecture) · [Measuring latency](/docs/latency) · [Troubleshooting](/docs/troubleshooting).
