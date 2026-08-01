# X API v2 adapter

Registry name: `x/v2`

Implemented capabilities:

- OAuth 2.0 Authorization Code with PKCE and token refresh helpers
- Create, delete, and retrieve posts
- Retrieve users, user timelines, and recent conversation replies
- Like/unlike and repost/undo repost
- X API v2 chunked media upload (`INIT`, `APPEND`, `FINALIZE`, `STATUS`)

The initial adapter does not expose Direct Messages or X Activity webhooks.
Those products have separate permission and delivery requirements and are
reported as unsupported capabilities rather than silently emulated.

Required scopes depend on the operation. Typical user-context integration uses
`tweet.read`, `tweet.write`, `users.read`, `like.write`, `media.write`, and
`offline.access`.

Official documentation: <https://docs.x.com/x-api>

Last verified: 2026-08-01.
