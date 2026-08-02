# Discourse REST API adapter

Adapter name: `discourse/rest-api`

This adapter targets the current official Discourse REST API contract published
as `latest` at [docs.discourse.org](https://docs.discourse.org/). Each configured
account represents one Discourse instance and one API username.

## Authentication

Discourse API keys are created by an instance administrator. The adapter sends
the key only in the `Api-Key` header and sends `account.settings.api_username`
in the `Api-Username` header. Put the API key reference in
`access_token_ref`; it is not an OAuth access token.

```yaml
version: 1
platforms:
  - adapter: discourse/rest-api
    product: rest-api
    accounts:
      - id: community
        access_token_ref: env://DISCOURSE_API_KEY
        settings:
          base_url: https://community.example.com
          api_username: system
        webhook:
          secret_ref: env://DISCOURSE_WEBHOOK_SECRET
```

Use HTTPS outside local development. The adapter allows same-origin redirects
but rejects cross-origin redirects so that `Api-Key` cannot follow a redirect to
another host.

## Common capabilities

| Capability | Coverage |
|---|---|
| `Fetcher` | User by username, Post by ID, and direct replies through `GET /posts/{id}/replies.json` |
| `Publisher` | Reply to an existing Post; `reply_to_id` must be a Discourse Post ID |
| `MediaUploader` | One-part synchronous `POST /uploads.json` composer/background/custom-emoji uploads |
| `Reactor` | Like (post action type `2`), reply/comment, and Post/comment deletion |
| `WebhookHandler` | `X-Discourse-Event-Signature` HMAC-SHA256 verification and event-header decoding |

`Fetcher.ListPosts` is intentionally unsupported because official
`GET /posts.json` is a site-wide feed, while the common contract is an account
feed. Use `TopicWorkflow.ListLatestPosts` for the documented site-wide endpoint.

`RemoveReaction` is intentionally unsupported because the current official
OpenAPI contract documents creating post action type `2`, but not an unlike
endpoint. The adapter does not use undocumented endpoints or browser reverse
engineering.

## Typed workflows

Discourse Topics and private messages have required fields that are not present
in the common Post and Message requests. They remain explicit typed workflows:

```go
common, err := adapter.Client(ctx, "community")
if err != nil {
    return err
}
client := common.(*discourse.Client)

topic, err := client.TopicWorkflow().CreateTopic(ctx, discourse.CreateTopicRequest{
    Title:      "Release notes",
    Raw:        "Version 1.2 is available.",
    CategoryID: "5",
})

pm, err := client.PrivateMessageWorkflow().CreatePrivateMessage(ctx,
    discourse.CreatePrivateMessageRequest{
        Title:      "Account follow-up",
        Raw:        "Please review the attached details.",
        Recipients: []string{"alice", "bob"},
    },
)
```

`TopicWorkflow.GetTopic` returns the initial `post_stream.posts` plus all IDs in
`post_stream.stream`. Call `Fetcher.GetPost` for stream entries that the Topic
response did not expand.

## Upload notes

The common upload lifecycle wraps Discourse's synchronous multipart endpoint:

1. `BeginUpload` creates an in-memory session.
2. `UploadPart` sends exactly one part numbered `0` and verifies the byte count.
3. `CompleteUpload` returns and caches the resulting Upload for this Client.

The official OpenAPI contract has no general Upload lookup endpoint. Therefore
`MediaStatus` only returns uploads completed by the same Client; an unknown ID
returns `ErrUnsupported`. Avatar uploads are not exposed because the official
endpoint additionally requires a numeric `user_id`.

## Webhooks and rate limits

Configure an instance webhook secret and retain the raw request bytes. Discourse
signs those bytes as `sha256=<hex HMAC>` in
`X-Discourse-Event-Signature`. Decoded events preserve
`X-Discourse-Event-Id`, `X-Discourse-Event-Type`, `X-Discourse-Event`, and the
raw JSON body. Consumers should deduplicate on the event ID.

Rate limits are instance-configurable. HTTP `429` maps to `ErrRateLimited`, and
the adapter preserves `Retry-After` and `Discourse-Rate-Limit-Error-Code` when
present.

Official references:

- [Discourse API documentation and OpenAPI download](https://docs.discourse.org/)
- [Discourse REST API authentication and rate-limit overview](https://meta.discourse.org/t/discourse-rest-api-documentation/22706)
- [Discourse webhook setup and signature contract](https://meta.discourse.org/t/configure-webhooks-that-trigger-on-discourse-events-to-integrate-with-external-services/49045)
- [Discourse global rate-limit settings](https://meta.discourse.org/t/available-settings-for-global-rate-limits-and-throttling/78612)

Discourse's official integration documentation recommends its Ruby API gem but
does not provide a current official Go SDK. This adapter therefore reuses
`internal/transport` and implements only endpoints present in the official
OpenAPI or webhook documentation.

All tests use deterministic local HTTP fixtures. No real Discourse instance has
been used for validation yet.
