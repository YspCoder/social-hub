# Kick Developer Public API adapter

Adapter name: `kick/public-api-v2`

This adapter targets Kick's official Developer Public API at
`https://api.kick.com`. It uses the current V2 category and livestream list
contracts and the non-deprecated V1 endpoints for users, channels, chat, event
subscriptions, and the webhook public key. Kick website endpoints, cookies,
scraping, and reverse-engineered APIs are deliberately excluded.

## Configuration

Every account must identify whether its bearer credential is an `app` or
`user` access token. Credentials remain secret references and are resolved only
when a client is created.

```yaml
version: 1
platforms:
  - adapter: kick/public-api-v2
    product: developer-public-api
    accounts:
      - id: creator-main
        client_id: ${KICK_CLIENT_ID}
        secret_ref: env://KICK_CLIENT_SECRET
        access_token_ref: env://KICK_USER_ACCESS_TOKEN
        approval:
          scopes:
            - user:read
            - channel:read
            - channel:write
            - chat:write
            - events:subscribe
            - moderation:chat_message:manage
        settings:
          token_type: user
          broadcaster_user_id: "123456"
          channel_slug: creator-name
```

`broadcaster_user_id` is used as the default for account channel lookup and
user-mode chat. `channel_slug` is the fallback channel lookup when no
broadcaster ID is configured. App-token event subscription creation requires a
broadcaster user ID; user-token subscriptions infer it from the token.

`client_id` and `secret_ref` are only required when using `Adapter.OAuth` to
create or refresh tokens. `access_token_ref` is always required to create an
API client.

## OAuth 2.1

The helper supports Kick's two official token flows:

- Authorization Code with PKCE S256 for user access tokens.
- Client Credentials for app access tokens.

It also implements refresh, revoke, and the current
`POST https://id.kick.com/oauth/token/introspect` endpoint. The deprecated
`POST /public/v1/token/introspect` endpoint is not used.

```go
oauth, err := adapter.OAuth(ctx, "creator-main")
if err != nil {
    return err
}
pkce, err := kick.NewPKCE()
if err != nil {
    return err
}
authorizationURL, err := oauth.AuthorizationURL(
    "https://app.example/kick/callback",
    state,
    pkce,
    []string{"user:read", "channel:read", "chat:write"},
)
```

Persist both tokens returned by `Exchange` or `Refresh`. Kick refresh tokens
are reusable and use a sliding expiry window, but a refresh response still
returns the current refresh token and callers should persist that returned
value.

## Capabilities

The common capability surface is intentionally narrow:

| Common interface | Coverage |
|---|---|
| `WebhookHandler` | RSA-SHA256 verification and typed decoding for all ten documented V1 events |
| `Publisher` | Not supported; channel metadata updates are not generic posts |
| `Fetcher` | Not exposed; active livestreams do not satisfy common post/comment lookup |
| `MediaUploader` | Not supported by the Public API |
| `Reactor` | No matching Public API contract |
| `Messenger` | Not exposed because chat has no arbitrary message lookup |

Use the typed workflows for the actual Kick resource model:

| Workflow | Official endpoints |
|---|---|
| `UserWorkflow` | `GET /public/v1/users` |
| `ChannelWorkflow` | `GET/PATCH /public/v1/channels` |
| `LivestreamWorkflow` | `GET /public/v2/livestreams`, `GET /public/v1/users/livestreams` |
| `CategoryWorkflow` | `GET /public/v2/categories` |
| `ChatWorkflow` | `POST /public/v1/chat`, `DELETE /public/v1/chat/{message_id}` |
| `SubscriptionWorkflow` | Event subscription CRUD and `GET /public/v1/public-key` |

V2 category and livestream pages preserve Kick's opaque `next_cursor` in
`socialhub.Page`. Livestream results are returned in the platform-defined
oldest-to-newest order. The adapter enforces the documented 1-1000 page size,
25-category/language filters, 100-user livestream lookup, 50-channel lookup,
10 custom tags, and 500-character chat limit.

Chat send requires `chat:write`. Chat deletion separately requires
`moderation:chat_message:manage`. Channel metadata update requires a user token
with `channel:write`. A channel response can include the stream URL and key when
the token has `streamkey:read`; treat those fields as credentials and never log
the complete response.

## Webhooks

The router must pass the exact raw body to `Verify` before calling `Decode`.
Kick signs:

```text
Kick-Event-Message-Id + "." + Kick-Event-Message-Timestamp + "." + raw_body
```

The signature is Base64-encoded RSA PKCS#1 v1.5 with SHA-256. Decoded payloads
are concrete Go types for:

- `chat.message.sent`
- `channel.followed`
- `channel.subscription.renewal`
- `channel.subscription.gifts`
- `channel.subscription.new`
- `channel.reward.redemption.updated`
- `livestream.status.updated`
- `livestream.metadata.updated`
- `moderation.banned`
- `kicks.gifted`

The official public key published in Kick's documentation is the default. For
key rotation, fetch the current key with `FetchWebhookPublicKey`, validate it,
then deploy it as `account.settings.webhook_public_key`. Delivery message IDs
are idempotency keys; applications must persist them to reject replayed
deliveries. Kick automatically removes a subscription after its webhook fails
continuously for more than one day.

## Limits and compatibility

Kick does not publish one global numeric REST rate limit. HTTP `429` maps to
`ErrRateLimited`, is marked retryable, and preserves a valid `Retry-After`
value. Event subscriptions are limited to 10,000 per event type per app;
unverified apps are limited to 1,000 `chat.message.sent` subscriptions.

The community `Scorfly/gokick` v1.18.0 client was evaluated because it tracks
the current public contract. It is not used as a dependency because its error
and automatic-refresh paths do not preserve social-hub's bounded error,
response-header, `CallOption`, and webhook verification contracts. This adapter
uses the shared `internal/transport` instead.

All tests use deterministic local HTTP fixtures and locally generated RSA
keys. No real Kick account has been used for validation yet.

Official references:

- <https://docs.kick.com>
- <https://docs.kick.com/getting-started/generating-tokens-oauth2-flow>
- <https://docs.kick.com/events/webhook-security>
- <https://docs.kick.com/events/event-types>
- <https://api.kick.com/swagger/doc.yaml>
- <https://github.com/KickEngineering/KickDevDocs>
