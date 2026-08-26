# Tradedoubler Products API 1.0 adapter

Registration name: `tradedoubler/products-api-v1.0`

This package implements the current public publisher Products API contract. It
is deliberately a commerce-feed adapter, not an organic social or advertiser
management client.

## Implemented workflows

| social-hub method | Tradedoubler operation | Notes |
|---|---|---|
| `Tradedoubler().SearchProducts` | `GET /1.0/products.json;fid=...` | typed products, offers, tracked product URLs, filters, ordering, and bounded pagination |
| `Tradedoubler().ListProductFeeds` | `GET /1.0/productFeeds.json` | feed metadata and the official embedded program summaries; optional `programId` filtering |
| `Tradedoubler().GetUnlimitedFeedLastUpdated` | `GET /1.0/productsUnlimited/lastUpdated.json;fid=...` | freshness preflight only; it does not consume an unlimited-file download |

The former `/1.0/programs.json` route is not implemented because the current
gateway reports no active route for it. Program information is exposed only as
the documented `productFeeds[].programs[]` summary. No current public
publisher contract was found for transaction reads, so this package does not
invent one. Arbitrary-URL deeplink generation is also excluded: the supported
commission link is `offers[].productUrl` returned by an authenticated product
search.

## Access and authentication

A publisher needs a Tradedoubler account, at least one registered site, and a
connection to each advertiser whose products will be read. In the Publisher
UI, create or retrieve the token under `Account > Manage tokens`; its system
must be `PRODUCTS`.

Products API authentication is not OAuth. The credential is a static 40-character
hexadecimal SHA-1 token sent as the regular `token` query parameter. There is
no refresh flow; rotate the UI token and replace the referenced secret when
needed. Matrix parameters remain in the path and the token is never placed
there.

```yaml
version: 1
platforms:
  - adapter: tradedoubler/products-api-v1.0
    product: products-api
    accounts:
      - id: primary-publisher
        access_token_ref: env://TRADEDOUBLER_PRODUCTS_TOKEN
        approval:
          account_type: connected-publisher
```

`client_id`, `secret_ref`, `app_id`, `token_store`, webhook fields, OAuth
scopes, and account-specific settings are rejected because they are not part
of this contract. Redirect following is disabled so the query credential
cannot be forwarded to another origin. A `base_url` override is available only
for a controlled contract-verification gateway.

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/tradedoubler"
```

```go
base, err := adapter.Client(ctx, "primary-publisher")
if err != nil {
	return err
}
client := base.(*tradedoubler.Client)

result, err := client.Tradedoubler().SearchProducts(ctx, tradedoubler.SearchProductsRequest{
	FeedIDs:         []int64{19750},
	Keyword:         "running shoes",
	PageSize:        50,
	Limit:           50,
	DateOutputFormat: tradedoubler.DateOutputISO8601,
})
```

Provider IDs, counts, prices, and variable-format dates use `ExactValue`.
Custom product fields use `RawValue`. Products, offers, feeds, and all three
response envelopes retain the complete successful provider object in `Raw`,
so new fields remain available before the typed model is updated.

All three operations document HTTP `200` as their success status. The adapter
also requires a JSON media type, present top-level collections, valid and
unique feed/program/offer identities, offers belonging to a requested feed,
and a last-updated response containing the requested feed plus one of the
documented ISO-8601 timestamp forms. On a response-side error, the returned
envelope retains response metadata and a bounded, publisher-token-redacted
diagnostic `Raw`; structured HTTP failures expose the same diagnostic body on
`APIError.Raw`.

## Limits and failure handling

- Search responses are capped by the provider at 1,000 products. When page
  parameters are supplied, the SDK requires a global `limit` large enough for
  the requested page and never accepts a value above 1,000.
- Tradedoubler does not publish a standard per-second Search or Product Feed
  quota in the current publisher documentation. The adapter passes through
  rate-limit headers if the gateway supplies them but does not invent a local
  quota.
- Unlimited product files may be downloaded at most three times per file
  version in 24 hours. A new version resets the allowance, and the first
  request has a five-day grace period. This package intentionally implements
  only the recommended last-updated preflight, not the potentially
  multi-million-product download.
- Documented token codes `1`, `2`, and `PF_300`, plus the current gateway's
  observed `4000` and `4001`, map to `socialhub.CodeUnauthenticated`.
  `PF_392` maps to `CodePermissionDenied`; documented service code `5` and
  `PF_250` are retryable temporary-unavailability errors; HTTP 429 is retryable
  `CodeRateLimited`; and the remaining documented Products API request codes
  map to `CodeInvalidArgument`.
- Successful JSON bodies are bounded by the shared 8 MiB transport limit.
  Redirects, JSONP, XML, and caller-supplied request-ID, idempotency, or field
  selection options are not supported.

## Official sources verified 2026-08-26

- [Products API for publishers](https://dev.tradedoubler.com/products/publisher/)
- [Link Converter for publishers](https://dev.tradedoubler.com/link-converter/publisher/)

The review used immutable local captures with these SHA-256 hashes:

| Capture | SHA-256 |
| --- | --- |
| Products API for publishers | `3F9D8A714C8F3FA0161BE94E64725AD4C9709BA49B23A5249B5AEDBC2F35F708` |
| Link Converter for publishers | `6331EC5AB923F77A283615D4EFD4A11BA4C10E5E0828D5B2B06E6503D886C693` |
