# Pinterest Ads API v5.28 adapter

Package `social-hub/adapters/pinterest/ads` implements ad-account-scoped paid
media workflows for Pinterest Ads API v5.28. Campaigns, Ad Groups, Ads, and
Analytics intentionally remain separate from common social `Post`, `Fetcher`,
and `MediaUploader` operations.

## Configuration

Import the package for registration and configure one social-hub account per
Pinterest Ad Account:

```go
import _ "social-hub/adapters/pinterest/ads"
```

```yaml
adapter: pinterest/ads-v5.28
product: ads
accounts:
  - id: visual-commerce
    client_id: pinterest-app-id
    secret_ref: secret://pinterest/client-secret
    access_token_ref: secret://pinterest/access-token
    approval:
      scopes:
        - ads:read
        - ads:write
    settings:
      ad_account_id: "111111111111"
```

Access tokens and client secrets are always resolved through secret references.
The configured `ad_account_id` is checked against returned resources, and
redirects are rejected so bearer or Basic credentials cannot be forwarded to
another origin. Endpoint overrides are intended only for tests and controlled
gateways.

## Access and OAuth

Pinterest Ads access requires an approved app, Business Access, an advertiser
role, and an eligible Ad Account. Billing state and Pinterest ad review can
still prevent delivery after an API mutation succeeds.

`Adapter.OAuth` supports OAuth 2.0 authorization-code, refresh-token, and
client-credentials grants for `ads:read` and `ads:write`. It preserves an
existing refresh token when a refresh response omits a replacement. The
application remains responsible for encrypted token storage and proactive
refresh before access or refresh-token expiry.

## Supported workflows

- OAuth-accessible Ad Account list and configured Ad Account lookup.
- Campaign list/get, paused create, update, explicit status change, and
  archive.
- Ad Group list/get, paused create, update, explicit status change, and
  archive.
- Ad list/get, paused create from an existing Pin ID, update, explicit status
  change, and archive.
- Synchronous Ad Account Analytics with typed identity/date fields and raw JSON
  metric values.

Campaign, Ad Group, and Ad creation always sends `PAUSED`. Enabling delivery
requires a separate explicit status call. Money fields use `int64` micro-units
to avoid floating-point rounding. Targeting requires at least `LOCATION` or
`GEO`; create and update requests validate IDs, enums, schedules, URLs, and
bounded extension maps before network access.

Pinterest mutation endpoints use JSON arrays even for one item and may return
item-level `exceptions` inside HTTP 200 responses. The adapter inspects that
batch envelope and maps exceptions to typed `socialhub.Error` values. It
accepts both the documented Campaign exception array and the Ads schema's
single exception object.

Ads reference existing Pin IDs. Use the organic `pinterest/v5` adapter to
create image or video Pins before creating an Ad. The initial Ads adapter does
not implement Boards/Pins, catalogs, product groups, audiences, targeting
discovery, conversion tags, lead forms, bulk jobs, asynchronous analytics, or
ads webhooks. Those surfaces need separately reviewed typed workflows.

## Versions and quotas

The adapter is pinned to Pinterest API description tag `v5.28.0`. List
operations use Pinterest `bookmark` pagination and cap `page_size` at 250.
Synchronous Analytics accepts ranges up to 90 days; `HOUR` granularity is
limited to 8 days.

Pinterest documents a Trial limit of 1,000 requests per day. Standard access
has a universal limit of 100 requests per second per user per app, plus Ads
category limits of 1,000 reads, 400 writes, and 300 Analytics requests per
minute. Account- and app-specific responses remain authoritative. Pinterest
reports quota state in `x-ratelimit-limit`, `x-ratelimit-remaining`, and
`x-ratelimit-reset`; the adapter uses the reset value as a retry-delay fallback
and maps HTTP 429 or throttling messages to the shared retryable error contract.

## Contract sources

The official [Pinterest API v5 documentation](https://developers.pinterest.com/docs/api/v5/),
[OAuth guide](https://developers.pinterest.com/docs/getting-started/authentication/),
and tagged [OpenAPI description](https://github.com/pinterest/api-description/tree/v5.28.0)
are the source of truth.

Pinterest publishes a generated Python SDK, but there is no comparably current
official or mature community Go SDK for this surface. This package therefore
uses social-hub's bounded shared transport and the official v5.28 wire contract
without adding a Pinterest Ads-specific third-party dependency.
