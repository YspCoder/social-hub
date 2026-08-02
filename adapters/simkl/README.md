# Simkl API v1 adapter

`simkl/v1` implements focused typed workflows for Simkl movie, TV, and anime
catalog discovery, public trending data, user-library synchronization, ratings,
and playback scrobbling. These resources are not flattened into common social
posts because doing so would lose watch status, episode, rating, and external-ID
semantics.

## API status

The adapter targets the official [Simkl API](https://api.simkl.org/) OpenAPI
`1.0.0` contract at `https://api.simkl.com` and static data files at
`https://data.simkl.in`. The contract and public movie, TV, and anime trending
files were verified on 2026-08-02.

Every production request should identify the application with `client_id`,
`app-name`, `app-version`, and `User-Agent`. The API accepts `simkl-api-key` as
an alternative location for `client_id`; this adapter uses only the documented
query parameter and never sends both. Current CDN files also respond without a
client ID, which enables the credential-free live smoke test, but production
integrations should still configure one as required by the official headers
contract.

## Configuration

Import the adapter for registration:

```go
import _ "social-hub/adapters/simkl"
```

Public trending files can currently be read without credentials:

```yaml
version: 1
platforms:
  - adapter: simkl/v1
    accounts:
      - id: public-trending
```

Catalog searches and details require a free Simkl application. User workflows
also require a caller-managed access token:

```yaml
version: 1
platforms:
  - adapter: simkl/v1
    settings:
      app_name: my-media-app
      app_version: 1.0.0
      user_agent: my-media-app/1.0.0
    accounts:
      - id: primary-user
        client_id: your-client-id
        secret_ref: env://SIMKL_CLIENT_SECRET
        access_token_ref: env://SIMKL_ACCESS_TOKEN
```

`secret_ref` is needed only for the confidential Authorization Code exchange.
Public mobile, SPA, desktop, and browser-extension clients should use PKCE S256
without a secret. `access_token_ref` is needed only for settings, sync, ratings,
and scrobble calls. Multiple accounts can share the same `client_id` while using
different access-token references.

## Authentication

`OAuthWorkflow` implements:

- confidential OAuth 2.0 Authorization Code URL and token exchange;
- public-client OAuth 2.0 with RFC 7636 PKCE S256;
- Simkl's custom PIN flow for TV, CLI, and media-server clients.

`NewPKCE` creates a 43-character verifier and S256 challenge. PKCE redirects may
be an HTTPS URL, a registered custom scheme, or omitted when the Simkl
application has no redirect URI. The confidential flow requires a byte-exact
registered redirect URI.

The PIN flow is not RFC 8628: request `GET /oauth/pin`, display `user_code`, and
poll `GET /oauth/pin/{user_code}`. `PollPIN` performs exactly one request and
returns a retryable `authorization_pending` error with `RetryAfter`; callers
must honor `PINAuthorization.Interval` and `ExpiresAt`. A response containing a
new `device_code` means the original code is gone and is mapped to
`pin_code_gone` instead of silently switching codes.

Simkl access tokens are long-lived, normally reporting `expires_in: 157680000`
(about five years), and remain usable until revoked. Simkl does **not** issue a
refresh token. Persist the token returned by an exchange or PIN poll and rerun
authorization after revocation; do not build a refresh loop. The adapter never
sends `client_secret` outside the `/oauth/token` form body.

## Typed workflows

| Workflow | Operations | Authentication |
|---|---|---|
| `OAuthWorkflow` | confidential/PKCE authorization and exchange, PIN request/poll | `client_id`; confidential exchange also needs `client_secret` |
| `CatalogWorkflow` | text search, movie details, TV details, anime details | `client_id` |
| `TrendingWorkflow` | movie/TV/anime today, week, or month top 100/500 CDN files | public CDN; production should identify the app |
| `UserWorkflow` | current-user settings | `client_id` + bearer token |
| `SyncWorkflow` | activities, all-items snapshots/deltas, watchlist placement, history and ratings add/remove | `client_id` + bearer token |
| `ScrobbleWorkflow` | start, pause, stop, and check-in | `client_id` + bearer token |

Search cursors are validated page numbers from 1 through 20, page size is capped
at the documented 50, and `X-Pagination-*` headers are mapped into
`socialhub.Page`. Search and CDN `ids.simkl_id` and API `ids.simkl` are decoded
through one structured `IDs` model; outgoing writes emit only `ids.simkl`.

`GetSettings` intentionally sends an empty `POST /users/settings`: this is a
read-only endpoint whose historical HTTP method remains POST in the current
contract.

## Sync and scrobble rules

Simkl requires a two-phase sync model. Perform an initial `ListAllItems` pull,
then call `GetActivities` before every later pull and use `date_from` only when
the relevant activity timestamp changed. Do not run unconditional background
polling. `ListAllItems` uses explicit `all/all` path segments and supports the
documented full/ID-only modes, episode timestamps, episode inclusion, next-watch
information, and memos.

Every write endpoint accepts arrays. Batch changes into `AddToList`,
`AddHistory`, `RemoveHistory`, `AddRatings`, and `RemoveRatings` instead of
sending one request per item. Movie watchlists reject `watching` and `hold`.
Ratings are integers from 1 through 10. Rewatch sessions are deliberately not
exposed in this initial adapter: they are Pro/VIP-gated and require callers to
pin a `rewatch_id` across writes, so a partial abstraction would be unsafe.

Scrobble requests accept exactly one of a movie, a show plus episode, or anime
plus episode. Episodes use either season/number or one episode-level TVDB/AniDB
ID. Progress is validated from 0 through 100.

## Operational and security notes

- Limits are 10 GET/second and 1 POST/second per unauthenticated `client_id`, or
  per authenticated user token. Public CDN files do not consume this quota.
- `429` is retryable and carries `Retry-After`. `412 client_id_failed` is
  intentionally classified as operator action because it can mean an invalid,
  disabled, quota-exhausted, or temporarily blocked client ID.
- Simkl recommends exponential backoff for transient reads. This adapter does
  not retry internally, and callers must not automatically retry writes because
  Simkl documents no idempotency-key contract.
- API and OAuth responses use the SDK's 8 MiB and 1 MiB bounds respectively.
  Redirects are rejected so bearer tokens, client IDs, authorization codes, and
  client secrets cannot cross origins.
- Public catalog/PIN requests and authenticated user requests use separate
  transports so a bearer token cannot accidentally move public traffic into a
  user's quota bucket.

## Terms and production use

Review the current [API rules](https://api.simkl.org/api-rules) before release.
Displaying Simkl data requires attribution and deep links; trending UI must name
Simkl. A tracker that also integrates another tracking service must expose Simkl
login/sync rather than using Simkl only as a hidden metadata source. The rules
currently allow free non-commercial use and commercial use below USD 150/month
in revenue; applications at or above that threshold require a commercial
license.

## Go SDK assessment

No maintained, permissively licensed Go Simkl SDK was suitable for direct use:

- [`Mahcks/blockbusterr`](https://github.com/Mahcks/blockbusterr) is an active
  MIT application and useful for integration discovery, but is not a reusable
  Simkl SDK.
- [`5rahim/seanime`](https://github.com/5rahim/seanime) and
  [`ad-on-is/odin-server`](https://github.com/ad-on-is/odin-server) are mature
  applications under GPL-3.0, so they cannot be linked into this SDK.
- [`Silo-Server/silo-server`](https://github.com/Silo-Server/silo-server) is an
  AGPL-3.0 application, while `godver3/mediastorm` publishes no usable license.

The adapter therefore uses social-hub's shared bounded transport and adds no
runtime dependency.
