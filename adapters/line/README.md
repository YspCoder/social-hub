# LINE Messaging API adapter

Package `social-hub/adapters/line` implements the official LINE Messaging API.
It uses documented HTTPS endpoints only and does not use cookies, scraping,
browser automation, private mobile endpoints, LINE Login, or LIFF APIs.

Implemented contracts:

- channel access token authentication through `access_token_ref`;
- common text `Messenger` push messages;
- typed text, sticker, image, video, audio, and location message objects;
- typed push, reply, multicast, and broadcast workflows;
- direct, group-member, and room-member profiles;
- streaming inbound message content and preview downloads;
- video/audio transcoding status and monthly message quota reads;
- Base64 HMAC-SHA256 webhook verification through `X-Line-Signature`;
- typed message and postback webhook events, with raw JSON retained for other
  evolving event fields;
- short-lived and stateless channel token issue, plus short-lived/long-lived
  token verification and revocation.

The common `Publisher`, `Fetcher`, `MediaUploader`, and `Reactor` capabilities
are intentionally unavailable. Messaging API does not expose generic social
posts, arbitrary message history, reusable outbound media IDs, or post
reactions. Outbound media references public HTTPS URLs; inbound media is a
download-only stream that the caller must close.

## Configuration

Each account represents one LINE Messaging API channel and requires a channel
access token. Configure `secret_ref` with the channel secret to enable webhook
verification. Add `client_id` (the channel ID) with `secret_ref` to use the
token-management helper.

```yaml
adapter: line/messaging-api
settings:
  base_url: https://api.line.me
  data_base_url: https://api-data.line.me
  token_base_url: https://api.line.me
accounts:
  - id: line-main
    client_id: ${LINE_CHANNEL_ID}
    secret_ref: env://LINE_CHANNEL_SECRET
    access_token_ref: env://LINE_CHANNEL_ACCESS_TOKEN
    settings:
      bot_user_id: U00000000000000000000000000000000
```

`account.settings.bot_user_id` is optional. When configured, webhook decoding
rejects payloads whose `destination` identifies another bot, preventing an
account-routing mistake from being accepted silently.

## Delivery and token semantics

LINE accepts at most five message objects in one send request. Multicast accepts
at most 500 unique user IDs. `socialhub.WithIdempotencyKey` is supported for
push, multicast, and broadcast and is sent as `X-Line-Retry-Key`; LINE requires
that value to be a UUID. Reply tokens are single-use and reply requests do not
send a retry key.

Channel access tokens do not have a refresh-token flow. The helper can issue a
short-lived token (normally 30 days) or a stateless token (normally 15 minutes).
The legacy verify and revoke methods apply only to short-lived and long-lived
tokens; LINE stateless tokens cannot be verified or revoked. The v2.1 JWT
assertion flow is not implemented because private-key ownership and signing
policy belong to the application or its credential service. Issued tokens are
returned to the caller and are never persisted by this package.

All current tests use deterministic local HTTP fixtures. The adapter has not
been validated with a real LINE Developers channel or user.

Official resources:

- <https://developers.line.biz/en/reference/messaging-api/>
- <https://developers.line.biz/en/docs/messaging-api/channel-access-tokens/>
- <https://developers.line.biz/en/docs/messaging-api/verify-webhook-signature/>
- <https://github.com/line/line-openapi>
- <https://github.com/line/line-bot-sdk-go>
