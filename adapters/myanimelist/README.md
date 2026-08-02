# MyAnimeList API v2 adapter

`myanimelist/v2` implements a focused subset of the official
[MyAnimeList API v2](https://myanimelist.net/apiconfig/references/api/v2).
It exposes anime, manga, profile, and personal-list resources through typed
workflows rather than forcing media-tracking data into common social posts.

## Access status

Applications must be registered in the
[MyAnimeList API configuration console](https://myanimelist.net/apiconfig).
Public catalog and public user-list reads can use a client ID. The current
user, personalized suggestions, and all list mutations require an OAuth user
token. List mutations additionally require the `write:users` scope.

This package provides an implementation and deterministic local contract
tests. It does not grant API access and has not been exercised against a live
MyAnimeList application or account.

## Configuration

Import the adapter for registration:

```go
import _ "social-hub/adapters/myanimelist"
```

Configure a public catalog client with only a client ID:

```yaml
version: 1
platforms:
  - adapter: myanimelist/v2
    accounts:
      - id: public-catalog
        client_id: your-client-id
```

For user-authorized workflows, store credentials behind secret references:

```yaml
version: 1
platforms:
  - adapter: myanimelist/v2
    accounts:
      - id: anime-fan
        client_id: your-client-id
        secret_ref: env://MAL_CLIENT_SECRET
        access_token_ref: env://MAL_ACCESS_TOKEN
        approval:
          scopes: [write:users]
```

`secret_ref` is optional for public OAuth clients. `access_token_ref` may be
omitted while `OAuthWorkflow` is used to obtain the first token. The returned
access and refresh tokens must be persisted by the application and supplied
through its token lifecycle.

## OAuth

`OAuthWorkflow` implements Authorization Code with PKCE and Refresh Token.
MyAnimeList currently requires the `plain` PKCE method, so the verifier and
challenge must be identical RFC 7636 values between 43 and 128 ASCII
characters. `NewPKCE` creates an appropriate random pair.

Confidential clients send `client_secret` in the token form; public clients
omit it. Token expiry is calculated from the response's `expires_in` value.
The adapter retains the previous refresh token when a refresh response does
not rotate it.

## Typed workflows

| Workflow | Operations | Authentication |
|---|---|---|
| `OAuthWorkflow` | authorize, exchange, refresh | client ID; secret optional |
| `AnimeWorkflow` | search, details, ranking, seasonal catalog, suggestions | client ID or user token; suggestions need user token |
| `MangaWorkflow` | search, details, ranking | client ID or user token |
| `UserWorkflow` | current user profile | user token |
| `AnimeListWorkflow` | public/own list reads, update, delete | client ID for public reads; user token for `@me`; writes need `write:users` |
| `MangaListWorkflow` | public/own list reads, update, delete | client ID for public reads; user token for `@me`; writes need `write:users` |

Pagination exposes only validated numeric `offset` cursors. Callers may use
`socialhub.WithFields` on reads to request documented optional fields. List
mutations use URL-encoded forms and deliberately reject field selection.
Pointer fields in update requests distinguish omission from explicit zero or
`false`; non-nil empty tags and comments clear those values upstream.

Forum endpoints are outside the initial scope. The common `Publisher`,
`Fetcher`, `MediaUploader`, `Reactor`, `Messenger`, and `WebhookHandler`
interfaces are intentionally not exposed: anime and manga catalog/list
semantics do not map losslessly to posts or reactions, and API v2 does not
document media upload, messaging, or signed webhooks.

## Operational notes

- Public API calls send `X-MAL-CLIENT-ID`; user calls send a bearer token.
- OAuth responses are bounded to 1 MiB and API responses use the shared 8 MiB
  transport limit.
- Redirects are not followed, preventing credentials from crossing origins.
- HTTP `429` and a valid `Retry-After` header map to the common retryable error
  model. MyAnimeList does not publish a fixed general request quota in the
  referenced API contract, so the adapter does not hard-code one.
- Platform messages and request IDs are bounded. Request/response bodies,
  paging URLs, and credentials are never included in returned errors.

## SDK assessment

`github.com/nstratos/go-myanimelist` was reviewed but is not linked directly.
Its client model does not provide social-hub's bounded transport, unified
error sanitization, multi-account configuration, or `CallOption` behavior.
The adapter therefore reuses social-hub's internal transport and adds no new
dependency.
