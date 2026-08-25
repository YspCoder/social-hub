# Tool workflow reference

Every account tool takes an explicit `target` object:

```json
{"adapter":"x/v2","account_id":"brand-x"}
```

Optional `call` fields are `request_id`, `idempotency_key`, `timeout_ms` (maximum 120000), and `fields`. Preserve platform IDs and cursors as opaque strings.

## Discovery and reads

1. `socialhub_list_targets`
2. `socialhub_get_capabilities`
3. Call the smallest read tool that answers the request.
4. For lists, pass the returned `next_cursor` unchanged. Do not paginate without a user need.

`fetch`, `message`, or `publish` capability support is group-level. A specific call can still return `unsupported`.

## Publishing

Confirm target, final text, media IDs, reply/quote target, and visibility. Call `socialhub_publish_post` once. If the result represents asynchronous publication, use `socialhub_get_publish_status` with the returned publication/container ID.

## Reactions and comments

Confirm `actor_id`, target post ID, reaction kind, or final comment text. Use `socialhub_add_reaction`, `socialhub_remove_reaction`, `socialhub_create_comment`, or `socialhub_delete_comment` only when the exact tool is exposed.

## Messages

Confirm the target account, recipients or conversation, final text, media IDs, and reply target. Never infer an ambiguous recipient from a display name. Call `socialhub_send_message` once.

## Errors

The output envelope contains either `data` or a sanitized `error`. Respect `class`, `retry_after_ms`, `required_scopes`, and `approval_url`. Do not ask for raw secrets or attempt to bypass a missing tool.
