# Awin Publisher API 1.0 adapter

Registration name: `awin/publisher-api-v1.0`

This adapter implements a bounded affiliate-commerce surface for an Awin
publisher account. It does not implement advertiser-side management, the old
query-token examples, or an OAuth refresh flow that Awin does not provide.

## Implemented workflows

| social-hub method | Awin operation | Notes |
|---|---|---|
| `Awin().ListPrograms` | `GET /publishers/{publisherId}/programmes` | country, hidden-program, and relationship filters; top-level array, no pagination |
| `Awin().DownloadEnhancedFeed` | `GET /publishers/{publisherId}/awinfeeds/download/{advertiserId}-retail-{locale}.jsonl` | bounded streaming JSONL download with terminal-error detection |
| `Awin().GenerateTrackingLink` | `POST /publishers/{publisherId}/linkbuilder/generate` | destination, campaign/clickref1-6, and optional short URL |
| `Awin().ListTransactions` | `GET /publishers/{publisherId}/transactions/` | at most 31 days per call; optional advertiser, date-type, status, basket, and timezone filters |
| `Awin().GetAdvertiserPerformance` | `GET /publishers/{publisherId}/reports/advertiser` | date, singular `region`, date-type, and timezone filters |

The initial surface excludes account discovery, batch link generation,
short-link quota lookup, campaign reporting, advertiser-side feeds, and other
account types. None of the five current workflows exposes API pagination.

## Configuration

Awin describes its personal API token as an OAuth 2.0 Bearer token, but the
token is generated and revoked in the Awin UI. There is no authorization-code,
refresh-token, or refresh endpoint. A 401 therefore requires token rotation or
permission correction by the user.

```yaml
version: 1
platforms:
  - adapter: awin/publisher-api-v1.0
    product: publisher-api
    accounts:
      - id: primary-publisher
        access_token_ref: env://AWIN_PERSONAL_API_TOKEN
        approval:
          account_type: approved-awin-publisher
        settings:
          publisher_id: 123456
```

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/awinpublisher"
```

The token is sent only as `Authorization: Bearer <token>`. Some generated Awin
endpoint tables still show a required `accessToken` query parameter; this is a
legacy Swagger artifact and the adapter never emits it. The adapter-level
`base_url` override is intended only for a controlled contract-verification
gateway. Redirects are rejected so credentials cannot move to another origin.

## Enhanced Feed guarantees

Enhanced Feed is UTF-8 JSON Lines rather than one JSON array. The download API
writes each validated product line to the supplied `io.Writer` as soon as it is
received, without buffering the whole feed. It does not use `bufio.Scanner`, so
records are not subject to Scanner's 64 KiB token limit.

- `MaxBytes` defaults to 256 MiB and bounds normalized bytes written. Callers
  processing larger catalogs can choose a larger value.
- `MaxLineBytes` defaults to 16 MiB and can be raised up to the hard 64 MiB
  provider-object limit.
- Stable `meta` and `product_basic` fields are typed. Vertical-specific
  sections remain `json.RawMessage`, and every decoded product retains its
  complete line in `Raw`.
- Awin can return HTTP 200 and place
  `{"error":500,"message":"Internal server error"}` on the final line. The
  adapter does not write that sentinel and returns a retryable `APIError`.
- If any error occurs after products were written, `FeedDownloadResult`
  reports the completed product and byte counts. The caller must treat the
  output as partial until the method returns `nil` error.
- Awin forbids concurrent downloads of the same advertiser feed. One client
  rejects duplicate `(publisher, advertiser, retail, locale)` downloads with a
  conflict error. Coordination across processes remains the application's
  responsibility.

## Other request and data guarantees

- Link generation uses a JSON body. An omitted destination uses the
  advertiser's landing page. Optional `parameters` is absent rather than an
  empty object. If Awin returns HTTP 200 with only `description`, the adapter
  maps it to an approval-required business error instead of reporting success.
  When `shorten=true`, both the primary and short URLs must be present.
- Transactions require RFC 3339 start/end instants and a timezone; an omitted
  timezone is sent explicitly as `UTC`. The window is bounded to 31 days.
  There is no page token, so larger synchronization ranges must be divided into
  adjacent windows by the caller.
- Transaction money and provider numeric values use `ExactValue`, avoiding
  `float64` coercion. Transactions and list envelopes retain `Raw`. Awin's
  current official 200 schema is incomplete; the typed optional fields reflect
  established production responses and `Raw` remains the forward-compatible
  source of truth. Order references, custom parameters, and basket products
  appear only when the advertiser grants and supplies them.
- Advertiser performance sends the documented singular `region=GB` form.
  `BR` represents Brazilian programs reported in BRL, while the legacy `BU`
  region represents Brazilian programs reported in USD. The primary response
  contract is a top-level array; the adapter defensively accepts the generated
  documentation's `{ "body": [...] }` wrapper and rejects other wrappers.
- Errors preserve the best-effort Awin `error`, `description`, and `message`
  fields, bounded raw body, request ID when present, and `Retry-After`. Awin
  does not guarantee a uniform error schema or rate-limit headers.

## Rate limits and access

The main Publisher API limit is 20 calls per minute per Awin user. One personal
token can authorize multiple publisher accounts, so a limiter must be keyed by
credential/user rather than only `publisherId`. Enhanced Feed has a stricter
limit of five downloads per minute plus the same-advertiser concurrency rule.
Awin does not guarantee `X-RateLimit-*` or `Retry-After`; HTTP 429 is retryable
even when the server supplies no reset hint.

Program relationships, Link Builder, reporting fields, order references, and
basket details remain subject to the token user's publisher access and each
advertiser's sharing settings. Permission changes can take about ten minutes to
propagate.

Official documentation reviewed 2026-08-26:

- <https://help.awin.com/apidocs/api-authentication>
- <https://help.awin.com/apidocs/introduction-1>
- <https://help.awin.com/apidocs/get-program-information>
- <https://help.awin.com/apidocs/retail-publisher-productapidocumentation-1>
- <https://help.awin.com/developers/docs/enhanced-feeds-publisher-faq>
- <https://help.awin.com/apidocs/generatelink>
- <https://help.awin.com/apidocs/returns-a-list-of-transactions-for-a-given-publisher>
- <https://help.awin.com/apidocs/get-advertiser-performance-report>
- <https://help.awin.com/apidocs/response-codes>

`GtheSheep/tap-awin` and current production-client observations were reviewed
only to model the response fields that Awin's current transaction schema omits;
they are not runtime dependencies or substitutes for the official contract.
