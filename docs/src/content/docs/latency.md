---
title: "Measuring latency"
description: "Measure submission, webhook acknowledgment, and dot execution."
order: 7
category: Guides
---

`dm-dot` is event-driven, but **webhook acceptance and dot execution are different milestones**. A single synchronous tool call cannot measure the entire path to the dot's first action.

## Three useful timestamps

| Label | Timestamp | Captured by |
| --- | --- | --- |
| `T0` | Immediately before invoking `send_to_dot` or `POST /v1/messages` | Sending application |
| `T1` | Immediately after the tool or REST call returns successfully | Sending application |
| `T2` | Earliest observable time at which the dot actually starts handling the event | Receiving dot or runtime |

`T1 - T0` measures **submission and webhook acknowledgment**. `T2 - T0` approximates **sender-to-dot-start latency**. If the platform independently exposes a verified webhook-ingress timestamp, record it as `T_ingest` and compare it separately.

The event payload's own `timestamp` is set by `dm-dot` before delivery; it is **not** a receipt timestamp from ChatGPT.

## Recommended experiment

1. Generate a unique test ID to correlate the send with the dot's acknowledgment.
2. Read a real synchronized UTC epoch time **immediately before** calling the sending tool or API, and include it in the message.
3. Capture the HTTP/tool return time immediately after it finishes.
4. Instruct the dot to make its **first available tool action** a runtime UTC clock read, then report that time, its source, and the test ID through an independently authorized communication channel.
5. Calculate deltas in milliseconds. Repeat several times and report median, variability, and possible clock skew.

Example payload:

```json
{
  "message": "LATENCY TEST. Test ID: run-abc123. Sender T0 epoch_ms: <capture just before sending>. On arrival, immediately obtain a trusted UTC runtime timestamp as your first action and report the exact timestamp and test ID through our existing channel. Do not estimate the time."
}
```

This experiment does **not** add a reply channel to `dm-dot`. It relies on a separate channel already available to your dot. A model's text assertion about the current time is not a reliable measurement; ask it to use a real runtime clock.

## Interpreting results

- The sender and dot may run on different machines with clocks that have small offsets. Prefer synchronized UTC clocks and mention uncertainty.
- If the dot's first clock read happens after other operations, `T2 - T0` is an **upper bound** for arrival-to-execution timing, not an exact webhook queue delay.
- ChatGPT can accept an event significantly before the dot begins processing it.
- A callback HTTP `2xx` only proves event acceptance; it is not proof of execution or success.
- Duplicated events can occur after retries, so correlate by test ID **and** webhook event ID where possible.

Keep timestamps and results free of secrets. See [MCP & Events reference](/docs/protocol#webhook-payload) for event metadata and [Architecture](/docs/architecture) for the delivery sequence.
