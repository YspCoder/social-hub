# Rakuten Advertising Affiliate APIs adapter

This package implements a bounded, publisher-facing subset of Rakuten
Advertising's current **Affiliate APIs** catalogue. Its registration name is:

```text
rakuten-advertising/affiliate-apis-v1.0.0
```

The name follows the official OpenAPI title (`Affiliate APIs`) and version
(`1.0.0`). The endpoint families keep their own versions.

## Implemented workflows

| Method | Official endpoint | Contract | Capability |
| --- | --- | --- | --- |
| `SearchAdvertisers` | `GET /v2/advertisers` | JSON; page size up to 200 | advertiser discovery |
| `ListPartnerships` | `GET /v1/partnerships` | JSON; status, network, date-range and pagination filters | partnership discovery |
| `SearchProducts` | `GET /productsearch/1.0` | XML; page size up to 100; repeated sort fields | partner product search |
| `CreateDeepLink` | `POST /v1/links/deep_links` | one JSON deep link per request | tracking link |
| `ListTransactions` | `GET /events/1.0/transactions` | JSON array; recent process/transaction date windows | transaction events |

Successful responses expose typed fields plus bounded `Raw` provider data.
JSON identifiers and monetary values whose wire type varies use `ExactValue`.
Product Search retains its XML response bytes. Failed provider responses retain
bounded, credential-redacted diagnostic data and a bounded request ID when one
is available.

## Authentication and account access

All implemented workflows use `Authorization: Bearer {access-token}`. A
publisher must have:

- a Rakuten Advertising publisher account;
- an Application in the Developer Portal and subscriptions to the APIs/plans
  it will call;
- an API access token scoped to the publisher account ID.

Developer Portal access tokens expire after 3,600 seconds. This adapter accepts
either an externally managed token through `access_token_ref`, or managed
refresh credentials through `client_id`, `secret_ref`, and
`account.settings.refresh_token_ref`. Managed refresh sends the official form
fields `refresh_token` and `scope` to `POST /token`, authenticated with the
Bearer token-key `base64(client_id:client_secret)`. A supplied
`socialhub.TokenStore` caches complete rotating token bundles. Rakuten can
return a replacement refresh token, so the adapter updates its in-memory token
before persisting it. If a store write fails, the call returns retryable
`token_cache_put`; later calls retry that write without reloading a stale
refresh token. A production store must encrypt both access and refresh tokens
at rest.

Initial password-style token issuance is deliberately not implemented: the
current guide's prose and command disagree about `grant_type`. Generate the
initial access/refresh token in the Developer Portal or with the documented
Token API flow, then configure one of the modes above.

Static-token configuration:

```yaml
adapter: rakuten-advertising/affiliate-apis-v1.0.0
product: affiliate-apis
accounts:
  - id: publisher-us
    access_token_ref: env://RAKUTEN_ACCESS_TOKEN
    settings:
      publisher_id: "1234567"
```

Managed-refresh configuration:

```yaml
adapter: rakuten-advertising/affiliate-apis-v1.0.0
product: affiliate-apis
accounts:
  - id: publisher-us
    client_id: your-developer-portal-client-id
    secret_ref: env://RAKUTEN_CLIENT_SECRET
    token_store: encrypted-tokens
    settings:
      publisher_id: "1234567"
      refresh_token_ref: env://RAKUTEN_REFRESH_TOKEN
```

## Commercial and data constraints

- Advertiser discovery can include advertisers to which the publisher has not
  yet been accepted.
- Product Search is limited to product feeds from partner advertisers.
- Deep-link creation requires an approved advertiser partnership, and that
  advertiser must enable deep linking. Provider errors such as
  `ACCESS_DENIED` and `DEEP_LINKING_NOT_ENABLED` map to
  `approval_required`.
- Events data is directional and near-real-time. Rakuten explicitly says it
  stores roughly one to two weeks and is not the source for calculating final
  commission; use reconciliation/reporting products for finalized historical
  data.
- The Events guide requires paired dates, limits each range to 30 days, limits
  process-date history to 30 days and transaction-date history to 100 days.

## Rate limits

The current official guide for each implemented endpoint states **100 calls per
minute**. The Token API is also limited to **100 calls per minute**. The OpenAPI
contract documents a `403` response when the per-minute request allowance is
exceeded; generic `403` responses therefore map to retryable `rate_limited`,
while documented deep-link business codes retain permission/approval semantics.
`Retry-After` is preserved when present and capped at 24 hours.

## Response and replay safety

The five implemented API endpoints and `POST /token` document HTTP `200` as
their only success status. The adapter rejects every other status, including
unexpected 2xx responses, as a provider contract violation. It also verifies
that list collections are present, provider IDs are valid and unique, Product
Search pagination is non-negative, a deep link belongs to the requested
advertiser and contains an HTTP(S) URL, and every Events transaction belongs to
the configured publisher ID.

Deep-link creation has no documented idempotency key. Transport failures,
HTTP `408`/5xx responses, unexpected 2xx responses, and success-response decode
or contract failures return `conflict` / `user_action` and match
`errors.Is(err, rakutenadvertising.ErrOutcomeUnknown)`. Reconcile publisher
state before retrying. Explicit provider rejections, including an error object
inside HTTP `200`, keep their provider classification and are not converted to
an unknown outcome.

## Official sources verified 2026-08-26

- [Affiliate APIs OpenAPI catalogue](https://developers.rakutenadvertising.com/documentation/en-US/affiliate_apis)
- [Access Tokens guide](https://developers.rakutenadvertising.com/guides/access_tokens)
- [Advertisers v2 guide](https://developers.rakutenadvertising.com/guides/advertisers)
- [Partnerships guide](https://developers.rakutenadvertising.com/guides/partnerships)
- [Product Search guide](https://developers.rakutenadvertising.com/guides/product_search)
- [Deep Links guide](https://developers.rakutenadvertising.com/guides/deep_link)
- [Events guide](https://developers.rakutenadvertising.com/guides/events)

The review used immutable local captures with these SHA-256 hashes:

| Capture | SHA-256 |
| --- | --- |
| Affiliate APIs catalogue | `EEC2CDF4A2B32D6BA617D7DFF530BA389DFABF7E3F81EF25343D1D7103B7C624` |
| Access Tokens guide | `6A29090BD7E92E7085695C79569DBD8AD8DE18AD7EE069C946226982F8943975` |
| Advertisers v2 guide | `5440022783270B878609CC1401D3548547084E7BA3B061CB3DFAC80DA532F09D` |
| Partnerships guide | `6D2E93FAC99DEF5C2B293680BB95BED9BDA00CB44507D9032EF26FCE77B62996` |
| Product Search guide | `305AAB22511507EAD28D0AA0175D73B77D9A5875332CD6A01115F4F597DBEE69` |
| Deep Links guide | `DD86476C4B9E815D6CCC833EA2BAD4D0462102EA77654507946B9B6A4CE3DF31` |
| Events guide | `947877AA2B0F6C5A6672B4549D402C9C61BFF3FB9C760AE94EAFF107DC65FAC3` |

The Product Search guide currently conflicts with its own reference/OpenAPI on
the overall result cap (5,000 versus 1,000). The adapter enforces only the
unambiguous per-page maximum of 100 and exposes provider pagination unchanged.
The Events OpenAPI schema describes one object while the current guide shows an
array; the adapter accepts both official response shapes. It also accepts the
guide/OpenAPI aliases `offer`/`offer_id` and
`commission_list_id`/`commissions_list_id`. Partnerships examples alternate
between `metadata`/`links` and `_metadata`/`_links`, so both forms are decoded.

A public GitHub repository search on the verification date found no mature Go
SDK specifically covering these current Affiliate APIs. No third-party code was
copied; the package uses the Go standard library and social-hub internals only.
