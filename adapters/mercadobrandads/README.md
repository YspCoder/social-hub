# Mercado Libre Brand Ads adapter

Adapter registration name: `mercadolibre/brand-ads-api`.

This package implements the currently documented, read-only Mercado Libre
Brand Ads surface. Brand Ads resources are not assigned a public version on
the current page. Only shared advertiser discovery sends the documented
`Api-Version: 1` header.

## Implemented contract

| Workflow | SDK operation | Official resource |
|---|---|---|
| Advertiser discovery | `ListAdvertisers` | `GET /advertising/advertisers?product_id=BADS` |
| Campaign search | `ListCampaigns` | `GET /advertising/advertisers/{advertiser_id}/brand_ads/campaigns` |
| Campaign detail | `GetCampaign` | The advertiser-scoped Campaign resource |
| Campaign Items | `ListItems` | The Campaign `items` resource |
| Campaign Keywords | `ListKeywords` | The Campaign `keywords` resource |
| Advertiser Campaign metrics | `ListAdvertiserCampaignMetrics` | The advertiser Campaign `metrics` resource |
| Campaign metrics | `GetCampaignMetrics` | The individual Campaign `metrics` resource |
| Keyword metrics | `GetKeywordMetrics` | The Campaign `keywords/metrics` resource |

Campaign, Item, and Keyword results are validated against their configured or
requested parent IDs. Metrics preserve the provider's daily and summary
shapes, including the Keyword endpoint's date-grouped rows and summary array.
Amounts, ratios, counters, CPC values, and attribution values use `ExactValue`
and are never coerced through `float64`. Successful provider values are
validated as JSON numbers rather than arbitrary JSON scalars.

The current public page documents reads and metrics only. It describes how
automatic and custom Campaigns behave in the Mercado Ads product, but publishes
no create, update, pause, keyword mutation, budget mutation, or Item assignment
endpoint. The adapter does not infer those writes from the UI. Product Ads and
Display Ads remain independent, versioned adapters.

## Configuration

An externally managed access token is the simplest production configuration:

```yaml
version: 1
platforms:
  - adapter: mercadolibre/brand-ads-api
    product: brand-ads-api
    accounts:
      - id: mla-brand-ads
        access_token_ref: env:MELI_ACCESS_TOKEN
        approval:
          account_type: brand-ads-enabled
          scopes: [read]
        settings:
          advertiser_id: 123456
```

Set `advertiser_id` to zero or omit it while discovering the advertisers
available to a token with `ListAdvertisers`. All other workflows remain
unavailable until the account is bound to a positive advertiser ID.

For SDK-managed rotation, configure the App ID, App Secret reference, initial
refresh-token reference, and an encrypted `socialhub.TokenStore`:

```yaml
version: 1
platforms:
  - adapter: mercadolibre/brand-ads-api
    product: brand-ads-api
    accounts:
      - id: mlm-brand-ads
        client_id: "123456789"
        secret_ref: env:MELI_CLIENT_SECRET
        settings:
          advertiser_id: 987654
          refresh_token_ref: env:MELI_REFRESH_TOKEN
    settings:
      auth_url: https://auth.mercadolibre.com.mx/authorization
```

`AuthURL` is country-specific. The default is Argentina; use the official
Mercado Libre authorization domain for the advertiser's country. API and token
requests are always sent to `https://api.mercadolibre.com`; custom origins are
rejected so bearer tokens and App Secrets cannot be redirected.

## Eligibility and Campaign semantics

The official page says Brand Ads is available to brands and sellers with an
Official Store or My Page, green reputation or higher, and at least three
listings. Marketplace availability is documented for MLA, MLB, MLM, MLC, MCO,
MLU, and MPE. VIS-Motors availability is documented for MLA, MLB, MLM, and MCO.
Product eligibility does not replace OAuth authorization or advertiser access.

Automatic Campaigns are managed by Mercado Ads from the inventory associated
with `official_store_id` and `destination_id`; their generated Keywords cannot
be edited or deleted. Custom Campaigns are user-managed and the product rules
require 3-10 Items and 1-200 Keywords. These are read-model facts only because
the public API page does not expose the corresponding writes.

## OAuth lifecycle

`Adapter.OAuth` exposes the server-side Authorization Code flow. The SDK
requires `state` even though Mercado Libre describes it as a recommendation.
PKCE is optional unless enabled for the application; `S256` should be used
when enabled. The callback `redirect_uri` must exactly match the URI registered
on the application.

Mercado Libre refresh tokens are single-use. Every refresh response supplies a
replacement and only the latest token remains valid. Managed rotation therefore
requires an encrypted `socialhub.TokenStore`; a failed store write remains
pending and is retried before the cached access token is returned again. Use one
coordinated refresh writer per user grant because a shared `TokenStore` does not
itself provide a distributed lock.

The official authentication page is internally inconsistent: its JSON example
returns `expires_in: 10800`, while adjacent prose says the access token lasts
six hours. The adapter computes expiry from the response's positive
`expires_in` field rather than hard-coding either duration.

## Operational boundaries

- The documented `404 No permissions found for user_id` response is mapped to
  `CodeApprovalRequired` with the official Brand Ads documentation URL.
- Brand Ads metrics are available from `2023-02-09`. The SDK validates
  `YYYY-MM-DD` and ordered ranges. The public page states no maximum range, so
  the adapter does not invent the 90-day rule used by other Mercado Ads APIs.
- Metrics support documented `limit`, `offset`, `aggregation_type`, and typed
  `fields`. Advertiser metrics additionally expose `status` and
  `destination_id` filters. A requested field must be present in the response.
- `aggregation_type=daily` requires the daily `metrics` part but permits an
  omitted `summary`; `aggregation_type=total` does the inverse. Omitting the
  parameter requires both documented response parts.
- Campaign search preserves the returned `paging` metadata. The current page
  does not document Campaign search query parameters, so the adapter does not
  synthesize `limit` or `offset` for that endpoint.
- `competitive` metrics are accepted only for an individual Campaign with
  `aggregation_type=total`. The official page says they appear in the summary,
  use the latest seven days, and currently do not support a selectable period.
- All three official metric examples request July 2024 but show January 2024
  rows. The adapter binds returned daily rows to the requested date range
  rather than accepting those inconsistent example dates.
- Rate controls are dynamic and primarily applied per Client ID and endpoint;
  Mercado Libre publishes no fixed Brand Ads RPM. HTTP `429` is retryable and
  `Retry-After` is preserved when present.
- The official `mercadolibre/golang-sdk` declares itself unmaintained since
  April 2021 and only offers generic HTTP calls. The current third-party Go SDK
  reviewed for reference has no Brand Ads service, so this adapter introduces
  no external SDK dependency.

## Official references

- Brand Ads: <https://developers.mercadolibre.com.ar/en_us/ads-bads>
- Authentication and Authorization: <https://developers.mercadolibre.com.ar/en_us/authentication-and-authorization>
- Authorization and Token Recommendations: <https://developers.mercadolibre.com.ar/en_us/authorization-and-token-recommendations>
- Rate limit / 429 Error: <https://developers.mercadolibre.com.ar/en_us/rate-limit-429-error>
- Deprecated official Go SDK: <https://github.com/mercadolibre/golang-sdk>

Contract reviewed on 2026-08-25. The Brand Ads page reports a last update of
2025-06-26.
