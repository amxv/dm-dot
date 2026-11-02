# Development

`dm-dot` is deliberately small: a Go MCP + REST backend and a static, editorial website generated from Markdown. All operational details live in this reference section, not the marketing landing page.

## Project structure

```text
api/mcp.go                 MCP Vercel Function
api/messages/index.go      REST Vercel Function
pkg/server/                Authentication, tools, events, Redis, webhook delivery
web/                       Landing page, reference templates, styles and build script
docs/                      Canonical technical Markdown documents
public/                    Generated static website (ignored by Git)
vercel.json                Static output, Go functions and rewrites
```

## Local checks

Requires Go 1.26+, Node.js 22+ and npm.

```bash
go test ./...
go vet ./...
npm ci
npm run check
make check
```

`npm run build` renders `web/landing.html` and converts the `docs/*.md` reference pages to static HTML under `public/`. `npm run docs-check` validates generated pages and local links. `make check` covers both Go and website verification.

For a local static preview:

```bash
npm run build
python3 -m http.server 4173 --directory public
```

Open `http://localhost:4173`. The generated static site does **not** send live messages. API routes only work when deployed to Vercel or when tested with an appropriate local Go/Vercel HTTP runtime.

## Protocol implementation

The official Go MCP SDK implements tools, transport and version negotiation. The pinned SDK version does not natively expose the MCP Events capability, so `pkg/server/server.go` inserts `events: {}` into both handshake response paths and handles `events/*` methods itself.

Keep regression coverage for `initialize` **and** MCP 2.0 `server/discover`. A correct `events/list` alone is insufficient for ChatGPT's subscription discovery.

The common `SendMessage` function is used by both the MCP tool and REST ingress. A change to message validation, subscription storage, signing or retries must be tested through **both** surfaces.

## Testing and security

- Go tests use local mock REST and webhook servers. They do not contact the live dot or real Upstash database.
- Never expose keys or webhook callback signing secrets in tests, docs, commits or screenshots.
- Keep the GitHub repository private until its owner explicitly changes visibility.
- The live API must remain authenticated even if source code becomes public later.
- Verify `make check`, `git diff --check`, and hosted HTTP responses before shipping changes.

For future contributions, see [CONTRIBUTING.md](../CONTRIBUTING.md) in the repository. Security considerations are detailed in [Security](security.md).
