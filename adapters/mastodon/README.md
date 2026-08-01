# Mastodon REST API adapter

Package `social-hub/adapters/mastodon` targets the official Mastodon REST API
across independently operated instances. Each account supplies its own
`instance_url`; the adapter does not assume mastodon.social or any other hosted
service.

Implemented contracts:

- dynamic OAuth application registration, authorization-code OAuth 2.0,
  optional S256 PKCE, and client-credentials app tokens;
- profile lookup and credential verification, account status pages, status
  lookup, thread-context comments, and authenticated home timeline;
- status publishing, replies, deletion, favourites, boosts, and API-v7 quote
  IDs on instances that advertise support;
- asynchronous `/api/v2/media` upload as one common upload part, completion,
  and `/api/v1/media/:id` processing status;
- `/api/v2/instance` discovery for server version, Mastodon API version, status
  limits, attachment counts, and media-size limits;
- RFC Link header pagination plus Mastodon HTTP and rate-limit error mapping.

Status `content`, account `note`, and profile fields are HTML by API contract.
The common `Text` fields preserve that content without lossy ad-hoc stripping,
and the original values remain under `mastodon.*` extensions. Direct-visibility
statuses are available through publishing, but the common `Messenger` remains
disabled because Mastodon conversations do not expose the common message
retrieval contract. Administrative webhooks and streaming are not included in
this first adapter.

The implementation was cross-checked against the mature MIT-licensed
[`mattn/go-mastodon`](https://github.com/mattn/go-mastodon) client. It retains
social-hub's transport and error contracts, while using the same focused
`linkheader` parser for server-provided pagination instead of importing a
second complete client stack.

Mastodon defaults are commonly 300 requests per five minutes per account and
per IP, with narrower endpoint limits, but instance administrators may change
them. Consume `X-RateLimit-*`, `Retry-After`, and HTTP 429 rather than hard-code
the published defaults.

Example account settings:

```yaml
adapter: mastodon/rest
accounts:
  - id: fediverse-main
    client_id: "registered-client-id"
    secret_ref: env://MASTODON_CLIENT_SECRET
    access_token_ref: env://MASTODON_ACCESS_TOKEN
    settings:
      instance_url: https://mastodon.example
      user_id: "109000000000000000"
    approval:
      scopes:
        - read:accounts
        - read:statuses
        - write:statuses
        - write:media
        - write:favourites
```

Official documentation:

- <https://docs.joinmastodon.org/methods/apps/>
- <https://docs.joinmastodon.org/methods/oauth/>
- <https://docs.joinmastodon.org/methods/accounts/>
- <https://docs.joinmastodon.org/methods/statuses/>
- <https://docs.joinmastodon.org/methods/media/>
- <https://docs.joinmastodon.org/methods/timelines/>
- <https://docs.joinmastodon.org/methods/instance/>
- <https://docs.joinmastodon.org/api/rate-limits/>
