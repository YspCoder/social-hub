# Deployment reference

The MCP server is self-hosted. Do not imply that social-hub provides a remote service.

## Minimum setup

1. Build `./cmd/social-hub-mcp` with Go 1.26.
2. Create a social-hub YAML/JSON config containing only credential references such as `env://TOKEN_NAME`.
3. Import every required adapter into the deployment binary. The default bundle contains `x/v2`, `facebook/page`, `telegram/bot-api`, `wechat/official-account`, `weibo/v2`, and `douyin/openapi`.
4. Start local stdio or stateless Streamable HTTP.
5. Register the process or URL in the Agent host's MCP configuration.

Default operation is read-only. Enable additive tools with `--allow-write` and destructive tools with `--allow-destructive`. Values are comma-separated operation names documented in `docs/mcp.md`. The allowlist applies to every target in that process; isolate accounts into separate processes when their permissions differ.

For remote HTTP, a non-loopback listener requires `--bearer-token-ref env://NAME`. Put the service behind TLS, forward the Bearer token from the Agent host, and retain Origin and request-size protections.

Refer deployers to the repository's `docs/mcp.md` for complete commands, Codex `config.toml` examples, environment variables, and security constraints.
