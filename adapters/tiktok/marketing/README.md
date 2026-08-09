# TikTok Business Marketing API adapter

Package `social-hub/adapters/tiktok/marketing` implements advertiser-scoped
paid-media workflows for TikTok API for Business Marketing API v1.3. Paid
Campaign, Ad Group, Ad, and reporting resources intentionally remain separate
from the organic `social-hub/adapters/tiktok` adapter and are not exposed as
common social `Post`, `Fetcher`, or `MediaUploader` operations.

## Configuration

Import the package for registration, configure one account per advertiser, and
resolve credentials through the application's `socialhub.SecretResolver`:

```go
import _ "social-hub/adapters/tiktok/marketing"
```

```yaml
adapter: tiktok/business-marketing-api-v1.3
product: business-marketing-api
accounts:
  - id: brand-global
    app_id: "1234567890"
    secret_ref: secret://tiktok-business/app-secret
    access_token_ref: secret://tiktok-business/brand-global/access-token
    settings:
      advertiser_id: "9876543210"
```

`app_id` and `secret_ref` are needed by `Adapter.OAuth`; API calls use the
resolved `access_token_ref` in the `Access-Token` header. Redirects are rejected
for both API and token requests so credentials cannot be forwarded to another
origin.

## Access and OAuth

Production access requires a TikTok for Business developer app, approval for
the requested Marketing API scopes, and authorization from each advertiser.
Account roles and regional or industry restrictions still apply after an app
has been approved.

Advertiser authorization starts at
`https://ads.tiktok.com/marketing_api/auth`. The returned `auth_code` is valid
for one hour and one use. `OAuthClient.Exchange` posts the code to
`/v1.3/oauth2/access_token/` and returns a Marketing long-term access token,
authorized advertiser IDs, and numeric scope IDs. This long-term Marketing
token does not expire and has no refresh token, so the adapter deliberately
does not expose `Refresh`.

Do not substitute the `/tt_user/oauth2/token/` flow: that separate TikTok
Business user-token product returns a one-day access token and a one-year
refresh token. The adapter rejects such short-term token response fields to
prevent the two credential lifecycles from being mixed. Applications remain
responsible for encrypted token storage and explicit revocation or rotation.

## Supported workflows

- Advertiser information lookup.
- Campaign list, paused create, update, and status changes.
- Ad Group list, paused create, update, and status changes.
- Batch Ad list, paused create (up to 20 creatives), and status changes.
- Synchronous integrated auction reports at Advertiser, Campaign, Ad Group, or
  Ad level.
- Browser authorization and long-term Marketing token exchange.

Campaigns, Ad Groups, and every Ad creative are created with
`operation_status=DISABLE`. Reach & Frequency Campaigns require an enabled or
omitted creation status in TikTok's contract, so this adapter returns a typed
`unsupported` error for `RF_REACH` rather than silently creating live spend.
Use a separately reviewed RF workflow when that product is required.

Complex provider fields can be supplied through `Fields` while the API surface
evolves. Advertiser ownership, resource IDs, names, creation status, and other
adapter-controlled keys cannot be overridden. Asset-library uploads, Smart+,
GMV Max, Reach & Frequency, audiences, asynchronous reports, Subscription API,
and organic TikTok publishing are outside this initial package.

## Versions and limits

The implemented routes are pinned to the official Marketing API `v1.3` path.
This package is independent of the organic TikTok API v2 adapter.

Default Basic access is documented as 10 QPS, 600 QPM, and 864,000 QPD.
Advanced, Premium, and Ultimate tiers increase those limits to 20/1,200/
1,728,000, 30/1,800/2,592,000, and 50/3,000/4,320,000 respectively. The Basic
`/ad/create/` endpoint is lower at 5 QPS, 150 QPM, and 86,400 QPD. Error `40100`
indicates throttling; a QPM suspension requires a five-minute wait, while QPD
resets at 00:00 UTC. Applications should still treat dashboard-assigned quotas
and response errors as authoritative.

Synchronous integrated reports allow one inclusive date with
`stat_time_hour`, 30 inclusive dates with `stat_time_day`, or 365 inclusive
dates without a time dimension. Page size is limited to 1,000. The adapter
validates these boundaries before sending a request.

## Contract sources

The official [TikTok API for Business documentation](https://business-api.tiktok.com/portal/docs)
is the source of truth. The public
[`bububa/tiktok-business`](https://github.com/bububa/tiktok-business) project
was audited at tag `v1.5.8` (commit
`46aa1a10cc2ba79bbdfce6b2309f25bbc641066f`) as an Apache-2.0-licensed model and
endpoint reference. Its source was not imported; where it differs from current
official documentation, the official contract takes precedence.
