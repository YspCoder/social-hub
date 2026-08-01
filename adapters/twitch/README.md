# Twitch Helix adapter

Package `social-hub/adapters/twitch` implements the official Twitch Helix API.
Helix is the current API product name rather than a numbered URL version. API
requests use `https://api.twitch.tv/helix` with both `Client-Id` and
`Authorization: Bearer ...` headers.

Implemented contracts:

- OAuth2 authorization-code exchange, refresh, client credentials, token
  validation, and revocation;
- common user lookup and published VOD lookup/listing;
- common channel-chat send with reply support and dropped-message handling;
- typed active-stream discovery, channel metadata, schedules, clip discovery,
  and asynchronous clip creation;
- typed EventSub webhook subscription create/list/delete;
- EventSub HMAC-SHA256 verification, RFC3339Nano timestamp enforcement,
  callback challenge handling, revocation decoding, and raw event preservation.

The common `Post` model represents published VODs, not live chat messages.
Twitch does not expose generic social-post publication, VOD comments, arbitrary
chat-message lookup, or general VOD upload through Helix, so `Publisher`,
`Reactor`, and `MediaUploader` are disabled. `Messenger.GetMessage` returns an
unsupported error because Helix cannot retrieve an arbitrary chat message.

## Authentication boundary

Every account needs a Twitch application Client ID and a primary access token.
The primary token may be a user access token for chat and user-authorized
operations, or an app access token for public reads. Configure `user_id` when
using common chat; Twitch requires `sender_id` to match the user represented by
the user token.

Webhook EventSub management requires an app access token. If one configured
client needs a user token for chat and an app token for EventSub, set
`app_access_token_ref`; both tokens must belong to the configured Client ID.
When it is omitted, the EventSub workflow reuses the primary token and Twitch
remains the authority on token-type validation.

For EventSub types that require user authorization, the relevant user must
first grant the required scopes to the same Client ID. Creating the webhook
subscription still uses that app's app access token.

Third-party applications that maintain Twitch OAuth sessions must call
`OAuthClient.Validate` when the session starts and at least hourly thereafter.
Token rotation and persistence remain the application's responsibility.

Example account settings:

```yaml
adapter: twitch/helix
accounts:
  - id: twitch-main
    client_id: ${TWITCH_CLIENT_ID}
    secret_ref: env://TWITCH_CLIENT_SECRET
    access_token_ref: env://TWITCH_USER_ACCESS_TOKEN
    approval:
      scopes:
        - user:write:chat
        - clips:edit
    settings:
      user_id: "141981764"
      app_access_token_ref: env://TWITCH_APP_ACCESS_TOKEN
      eventsub_secret_ref: env://TWITCH_EVENTSUB_SECRET
```

Twitch application registration requires a Twitch account with 2FA enabled.
The Client ID is public, while the client secret, access tokens, refresh tokens,
and EventSub secret must remain in a secret store.

## EventSub delivery

The configured EventSub secret must be 10-100 printable ASCII characters. The
subscription workflow accepts only HTTPS callbacks on the default port, as
required by Twitch. Verification computes HMAC-SHA256 over the exact
concatenation of message ID, message timestamp, and raw body, then uses a
constant-time comparison. Deliveries outside a ten-minute clock window are
rejected.

Twitch delivers notifications at least once. The adapter exposes
`Twitch-Eventsub-Message-Id` as `socialhub.Event.ID`; consumers should make
event handling idempotent and deduplicate that ID in durable storage. The
adapter does not keep an in-memory replay cache because process-local state
would not protect a multi-instance deployment.

`HandleChallenge` reads, verifies, and restores the request body before
returning the plain-text challenge. Event payloads remain `json.RawMessage`
inside `EventSubPayload`, so new subscription fields do not require a common
model release. WebSocket and Conduit transports are not implemented in this
adapter.

## Rate limits

Helix uses one-minute token buckets. App-token requests consume an app bucket;
user-token requests consume a per-Client-ID, per-user bucket. Endpoint-specific
costs and non-bucket 429 responses may also apply. Use HTTP 429 plus
`Ratelimit-Limit`, `Ratelimit-Remaining`, and the Unix timestamp in
`Ratelimit-Reset` as the source of truth instead of hard-coding a request
count.

All current tests use deterministic local HTTP fixtures. The adapter has not
yet been validated with a real Twitch application or broadcaster account.

Official resources:

- <https://dev.twitch.tv/docs/api/>
- <https://dev.twitch.tv/docs/api/reference/>
- <https://dev.twitch.tv/docs/api/guide/#twitch-rate-limits>
- <https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/>
- <https://dev.twitch.tv/docs/authentication/validate-tokens/>
- <https://dev.twitch.tv/docs/authentication/scopes/>
- <https://dev.twitch.tv/docs/chat/send-receive-messages/>
- <https://dev.twitch.tv/docs/eventsub/manage-subscriptions/>
- <https://dev.twitch.tv/docs/eventsub/handling-webhook-events/>
- <https://github.com/twitchdev/twitch-cli>
