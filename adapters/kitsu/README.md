# Kitsu JSON:API edge adapter

`kitsu/edge` implements a focused subset of Kitsu's JSON:API for anime and
manga discovery, profiles, library tracking, posts, and comments. These
resources are exposed as typed workflows so their JSON:API relationships are
not flattened into lossy common post models.

## API status

The adapter targets `https://kitsu.io/api/edge`. `edge` is an unversioned API
surface and can change without a version migration window. The current
[OpenAPI documentation](https://hummingbird-me.github.io/api-docs/) covers the
catalog surface; the older official [Apiary contract](https://kitsu.docs.apiary.io/)
and current server routes describe the broader users, library, posts, and
comments surface.

Public Anime, Manga, User, Library Entry, Post, and Comment reads were
smoke-tested against `kitsu.io` on 2026-08-02. OAuth refresh and all mutations
use deterministic local fixtures because this repository does not carry a
Kitsu user credential.

## Configuration

Import the adapter for registration:

```go
import _ "social-hub/adapters/kitsu"
```

Public catalog and social reads need no credential:

```yaml
version: 1
platforms:
  - adapter: kitsu/edge
    accounts:
      - id: public-kitsu
```

Owner mutations require both a caller-managed bearer token and the positive
decimal Kitsu user ID used in JSON:API relationships:

```yaml
version: 1
platforms:
  - adapter: kitsu/edge
    accounts:
      - id: anime-fan
        client_id: optional-oauth-client-id
        secret_ref: env://KITSU_CLIENT_SECRET
        access_token_ref: env://KITSU_ACCESS_TOKEN
        settings:
          user_id: "42"
```

`client_id` and `secret_ref` are optional and are sent only when a refresh
token is bound to an OAuth client. Kitsu OAuth does not expose scopes, so
configured scopes are rejected.

## Authentication

Kitsu's current server enables Resource Owner Password Credentials and an
assertion grant, with refresh tokens and 30-day access tokens. This SDK
deliberately does **not** implement password login: applications must not hand
Kitsu usernames or passwords to social-hub. Supply an already-issued bearer
token through `access_token_ref`; use `TokenWorkflow.Refresh` to rotate its
refresh token. The returned token must be persisted by the application and
then supplied through its secret resolver.

Do not assume Authorization Code or PKCE support: those grants are not enabled
by the current server configuration.

## Typed workflows

| Workflow | Operations | Authentication |
|---|---|---|
| `TokenWorkflow` | refresh an existing token | refresh token; client credentials when required by the original grant |
| `AnimeWorkflow` | search and details | public |
| `MangaWorkflow` | search and details | public |
| `UserWorkflow` | ID/slug lookup and current user | public; current user requires bearer token |
| `LibraryWorkflow` | filtered list/details, create/update/delete | public reads; owner mutations require bearer token and configured `user_id` |
| `PostWorkflow` | global recent list/details, create/update/delete | public reads; owner mutations require bearer token and configured `user_id` |
| `CommentWorkflow` | post comments/details, create/update/delete | public reads; owner mutations require bearer token and configured `user_id` |

Library writes preserve pointers for explicit `0`, `false`, empty notes, and
dates. `ratingTwenty`, when present, must be in the Kitsu range `2..20`.
Library statuses are `current`, `planned`, `completed`, `on_hold`, and
`dropped`. Post and comment content is limited to 9,000 bytes before transport.

Collection cursors are validated decimal `page[offset]` values and page size
is capped at the documented maximum of 20. The adapter extracts only the
numeric offset from response links and never follows a pagination URL. Arbitrary
`socialhub.WithFields` selections are rejected because the typed response
models and response-size contract use fixed fields.

The common `Publisher`, `Fetcher`, `MediaUploader`, `Reactor`, `Messenger`, and
`WebhookHandler` interfaces are intentionally not exposed. Kitsu's global post
list has no working user filter, profile feeds require authentication, library
entries are not posts, and media upload, likes, and webhooks are outside the
initial adapter scope.

## Operational notes

- Requests use `application/vnd.api+json`; API responses share the SDK's 8 MiB
  bound and OAuth responses are limited to 1 MiB.
- Redirects are rejected so bearer tokens, refresh tokens, and optional client
  credentials cannot cross origins.
- JSON:API `errors[]`, OAuth errors, request IDs, HTTP status, and
  `Retry-After` are mapped into `socialhub.Error` without retaining bodies or
  request URLs.
- Kitsu publishes no global quota or stable `X-RateLimit-*` contract. Current
  server rules separately throttle post creation at 3/minute/token, post and
  comment likes at 40/120 seconds/token, and follows at 50/300 seconds/token.
  The adapter therefore does not invent a global limiter and instead maps
  `429` plus `Retry-After` for caller policy.

## Terms and production use

Review the current [Kitsu Terms](https://kitsu.io/terms) before production use.
The official materials do not publish a blanket commercial-use allowance, so
commercial deployments should confirm that their data use and automation are
permitted.

## SDK assessment

Existing Go clients were reviewed but not linked directly:

- [`github.com/nstratos/go-kitsu`](https://github.com/nstratos/go-kitsu) is
  MIT-licensed and useful for JSON:API catalog, user, and library modeling. It
  describes itself as under development, targets unstable `edge`, and does not
  provide social-hub's bounded transport, multi-account configuration, current
  post/comment workflows, or unified errors.
- [`github.com/animenotifier/kitsu`](https://github.com/animenotifier/kitsu) is
  MIT-licensed but has not been updated since 2019.

The adapter therefore uses social-hub's shared transport and adds no runtime
dependency.
