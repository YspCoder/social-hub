# Pinterest Conversions API v5.28 adapter

`pinterest/conversions-api-v5.28` submits typed Web, Android App, iOS App,
and Offline conversion events to one configured Pinterest Ad Account. It is
separate from both organic `pinterest/v5` and `pinterest/ads-v5.28` because
conversion ingestion has dedicated credentials, batching rules, quotas, and
customer-data handling requirements.

The implementation was checked against Pinterest's official OpenAPI tag
`v5.28.0` and conversion guides on 2026-08-25. It covers the current
`POST /ad_accounts/{ad_account_id}/events` operation without depending on an
outdated generated SDK.

## Configuration and authentication

Each social-hub account owns exactly one Pinterest Ad Account ID. The token is
resolved from `access_token_ref`, sent as a Bearer header, and never put in a
query string. The official API origin is fixed to `https://api.pinterest.com/v5`
and cannot be overridden. Redirects are rejected and the supplied HTTP client's
cookie jar is disabled so credentials and ambient cookies cannot cross origins.

```yaml
version: 1
platforms:
  - adapter: pinterest/conversions-api-v5.28
    product: conversions-api
    accounts:
      - id: pinterest-store-events
        access_token_ref: env://PINTEREST_CONVERSION_TOKEN
        settings:
          ad_account_id: "549768257474"
```

For conversion-only use, generate a Conversion Token in Ads Manager. It does
not require a Pinterest developer application and has no refresh lifecycle.
An externally managed Pinterest OAuth token is also accepted; configure
`approval.scopes: ["ads:write"]` to enable the local scope guard. OAuth grant,
refresh, and token storage remain caller-managed here; the separate Ads
adapter provides an OAuth helper for applications using the wider Ads API.

## Usage

Importing the package registers the adapter. The typed workflow is available
from `Client.Conversions()`:

```go
import (
    "context"
    "time"

    conversions "social-hub/adapters/pinterest/conversions"
    "social-hub/pkg/socialhub"
)

func sendCheckout(ctx context.Context, common socialhub.Client) error {
    client := common.(*conversions.Client)
    count := int64(1)
    _, err := client.Conversions().SubmitEvents(ctx, conversions.SubmitEventsRequest{
        Events: []conversions.ConversionEvent{{
            ActionSource:   conversions.ActionSourceWeb,
            EventID:        "order-1001",
            EventName:      conversions.EventCheckout,
            EventTime:      time.Now().Unix(),
            EventSourceURL: "https://shop.example/checkout",
            UserData: conversions.UserData{
                Emails:          []string{"buyer@example.com"},
                ClientIPAddress: "192.0.2.10",
                ClientUserAgent: "example-store/1.0",
            },
            CustomData: &conversions.CustomData{
                Currency: "USD",
                Value:    "49.95",
                NumItems: &count,
                OrderID:  "order-1001",
            },
        }},
    })
    return err
}
```

`Decimal` is a base-10 string in Go and remains a JSON string on the wire, as
Pinterest specifies. It never passes through `float64`. Invalid, negative,
exponential, and ambiguous leading-zero forms are rejected before networking.

## Customer data policy

The adapter accepts plaintext identifiers or exact lowercase 64-character
SHA-256 values. Plaintext is normalized and hashed in a temporary wire copy;
the caller's event is not mutated. Uppercase SHA-256 and legacy MD5 values are
rejected because their intent is ambiguous.

| Wire field | Normalization before SHA-256 |
|---|---|
| `em` | Remove whitespace and lowercase the email |
| `ph` | Keep digits only and remove leading zeros |
| `ge` | Lowercase `f`, `m`, or `n` |
| `db` | Convert separators to `YYYYMMDD` and validate the date |
| `fn`, `ln` | Trim surrounding whitespace and lowercase |
| `ct` | Lowercase and remove spaces and punctuation |
| `st`, `country` | Lowercase two-letter code |
| `zp` | Keep digits only, preserving leading zeros |
| `external_id` | Preserve the trimmed, case-sensitive identifier |
| `hashed_maids` | Validate UUID form, lowercase, then hash |

Every event must contain `em`, `hashed_maids`, or both
`client_ip_address` and `client_user_agent`. IP addresses, user agents,
`click_id`, and partner IDs are validated but not hashed.

Request validation errors expose field paths, not values. Pinterest's top-level
and per-event free-form messages are discarded because they can echo customer
data. `SubmitResult.Events` preserves event order, `processed`/`failed` status,
and error/warning presence without retaining message text. Numeric platform
codes, HTTP status, request ID, and retry timing remain available through
`socialhub.Error`.

## Event contract

- Production requests contain 1-1,000 events. Test requests set `?test=true`
  and contain 1-20 events because Pinterest processes only the first 20.
- Required fields are `action_source`, `event_id`, `event_name`, `event_time`,
  and `user_data`. `event_time` is Unix epoch seconds, not milliseconds.
- `partner_name` must be omitted for direct advertiser integrations. Pinterest
  partners may use only the lowercase `ss-company` value assigned with
  Pinterest; arbitrary values are rejected locally.
- Standard event constants cover all current documented names. A custom name
  may use ASCII letters, digits, `_`, and `-`, is case-insensitive at Pinterest,
  and is limited to 100 characters. Pinterest enforces the advertiser-wide
  limit of 15 custom event types.
- `event_source_url` is accepted only on Web events and must be an absolute
  HTTP(S) URL without credentials or a fragment.
- `CustomData` covers content, order, currency/value, predicted LTV, search,
  and Limited Data Processing fields. Internal-only `np` and external
  measurement fields are deliberately not exposed.
- The v5.28 `AppInfo` and `DeviceInfo` structures are fully typed, including
  viewport, install, OS, network, storage, screen, locale, and battery fields.
  Numeric ranges and enums follow the tagged OpenAPI schema.

Use the same `event_id` and `event_name` in Pinterest Tag and Conversions API
copies. Pinterest retains the first duplicate it receives and deduplicates
matching copies within 48 hours.

## Rate limits and retries

Official Pinterest sources currently disagree. The v5.28 endpoint description
states 5,000 calls per minute per Ad Account. The current Rate Limits guide
lists Standard OAuth `ads_conversions` access at 120,000 requests per minute
per Ad Account per app, test requests at 10 per app per second, and recommends
a Conversion Token for unlimited conversion tracking. Account responses and
`x-ratelimit-*` headers are therefore authoritative; use the lower documented
limit until Pinterest resolves the discrepancy.

HTTP 429 and 5xx responses map to retryable social-hub errors. The adapter
honors `Retry-After` and uses `x-ratelimit-reset` as a fallback. It does not
retry or queue automatically; callers should retry only when `Retryable()` is
true and must preserve `event_id` across attempts.

Pinterest explicitly states that Offline events cannot be deduplicated. For a
batch containing an Offline event, ambiguous transport and 5xx failures are
therefore classified as requiring caller action rather than automatic retry;
an explicit HTTP 429 remains retryable. Split Offline events from Web/App
batches when their retry policy differs.

Only `socialhub.WithCallTimeout` is supported per submission. Caller request
IDs, `Idempotency-Key`, and field selection are rejected because Pinterest does
not document those request controls. Deduplication uses `event_id` together with
`event_name`; exact duplicate pairs in one batch are rejected locally.

## Deliberate exclusions

- Conversion Deletion API;
- advertiser-defined event mapping and Event Quality Score endpoints;
- SAN, MMP, Conversion Tag, and automatic partner-only fields;
- OAuth exchange/refresh helpers;
- automatic retry, buffering, raw payload logging, or message retention.

No credentialed request has been run against a production Ad Account. Package
builds and production-file static checks are the current verification baseline.

Official references:

- <https://developers.pinterest.com/docs/track-conversions/track-conversions-in-the-api/>
- <https://developers.pinterest.com/docs/reference/rate-limits/>
- <https://github.com/pinterest/api-description/tree/v5.28.0>
