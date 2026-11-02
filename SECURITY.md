# Security policy

Security matters because `dm-dot` connects external clients to an autonomous assistant through authenticated webhook events.

## Reporting a vulnerability

Please do **not** disclose a security issue in a public GitHub issue or pull request. Contact the repository maintainer through a private, authenticated channel, or use GitHub's **Report a vulnerability** feature if enabled for the repository. Include a minimal reproduction that does not leak real credentials or webhook secrets.

## Supported versions

This project is pre-release. Fixes are applied to the `main` branch; there is not yet a published support lifecycle for versioned releases.

## Deployment safety

- Use a unique, high-entropy `MCP_API_KEY` for the MCP connection.
- Use a **different** `DM_DOT_REST_API_KEY` for REST sending.
- Store tokens as sensitive environment variables, not in client-side code or public files.
- Treat the full MCP URL (including query-string key) as a secret.
- Do not assume a successful webhook response means a dot has processed the event.
- Do not expose this single-user bridge as a multi-tenant service without additional authentication, isolation, rate limits, and a security review.

More detail: [docs/security.md](docs/security.md).
