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

`--token-stdin` reads the key from standard input without putting it in a shell argument or printing it. The CLI saves the REST token in your operating system's **native credential store**: macOS Keychain, Windows Credential Manager, or Linux Secret Service (for example GNOME Keyring). The local config file stores **only the API URL**, never the bearer token. On macOS and Linux, the URL-only file has mode `0600`.

On Linux, an active D-Bus Secret Service and unlocked credential collection are required for saved credentials. For headless servers without one, use Option B instead; the CLI **never falls back to plaintext credential storage**. Use `dm-dot config clear` to remove the native credential associated with the current API origin and its local URL configuration. For key rotation or changing the API host, rerun `configure` with the corresponding REST key.

**Automatic v0.1 migration:** When v0.2 first accesses an existing `config.json` containing a plaintext token, it stores the key in the native OS credential manager, confirms that it can read the key back, and atomically replaces the config with a URL-only file. If the credential store is unavailable, the migration fails safely and the old config is left untouched rather than losing access. Restore/unlock the OS keyring and retry. If you want to use environment variables only, first clear the old local config.

Option B: Do not save credentials on disk. Supply the variables from a shell environment, secret manager, or CI secret store:

```sh
export DM_DOT_API_URL="https://YOUR-API-HOST"
# Set DM_DOT_REST_API_KEY using your preferred secret manager.
dm-dot send "Hello from my agent"
```

Environment values can override saved settings **only when they target the same API origin**. `--url` also accepts only a URL on the already configured origin. The CLI rejects non-HTTPS URLs, embedded credentials, query parameters, and attempts to redirect a saved or environment-provided token to another host or port. To switch to a different API origin, run `dm-dot configure` with the corresponding key (or clear the saved config and provide both `DM_DOT_API_URL` and `DM_DOT_REST_API_KEY`). For an environment-only agent, both variables are required; the CLI cannot infer a safe destination for a standalone key.

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

Alternatively, set `CLI_VERSION=0.2.0` when running the website installer to pin an exact stable release. No background or automatic upgrades are enabled.

## Automation and security

Treat the CLI as a **trusted write-capable operation**. Any process able to invoke the CLI with access to your unlocked native credential store can send a note that may trigger the dot's own tools, subject to the dot's permissions and confirmation requirements. Native storage prevents a plaintext copy of the key in the config file, but **does not isolate untrusted processes running under the same user**. Never expose the token in a command-line argument or log. Keep shell scripts, CI jobs, and local agent permissions scoped appropriately.

Both POSIX and Windows installers support AMD64 and ARM64. If your target architecture is not supported, the installer stops without replacing the current executable.

See [REST API](/docs/rest-api) for error codes, [Getting started](/docs/getting-started) for dot subscriptions, and [Security](/docs/security) for trust boundaries.
