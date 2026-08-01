# Spotify Web API v1 adapter

This package implements the current public Spotify Web API v1 contract. It
does not use cookies, scraping, browser automation, private endpoints, or
reverse-engineered behavior. It never downloads or uploads Spotify audio.

## Capabilities

| Workflow | Support | OAuth scopes |
|---|---|---|
| `ProfileWorkflow` | Current authenticated profile; immutable `account_id` is used for account linking | Basic profile; `user-read-private` / `user-read-email` enrich fields |
| `CatalogWorkflow` | Single track and track search; search pages are capped at 10 | None for app-authorized catalog access |
| `LibraryWorkflow` | Saved-track listing plus generic save/remove/contains for Spotify URIs | Entity-specific `user-library-*`, `user-follow-*`, or playlist scopes |
| `PlaylistWorkflow` | Current-user playlists, metadata, current `/items` routes, and snapshot mutations | `playlist-read-private`, `playlist-modify-public`, `playlist-modify-private` |
| `PlaybackWorkflow` | State, devices, queue, transfer, play/pause, seek, skip, repeat, volume, and shuffle | `user-read-playback-state`, `user-read-currently-playing`, `user-modify-playback-state` |
| Common social interfaces | Not exposed | Spotify catalog/library objects are not portable posts, reactions, media uploads, or messages |
| Webhooks | Not exposed | Spotify Web API has no signed webhook contract |

Playback controls and queue mutation require Spotify Premium. A configured
non-Premium `approval.account_type` fails locally with
`socialhub.ErrApprovalRequired` before making a request.

## Configuration

```yaml
adapter: spotify/web-api-v1
product: spotify-web-api
accounts:
  - id: listener
    client_id: ${SPOTIFY_CLIENT_ID}
    secret_ref: env://SPOTIFY_CLIENT_SECRET
    access_token_ref: env://SPOTIFY_ACCESS_TOKEN
    approval:
      account_type: premium
      scopes:
        - user-read-private
        - user-library-read
        - user-library-modify
        - playlist-read-private
        - playlist-modify-private
        - user-read-playback-state
        - user-modify-playback-state
    settings:
      account_id: "aB3dE5fG7h"
```

`access_token_ref` is required for an API client. `client_id` is required only
for `Adapter.OAuth`; `secret_ref` is optional for PKCE but required for Client
Credentials. `settings.account_id` is optional. When set, it verifies the
immutable `account_id` returned by `/me` and prevents accidental account
switches.

## OAuth

`NewPKCE`, `AuthorizationURL`, and `Exchange` implement Authorization Code with
PKCE S256. PKCE exchange and refresh send `client_id` in the form; Client
Credentials uses HTTP Basic. `Refresh` retains the previous refresh token when
Spotify omits a replacement.

Spotify access tokens normally expire after one hour. Dashboard-issued refresh
tokens currently expire after six months and must then be reauthorized. The
adapter accepts HTTPS redirect URIs and loopback
`http://127.0.0.1[:port]/...`; it rejects `localhost`, insecure non-loopback
HTTP, credentials, and fragments. Implicit Grant is intentionally unsupported.

## Quotas and platform policy

Spotify applies a rolling 30-second API rate window. Callers should retry a
normalized `socialhub.ErrRateLimited` only after its `RetryAfter` duration.
Development Mode quota is shared across the developer account's Client IDs; a
quota rejection may use `reason: QUOTA_EXCEEDED`. Extended Quota requires
Spotify approval.

Development Mode currently limits an app to a small allowlist (typically five
authorized users), requires the app owner to have Premium, and allows up to 25
Client IDs per developer account. These platform-side restrictions cannot be
bypassed by OAuth scopes.

Spotify metadata, artwork, and playback are subject to Spotify's Developer
Terms. In particular, applications must link and attribute Spotify content,
must keep artwork in its original form, must not enable downloads or stream
ripping, and must not use Spotify content to train machine-learning or AI
models. Playback integrations have additional non-commercial streaming and
content-handling restrictions.

The implementation consulted the mature
[`zmb3/spotify`](https://github.com/zmb3/spotify) client for model and player
API conventions, but does not depend on it. Keeping the HTTP surface local
allows this adapter to exclude endpoints removed in Spotify's February 2026
migration.

## API references

- [Web API reference](https://developer.spotify.com/documentation/web-api/reference)
- [Official OpenAPI schema](https://developer.spotify.com/reference/web-api/open-api-schema.yaml)
- [February 2026 migration guide](https://developer.spotify.com/documentation/web-api/tutorials/february-2026-migration-guide)
- [Authorization Code with PKCE](https://developer.spotify.com/documentation/web-api/tutorials/code-pkce-flow)
- [Refreshing tokens](https://developer.spotify.com/documentation/web-api/tutorials/refreshing-tokens)
- [Rate limits](https://developer.spotify.com/documentation/web-api/concepts/rate-limits)
- [July 2026 quota update](https://developer.spotify.com/blog/2026-07-23-web-api-quota-updates)
- [Spotify developer policy](https://developer.spotify.com/policy)
