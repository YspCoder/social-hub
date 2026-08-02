# Trakt API adapter

`trakt` implements the official [Trakt API v2](https://docs.trakt.tv). It
exposes movie and TV discovery, user history, watchlists, ratings, comments,
and scrobbling through typed workflows. Media resources are intentionally not
represented as portable social posts, and the adapter uses only documented
public endpoints.

## Configuration

Import the adapter for registration:

```go
import _ "social-hub/adapters/trakt"
```

Configure the Trakt application client ID directly. Store the application
secret and authorized user access token behind secret references:

```yaml
version: 1
platforms:
  - adapter: trakt/v2
    settings:
      user_agent: my-service/1.0
    accounts:
      - id: viewer
        client_id: 0123456789abcdef
        secret_ref: env://TRAKT_CLIENT_SECRET
        access_token_ref: env://TRAKT_ACCESS_TOKEN
        settings:
          username: example-user
```

The client ID enables public catalog, profile, and comment reads. The client
secret enables browser and device OAuth workflows. An access token enables
current-user settings, sync, scrobbling, and comment writes.

## OAuth token lifecycle

`OAuthWorkflow` supports the browser authorization-code flow and Trakt's
device flow. `PollDevice` performs exactly one request; callers must schedule
subsequent attempts using `DeviceAuthorization.Interval` and stop at
`ExpiresAt`.

Access tokens currently expire after seven days. Refresh tokens are single-use:
after every successful `Refresh`, atomically persist both the new access token
and the new refresh token before issuing another refresh. Trakt does not return
the replacement token to an application that loses it.

## Typed workflows

- `CatalogWorkflow`: search and movie, show, episode, trending, and popular data
- `UserWorkflow`: profiles, settings, history, watchlist, and ratings
- `SyncWorkflow`: add or remove history, watchlist items, and ratings
- `ScrobbleWorkflow`: start, pause, and stop playback progress
- `CommentWorkflow`: activity, replies, creation, editing, deletion, and likes
- `OAuthWorkflow`: browser authorization, device authorization, refresh, revoke

Trakt accepts title and year matching for movie and show sync operations. IDs
are required for seasons, episodes, ratings, and scrobbling. A stop scrobble
below one percent is rejected locally to match the API contract.

## Operational limits

Trakt publishes separate limits for authenticated writes (one request per
second), authenticated reads (1,000 requests per five minutes), and public
reads (1,000 requests per five minutes). The adapter maps HTTP `429` and
`Retry-After` into the common retryable error model; applications should apply
the matching limiter policy rather than retrying writes concurrently.

Trakt image URLs must be cached and must not be hotlinked from application
clients. Review the official [API documentation](https://docs.trakt.tv),
[API application settings](https://trakt.tv/oauth/applications), and current
Trakt terms before production use.

## Verification scope

Tests use deterministic local HTTP fixtures and do not contact Trakt. The
adapter has not yet been validated with a live Trakt application or user
account.
