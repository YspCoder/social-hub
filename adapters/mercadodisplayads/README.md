# Mercado Libre Display Ads adapter

Adapter registration name: `mercadolibre/display-ads-api-v1`.

This package implements the currently documented, read-only Mercado Libre
Display Ads surface. Every API request uses `Api-Version: 1`.

## Implemented contract

| Workflow | SDK operation | Official resource |
|---|---|---|
| Advertiser discovery | `ListAdvertisers` | `GET /advertising/advertisers?product_id=DISPLAY` |
| Campaign search | `ListCampaigns` | `GET /advertising/advertisers/{advertiser_id}/display/campaigns` |
| Campaign metrics | `CampaignMetrics` | `GET /advertising/advertisers/{advertiser_id}/display/campaigns/{campaign_id}/metrics` |
| Line Item search | `ListLineItems` | The Campaign `line_items` resource |
| Line Item metrics | `LineItemMetrics` | The Display metrics resource with `dimension=line_items` |
| Creative search | `ListCreatives` | The Line Item `creatives` resource |
| Creative metrics | `CreativeMetrics` | The Display metrics resource with `dimension=creatives` |

Campaign, Line Item, and Creative results are validated against their requested
parent IDs after decoding the documented `results` response envelope. Metric
groups are similarly bound to the requested Campaign, Line Item, or explicit
`ids` filter. Provider numeric response values use
`ExactValue`; amounts, ratios, counters, and attribution values are never
coerced through `float64`.

The current public page documents reads and metrics only. This adapter does not
infer Campaign, Line Item, Creative, targeting, budget, or asset mutations from
the Mercado Ads UI. Product Ads and Brand Ads use separate resources and
versioned contracts and are intentionally separate adapters.

## Configuration

An externally managed access token is the simplest production configuration:

```yaml
version: 1
platforms:
  - adapter: mercadolibre/display-ads-api-v1
    product: display-ads-api
    accounts:
      - id: mla-display-ads
        access_token_ref: env:MELI_ACCESS_TOKEN
        approval:
          account_type: commercial-advisor-enabled
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
  - adapter: mercadolibre/display-ads-api-v1
    product: display-ads-api
    accounts:
      - id: mlm-display-ads
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

## OAuth lifecycle

`Adapter.OAuth` exposes the server-side Authorization Code flow. The SDK
requires `state` even though Mercado Libre describes it as a recommendation.
PKCE is optional unless it is enabled for the application; `S256` should be
used when enabled. The callback `redirect_uri` must exactly match the static
URI registered on the application.

Mercado Libre refresh tokens are single-use. Every refresh response supplies a
replacement and only the latest token remains valid. Managed rotation therefore
requires an encrypted `socialhub.TokenStore`; a failed store write remains
pending and is retried before the cached access token is returned again. Use one
coordinated refresh writer per user grant because a basic shared `TokenStore`
does not itself provide a distributed lock.

The official authentication page is internally inconsistent: its JSON example
returns `expires_in: 10800`, while adjacent prose says the access token lasts
six hours. The adapter does not hard-code either value and computes expiry from
the response's positive `expires_in` field.

## Operational boundaries

- Display Ads access must be enabled through a Mercado Libre Commercial
  Advisor. The documented `404 No permissions found for user_id` response is
  mapped to `CodeApprovalRequired` with the official documentation URL.
- Metrics data starts on `2022-09-01`. The SDK requires `YYYY-MM-DD`, an ordered
  range, and enforces the documented maximum inclusive 90-day window.
- Rate controls are dynamic and primarily applied per Client ID and endpoint;
  Mercado Libre publishes no fixed Display Ads RPM. HTTP `429` is retryable and
  `Retry-After` is preserved when present.
- Each `ids` filter is limited to 100 unique positive IDs to keep request URLs
  bounded. The public page does not publish a smaller provider-side maximum.
- The Creative sort documentation enumerates only `id` and `name`. An official
  request example uses `start_date`, but the adapter follows the documented
  enum and rejects that example-only value.
- The Campaign metrics section repeats Campaign resource sort fields even
  though its request example omits them and its response contains daily metric
  rows. The adapter does not send those unrelated sort parameters.
- The Line Item example misspells `campaign_id` as `campaing_id`. Decoding
  accepts either spelling; if both occur with different values, the response is
  rejected as a platform contract error.
- The Campaign metrics example requests April 2024 but shows a February 2024
  row. The adapter binds returned daily rows to the requested date range rather
  than accepting that inconsistent example data.
- Creative status values are inconsistent across the current page, so status
  remains an open string rather than a closed enum.

## Official references

- Display Ads: <https://developers.mercadolibre.com.ar/en_us/display-ads>
- Authentication and Authorization: <https://developers.mercadolibre.com.ar/en_us/authentication-and-authorization>
- Authorization and Token Recommendations: <https://developers.mercadolibre.com.ar/en_us/authorization-and-token-recommendations>
- Rate limit / 429 Error: <https://developers.mercadolibre.com.ar/en_us/rate-limit-429-error>

Contract reviewed on 2026-08-25. The Display Ads page reports a last update of
2025-03-07.
