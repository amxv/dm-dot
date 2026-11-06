---
title: "Development"
description: "Project structure, checks, and contribution workflow."
order: 9
category: Guides
---

`dm-dot` is deliberately small: a Go MCP + REST backend, a native Go CLI, and an Astro + ZueDocs site restored from the original Go MCP template. All operational details live in ZueDocs, not the marketing landing page.

## Project structure

```text
api/mcp.go                 MCP Vercel Function
api/messages/index.go      REST Vercel Function
pkg/server/                Authentication, tools, events, Redis, webhook delivery
pkg/message/               Shared message request/response and validation contract
pkg/client/                Authenticated REST sender for local callers
cmd/dm-dot/                CLI entry point
internal/cli/              CLI commands and stdin/file handling
internal/config/           OS credential store, origin pinning, and v0.1 migration
internal/upgrade/          Latest-release checks and explicit upgrades
docs/public/install.*      Website-hosted native CLI installers
.github/workflows/release.yml    Six-platform-architecture binary release matrix
docs/src/pages/index.astro       Landing page design (Astro)
docs/src/pages/docs/            ZueDocs routes, layouts, and code enhancements
docs/src/content/docs/          Canonical technical Markdown documents
docs/src/data/docs.ts           ZueDocs categories, navigation and metadata
docs/public/assets/            Landing page CSS and interaction script
docs/dist/                      Generated Astro site (ignored by Git)
docs/vercel.json                Separate documentation Vercel project
vercel.json                     API-only Go functions, rewrites and redirects
```

## Local checks

Requires Go 1.26+, Node.js 22+, and Bun 1.4+.

```bash
go test ./...
go vet ./...
go build -o bin/dm-dot ./cmd/dm-dot
make docs-install
make docs-check
make check
```

ZueDocs/Astro renders all pages. `make docs-check` validates Astro source and types; `make docs-build` builds the static site, and `make check` covers Go plus ZueDocs. The website deploys from `docs/` independently of the repository-root Go API project. There is no separate custom Markdown renderer.

For a local static preview:

```bash
cd docs
bun run build
python3 -m http.server 4173 --directory dist
```

Open `http://localhost:4173`. The ZueDocs site does **not** send live messages. API routes only work when deployed to Vercel or when tested with an appropriate local Go/Vercel HTTP runtime.

## Protocol implementation

The official Go MCP SDK implements tools, transport and version negotiation. The pinned SDK version does not natively expose the MCP Events capability, so `pkg/server/server.go` inserts `events: {}` into both handshake response paths and handles `events/*` methods itself.

Keep regression coverage for `initialize` **and** MCP 2.0 `server/discover`. A correct `events/list` alone is insufficient for ChatGPT's subscription discovery.

The common `SendMessage` function is used by both the MCP tool and REST ingress. The CLI uses the REST endpoint with a shared `pkg/message` contract. A change to message validation, subscription storage, signing or retries must be tested through **all three** surfaces.

CLI `config.Effective` ensures URL overrides match the credential's configured HTTPS origin before retrieving the native key. Tests inject a fake OS keyring to exercise macOS/Windows/Linux-independent migration and verify that plaintext credentials remain in their original config after a failed migration. New CLI configs store only the API URL. The native key is stored with the account name equal to the normalized origin under the `dm-dot-cli` service.

## CLI publishing

`docs/public/install.sh` and `install.ps1` are served at the documentation site's root. The tag-triggered GitHub Actions workflow builds `darwin`, `linux`, and `windows` binaries for `amd64` and `arm64`, creates `SHA256SUMS`, and publishes a GitHub Release. The installers select the right asset and verify its checksum before replacing a local binary. Releases are tagged `vMAJOR.MINOR.PATCH` from `main` after checks pass. Use `dm-dot upgrade --check` to inspect newer releases.

## Testing and security

- Go tests use local mock REST and webhook servers. They do not contact the live dot or real Upstash database.
- Never commit real API keys or webhook signing secrets. Use synthetic fixtures in tests and examples.
- Keep production subscriptions and deployment-specific secrets out of the repository.
- The deployed API must always require authentication, regardless of repository visibility.
- Verify `make check`, `git diff --check`, and hosted HTTP responses before shipping changes.

For contributions, see [CONTRIBUTING.md](https://github.com/amxv/dm-dot/blob/main/CONTRIBUTING.md) in the repository. Security considerations are detailed in [Security](/docs/security).
