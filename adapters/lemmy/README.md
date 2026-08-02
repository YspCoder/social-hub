# Lemmy API v3 adapter

Adapter name: `lemmy/api-v3`

This adapter targets the official HTTP API v3 contract used by stable Lemmy
`0.19.x` instances. Each configured account represents one user and JWT on one
Lemmy origin. API v4 is intentionally excluded because it has a different
contract and migration path.

## Authentication and configuration

Lemmy `0.19.x` accepts the login JWT in `Authorization: Bearer <token>`. Put a
secret reference, not the JWT itself, in `access_token_ref`. The configured
`username` identifies the authenticated account for self-profile, account-post,
reaction actor, and private-message direction mapping.

```yaml
version: 1
platforms:
  - adapter: lemmy/api-v3
    product: api
    accounts:
      - id: community-main
        access_token_ref: env://LEMMY_JWT
        settings:
          base_url: https://lemmy.example
          username: alice
```

`base_url` must be an HTTP(S) origin without credentials, a path, query, or
fragment. Use HTTPS outside local development. Same-origin redirects are
allowed, while cross-origin redirects are rejected so the JWT cannot be
forwarded to another host.

## Common capabilities

| Capability | Coverage |
|---|---|
| `Fetcher` | Person by ID/username, Post by ID, configured/person Post pages, and Post comments |
| `MediaUploader` | One-part image or animated-image upload through the instance Pictrs endpoint |
| `Reactor` | Post upvote/removal, Comment creation/replies, and Comment deletion |
| `Publisher` | Not exposed because Lemmy requires a title and `community_id`, which the common request cannot express |
| `Messenger` | Not exposed because API v3 has no direct private-message get-by-ID endpoint |
| `WebhookHandler` | Not exposed because API v3 publishes no signed inbound webhook contract |

The common `ReactionLike` maps to score `1`; removal maps to score `0`.
Downvotes remain explicit through `VoteWorkflow` so cross-platform code cannot
accidentally treat a downvote as a generic unlike.

## Typed workflows

Community-aware publishing is available through `PostWorkflow`:

```go
common, err := adapter.Client(ctx, "community-main")
if err != nil {
    return err
}
client := common.(*lemmy.Client)

post, err := client.PostWorkflow().CreatePost(ctx, lemmy.CreatePostRequest{
    Title:       "social-hub v0.1 preview",
    CommunityID: "42",
    Body:        "The first integration build is ready for review.",
})
```

`PostWorkflow` supports create, get, update, delete, and feed listing. Feed
listing preserves Lemmy's opaque `next_page` value as `NextCursor`. Common
person Post pages and Comment pages use positive decimal page numbers because
those API v3 responses do not return a next cursor; a full page is reported as
potentially having more results.

Lemmy vote scores are exposed without loss:

```go
err = client.VoteWorkflow().VotePost(ctx, post.Common.ID, -1)
err = client.VoteWorkflow().VoteComment(ctx, "123", 0)
```

Allowed scores are `-1` (downvote), `0` (remove vote), and `1` (upvote).

Private messages use a separate typed workflow:

```go
message, err := client.PrivateMessageWorkflow().SendPrivateMessage(
    ctx,
    "84", // recipient person ID
    "Please review the release notes.",
)
```

The workflow supports send, numeric-page list with optional creator filtering,
edit, delete, and read-state mutation. It deliberately does not implement the
common `Messenger` partially.

## Pictrs uploads

The common upload lifecycle wraps `POST /pictrs/image`:

1. `BeginUpload` creates an in-memory image session.
2. `UploadPart` sends exactly one part numbered `0` using multipart field
   `images[]` and verifies the declared byte count.
3. `CompleteUpload` returns and caches the Pictrs file for this Client.

`MediaStatus` can only return media completed by the same Client because API v3
does not publish an authenticated Pictrs upload lookup endpoint. The Pictrs
`delete_token` is retained in `Media.Extensions["lemmy.pictrs_file"]`; this
adapter does not expose image deletion through the common media contract.

Uploaded media IDs can be passed as `CreatePostRequest.MediaID`. The adapter
then publishes the corresponding Pictrs URL as the Post URL. Videos are not
accepted by this workflow because this API contract documents Pictrs image
upload, not a general resumable video pipeline.

## Limits and compatibility

The adapter enforces the stable `0.19.x` content limits before sending:

| Field | Limit |
|---|---:|
| Post title | 3-200 characters, no newline |
| Post body | 50,000 UTF-16 code units |
| Comment/private message | 10,000 UTF-16 code units |
| Alt text | 1,500 UTF-16 code units |
| Post URL | 2,000 UTF-16 code units |

Rate limits are instance-configurable and may vary by operation. HTTP `429`
maps to `ErrRateLimited`, preserves `Retry-After`, and is classified as
retryable. Applications should use observed response metadata rather than
hard-code a network-wide quota.

No external Go SDK is used. The available clients evaluated for this adapter
either cover only a small read subset or target older generated bindings; none
provides a mature stable-`0.19.x` implementation for the write, vote, Pictrs,
and private-message contracts required here.

Official references:

- <https://join-lemmy.org/docs/contributors/04-api.html>
- <https://join-lemmy.org/lemmy-js-client-docs/v0.19/classes/LemmyHttp.html>
- <https://join-lemmy.org/news/2023-12-15_-_Lemmy_Release_v0.19.0_-_Instance_blocking%2C_Scaled_sort_and_Federation_Queue>
- <https://join-lemmy.org/docs/contributors/09-api-v4.html>

All tests use deterministic local HTTP fixtures. No real Lemmy instance has
been used for validation yet.
