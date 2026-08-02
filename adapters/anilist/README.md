# AniList GraphQL API v2 adapter

`anilist/graphql-v2` implements a focused subset of the official
[AniList GraphQL API v2](https://docs.anilist.co/). It exposes anime and manga
discovery, user profiles, media-list tracking, and social activities through
typed workflows rather than representing tracking records as common posts.

## Access status

Public GraphQL queries do not require credentials. Viewing private user data
or executing mutations requires an AniList user access token. Register OAuth
applications in the [developer settings](https://anilist.co/settings/developer).

The complete public `Media` and `User` field selections were smoke-tested
against `https://graphql.anilist.co` on 2026-08-02. OAuth and mutation tests use
deterministic local fixtures; this package has not been exercised with a live
AniList application or user account.

## Configuration

Import the adapter for registration:

```go
import _ "social-hub/adapters/anilist"
```

A credential-free account can use public catalog, profile, media-list, and
activity reads:

```yaml
version: 1
platforms:
  - adapter: anilist/graphql-v2
    accounts:
      - id: public-catalog
```

For Authorization Code and user-authorized workflows, store the client secret
and access token behind secret references:

```yaml
version: 1
platforms:
  - adapter: anilist/graphql-v2
    accounts:
      - id: anime-fan
        client_id: "12345"
        secret_ref: env://ANILIST_CLIENT_SECRET
        access_token_ref: env://ANILIST_ACCESS_TOKEN
```

An implicit-flow client needs `client_id` but not `secret_ref`. The access
token reference may be omitted while `OAuthWorkflow` obtains the first token.
AniList does not support OAuth scopes, so the adapter rejects configured
scopes instead of implying permissions that do not exist.

## OAuth

`OAuthWorkflow` supports Authorization Code and Implicit grants. Authorization
Code exchange requires the client secret; the Implicit grant returns the token
in the callback URL fragment and is intended only for clients that cannot keep
a secret. Both authorization URL helpers require a non-empty `state`; callers
must compare it with the callback value before accepting a code or token.

AniList access tokens currently remain valid for one year. Refresh tokens and
scopes are not supported, so an expired or revoked token requires the user to
authorize again. The returned token must be persisted outside the adapter and
supplied through `access_token_ref` for authenticated workflows.

## Typed workflows

| Workflow | Operations | Authentication |
|---|---|---|
| `OAuthWorkflow` | authorization URL, implicit URL, code exchange | client ID; code exchange also needs secret |
| `MediaWorkflow` | search, details, trending, seasonal anime | public |
| `UserWorkflow` | public user lookup, authenticated Viewer | public or user token for Viewer |
| `MediaListWorkflow` | paginated list reads, save/update/delete entries | public reads; user token for mutations |
| `ActivityWorkflow` | text/list activity reads, create/update/delete text, replies, `ToggleLikeV2` | public reads; user token for mutations/following feed |

GraphQL selections are fixed by each typed operation. `socialhub.WithFields`
is deliberately rejected because accepting arbitrary fields would bypass the
response model and response-size contract. Numeric page cursors are validated,
and AniList's documented maximum of 50 items per page is enforced.

Pointer fields in media-list and text-activity mutations distinguish omission
from explicit zero, `false`, an empty string, or an empty custom-list set.
`ToggleLikeV2` is exposed only by the typed workflow because it toggles state
and is not an idempotent implementation of the common `Reactor` contract.

The common `Publisher`, `Fetcher`, `MediaUploader`, `Reactor`, `Messenger`, and
`WebhookHandler` interfaces are intentionally not exposed. AniList media-list
entries and activity unions do not map losslessly to generic posts, the API
does not expose media upload or signed webhooks, and private message activities
are outside the initial adapter scope.

## Operational notes

- The documented normal limit is 90 requests per minute, but the API is
  currently degraded to 30 requests per minute and also applies burst limits.
  The adapter therefore does not hard-code a fixed limiter. It maps HTTP or
  GraphQL `429` responses and `Retry-After` into the common retryable error
  model so applications can follow current response metadata.
- API responses use the shared 8 MiB bound; OAuth responses are limited to
  1 MiB.
- Redirects are not followed, preventing bearer tokens, authorization codes,
  or client credentials from crossing origins.
- Both non-2xx GraphQL errors and `HTTP 200` responses containing `errors` are
  classified. Transport errors are stripped of nested request URLs before
  entering the common error model.
- AniList pagination metadata such as `total` and `lastPage` can be inaccurate;
  pagination follows only `currentPage` and `hasNextPage`.

## Terms and production use

Review AniList's [API terms](https://anilist.gitbook.io/anilist-apiv2-docs/docs/guide/terms-of-use)
before production use. They prohibit using the API as backup storage and mass
data collection. Competing anime/manga trackers may require explicit approval.
Commercial products earning more than USD 150 per month require a commercial
license, and products using the AniList name must follow the unofficial naming
requirements.

## SDK assessment

Existing Go clients were reviewed but not linked directly:

- [`github.com/rl404/verniy`](https://github.com/rl404/verniy) is MIT-licensed
  and useful as a reference for catalog field modeling, but it does not cover
  the OAuth lifecycle, typed mutations, unified errors, multi-account config,
  or social-hub `CallOption` behavior required here.
- [`github.com/minnasync/anilist-go`](https://github.com/minnasync/anilist-go)
  is GPL-3.0 and explicitly describes itself as incomplete and not production
  ready outside its parent application.

The adapter therefore reuses social-hub's bounded authenticated transport and
adds no dependency.
