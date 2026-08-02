# TMDB API v3 adapter

`tmdb` implements the official [TMDB API v3](https://developer.themoviedb.org/reference/getting-started).
It exposes global movie, TV, and person metadata plus approved-user favorites,
watchlists, and ratings through typed workflows. TMDB resources are
intentionally not represented as portable social posts.

## Configuration

Import the adapter for registration:

```go
import _ "social-hub/adapters/tmdb"
```

The recommended application credential is TMDB's API Read Access Token. Store
it behind `secret_ref`. Store an approved v3 user `session_id` behind
`access_token_ref`:

```yaml
version: 1
platforms:
  - adapter: tmdb/v3
    accounts:
      - id: viewer
        secret_ref: env://TMDB_API_READ_ACCESS_TOKEN
        access_token_ref: env://TMDB_SESSION_ID
        settings:
          account_id: 42
          guest_session_ref: env://TMDB_GUEST_SESSION_ID
```

For legacy applications, `client_id` can contain a v3 API key when
`secret_ref` is absent. The adapter then sends `api_key` in the query string.
The bearer token is preferred because the same application authentication can
be used across TMDB v3 and v4.

`account_id` and `access_token_ref` are required only for `AccountWorkflow`
and account library operations. `guest_session_ref` is optional and enables
movie and TV rating operations without a full user session.

## User authentication

`AuthWorkflow` implements TMDB's v3 browser flow:

1. `RequestToken` creates an intermediate token.
2. `ApprovalURL` sends the user to TMDB to approve that token.
3. `CreateSession` exchanges the approved token for a `session_id`.

Request tokens expire after 60 minutes. TMDB v3 user sessions do not have a
refresh-token flow; persist the session ID as a secret and use `DeleteSession`
when disconnecting the account. `CreateGuestSession` creates a limited session
that TMDB deletes if it is not used within 60 minutes of issuance.

The adapter deliberately omits username/password validation because browser
approval is TMDB's preferred authorization method.

## Typed workflows

- `CatalogWorkflow`: multi-search, movie/TV/person details, trending, popular,
  and image configuration
- `AccountWorkflow`: approved account details
- `LibraryWorkflow`: favorite, watchlist, and rated lists; favorite/watchlist
  changes; movie and TV ratings
- `AuthWorkflow`: request-token approval, user session lifecycle, guest session

Rating values must be from 0.5 through 10 in half-point increments. A configured
user session takes precedence over a guest session for rating requests.

TMDB returns image paths rather than complete URLs. Use
`GetConfiguration().Images.SecureBaseURL`, a supported size such as `w500`, and
the returned poster/profile path to construct the image URL. Do not assume the
set of image sizes is static.

## Operational and licensing requirements

TMDB no longer applies its old 40 requests per 10 seconds limit, but documents
a variable upper limit around 40 requests per second. The adapter maps HTTP
`429` and `Retry-After` into the common retryable error model; clients should
limit bulk discovery traffic and honor the server response.

Non-commercial API use is available with required TMDB attribution. Commercial
products must contact TMDB for a commercial license. Production applications
must include TMDB's required notice: "This product uses the TMDB API but is not
endorsed or certified by TMDB." Review the official
[FAQ and attribution rules](https://developer.themoviedb.org/docs/faq) before
shipping.

## SDK assessment and verification scope

The active `github.com/cyruzin/golang-tmdb` project was reviewed as a reference.
It is not linked directly because its package-global base URL, internally
created contexts, and raw error formatting do not satisfy social-hub's
multi-account transport and error-sanitization contracts.

Tests use deterministic local HTTP fixtures and do not contact TMDB. The
adapter has not yet been validated with a live TMDB application or account.
