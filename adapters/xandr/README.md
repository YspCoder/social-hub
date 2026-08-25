# Xandr Digital Platform API adapter

Registration name: `xandr/digital-platform-api`

This package implements a deliberately small, read-only slice of the current
Microsoft Advertising/Xandr Digital Platform API. The service is continuously
deployed and does not publish a stable version segment in its production URL;
`Metadata.APIVersion` is therefore `continuous` rather than a fabricated
numeric version.

Official references reviewed on 2026-08-25:

- [Digital Platform API](https://learn.microsoft.com/en-us/xandr/digital-platform-api/)
- [Authentication Service](https://learn.microsoft.com/en-us/xandr/digital-platform-api/authentication-service)
- [API Semantics](https://learn.microsoft.com/en-us/xandr/digital-platform-api/api-semantics)
- [API Usage Constraints](https://learn.microsoft.com/en-us/xandr/digital-platform-api/api-usage-constraints)
- [Advertiser Service](https://learn.microsoft.com/en-us/xandr/digital-platform-api/advertiser-service)
- [Campaign Service](https://learn.microsoft.com/en-us/xandr/digital-platform-api/campaign-service)

## Implemented contract

| Workflow | Official endpoint |
|---|---|
| Authenticate a managed API user | `POST /auth` |
| Read an Advertiser | `GET /advertiser?id={advertiser_id}` |
| List Advertisers | `GET /advertiser` |
| Read an advertiser's Campaign | `GET /campaign?id={campaign_id}&advertiser_id={advertiser_id}` |
| List an advertiser's Campaigns | `GET /campaign?advertiser_id={advertiser_id}` |

Writes, reporting, creatives, line items, segments, pixels, and other services
are outside this initial adapter. Paid-media resources are intentionally not
projected onto social-hub's organic `Fetcher` or `Publisher` interfaces.

This is the Xandr Digital Platform API, not the separate Microsoft Advertising
Campaign Management API. Its endpoints are available only to current Xandr UI
or Invest customers whose supported user type and existing user/member IDs have
been approved for API access by Microsoft Advertising support. The adapter
cannot substitute for that commercial onboarding or downgrade to another ads
product.

## Authentication

The Digital Platform API uses a proprietary session token, not OAuth. Configure
the Xandr username in `client_id` and keep the password behind `secret_ref`:

```yaml
version: 1
platforms:
  - adapter: xandr/digital-platform-api
    product: digital-platform-api
    accounts:
      - id: production-api-user
        client_id: xandr-username
        secret_ref: env://XANDR_PASSWORD
        approval:
          account_type: digital-platform-api
```

The adapter calls `POST /auth` on first use and sends the returned value as the
entire `Authorization` header. It keeps the token only in process and expires it
conservatively two hours after authentication. Xandr sessions remain active for
two hours after the most recent API call but have a hard 24-hour lifetime. To
avoid the documented limit of ten successful authentications per five minutes,
the adapter reauthenticates after a response only when `error_id` is exactly
`NOAUTH`, and retries that read once. `NOAUTH_DISABLED` and `NOAUTH_EXPIRED`
remain user-action errors.

Session tokens are deliberately not written to `socialhub.TokenStore` and a
static `access_token_ref` is rejected because Xandr sessions are short-lived.
Microsoft also documents JWT-based API authentication; it is outside this
managed username/password session adapter's initial surface.
`Client.Close` clears the resolved username, password, and cached session token
and prevents subsequent session acquisition.
The API and authentication origins are fixed to `https://api.appnexus.com`;
adapter-level endpoint settings are rejected. Redirects are not followed and
the cloned HTTP client discards its Cookie Jar, so the username, password, and
session token cannot be redirected or replayed by inherited cookies.

Caller request IDs, idempotency keys, and field-selection CallOptions are not
forwarded by this typed read surface. The original CallOptions are evaluated
once, and one timeout deadline covers initial authentication plus the optional
single `NOAUTH` reauthentication retry.

## Pagination and rate limits

`ListOptions` supports `state`, `search`, `start_element`, and `num_elements`.
Offsets are zero-based; page size defaults to and is capped at 100. Campaign
operations always require `advertiser_id`, and returned Campaign ownership is
checked before data is exposed.

Rate limits are dynamic at both API-user and service levels. HTTP `429` is
treated as user throttling. HTTP `503` with `x-ratelimit-code` is treated as
service throttling; other `503` responses are temporary availability failures.
`Retry-After` is preserved on `socialhub.Error`, while current
numeric `x-ratelimit-code` and `x-ratelimit-count` values are exposed through
`ResponseMeta` on successful reads and `APIError` on failures. The current
bounded `x-b3-traceid` is exposed as the request ID. `x-an-user-id` is not
exposed because it is an account identifier rather than a correlation ID.
Deprecated `x-count-*` and older `x-ratelimit-*` headers are not consumed.

Provider free-form error text is replaced by a fixed local message. The
documented `dbg_info` object is discarded because Microsoft marks it for Xandr
internal support use only. Individual advertiser/campaign JSON objects remain
available as `json.RawMessage`, preserving resource extensions and exact budget
values without exposing the full response envelope.
