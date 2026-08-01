# Discord HTTP API v10 adapter

Package `social-hub/adapters/discord` implements the stable Discord bot REST
subset against the explicitly versioned v10 API.

Implemented contracts:

- Bot Token authentication through `access_token_ref`;
- common text `Messenger` for guild, thread, and DM channels;
- common `Publisher` when `account.settings.default_channel_id` is configured;
- user lookup, composite message lookup, and default-channel history;
- common comments as Discord replies and common likes as the `👍` reaction;
- typed current-bot and Gateway Bot discovery;
- Discord error-code and fractional `retry_after` mapping;
- a non-empty Discord-compatible `User-Agent` and bounded HTTP responses.

Discord message IDs are channel-scoped REST resources. Normalized message,
post, and comment IDs therefore use the opaque form `channel_id/message_id`.
A raw message snowflake is accepted only when a default channel is configured.
Outgoing content always sends `allowed_mentions.parse: []` so user-provided
text cannot unexpectedly notify users, roles, `@here`, or `@everyone`.

The initial adapter does not start a Gateway WebSocket connection. Gateway
lifecycle, heartbeats, resume state, sharding, and privileged intents belong in
a separately managed event consumer; `BotWorkflow.Gateway` exposes only the
official discovery response. Discord interactions use a separate Ed25519-signed
HTTP workflow and are not exposed as the common webhook capability.

Discord attachments are multipart fields on message creation rather than an
independent upload lifecycle, so the common `MediaUploader` is unavailable in
this first slice. The mature `bwmarrin/discordgo` library was evaluated, but its
latest tagged release uses API v9 and process-global endpoint variables. This
adapter uses social-hub's account-scoped v10 transport instead.

Example account settings:

```yaml
adapter: discord/v10
accounts:
  - id: community-bot
    access_token_ref: env://DISCORD_BOT_TOKEN
    settings:
      default_channel_id: "123456789012345678"
```
