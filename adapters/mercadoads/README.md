# Mercado Libre Product Ads reporting adapter

Adapter registration name: `mercadolibre/product-ads-api-v2`.

This package implements the currently documented, read-only Mercado Ads
Product Ads surface. It uses `Api-Version: 1` for advertiser discovery and
`Api-Version: 2` for Product Ads Campaign, item, and metrics requests.

This is a reporting/analytics adapter, not a general ad-buying or campaign
execution adapter. The public Product Ads page describes campaign management
in the Mercado Ads product UI, but every published API example is a `GET`; it
does not publish create, update, budget, bid, activation, or assignment request
contracts. Do not count this package as a writable media-buying integration.

## Implemented contract

| Workflow | SDK operation | Official resource |
|---|---|---|
| Advertiser discovery | `ListAdvertisers` | `GET /advertising/advertisers?product_id=PADS` |
| Campaign search | `ListCampaigns` | `GET /advertising/advertisers/{advertiser_id}/product_ads/campaigns` |
| Campaign detail | `GetCampaign` | `GET /advertising/product_ads/campaigns/{campaign_id}` |
| Campaign daily metrics | `ListCampaignDailyMetrics`, `GetCampaignDailyMetrics` | The Campaign resources with `aggregation_type=DAILY` |
| Item search | `ListItems` | `GET /advertising/advertisers/{advertiser_id}/product_ads/items` |
| Item detail | `GetItem`, `GetItemMetrics` | `GET /advertising/product_ads/items/{item_id}` |
| Item daily metrics | `ListItemDailyMetrics`, `GetItemDailyMetrics` | The item resources with `aggregation_type=DAILY` |

Campaign and item searches support the documented pagination, metrics,
summaries, status, Campaign, catalog, logistics, store, brand, and boolean item
filters. The SDK limits each ID-list filter to 100 unique values so query URLs
remain bounded. Provider numeric response values use `ExactValue`; amounts and
ratios are never coerced through `float64`.

The Campaign-detail daily response example on the official page contains an
invalid outer JSON brace around an array, while the item-detail daily example
uses a `results` envelope. The decoder accepts only those two intended shapes:
a direct array or a `results` envelope. This remains a provider-documentation
risk until Mercado Libre publishes a valid Campaign-detail example or schema.

Display Ads and Brand Ads use different resources and contracts and are
intentionally separate from this package.

## Configuration

An externally managed access token is the simplest production configuration:

```yaml
version: 1
platforms:
  - adapter: mercadolibre/product-ads-api-v2
    product: product-ads-api
    accounts:
      - id: mla-product-ads
        access_token_ref: env://MELI_ACCESS_TOKEN
        approval:
          account_type: product-ads-enabled
          scopes: [read]
        settings:
          advertiser_id: 123456
```

Set `advertiser_id` to zero or omit it while discovering the advertisers
available to a token with `ListAdvertisers`. Campaign and item list operations
remain unavailable until the account is configured with a positive advertiser
ID. Direct Campaign and item detail resources are authorized by the token and
do not carry advertiser ID in their paths.

The `approval` block is an SDK-side record of an externally verified fact, not
a Mercado Libre-issued entitlement. Use exactly
`account_type: product-ads-enabled` with a `read` scope only after enablement
has been confirmed; otherwise omit it and capabilities report approval as
unknown. A successful `ListAdvertisers` call is the authoritative runtime
check.

For SDK-managed rotation, configure the App ID, App Secret reference, initial
refresh-token reference, and an encrypted `socialhub.TokenStore`:

```yaml
version: 1
platforms:
  - adapter: mercadolibre/product-ads-api-v2
    product: product-ads-api
    accounts:
      - id: mlm-product-ads
        client_id: "123456789"
        secret_ref: env://MELI_CLIENT_SECRET
        settings:
          advertiser_id: 987654
          refresh_token_ref: env://MELI_REFRESH_TOKEN
    settings:
      auth_url: https://auth.mercadolibre.com.mx/authorization
```

`auth_url` is country-specific. The default is Argentina; use the official
Mercado Libre authorization domain for the seller's country. The adapter
allowlists official country authorization hosts. API calls are fixed to
`https://api.mercadolibre.com`, and token exchange is fixed to
`https://api.mercadolibre.com/oauth/token`; custom base or token origins are
not accepted. A custom `RoundTripper` can provide controlled network routing
without changing credential destinations.

## OAuth lifecycle

`Adapter.OAuth` exposes the server-side Authorization Code flow. The SDK
requires `state` even though Mercado Libre describes it as a recommendation.
PKCE is optional unless it is enabled for the application; `S256` should be
used when enabled. The callback `redirect_uri` must exactly match the static
URI registered on the application. The caller must retain and compare `state`
when handling the callback. The SDK accepts HTTPS callbacks and permits HTTP
only for loopback development addresses; callback query parameters are
rejected in favor of `state`.

Mercado Libre refresh tokens are single-use. Every refresh response supplies a
replacement and only the latest token remains valid. A refresh token expires
after six months, and authorization can also be revoked by password or App
Secret changes, permission revocation, device unlinking, or fraud controls. Use
one coordinated refresh writer per user grant; a basic shared `TokenStore`
does not itself provide a distributed lock.

Managed refresh mode requires a non-nil `TokenStore`; production
implementations must persist and encrypt tokens. The client retains a newly
rotated token in memory and retries a failed store write before returning it
from a later call. Supply the store with `socialhub.WithTokenStore` when
initializing the adapter or constructing the account client.
`OAuthClient.Close` clears the App ID and App Secret and prevents new
authorization work.

The official authentication page is internally inconsistent: its JSON example
returns `expires_in: 10800`, while adjacent prose says the access token lasts
six hours. The adapter does not hard-code either value and always computes
expiry from the response's positive `expires_in` field.

## Operational boundaries

- Product Ads must be enabled for the seller. The documented `404 No permissions
  found for user_id` response is mapped to `CodeApprovalRequired`; the seller
  must enable advertising under Mercado Libre's profile advertising area.
- Authorization must be performed by the main seller account rather than a
  collaborator. Mercado Libre can also require the seller to complete account
  data or documents before granting or refreshing credentials.
- Every metrics query requires `date_from`, `date_to`, and at least one metric.
  The adapter enforces the documented maximum inclusive 90-day date window.
  Metrics validation data is updated at 10:00 AM GMT-3.
- Rate controls are dynamic and primarily applied per Client ID and endpoint;
  Mercado Libre publishes no fixed Product Ads RPM. HTTP `429` is retryable and
  `Retry-After` is preserved when present. Callers should use exponential
  backoff with jitter, bounded concurrency, and avoid retry spikes.
- `cost_usd` is documented for Campaign search but not Campaign detail or item
  metrics. Impression-share and ACOS benchmark metrics are limited to Campaign
  detail. SDK validation preserves these endpoint-specific differences.
- The adapter does not retry reads internally, choose a sort column whose
  allowed values are not enumerated by the current page, or emulate Display
  Ads, Brand Ads, marketplace messaging, notifications, or organic publishing.
- Successful API and OAuth responses must use a JSON media type and are read
  through bounded bodies (8 MiB for API reads and 1 MiB for OAuth). Redirects
  are not followed and caller Cookie Jars are not used, so bearer credentials
  and OAuth form data cannot be forwarded to another origin through those
  mechanisms.
- `Client.Close` is idempotent, releases the client's transport/token-source
  reference, and rejects new operations. It does not close the caller-owned
  shared HTTP transport. Operations already in flight may complete.

## Integration recommendation

Do not prioritize this adapter when the milestone requires actual campaign
creation, budget control, activation, bidding, or creative delivery. It is a
reasonable optional LATAM marketplace reporting adapter once a real
Product-Ads-enabled seller account is available for contract verification.
Until then, the absence of public write contracts and the malformed Campaign
daily example are explicit residual risks, not behaviors the SDK should guess.

## Official references

- Product Ads: <https://developers.mercadolibre.com.ar/en_us/product-ads-us-read>
- Authentication and Authorization: <https://developers.mercadolibre.com.ar/en_us/authentication-and-authorization>
- Authorization and Token Recommendations: <https://developers.mercadolibre.com.ar/en_us/authorization-and-token-recommendations>
- Rate limit / 429 Error: <https://developers.mercadolibre.com.ar/en_us/rate-limit-429-error>
- Mercado Ads introduction: <https://developers.mercadolibre.com.ar/en_us/mercado-ads-introduction>

Contract reviewed on 2026-08-25. The Product Ads page reports a last update of
2025-12-30; the rate-limit FAQ reports a last update of 2026-05-05.
