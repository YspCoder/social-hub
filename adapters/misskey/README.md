# Misskey per-instance API adapter

Package `social-hub/adapters/misskey` implements Misskey's public,
versionless HTTP API for independently operated instances. Every configured
account supplies its own `instance_url`; the adapter does not assume a single
host or a uniform instance policy.

Implemented contracts:

- access-token authentication through the documented Bearer header;
- MiAuth authorization URL generation and one-time session check without app
  registration;
- current-user and user lookup, Note lookup, user Notes, replies, and the
  authenticated home timeline;
- common publishing, replies, deletion, and emoji reactions;
- typed Note creation preserving CW, local-only delivery, all four visibility
  modes, specified recipients, replies, quotes/Renotes, channels, polls, and
  reaction acceptance;
- single-part multipart uploads to Drive plus Drive file lookup;
- public instance metadata and normalized Misskey error mapping.

The common `ReactionRepost` operation is intentionally unavailable. A Misskey
Renote creates a new Note ID, while the common `Reactor` method cannot return
that ID for symmetric deletion. Use `CreateNote` with `RenoteID` so the caller
retains the created Note. Misskey Chat is not included in this first adapter.
The Streaming API is WebSocket-based, so it is not exposed as the common signed
HTTP `WebhookHandler` contract.

## Configuration

Each account represents one user on one Misskey instance. `user_id` is
optional; when omitted, current-account operations resolve it through `/api/i`.
`default_reaction` controls the emoji used by common `ReactionLike` and defaults
to the Unicode thumbs-up reaction.

```yaml
adapter: misskey/api
accounts:
  - id: misskey-main
    access_token_ref: env://MISSKEY_ACCESS_TOKEN
    settings:
      instance_url: https://misskey.example
      user_id: 9abc123
      default_reaction: ":thumbsup:"
    approval:
      scopes:
        - read:account
        - write:notes
        - read:drive
        - write:drive
        - write:reactions
```

For user authorization, call `Adapter.MiAuth`, create a fresh UUID for each
authorization attempt, open the generated browser URL, and call `Check` after
the callback. MiAuth returns an access token but no refresh token or refresh
flow. The helper returns the credential to the caller and does not persist it.

## Limits and instance variance

The current upstream defaults limit `notes/create` to 300 calls per hour and
`drive/files/create` to 120 calls per hour. Other write endpoints have their
own windows and minimum intervals. Instance administrators and role policies
can change effective limits, so callers must treat HTTP 429 and `Retry-After`
as authoritative instead of hard-coding upstream defaults.

All current tests use deterministic local HTTP fixtures. The adapter has not
been validated against a real Misskey instance or user account.

The implementation was cross-checked against the official Misskey server
source and the archived GPL-3.0 `yitsushi/go-misskey` client for protocol
semantics. No code or dependency from that client is included.

Official references:

- <https://misskey-hub.net/en/docs/for-developers/api/>
- <https://misskey-hub.net/en/docs/for-developers/api/token/>
- <https://misskey-hub.net/en/docs/for-developers/api/token/miauth/>
- <https://misskey-hub.net/en/docs/for-developers/api/permission/>
- <https://github.com/misskey-dev/misskey>

Last verified: 2026-08-02.
