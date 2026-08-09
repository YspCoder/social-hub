# Google Ads API v25 adapter

Package `social-hub/adapters/googleads` implements customer-scoped paid-media
workflows for Google Ads API v25. Campaign Budgets, Search Campaigns, Ad Groups,
responsive search ads, and GAQL reports intentionally remain separate from
common social `Post`, `Fetcher`, and `MediaUploader` operations.

## Configuration

Import the package for registration and configure one social-hub account per
Google Ads customer:

```go
import _ "social-hub/adapters/googleads"
```

```yaml
adapter: googleads/api-v25
product: api
accounts:
  - id: brand-search
    client_id: 123.apps.googleusercontent.com
    secret_ref: secret://googleads/client-secret
    access_token_ref: secret://googleads/access-token
    settings:
      customer_id: "1234567890"
      login_customer_id: "0987654321"
      developer_token_ref: secret://googleads/developer-token
```

`customer_id` and optional manager `login_customer_id` must be ten digits
without hyphens. API requests resolve the OAuth access token and 22-character
developer token independently, then send `Authorization`, `developer-token`,
and, for manager access, `login-customer-id` headers. Redirects are rejected so
credentials cannot be forwarded to another origin.

## Access and OAuth

Google Ads API access requires an approved developer token and OAuth 2.0 user
authorization with `https://www.googleapis.com/auth/adwords`. Explorer and
Basic access levels have different production quotas, and test-account access
does not imply production access.

`Adapter.OAuth` implements Google's web-server authorization-code flow with
offline access. `OAuthClient.Exchange` returns the initial access and refresh
tokens; `OAuthClient.Refresh` preserves the existing refresh token when Google
returns only a new access token. Applications remain responsible for encrypted
token storage and proactive refresh before expiry.

## Supported workflows

- Current Customer lookup and OAuth-accessible Customer resource names.
- Campaign Budget list, create, update, and remove.
- Search Campaign list, paused create, update, explicit status change, and
  remove.
- Search Ad Group list, paused create, update, explicit status change, and
  remove.
- Responsive search ad list, paused create, content update, explicit status
  change, and remove.
- Bounded, paginated GAQL `Search` for custom read-only reporting.

Campaign, Ad Group, and Ad creates always send `status=PAUSED`. Campaign create
supports the initial `SEARCH` plus manual-CPC contract and requires the caller
to explicitly choose `CONTAINS_EU_POLITICAL_ADVERTISING` or
`DOES_NOT_CONTAIN_EU_POLITICAL_ADVERTISING`. RSA create enforces 3-15 headlines,
2-4 descriptions, URL validation, and Google text-length boundaries.

All returned resource names are checked against the configured Customer.
Extension fields cannot override Customer ownership, resource names, status,
or adapter-controlled workflow fields. Google REST `int64` values are encoded
as JSON strings, while update masks and GAQL retain proto `snake_case` field
paths.

The initial adapter does not implement Performance Max, Demand Gen, Shopping,
assets, audiences, Customer Match, BatchJob, `SearchStream`, or asynchronous
jobs. Those products require separately reviewed typed workflows rather than
unbounded generic mutation access.

## Versions and quotas

The adapter is pinned to API v25, released July 22, 2026 and scheduled to sunset
in August 2027. GAQL `Search` returns fixed pages of at most 10,000 rows. The
deprecated `pageSize` request field is deliberately absent because v25 returns
`PAGE_SIZE_NOT_SUPPORTED` when it is set.

Explorer access is documented at 2,880 production operations per day and
15,000 test-account operations per day; Basic access is 15,000 operations per
day. A mutate request can contain at most 10,000 operations. `Search` and
`SearchStream` each count as one operation, while valid pagination requests do
not add daily operations. Failed `GoogleAdsFailure` requests still consume
quota. Account-specific dashboard limits and response errors remain
authoritative.

Google documents a 64 MB response ceiling. The shared social-hub transport
keeps its stricter 8 MiB bounded-response limit to protect SDK callers; split
large GAQL reports by date or entity when needed.

## Contract sources

The official [Google Ads API v25 reference](https://developers.google.com/google-ads/api/reference/rpc/v25/overview),
[REST authentication](https://developers.google.com/google-ads/api/rest/auth),
[mutate](https://developers.google.com/google-ads/api/rest/common/mutate),
[Search](https://developers.google.com/google-ads/api/rest/common/search),
[quota guidance](https://developers.google.com/google-ads/api/docs/best-practices/quotas),
and [responsive search ad guide](https://developers.google.com/google-ads/api/docs/responsive-search-ads/create-responsive-search-ads)
are the source of truth.

The `kritzware/google-ads-go`, `datrics-ltd/gads-cli`, and `Limetric/goads`
repositories were audited only for implementation patterns. None currently
provides a mature v25 Go client, so this package adds no third-party Google Ads
dependency and derives its wire contract from the official v25 REST discovery
document.
