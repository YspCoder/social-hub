# ListenBrainz API v1 adapter

The `listenbrainz/api-v1` adapter implements the official
[ListenBrainz API](https://listenbrainz.readthedocs.io/en/latest/users/api/index.html)
for public listening-history, recording-feedback, and JSPF playlist reads plus
token-authorized listen and feedback mutations.

Typed workflows expose:

- `AuthWorkflow`: validate the configured user token;
- `ListeningWorkflow`: user search, listens, playing now, listen count, `single`,
  `import`, and `playing_now` submissions, and scheduled listen deletion;
- `FeedbackWorkflow`: public recording-feedback reads and token-authorized
  love, hate, or removal submissions;
- `PlaylistWorkflow`: public playlist search, user playlist listing, and JSPF
  playlist retrieval.

ListenBrainz resources are deliberately not coerced into portable social posts
or reactions. `additional_info` preserves client-submitted identifiers while
the separate `MBIDMapping` type represents ListenBrainz's read-only canonical
MusicBrainz resolution. JSPF extensions use `map[string]json.RawMessage` because
their keys are specification URLs.

## Configuration

Import the adapter for registration and configure an SDK-local account. Public
reads need no credentials. Store a user token behind `access_token_ref` to
enable mutations and token validation:

```go
import _ "social-hub/adapters/listenbrainz"
```

```yaml
version: 1
platforms:
  - adapter: listenbrainz/api-v1
    accounts:
      - id: listener
        access_token_ref: env://LISTENBRAINZ_USER_TOKEN
        settings:
          username: example-user
```

Tokens are created and reset in [ListenBrainz settings](https://listenbrainz.org/settings/).
They are private API keys, not OAuth tokens, and requests use
`Authorization: Token <user token>`. The adapter resolves the token once when a
client is created and never sends an authorization header for a public client.

```go
common, err := adapter.Client(ctx, "listener")
if err != nil {
    return err
}
client := common.(*listenbrainz.Client)
page, err := client.ListListens(ctx, listenbrainz.ListensRequest{Count: 25})
```

## Contract and operational boundaries

- `single` accepts exactly one timestamped listen; `playing_now` accepts exactly
  one listen without a timestamp; `import` accepts 1 through 1000 listens.
- The adapter enforces the documented 10,240-byte per-listen and 10,240,000-byte
  request limits, plus 50 tags per listen and 64 characters per tag.
- `min_ts` and `max_ts` are exclusive. `ListenPage` retains the API's timestamp
  metadata instead of pretending this is offset pagination.
- Playlist responses preserve JSPF and MusicBrainz extensions. This first
  version intentionally exposes read-only playlist operations.
- Every response includes rate-limit headers. `429` maps to a retryable SDK
  error and uses `Retry-After` before `X-RateLimit-Reset-In` for `RetryAfter`.
  Authenticated requests may receive a higher quota.
- User listen data and user text are published under CC0 according to the
  [ListenBrainz terms](https://listenbrainz.org/terms-of-service/). MusicBrainz
  metadata included in enriched responses can have separate licensing terms;
  consumers remain responsible for the fields they retain.

Official references: [API and rate limits](https://listenbrainz.readthedocs.io/en/latest/users/api/index.html),
[listen JSON](https://listenbrainz.readthedocs.io/en/latest/users/json.html),
[feedback JSON](https://listenbrainz.readthedocs.io/en/latest/users/feedback-json.html),
and [playlist API](https://listenbrainz.readthedocs.io/en/latest/users/api/playlist.html).

## Go SDK assessment

No mature reusable Go SDK met this module's maintenance and licensing bar as of
2026-08-02, so the adapter uses social-hub's bounded transport without a new dependency.

| Project | Assessment | Decision |
|---|---|---|
| [luizdemilon/go-listenbrainz](https://github.com/luizdemilon/go-listenbrainz) | Small GPL-3.0 client | Cannot link into this SDK |
| [faelau/listenbrainz-go](https://github.com/faelau/listenbrainz-go) | Generated client with no established adoption | Reference only |
| [rain0r/listenbrainz-openapi](https://github.com/rain0r/listenbrainz-openapi) | Active GPL-3.0 community schema; linked by official docs but not an official SDK | Schema reference only |
| [navidrome/navidrome](https://github.com/navidrome/navidrome) | Mature application with an internal ListenBrainz client | Behavior reference only |
| [LumePart/Explo](https://github.com/LumePart/Explo) | MIT application with internal integration, not a reusable Go SDK | Behavior reference only |
| [gabehf/Koito](https://github.com/gabehf/Koito) | MIT scrobbler/server, not a client SDK | Behavior reference only |

## Verification scope

Deterministic tests cover authentication, public-read degradation, request and
response contracts, validation, pagination, rate-limit errors, and redirect
refusal. Set `LISTENBRAINZ_LIVE_TEST=1` to run the credential-free public smoke
test; it never mutates remote state.
