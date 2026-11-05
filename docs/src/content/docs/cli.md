---
title: "Command-line interface"
description: "Install dm-dot and message your subscribed dot from a terminal or local agent."
order: 3.5
category: Reference
---

The `dm-dot` CLI sends messages to the same subscribed ChatGPT dot as the MCP tool and REST API. It connects to **your self-hosted REST endpoint** using HTTPS, so local agents need no MCP client, Redis connection, OpenAI API key, or running daemon.

## Install

Use the website-hosted installer to download the latest GitHub Release for your operating system and architecture. It verifies the published SHA-256 checksum before installing a native executable.

**macOS and Linux** (Apple Silicon/ARM64 and x86-64/AMD64):

```sh
curl -fsSL https://dm-dot.ashray.xyz/install.sh | sh
```

The default install location is `~/.local/bin/dm-dot`. Ensure `~/.local/bin` is on your `PATH`. To install somewhere else, use an absolute `CLI_INSTALL_DIR`:

```sh
curl -fsSL https://dm-dot.ashray.xyz/install.sh -o install.sh
CLI_INSTALL_DIR="$HOME/bin" sh install.sh
```

**Windows** (PowerShell, x64 or ARM64):

```powershell
irm https://dm-dot.ashray.xyz/install.ps1 | iex
```

The default location is `%LOCALAPPDATA%\dm-dot\bin\dm-dot.exe`. Add it to your user `PATH` if necessary. The Windows installer also accepts `$env:CLI_INSTALL_DIR` to choose another absolute directory.

To review a script before running it, open [install.sh](https://dm-dot.ashray.xyz/install.sh) or [install.ps1](https://dm-dot.ashray.xyz/install.ps1). The source code is in the [dm-dot repository](https://github.com/amxv/dm-dot/tree/main/docs/public). Both installers download release assets directly from GitHub and verify `SHA256SUMS`. The install scripts themselves are trusted via the documentation site's HTTPS connection.

## Configure your deployment

**The CLI does not connect to dm-dot's maintainer API.** Supply the HTTPS address and independent REST token for your own deployed API. The token is not your MCP query-string key.

Option A: Configure once, then allow trusted local agents to call `dm-dot` without environment setup:

```sh
# Retrieve the REST token from your credential manager first.
printf %s "$DM_DOT_REST_API_KEY" | dm-dot configure \
  --url https://YOUR-API-HOST --token-stdin

dm-dot config show
```

`--token-stdin` reads the key from standard input without putting it in a shell argument or printing it. The CLI stores the key in a per-user configuration file with private permissions on Unix (mode `0600`). **It is stored as plaintext**, so only configure trusted machines or trusted OS users. On Windows, the file inherits the user profile's ACLs. Use `dm-dot config clear` to remove the saved configuration. For key rotation, repeat `configure` with your new REST key.

Option B: Do not save credentials on disk. Supply the variables from a shell environment, secret manager, or CI secret store:

```sh
export DM_DOT_API_URL="https://YOUR-API-HOST"
# Set DM_DOT_REST_API_KEY using your preferred secret manager.
dm-dot send "Hello from my agent"
```

Environment values override saved settings. `--url` overrides the deployment URL for one `send` command. The CLI rejects non-HTTPS URLs and URLs containing query-string credentials.

## Send a message

```sh
dm-dot send "A deployment just finished. Review the release and report issues."

# Send a message from a pipeline:
printf '%s\n' 'The build failed. Check CI.' | dm-dot send

# Read a larger prompt or report from a file:
dm-dot send --file /path/to/report.md

# Emit a machine-readable receipt for an agent:
dm-dot send --json "Inspect the latest staging errors"
```

On success, human output looks like:

```text
ChatGPT accepted event evt_0123456789abcdef for asynchronous processing by your dot.
```

`--json` prints the same REST receipt shape:

```json
{"status":"accepted","event_id":"evt_0123456789abcdef"}
```

The CLI exits with status `0` on HTTP `202 Accepted` and nonzero on local validation, authentication, subscription, network, and delivery errors. Errors go to stderr. A message is trimmed to 1–16,384 UTF-8 bytes, matching the MCP and REST limits. The CLI does not retry a failed HTTP request automatically because delivery may already have happened before the response was lost.

**An event receipt is not a reply from the dot.** ChatGPT acknowledged the webhook, but the dot handles it asynchronously. There is no status polling endpoint, response streaming, completion callback, or message history.

## Version and upgrades

```sh
dm-dot --version
dm-dot upgrade --check
dm-dot upgrade
```

`upgrade --check` compares your installed version with the latest stable GitHub Release. `upgrade` only replaces the executable when a newer version exists. It downloads the installer from this documentation site and pins it to the chosen release, preserving checksum verification. On Windows, the upgrade helper completes replacement after the CLI process exits.

Alternatively, set `CLI_VERSION=0.1.0` when running the website installer to pin an exact stable release. No background or automatic upgrades are enabled.

## Automation and security

Treat the CLI as a **trusted write-capable operation**. Any process with access to the REST key can send a note that may trigger the dot's own tools, subject to the dot's permissions and confirmation requirements. Never expose the token in a command-line argument or log. Keep shell scripts, CI jobs, and local agent permissions scoped appropriately.

Both POSIX and Windows installers support AMD64 and ARM64. If your target architecture is not supported, the installer stops without replacing the current executable.

See [REST API](/docs/rest-api) for error codes, [Getting started](/docs/getting-started) for dot subscriptions, and [Security](/docs/security) for trust boundaries.
