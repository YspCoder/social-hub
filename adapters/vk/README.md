# VK API 5.199 adapter

Package `social-hub/adapters/vk` implements the official VK API 5.199 at
`https://api.vk.ru/method`. It uses documented API and Callback API endpoints
only. It does not use cookies, scraping, browser automation, private mobile
endpoints, or undocumented session flows.

Implemented contracts:

- user, community, and service access tokens supplied through
  `access_token_ref`;
- common wall publishing, scheduled publishing, deletion, publish status,
  profile reads, wall reads, and comment reads;
- typed wall publishing and repost workflows;
- three-stage wall photo upload through VK's server-owned upload URL;
- post likes for user tokens and wall comments for user/community tokens;
- single-peer messages, attachments, replies, and message lookup for
  user/community tokens;
- Callback API body-secret verification, common event mapping, raw unknown
  events, and confirmation-code retrieval.

## Configuration and token capabilities

Every account requires a non-zero `owner_id`, an explicit `token_kind`, and an
access-token reference. Community owner IDs are negative. For example, VK
community `456` is configured as `owner_id: -456`.

```yaml
adapter: vk/v5.199
settings:
  base_url: https://api.vk.ru/method
accounts:
  - id: personal
    access_token_ref: env://VK_USER_ACCESS_TOKEN
    settings:
      owner_id: 123
      token_kind: user
  - id: community
    access_token_ref: env://VK_COMMUNITY_ACCESS_TOKEN
    webhook:
      secret_ref: env://VK_CALLBACK_SECRET
    settings:
      owner_id: -456
      token_kind: community
```

| Capability | User token | Community token | Service token |
|---|---|---|---|
| Profiles, walls, posts, comments | Yes | Yes | Yes, subject to object visibility |
| Wall publish, repost, delete | Yes | No | No |
| Wall photo upload | Yes | No | No |
| Post likes | Yes | No | No |
| Create wall comments | Yes | Yes | No |
| Messages | Yes | Yes, when community messages are enabled | No |
| Callback verification | Yes, when configured for a negative community owner | Yes | No practical event-delivery use |
| Callback confirmation-code lookup | Yes, for a negative community owner | Yes | API permissions remain authoritative |

The adapter consumes an already issued access token and does not own VK login,
token refresh, or token persistence. Keep token acquisition and rotation in the
application's credential service. Platform permissions, application approval,
community settings, and object privacy remain authoritative even when a local
capability reports support.

## IDs, walls, and reposts

Common post and comment IDs use VK's composite `owner_id_item_id` form, for
example `-456_7`. Inputs may also include the `wall` prefix where VK normally
uses it, such as `wall-456_7`.

Media attachments retain VK identifiers such as `photo-456_7` or
`photo-456_7_access_key`. Pass those identifiers through `MediaIDs` when
publishing a wall post or sending a message. A common quote post maps to an
independent VK repost and is represented with `RelationRepost`.

`WallWorkflow.CreateWallPost` exposes `from_group`, `friends_only`, signing,
comment closure, muted notifications, and scheduled `publish_date` controls.
`WallWorkflow.Repost` accepts a community destination through its negative
owner ID. A personal destination must match the configured positive owner ID,
because `wall.repost` cannot name an arbitrary user's wall.

`socialhub.WithIdempotencyKey` maps to the VK `guid` form field for
`wall.post`. For `messages.send`, it must be a positive 32-bit decimal value and
maps to `random_id`. The adapter never forwards the common
`Idempotency-Key` HTTP header to VK.

## Wall photo upload

The common media workflow implements VK wall photos only:

1. `BeginUpload` calls `photos.getWallUploadServer`.
2. `UploadPart` streams multipart field `photo` to the returned upload URL.
3. `CompleteUpload` calls `photos.saveWallPhoto` and returns the reusable VK
   attachment ID.

Exactly one part numbered `0` is accepted. The declared byte count must match
the stream. JPEG, PNG, and GIF images up to 50 MiB are accepted locally; VK
performs authoritative content and policy validation. Authorization headers
are never sent to the upload host. HTTPS upload URLs are required in normal
operation; HTTP is allowed only when the API base itself is explicitly
overridden to HTTP for local testing or an approved gateway.

## Callback API

VK Callback API places the configured secret in the JSON body; it does not use
an HMAC signature header. `WebhookHandler.Verify` performs a constant-time
comparison and also checks the configured community ID. `Decode` maps common
message, wall-post, repost, and wall-reply events, while retaining raw JSON for
unknown event types.

The HTTP router remains responsible for the response body. Return `ok` for a
normal accepted event. During endpoint registration, return the value from
`CallbackWorkflow.GetCallbackConfirmationCode` for the `confirmation` event.
Do not log the raw callback secret.

## Limits and verification status

VK documents a baseline of three API calls per second for user and service
tokens and up to 20 calls per second for community tokens. Method-specific,
application, anti-spam, and dynamic limits may be stricter. Treat VK errors 6,
9, and 29 as the source of truth; the adapter maps them to retryable
`CodeRateLimited` errors but does not guess the exhausted quota.

All current tests use deterministic local HTTP fixtures, including multipart
upload and Callback API payloads. The adapter has not been validated with a
real VK application, user, community, or access token.

The wire contract was checked against the official API schema and the mature
`github.com/SevereCloud/vksdk/v3` client. The SDK is a design reference only
and is not included as a dependency, keeping this adapter's dependency surface
small.

Official resources:

- <https://dev.vk.ru/ru/reference>
- <https://dev.vk.ru/ru/api/callback/getting-started>
- <https://github.com/VKCOM/vk-api-schema>
- <https://github.com/SevereCloud/vksdk>

Last verified: 2026-08-01.
