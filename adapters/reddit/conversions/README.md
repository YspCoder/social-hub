# Reddit Conversions API v3 adapter

`reddit/conversions-api-v3` submits typed Web, App, and Offline conversion
events to one configured Reddit Pixel. It is separate from both the organic
`reddit/data-api` and `reddit/ads-api-v3`: conversion telemetry has a dedicated
endpoint, scope, rate policy, and customer-data contract.

The wire contract was re-verified on 2026-08-25 against Reddit's official
server-side GTM template at commit
`17d29059cc7a0eb644c4ad11c1e3d383a352244b` and the available v3 operation
references. It covers `POST /pixels/{pixel_id}/conversion_events` without
depending on a generated SDK.

## Configuration and authentication

Each social-hub account owns exactly one Pixel ID. The access token is resolved
from `access_token_ref` and sent as a Bearer header. Reddit requires a
descriptive User-Agent; redirects are rejected so neither header can be
forwarded to another origin. The endpoint is fixed to
`https://ads-api.reddit.com/api/v3`; caller Cookie Jars are not used. An Ads
advertiser ID is not part of this operation: the path identifier is always the
Events Manager Pixel ID (`t2_`, `a2_`, or `p2_`).

```yaml
version: 1
platforms:
  - adapter: reddit/conversions-api-v3
    product: conversions-api
    settings:
      user_agent: "linux:com.example.socialhub:v0.1.0 (by /u/example)"
    accounts:
      - id: reddit-store-pixel
        access_token_ref: env://REDDIT_CONVERSION_TOKEN
        settings:
          pixel_id: p2_abc123
```

Reddit recommends the non-expiring conversion access token generated in Events
Manager. An OAuth access token is also accepted when it carries the
`adsconversions` scope; put that scope in `approval.scopes` to enable local
approval checks. OAuth exchange and refresh are deliberately caller-managed in
this package because the recommended conversion token has no refresh lifecycle.

## Usage

Importing the package registers the adapter. The typed workflow is available
from `Client.Conversions()`:

```go
import (
    "context"
    "time"

    conversions "social-hub/adapters/reddit/conversions"
    "social-hub/pkg/socialhub"
)

func sendPurchase(ctx context.Context, common socialhub.Client) error {
    client := common.(*conversions.Client)
    itemCount := int32(1)
    _, err := client.Conversions().SubmitEvents(ctx, conversions.SubmitEventsRequest{
        Events: []conversions.ConversionEvent{{
            EventAt:        time.Now().UnixMilli(),
            ActionSource:   conversions.ActionSourceWebsite,
            EventSourceURL: "https://shop.example/checkout",
            Type: conversions.EventType{
                TrackingType: conversions.TrackingPurchase,
            },
            Metadata: &conversions.Metadata{
                ConversionID: "order-1001",
                Currency:     "USD",
                ItemCount:    &itemCount,
                Value:        "49.95",
            },
            User: &conversions.UserData{
                Email:       "buyer@example.com",
                IPAddress:   "192.0.2.10",
                UserAgent:   "example-store/1.0",
            },
        }},
    })
    return err
}
```

`Decimal` values are base-10 strings in Go and JSON numbers on the wire. They
never pass through `float64`. `value` and its three-letter currency are each
optional and independently validated; product prices, quantities, item count,
and screen sizes are validated as nonnegative values.

## Customer data policy

The adapter hashes customer identifiers locally so plaintext does not leave the
process. It also accepts an exact 64-character lowercase SHA-256 value and does
not hash it again. Uppercase digests and legacy MD5 values are rejected because
their intent is ambiguous.

| Wire field | Normalization before SHA-256 |
|---|---|
| `email` | Lowercase; remove dots and `+tag` from the local part |
| `phone_number` | Remove extension and nondigits; restore the leading `+` E.164 form |
| `external_id` | Preserve the trimmed, case-sensitive identifier |
| `idfa` | Validate UUID form and uppercase |
| `aaid` | Validate UUID form and lowercase |
| IP address, user agent, Pixel UUID, click ID | Validate and send without hashing; strip the timestamp prefix from a raw `_rdt_uuid` Cookie value |

Validation failures expose field paths, not values. Reddit's free-form CAPI
error messages, field messages, and successful response messages are discarded
because they may echo customer data. Numeric platform codes, HTTP status,
request ID, and retry timing remain available through `socialhub.Error`.

## Event contracts

- Production requests contain 1-1,000 events and are fully normalized before
  any HTTP request.
- `event_at` is Unix epoch milliseconds. Events must be no more than seven days
  old; the adapter allows five minutes of positive clock skew.
- Supported action sources are `WEBSITE`, `APP`, `OTHER`, and
  `PHYSICAL_STORE`.
- Standard tracking types are `PAGE_VISIT`, `VIEW_CONTENT`, `SEARCH`,
  `ADD_TO_CART`, `ADD_TO_WISHLIST`, `PURCHASE`, `LEAD`, and `SIGN_UP`.
  `CUSTOM` requires a case-sensitive UTF-8 name of at most 64 characters.
- `event_source_url` is accepted only for `WEBSITE` and must be an absolute
  HTTP(S) URL. Reddit can extract `rdt_cid` from it when `click_id` is absent.
- Limited Data Use requires exactly `LDU`, an ISO alpha-2 country, and an
  optional matching subdivision code.
- Setting `test_id` uses Events Manager testing. The adapter requires exactly
  one event because Reddit currently displays only one event from a test
  request.

Use a stable `conversion_id` across Pixel and CAPI copies of the same event.
Reddit requires events within two days for reliable cross-channel
deduplication, even though normal ingestion accepts events up to seven days old.

## Rate limits and retries

| Policy | Limit |
|---|---:|
| Events per request | 1,000 |
| Test events | 10 per second |

The currently reachable official template confirms the test-event limit, while
Reddit's hosted v3 documentation was unavailable during this verification. The
adapter therefore does not export unverified global request or event quotas;
HTTP 429 and bounded `Retry-After` remain authoritative. Explicit 429 responses
are retryable. Ambiguous transport and 5xx failures remain retryable only when
every event has a stable `conversion_id`. The adapter never retries or queues
automatically and rejects `Idempotency-Key` because Reddit does not document
that header for CAPI.

## Deliberate exclusions

- Data Deletion API and its `adsdatadeletion` scope;
- Pixel administration, `Get Last Fired At`, and Ads reporting;
- OAuth grant and refresh helpers;
- automatic retry, queueing, or raw-event logging; and
- v2 payload and `tracking_type` compatibility aliases.

No credentialed request has been run against a production Pixel. Package builds
and production-file static checks are the current verification baseline.

Official references:

- <https://ads-api.reddit.com/docs/v3/operations/Post%20Conversion%20Events>
- <https://ads-api.reddit.com/docs/v3/guides/programs/capi/direct-integration>
- <https://ads-api.reddit.com/docs/v3/guides/programs/capi/verify-events>
- <https://ads-api.reddit.com/docs/v3/guides/programs/capi/best-practices>
- <https://ads-api.reddit.com/docs/v3/guides/programs/capi/error-handling>
- <https://github.com/reddit/reddit-ss-gtm-template/tree/17d29059cc7a0eb644c4ad11c1e3d383a352244b>
