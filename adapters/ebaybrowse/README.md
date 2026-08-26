# eBay Buy Browse adapter

Registration name: `ebay/buy-browse-api-v1`

This adapter implements a bounded read-only surface of eBay Buy Browse API
v1.20.4: item search, item detail, legacy-ID bridging, item-group expansion,
and eBay Partner Network (EPN) attribution context. Marketplace commerce
resources do not implement the organic `Publisher`, `Fetcher`, `Reactor`, or
`Messenger` interfaces.

## Implemented workflows

| social-hub method | eBay method | Notes |
|---|---|---|
| `Browse().SearchItems` | `GET /item_summary/search` | keyword, GTIN/ePID, one current category ID, bounded raw filter/aspect/compatibility expressions, typed field groups and sort, limit 1-200, aligned offset 0-9,999 |
| `Browse().GetItem` | `GET /item/{item_id}` | REST item ID, typed PRODUCT/COMPACT/seller/charity field groups, optional shipping-estimate quantity |
| `Browse().GetItemByLegacyID` | `GET /item/get_item_by_legacy_id` | bridges one legacy item ID and an optional variation ID or seller SKU to a REST item |
| `Browse().GetItemsByGroup` | `GET /item/get_items_by_item_group` | expands item variations and preserves shared descriptions |

The initial surface excludes image search, compatibility checks, the
limited-release bulk `getItems` method, checkout/order flows, Feed API bulk
downloads, Marketing API recommendations, seller inventory, bidding, and user
OAuth. Those products have different grants and data contracts and should be
added as separate adapters or workflows.

## Configuration

For managed OAuth, `client_id` is the eBay App ID and `secret_ref` resolves to
the Cert ID. The adapter uses HTTP Basic authentication to mint an Application
access token with `https://api.ebay.com/oauth/api_scope`, caches it in the
configured `socialhub.TokenStore`, and obtains a new token before expiry. This
grant does not return a refresh token.

```yaml
version: 1
platforms:
  - adapter: ebay/buy-browse-api-v1
    product: buy-browse-api
    accounts:
      - id: main-epn
        client_id: ebay-app-id
        secret_ref: env://EBAY_CERT_ID
        approval:
          account_type: ebay-production-epn
          scopes:
            - https://api.ebay.com/oauth/api_scope
        settings:
          marketplace_id: EBAY_US
          affiliate_campaign_id: "1234567890"
          accept_language: en-US
```

An externally managed Application token can be supplied instead:

```yaml
      - id: external-token
        access_token_ref: env://EBAY_APPLICATION_ACCESS_TOKEN
        settings:
          marketplace_id: EBAY_GB
          accept_language: en-GB
```

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/ebaybrowse"
```

For Sandbox, override both endpoints together:

```yaml
    settings:
      base_url: https://api.sandbox.ebay.com/buy/browse/v1
      token_url: https://api.sandbox.ebay.com/identity/v1/oauth2/token
```

## Request and data guarantees

- Each call sends `X-EBAY-C-MARKETPLACE-ID`, defaulting to `EBAY_US`, and can
  override the marketplace and `Accept-Language` through `RequestContext`.
- If an EPN campaign is configured, the adapter constructs
  `X-EBAY-C-ENDUSERCTX` from the campaign, optional per-call affiliate
  reference, and optional delivery country/postal code. Contextual location is
  URL-encoded in the provider-required form, for example
  `contextualLocation=country%3DUS%2Czip%3D19406`.
- EPN commission attribution requires forwarding the buyer with the returned
  `itemAffiliateWebUrl`; `itemWebUrl` is not an attributed substitute.
- Keyword `q` is limited to 100 characters, rejects `*`, and cannot be mixed
  with GTIN/ePID. The effective page limit defaults to 50; offset must be zero
  or a multiple of the limit. eBay caps a search result set at 10,000 items.
- eBay filter grammars remain bounded strings because their valid fields and
  values are marketplace/category dependent. Callers own semantic correctness;
  the adapter owns URL encoding and request-size-safe validation. In particular,
  `distance` sorting still requires the provider-defined pickup filters.
- Prices and converted prices remain exact strings. Item summaries, items,
  search pages, and item groups retain the complete successful provider object
  in `Raw`; response warnings remain typed and do not fail an otherwise
  successful operation.
- Redirects are rejected so bearer tokens and EPN headers cannot move to a
  different origin. Successful responses must be bounded JSON. Provider errors
  map to `socialhub.Error`, with `errorId/domain/category` retained together in
  `PlatformCode`; error `11000`, category `SYSTEM`, HTTP 429, and 5xx failures
  are retryable as appropriate.

## Access, quota, and compliance

Production use requires an eBay Developers Program account, accepted API
License, production keyset, and the Buy API production access applicable to the
application. EPN enrollment and a valid campaign ID are separately required
for commission attribution; configuring a campaign ID does not prove that the
account is commercially approved.

eBay assigns call limits by application and API and exposes current allocation
through the developer account/API usage surfaces. This adapter does not
hard-code one global daily number. Rate-limit per application, product, and
marketplace; honor HTTP 429 and `Retry-After`, and monitor the account's current
allocation. Search pagination must stop before the 10,000-result boundary.

Affiliate reference IDs and delivery postal codes can be linkable end-user
data. Callers remain responsible for consent, minimization, retention, regional
disclosure requirements, and avoiding direct personal data in affiliate
reference values.

Official contracts reviewed on 2026-08-26:

- <https://developer.ebay.com/api-docs/buy/browse/overview.html>
- <https://developer.ebay.com/api-docs/buy/browse/resources/item_summary/methods/search>
- <https://developer.ebay.com/api-docs/buy/browse/resources/item/methods/getItem>
- <https://developer.ebay.com/api-docs/buy/browse/resources/item/methods/getItemByLegacyId>
- <https://developer.ebay.com/api-docs/buy/browse/resources/item/methods/getItemsByItemGroup>
- <https://developer.ebay.com/api-docs/static/oauth-client-credentials-grant.html>

Reference implementation reviewed for the official v1.20.4 OpenAPI schema,
endpoint routing, and OAuth Client Credentials behavior: `hendt/ebay-api`
v10.0.1 at commit `e20388bcf49cb7a46b5bf5ba8006b8d7d29ec3c8`
(MIT). Its `specs/buy_browse_v1_oas3.json` asset has SHA-256
`6ce90cbf7facc62b34c25a7eb5e5a164bbe18c3ba99e100dcbc33b7c7cf94414`.
It is not added as a runtime dependency.
