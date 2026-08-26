# Sovrn Commerce publisher adapter

Registration name: `sovrn/commerce-api-v20230630`

This package implements a deliberately bounded Sovrn Commerce publisher
surface. Sovrn does not publish one version for every Commerce product: Real-
Time Reports declares OpenAPI version `20230630`, Approved Merchants declares
`1.0.0`, and programmatic affiliate-link construction is unversioned. The
registration uses the reporting version while metadata and this document state
the complete product boundary.

## Implemented workflows

| social-hub method | Sovrn operation | Contract boundary |
|---|---|---|
| `Commerce().BuildAffiliateLink` | build `https://sovrn.co?...` | destination, CUID, UTM, bid-floor, and fallback parameters |
| `Commerce().ListTransactions` | `GET https://viglink.io/v1/reports/transactions` | at least one single-day click, commission, or update date |
| `Commerce().GetMerchantPerformance` | `GET https://viglink.io/v1/reports/merchants` | required exclusive date range of at most 31 days and documented filters |
| `Commerce().ListApprovedMerchants` | `POST https://viglink.io/merchants/rates/summaries` | campaign-scoped filters and page/pageSize pagination |

Campaign lookup and Link Check are intentionally excluded because their
current official pages do not publish a rate-limit contract. Merchant deltas,
other report dimensions, product recommendations, comparisons, promo codes,
and advertising/deal APIs are separate surfaces and are not inferred here.

## Authentication and configuration

Sovrn assigns two site-specific Commerce credentials with different purposes:

- the Commerce API Key is embedded in affiliate redirect links and is
  configured as `client_id`;
- the Secret Key authorizes report and merchant-rate requests and is resolved
  from `secret_ref`.

The HTTP authorization value is exactly `secret {SECRET_KEY}`. These are static
site credentials, not OAuth tokens; Sovrn publishes no refresh-token flow.
Different sites in the same Sovrn account have different Secret Keys.

```yaml
version: 1
platforms:
  - adapter: sovrn/commerce-api-v20230630
    product: commerce-api
    accounts:
      - id: primary-site
        client_id: your-commerce-api-key
        secret_ref: env://SOVRN_COMMERCE_SECRET_KEY
        approval:
          account_type: commerce-publisher
```

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/sovrncommerce"
```

The Secret Key is sent only in the `Authorization` header. The reports,
merchant-rates, and affiliate-link origins are fixed to the three official
hosts listed above. Redirects are rejected and the supplied HTTP client's
cookie jar is removed so credentials and ambient cookies cannot move to
another origin. Endpoint overrides, OAuth token stores, and approval scopes are
not accepted.

## Request and data guarantees

- `BuildAffiliateLink` URL-encodes the destination and fallback exactly once.
  The official prose calls CUID alphanumeric while its own example contains
  underscores, so this adapter enforces the documented 2048-character limit
  and safe text but does not invent a narrower character class.
- Transactions require at least one of `clickDate`, `commissionDate`, or
  `updateDate`. Each value selects one calendar day; longer ingestion windows
  must be split into daily calls. Campaign and merchant filters are positive,
  comma-separated IDs and program type is `CPA` or `CPC`.
- The transaction example uses nested `account`, `commission`, `click`, and
  `merchant` objects, while the same page's OpenAPI schema declares a flat
  `TransactionModel`. The decoder accepts both official representations and
  retains every transaction's original `Raw` JSON.
- Merchant performance treats `clickDateEnd` as exclusive and rejects windows
  longer than 31 calendar days. The typed request covers every filter currently
  listed in the OpenAPI operation, including page/link UTM dimensions.
- Approved Merchants defaults omitted pagination to `page=1` and
  `pageSize=1000`; the documented maximum page size is 2500. The combined
  filter value count is locally limited to 2500. `GROUP_ID` values are emitted
  as numbers; the other filter types use the textual values required by the
  official guide despite the generated schema's narrower integer item type.
  `GEO` values must be lower-case ISO country codes and cannot be combined with
  a requested CPC program type, matching the current guide.
- Provider identifiers, counters, and decimal values use `ExactValue` to avoid
  JSON `float64` coercion. Successful response entities and envelopes retain
  bounded `Raw` JSON; it may contain commercially sensitive transaction and
  merchant data and must be handled accordingly.
- Only `socialhub.WithCallTimeout` is accepted per operation. Sovrn does not
  define caller request IDs, idempotency-key headers, or generic field
  selection for these workflows, so those call options are rejected.
- Encoded request URLs and JSON bodies are limited to 1 MiB. Successful
  response objects are limited to the shared transport's 8 MiB boundary and
  must use HTTP 200 with a JSON content type and the documented top-level
  collection fields.
- The returned affiliate URL necessarily contains the Commerce API Key. Avoid
  logging it even though the key is designed to travel in click-through URLs.

## Rate limits

Sovrn currently documents these operation-specific limits:

| workflow | official limit |
|---|---|
| Transactions | 1 request every 60 seconds |
| Merchant performance | 1 request every 60 seconds |
| Approved Merchants | 1 request every 10 seconds |

HTTP 429 maps to `socialhub.CodeRateLimited` and a retryable class. A supplied
`Retry-After` header is bounded before exposure, but the documentation does not
promise that header. A central limiter should key these policies by site Secret
Key and operation rather than assume one global Commerce QPS. Affiliate-link
building is local string construction; the resulting redirect is used by the
clicker, not called by this SDK method.

The implemented products publish no shared error schema. HTTP error bodies are
therefore discarded rather than exposed because they may echo credentials or
report data. Secret references, both site keys, and unsafe response metadata
are filtered from returned errors. Transport timeouts and HTTP 5xx responses
are retryable; authentication and approval failures require user action.

Official documentation reviewed 2026-08-26:

- <https://developer.sovrn.com/docs/authorization>
- <https://developer.sovrn.com/reference/building-affiliate-links>
- <https://developer.sovrn.com/reference/get_reports-transactions>
- <https://developer.sovrn.com/reference/get_reports-merchants>
- <https://developer.sovrn.com/reference/post_summaries>

All five official HTML pages returned HTTP 200 on 2026-08-26 with these
SHA-256 digests:

- authorization: `ACE828B43C82F0D0C4AEFCAAC3F6DB5435B205647941B8FEB1F188ECCC2269AD`
- affiliate links: `19693368293E34B0183A9C113D249A70A73FA60DD1355279301F8679A725A8E6`
- transactions: `DA925F3C94A346723235A517ABF2D846CD5BAD85F37B72D890F8BE40757053A7`
- merchant performance: `E9EDC1758C7B3C4776FAB2A2765BE305EA8FF7E8A66352139ED5F9BAF4AEB5C2`
- approved merchants: `5EB04CD65DDDC8BE86A1310D167E3B7AC030C655CD173489984A50152F1AAC29`

No credentialed request was sent to a production Sovrn Commerce account.
