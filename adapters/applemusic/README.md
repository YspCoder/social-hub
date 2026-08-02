# Apple Music API adapter

This package implements the public Apple Music API v1 contract. It keeps music
catalog and personal-library entities in typed Apple Music workflows instead of
mapping them to social posts or reactions.

## Authentication

Every request needs an Apple developer token. Configure either:

- `access_token_ref` for an externally generated developer token; or
- `settings.team_id`, `settings.key_id`, and `secret_ref` for a Media Services
  `.p8` PKCS#8 P-256 private key.

The second form generates and caches an ES256 JWT locally. The default lifetime
is one hour and can be changed with `developer_token_ttl`, up to Apple's
`15777000`-second maximum.

Personal `/v1/me/*` operations also need `settings.music_user_token_ref`. The
referenced Music User Token must already have been obtained through MusicKit
with user authorization; Apple does not expose an OAuth exchange that this Go
adapter can perform.

```yaml
version: 1
platforms:
  - adapter: applemusic/api
    settings:
      developer_token_ttl: 1h
    accounts:
      - id: listener
        secret_ref: env://APPLE_MUSIC_PRIVATE_KEY_P8
        settings:
          team_id: TEAM123456
          key_id: KEY1234567
          storefront: US
          music_user_token_ref: env://APPLE_MUSIC_USER_TOKEN
```

## Typed workflows

- `StorefrontWorkflow`: public storefront discovery and current-user storefront
- `CatalogWorkflow`: songs, albums, artists, playlists, music videos, search,
  and charts
- `LibraryWorkflow`: personal-library lists/search and catalog additions
- `PlaylistWorkflow`: create library playlists and append catalog or library
  songs/music videos
- `HistoryWorkflow`: recently played resources and tracks

Apple returns relative `next` locations. The adapter validates that each
location remains on the expected API path and exposes only its numeric offset as
an opaque cursor. It never follows a server-provided URL.

This package does not expose social profiles, posts, comments, messaging,
webhooks, media upload, audio download, playback, or DRM bypass.

Official documentation:

- <https://developer.apple.com/documentation/applemusicapi>
- <https://developer.apple.com/documentation/applemusicapi/generating-developer-tokens>
- <https://developer.apple.com/documentation/applemusicapi/user-authentication-for-musickit>
