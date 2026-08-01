# QQ Bot OpenAPI adapter

Package `social-hub/adapters/qq` implements the public QQ Bot OpenAPI for one
bot application.

Implemented contracts:

- App Access Token retrieval, one-minute early refresh, optional
  `socialhub.TokenStore` persistence, and invalidation after authentication
  errors;
- common text `Messenger` sends to C2C, group, and channel targets;
- typed text, Markdown, C2C/group `file_info`, and channel image-URL messages;
- reply, event, wakeup, sequence, passive-reference, and message retraction
  fields where supported by the target scene;
- asynchronous message-audit results for platform codes `304023` and `304024`;
- scene-bound C2C/group image, video, voice, and file ingestion from public
  HTTP(S) URLs;
- Ed25519 callback verification, dispatch-event decoding, and POST `op=13`
  validation responses.

The common `Publisher`, `Fetcher`, `MediaUploader`, and `Reactor` are
intentionally unavailable. QQ bot messages are not social posts, arbitrary
message history is not exposed, and QQ `file_info` media is bound to the C2C or
group target used during upload.

## Configuration

`app_id` contains the QQ bot AppID. `secret_ref` contains its AppSecret and
enables automatic App Access Token retrieval.

```yaml
adapter: qq/bot-api
product: bot-api
accounts:
  - id: support-bot
    app_id: "102012345"
    secret_ref: env://QQ_BOT_APP_SECRET
```

Use `access_token_ref` instead of `secret_ref` when an external credential
service owns token retrieval and refresh. To verify callbacks in that mode,
configure the AppSecret separately:

```yaml
accounts:
  - id: support-bot
    app_id: "102012345"
    access_token_ref: env://QQ_BOT_ACCESS_TOKEN
    webhook:
      secret_ref: env://QQ_BOT_APP_SECRET
```

Secrets are always resolved through `socialhub.SecretResolver`; do not place
credential values directly in the configuration file. The adapter sends the
documented `Authorization: QQBot ACCESS_TOKEN` header.

## Messages and URL media

Common conversation IDs preserve the target namespace:

- `c2c:<user_openid>`
- `group:<group_openid>`
- `channel:<channel_id>`

Use `SendMessage` for portable text sends. Use
`client.MessageWorkflow().Send` with `TextContent`, `MarkdownContent`,
`MediaContent`, or `ChannelImageContent` when QQ-specific semantics are
required. `MessageResult.PendingAudit` is true when QQ accepts a channel
message for asynchronous review; `AuditID` contains the review identifier.

For C2C and group media, first call `client.MediaWorkflow().UploadURL`, then
send the returned `MediaAsset.FileInfo` to the same target with `MediaContent`.
The adapter preserves the returned TTL and calculated expiration time. QQ does
not expose the same `file_info` flow for channels; use `ChannelImageContent`
with a public HTTP(S) image URL instead.

## Webhooks

For each callback, read a bounded raw body, call the common `Verify` method,
then call `Decode` for dispatch payloads with `op=0`. Verification uses
`X-Signature-Ed25519`, `X-Signature-Timestamp`, and the configured AppSecret.

QQ validates a callback URL with a POST body using `op=13`. Route that body to
`client.WebhookWorkflow().ValidationResponse` and return the resulting JSON.
This protocol does not implement `socialhub.ChallengeHandler`, whose contract
is for GET challenges without a request body.

Bot permissions, message scenes, event subscriptions, sandbox availability,
and production quotas depend on the application's approval and QQ Open
Platform configuration. The adapter surfaces permission and approval failures
and does not fall back to cookies, scraping, browser automation, private
endpoints, or reverse-engineered protocols.

All current tests use deterministic local HTTP fixtures. The adapter has not
been validated with a real QQ bot application.

Official documentation:

- <https://bot.q.qq.com/wiki/develop/api-v2/>
- <https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/access-token.html>
- <https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/api-call-guide.html>
- <https://bot.q.qq.com/wiki/develop/api-v2/server-inter/message/overview.html>
- <https://bot.q.qq.com/wiki/develop/api-v2/server-inter/message/rich-media.html>
- <https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/event-emit/webhook.html>

Last verified: 2026-08-01.
