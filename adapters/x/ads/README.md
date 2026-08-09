# X Ads API v12 adapter

Package `social-hub/adapters/x/ads` implements Ads Account scoped paid-media
workflows for X Ads API v12. Advertising resources intentionally remain
separate from common social `Post`, `Publisher`, and `Fetcher` operations and
from the organic `x/v2` adapter.

## Configuration

Import the package for registration and configure one social-hub account per X
Ads Account:

```go
import _ "social-hub/adapters/x/ads"
```

```yaml
adapter: x/ads-api-v12
product: ads-api
accounts:
  - id: paid-social-us
    client_id: x-consumer-key
    secret_ref: secret://x/consumer-secret
    access_token_ref: secret://x/access-token
    approval:
      account_type: standard_access
    settings:
      ads_account_id: "18ce54d4x5t"
      access_token_secret_ref: secret://x/access-token-secret
```

`approval.account_type: standard_access` is an optional local declaration that
the application has X Ads API Standard Access. It is not an OAuth scope and is
never sent to X. Ads API authorization also depends on the authenticated user's
role for the configured Ads Account.

All four OAuth credentials are resolved from references at runtime. API
requests use OAuth 1.0a HMAC-SHA1 in the `Authorization` header; credentials are
never placed in query parameters. Redirects are rejected so a signed request
cannot be forwarded to another origin. Endpoint overrides are intended only
for tests and controlled gateways.

## Access and OAuth

The production Ads API requires X Ads API Standard Access. The authenticated
user must also retain an account role such as `ACCOUNT_ADMIN`, `AD_MANAGER`,
`CAMPAIGN_ANALYST`, or `CREATIVE_MANAGER`. Use
`GetAuthenticatedUserAccess` to inspect the current `permissions` returned by
`/authenticated_user_access` rather than assuming that a valid token can
mutate every resource.

`Adapter.OAuth` implements X's three-legged OAuth 1.0a request-token,
authorization, and access-token flow. X does not issue a refresh token for
this flow and documents access-token credentials as non-expiring unless the
user revokes authorization. Applications remain responsible for encrypted
credential storage and reauthorization after revocation.

## Supported workflows

- Configured Ads Account and authenticated-user permission lookup.
- Campaign list/get, paused create, and typed name/status/budget updates.
- Line Item list/get, paused create, and typed name/status/bid/budget updates.
- Promoted Tweet list/get and association of up to 50 existing Tweet IDs.
- Synchronous non-segmented Stats for up to 20 entity IDs and a maximum
  seven-day, whole-hour time range. Metric names and arrays retain their raw
  JSON representation.

Campaign creation first reads the selected Funding Instrument and verifies its
Ads Account ownership. Line Item creation reads its Campaign; updates read both
the Line Item and parent Campaign. Promoted Tweet association reads its parent
and only writes when the Line Item is non-deleted, `PAUSED`, and has
`PROMOTED_TWEETS` product type. Cross-account or mismatched response identities
are rejected as platform contract errors.

Campaign and Line Item creation always sends `entity_status=PAUSED`. Enabling
delivery requires a separate explicit update. X returns newly associated
Promoted Tweet entities as `ACTIVE`; the paused-parent precondition prevents
that association from immediately serving.

X's v12 Line Item reference currently labels `end_time` as required for create,
but the official create request omits it and its response contains
`"end_time": null`. This adapter follows the executable official example and
makes `EndTime` optional. When supplied, it must be later than `StartTime`.

## Pagination and limits

Entity list `count` is limited to 1-1,000 by X and defaults to 200 when omitted.
The adapter exposes X's opaque `next_cursor` without following platform URLs.
Synchronous Stats do not use cursors: callers must partition requests by time
window and entity ID.

Published limits include:

- Writes: 450 requests per one-minute category window.
- Synchronous analytics: 250 requests per 15-minute category window.
- Core entity reads: 10,000 requests per 15 minutes at Ads Account level.
- Other account reads: 2,000 requests per 15 minutes where account-level
  limits apply.

When X returns both header families, the adapter prefers
`x-account-rate-limit-*` over user-level `x-rate-limit-*`. Reset timestamps map
to the shared retryable error's `RetryAfter` value. Platform response headers
remain authoritative because access tier and endpoint category can change the
effective quota.

## Exclusions

The initial adapter does not implement Funding Instrument mutation, targeting
criteria, promoted-only Post creation, Cards or media creatives, Custom
Audiences, conversions, asynchronous analytics jobs, batch mutation, deletion,
or advertising webhooks. These surfaces need separately reviewed typed
contracts rather than arbitrary passthrough maps.

## Contract sources

The official [Versioning](https://docs.x.com/x-ads-api/fundamentals/versioning),
[Access](https://docs.x.com/x-ads-api/fundamentals/accessing-ads-accounts),
[Campaign Management reference](https://docs.x.com/x-ads-api/campaign-management/reference),
[Analytics](https://docs.x.com/x-ads-api/analytics),
[Pagination](https://docs.x.com/x-ads-api/fundamentals/pagination),
[Rate limiting](https://docs.x.com/x-ads-api/fundamentals/rate-limiting), and
[Errors](https://docs.x.com/x-ads-api/fundamentals/error-codes-and-responses)
documentation are the source of truth.

There is no comparably current official or mature community Go SDK for these
typed Ads workflows. This package therefore uses social-hub's bounded shared
transport and its existing general-purpose OAuth 1.0a dependency without
adding an X Ads wrapper dependency.
