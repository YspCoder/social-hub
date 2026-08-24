# Taboola Backstage API v1.0 adapter

Package `social-hub/adapters/taboola` implements advertiser-account-scoped
Taboola Backstage API v1.0 workflows. Campaigns, Campaign Items, and paid-media
reports intentionally remain separate from common social `Post`, `Fetcher`,
and `MediaUploader` operations.

## Configuration

Import the package for registration and configure one social-hub account per
Taboola advertiser account:

```go
import _ "social-hub/adapters/taboola"
```

```yaml
adapter: taboola/backstage-api-v1
product: backstage-api
accounts:
  - id: native-us
    client_id: taboola-client-id
    secret_ref: secret://taboola/client-secret
    settings:
      advertiser_account_id: demo-advertiser
```

The preferred configuration uses OAuth 2.0 Client Credentials. A caller that
manages token issuance externally can instead set `access_token_ref`; each
account must configure exactly one credential mode. `base_url` and `token_url`
overrides are reserved for tests and controlled gateways. Redirects are
rejected so bearer tokens and client credentials cannot be forwarded to
another origin.

## Access and OAuth

Backstage API credentials are issued through Taboola and normally require an
account manager or support request. Credentials must be authorized for the
configured advertiser account; ordinary Backstage UI access does not by itself
establish API access.

`OAuthClient.ClientCredentials` sends `client_id`, `client_secret`, and
`grant_type=client_credentials` to Taboola's token endpoint. Tokens commonly
expire after 43,200 seconds, and Taboola does not return a refresh token for
this flow. The adapter caches a still-valid token in memory and optionally in
the configured `socialhub.TokenStore`, with one concurrent refresh per client.

`ValidateConfiguredAccount` checks the configured account against
`users/current/allowed-accounts` and requires both `ADVERTISER` partner access
and `PAID` Campaign access.

## Supported workflows

- Current and allowed Account discovery, plus configured advertiser validation.
- Campaign list/get, paused create, update, and explicit activation changes.
- Campaign Item list/get, URL-based create, update, and explicit activation
  changes.
- Campaign Summary reports and Realtime Campaign Summary reports.

Campaign creation always sends `is_active=false` and rejects an unexpectedly
active response. Campaign Item creation is allowed only while the parent
Campaign is inactive, sends only the destination `url`, and requires the new
Item to enter `CRAWLING`. A crawling Item is read-only.

Enabling a Campaign requires at least one Item. Every Item must be explicitly
inactive and outside `CRAWLING` and `PENDING_APPROVAL`; the caller must then
make a separate activation request. Returned Campaign advertiser ownership and
Item Campaign ownership are checked before values are accepted.

The initial adapter does not implement audience targeting, conversion rules,
pixel management, publisher exclusions, bulk operations, motion ads, video
upload, or organic social capabilities. Those surfaces require separately
reviewed typed workflows rather than generic mutation access.

## Limits and errors

Taboola applies account- and endpoint-specific quotas. The Realtime Campaign
Summary endpoint is documented at 10 requests per minute. Runtime HTTP 429
responses remain authoritative and map to retryable `socialhub.ErrRateLimited`
errors; `Retry-After` is preserved when present. Standard and realtime report
methods expose only documented filters and do not invent client-side page
parameters.

Backstage errors may be JSON, XML, or HTML depending on the endpoint and
failure. The adapter maps them to `socialhub.Error`, preserves Taboola message
codes and request IDs when available, bounds response bodies, and redacts
credential-shaped values before returning platform text.

## Contract sources

The official [Backstage API documentation](https://developers.taboola.com/backstage-api/),
[Client Credentials flow](https://developers.taboola.com/backstage-api/reference/client-credentials-flow),
[allowed Accounts](https://developers.taboola.com/backstage-api/reference/get-allowed-accounts),
[Campaign creation](https://developers.taboola.com/backstage-api/reference/create-a-campaign),
[Campaign Item creation](https://developers.taboola.com/backstage-api/reference/create-a-campaign-item),
[Campaign Summary report](https://developers.taboola.com/backstage-api/reference/campaign-summary-report),
and [Realtime Campaign report](https://developers.taboola.com/backstage-api/reference/realtime-campaign-summary-report)
are the source of truth.

Taboola's official
[`backstage-api-java-client`](https://github.com/taboola/backstage-api-java-client)
and [`Backstage-API`](https://github.com/taboola/Backstage-API) repositories
were reviewed for contract and workflow context. Neither is a Go SDK, so this
adapter uses social-hub's bounded shared transport without adding a
Taboola-specific third-party dependency.
