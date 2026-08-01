# SoundCloud Public API adapter

This package implements the official SoundCloud Public API OpenAPI 1.0.0
contract. It does not use cookies, scraping, browser automation, private
endpoints, or reverse-engineered behavior.

## Capabilities

| Capability | Support |
|---|---|
| Common `Fetcher` | Users, tracks, account/user tracks, and track comments |
| Common `Reactor` | Like/unlike, repost creation, and comments |
| Typed `ActivityWorkflow` | `/me/feed` with Track, Playlist, and repost activity semantics retained |
| Typed `TrackUploadWorkflow` | Streaming multipart upload, metadata status/read, update, and deletion |
| Common `Publisher` / `MediaUploader` | Not exposed; an audio upload creates the Track resource directly |
| Messaging / webhooks | Not exposed by the SoundCloud Public API |

The public unrepost endpoint is deprecated, so `RemoveReaction` returns
`socialhub.ErrUnsupported` for `ReactionRepost`. SoundCloud does not publish a
comment-deletion endpoint or threaded comments.

## Configuration

```yaml
adapter: soundcloud/public-api-v1
product: soundcloud-public-api
accounts:
  - id: artist
    client_id: ${SOUNDCLOUD_CLIENT_ID}
    secret_ref: env://SOUNDCLOUD_CLIENT_SECRET
    access_token_ref: env://SOUNDCLOUD_ACCESS_TOKEN
    approval:
      account_type: artist_pro
    settings:
      user_urn: "soundcloud:users:123456"
```

`client_id` and `secret_ref` are needed only when using `Adapter.OAuth`.
`access_token_ref` is required for the API client. SoundCloud currently requires
Artist Pro to register an application, and all clients are treated as
confidential clients.

OAuth uses Authorization Code with PKCE S256, Client Credentials with HTTP
Basic credentials, and single-use refresh tokens. Persist every newly returned
refresh token atomically before discarding the previous token.

## Track uploads

`TrackUploadWorkflow.Upload` requires the exact audio byte size and streams the
multipart body with `io.Pipe`; it does not buffer the audio file. Each upload may
be up to 4 GB and 24 hours. SoundCloud queues successful uploads for transcoding.
The upload response is therefore returned as `pending`/`processing`.

The current public Track schema does not expose a distinct transcoding-state
field. `Status` retrieves current Track metadata; callers should inspect the
SoundCloud extension fields such as `streamable` and `access` instead of
assuming a portable encoding-state transition.

## API references

- [API guide](https://developers.soundcloud.com/docs/api/guide)
- [API reference](https://developers.soundcloud.com/docs/api/reference)
- [Official OpenAPI repository](https://github.com/soundcloud/api)
- [Rate limits](https://developers.soundcloud.com/docs/api/rate-limits)
