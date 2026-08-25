---
name: use-social-hub-mcp
description: "Use a self-hosted social-hub MCP server to discover configured social accounts, inspect capabilities, read normalized users/posts/comments/messages, and perform explicitly approved publishing, reaction, comment, deletion, or messaging actions. Apply when a user asks an Agent to operate supported social platforms through social-hub."
---

# Use social-hub MCP

Use only the tools exposed by the connected `social-hub` MCP server. The user deploys and configures this server; do not claim it is installed or hosted when the tools are unavailable.

## Required workflow

1. Call `socialhub_list_targets` before choosing an account. Select the exact `{adapter, account_id}` requested by the user. If more than one target could match, ask the user which target to use.
2. Call `socialhub_get_capabilities` before the first account operation. A supported capability group does not guarantee that every method in the group is implemented.
3. Use read tools directly when the user's request clearly authorizes the read. Preserve opaque cursors exactly and stop when `has_more` is false or the user's requested amount is reached.
4. Before any mutation, state the exact target and action and obtain explicit user confirmation unless the same message already unambiguously authorizes that exact action.
5. Call at most one mutation per confirmation. Do not turn a singular request into a bulk action.
6. Report the normalized result and any platform request ID. Never print raw credentials or ask the user to provide a token through tool arguments.

## Mutation rules

- Mutation tools may be absent because deployment policy is read-only. Explain that the deployer must explicitly enable the matching operation; do not work around the policy.
- Treat `socialhub_delete_post`, `socialhub_remove_reaction`, and `socialhub_delete_comment` as destructive.
- Supply `call.idempotency_key` for retry-prone writes when the caller has a stable key. Never invent a new key when retrying an earlier attempt; reuse the original.
- Use `media_ids` only when the user already has trusted, platform-valid IDs. This MCP server deliberately does not accept local paths, arbitrary media URLs, raw bytes, or base64 uploads.
- Do not infer recipient IDs, conversation IDs, actor IDs, post IDs, or comment IDs from display names when the mapping is ambiguous.

## Error handling

- `unsupported`: the capability accessor or specific method is unavailable. Do not repeatedly retry or assume another method in the same group works.
- `approval_required`: show `required_scopes` and `approval_url` when present. The user or deployer must complete platform approval.
- `rate_limited` or another `retryable` class: honor `retry_after_ms`. Ask before retrying a mutation unless the original authorization clearly includes retries.
- `unauthenticated` or `permission_denied`: tell the user the server-side account configuration or platform grant needs attention. Never request the secret value in chat.
- Service or tool unavailable: explain that the user must deploy/configure `social-hub-mcp`, then consult [deployment.md](references/deployment.md).

Read [tool-workflows.md](references/tool-workflows.md) when selecting parameters for a concrete read or mutation. Read [deployment.md](references/deployment.md) only for installation, transport, policy, or adapter-bundle questions.
