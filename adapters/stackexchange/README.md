# Stack Exchange API v2.3 adapter

Package `social-hub/adapters/stackexchange` targets the official Stack
Exchange API v2.3. One configured account selects a `site` such as
`stackoverflow`; the same adapter can be initialized again for another site or
application key.

## Implemented contracts

| Surface | Support |
|---|---|
| Common `Fetcher` | User lookup, generic question/answer lookup, a user's questions, and post comments |
| Common `Reactor` | Human-initiated upvote/undo and comment create/delete with `write_access` |
| Typed `QnAWorkflow` | Question creation with title/tags/body, answer creation with question context, and advanced question search |
| Common `Publisher` | Not exposed because the common request cannot preserve required question tags/title or an answer's question target |
| Media, messages, webhooks | Not exposed by Stack Exchange API v2.3 |

`body_markdown` is preferred for common post/comment text when the API returns
it; raw HTML, Markdown, ownership, question/answer IDs, title, tags, counts,
license, and other platform fields remain available under the
`stackexchange.*` model extensions. Display names are decoded from the API's
HTML entity representation.

## Configuration and OAuth

```yaml
adapter: stackexchange/api-v2.3
accounts:
  - id: stackoverflow-main
    client_id: "stack-app-oauth-client-id"
    secret_ref: env://STACKEXCHANGE_CLIENT_SECRET
    access_token_ref: env://STACKEXCHANGE_ACCESS_TOKEN
    settings:
      site: stackoverflow
      user_id: "123456"
      api_key_ref: env://STACKEXCHANGE_API_KEY
    approval:
      scopes: [write_access, no_expiry]
```

`api_key_ref` resolves the Stack Apps API `key`; `client_id` is the separate
OAuth client ID. For local fixtures, `app_id` can contain the key directly,
but `api_key_ref` is the production-safe form because current Stack Apps keys
should be stored securely. The API key is required. `access_token_ref` is
optional for public reads and search, but writes and the common `Reactor`
require a user token.

The adapter sends `social-hub/stackexchange` as its default `User-Agent`.
Set `settings.user_agent` to an identifiable application value in production.

The OAuth helper implements Authorization Code and the currently recommended
Authorization Code + PKCE exchange at Stack Overflow. Use
`AuthorizationURLPKCE` with an S256 challenge and `ExchangeWithVerifier` with
the retained verifier. PKCE public clients may omit `secret_ref`; the basic
confidential-client exchange requires it.

Stack Exchange does not issue refresh tokens: a normal token expires according
to the returned `expires` value and must then be authorized again. Requesting
`no_expiry` creates a non-expiring token that can still be revoked. API
requests place `key` and, when configured, `access_token` in the query as
required by the API contract. API and OAuth clients refuse redirects so those
credentials cannot be forwarded to another origin.

Write access additionally requires a registered Stack Apps post for the
application. Low-quality, CAPTCHA, or guidance checks fail the API write rather
than opening an interactive challenge. Auto-commenting and other heuristic
write automation violate the published write policy.

## Quota, throttling, and policy

The default application quota is 10,000 requests per day. Anonymous calls
share quota by IP and application key; authenticated calls use a user/app
quota. Stack Exchange also enforces a 30 requests/second IP ceiling and can
return a dynamic `backoff` value in any response wrapper.

The adapter records the latest `quota_max`, `quota_remaining`, and `backoff`
snapshot through `Client.Quota()`. A returned backoff creates a thread-safe
deadline for that API method; repeat calls fail locally with
`CodeRateLimited` and `RetryAfter` until the deadline passes. Do not repeatedly
send semantically identical requests more than once per minute, and cache
stable metadata in the application layer.

Upvote and undo methods must proxy an explicit human action one-for-one. They
must not be used for automated voting, coordinated amplification, or bulk
interaction.

## Official documentation

- <https://api.stackexchange.com/docs>
- <https://api.stackexchange.com/docs/authentication>
- <https://stackapps.com/help/api-authentication>
- <https://api.stackexchange.com/docs/write>
- <https://api.stackexchange.com/docs/throttle>
