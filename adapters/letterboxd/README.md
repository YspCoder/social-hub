# Letterboxd API v0 adapter

`letterboxd/api-v0` implements a focused third-party subset of the official
[Letterboxd API v0](https://api-docs.letterboxd.com/). It exposes film-social
resources through typed workflows instead of representing diary entries,
reviews, or films as portable social posts.

## Access status

Letterboxd API access is currently an application-only beta. The official
[API access page](https://letterboxd.com/api-beta/) says access is not granted
for data analysis, visualization, recommendation, LLM/GPT, private/personal
projects, or products that recreate paid subscription features. Approval is
not guaranteed.

This package provides an implementation and deterministic local contract
tests. It does not grant API access and has not been exercised against a live
Letterboxd application or account.

## Configuration

Import the adapter for registration:

```go
import _ "social-hub/adapters/letterboxd"
```

Store the client secret and access token behind secret references. Set
`token_kind` to `client` for public reads or `user` for an Authorization Code
token:

```yaml
version: 1
platforms:
  - adapter: letterboxd/api-v0
    accounts:
      - id: movie-club
        client_id: your-client-id
        secret_ref: env://LETTERBOXD_CLIENT_SECRET
        access_token_ref: env://LETTERBOXD_ACCESS_TOKEN
        approval:
          scopes: [content:modify, oauth:refresh]
        settings:
          token_kind: user
```

Omit `access_token_ref` only while using `OAuthWorkflow` to obtain the first
token. API workflows require a configured bearer token. The adapter rejects a
`user` token kind without an access-token reference and rejects unknown,
duplicate, or first-party scopes during initialization.

## OAuth and OIDC

`OAuthWorkflow` supports:

- Client Credentials for public resources
- Authorization Code for member-authorized operations
- Refresh Token when `oauth:refresh` was granted
- RFC 7009 access-token and refresh-token revocation

Token requests are URL-encoded forms. The authorization endpoint is taken
from Letterboxd's current
[OIDC discovery document](https://letterboxd.com/.well-known/openid-configuration),
while token requests use `https://api.letterboxd.com/api/v0/auth/token`.
Access and refresh tokens should be persisted outside the adapter and replaced
before `ExpiresAt`.

The following requestable scopes are accepted:

```text
profile:private:view profile:modify security:modify content:modify
oauth:refresh openid profile email
```

The automatically granted `user` and `user:owner` scopes cannot be requested.
The Password flow, `client:firstparty`, `cache:modify`, and all endpoints or
fields marked `FIRST PARTY` are deliberately excluded.

## Typed workflows

| Workflow | Operations | Authentication |
|---|---|---|
| `OAuthWorkflow` | authorize, exchange, client credentials, refresh, revoke | client credentials |
| `CatalogWorkflow` | search, film details, filtered film lists | client or user token |
| `MemberWorkflow` | member details, current member, activity, public watchlist | token; `/me` needs user token |
| `LogEntryWorkflow` | list/get diary and review entries, comments, create/update/delete | reads need token; writes need user + `content:modify` |
| `RelationshipWorkflow` | like, rate/unrate, watched state, watchlist membership | user + `content:modify` |

Cursor pagination supports `perPage` values through 100. Letterboxd enforces
an upstream maximum of 100,000 objects across a paginated result set. Ratings
must be from 0.5 through 5.0 in half-point increments.

`GetFilm` currently uses the documented but deprecated `/film/{id}` endpoint.
Letterboxd recommends `/production`, but its public third-party contract is not
yet documented sufficiently for this adapter. `GET /films`, search, members,
log entries, and the generic `/me/{relationship}/{id}` write endpoints remain
part of the published API contract at the verification date.

The common `Publisher`, `Fetcher`, `MediaUploader`, `Reactor`, `Messenger`, and
`WebhookHandler` interfaces are intentionally not exposed. Letterboxd has no
third-party media upload, direct-message, or signed-webhook contract, and its
film/review relationship semantics do not map losslessly to generic posts and
reactions.

## Operational notes

- HTTP `429` and a valid `Retry-After` header map to the common retryable error
  model; Letterboxd does not publish a fixed general request quota.
- OAuth responses are bounded to 1 MiB and API responses use the shared 8 MiB
  transport limit.
- Redirects are not followed, preventing bearer tokens or client credentials
  from crossing origins.
- Platform messages and request IDs are bounded before entering the common
  error model; request/response bodies and credentials are never included.

## SDK assessment

Existing Go clients were reviewed but not linked directly:

- `github.com/jtschelling/letterboxd-api-go-client` uses the retired request
  signature and first-party Password flow.
- `github.com/jtschelling/go-letterboxd` does not cover the current OAuth and
  typed workflow contract.
- `github.com/golusoris/goenvoy/metadata/video/letterboxd` focuses on catalog
  reads and deprecated relationship endpoints, without the required complete
  token lifecycle.

The adapter therefore reuses social-hub's bounded authenticated transport and
adds no new dependency.
