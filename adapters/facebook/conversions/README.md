# Meta Conversions API v26 adapter

`facebook/conversions-api-v26` submits typed server events to one configured
Meta Pixel or dataset. It is a separate product boundary from
`facebook/marketing-api-v25`: campaign management and conversion telemetry have
different privacy, retry, and response contracts.

The adapter contract was checked against the official Meta Node Business SDK
`v26.0.0`, released on 2026-08-06, and the Graph API v26 changelog. Meta's
v26 server-side event models are unchanged from the preceding stable SDK
release; this adapter nevertheless pins requests and metadata to v26.

## Configuration and authentication

Each social-hub account owns exactly one numeric `pixel_id`. The access token
is resolved from `access_token_ref` and sent only as a Bearer header. When
`secret_ref` is configured, the adapter adds the HMAC-SHA256
`appsecret_proof` query parameter. The API origin is fixed at
`https://graph.facebook.com/v26.0`, redirects are rejected, and caller cookie
jars are ignored so credentials cannot be forwarded to another origin.

```yaml
version: 1
platforms:
  - adapter: facebook/conversions-api-v26
    product: conversions-api
    accounts:
      - id: store-production
        access_token_ref: env://META_SYSTEM_USER_TOKEN
        secret_ref: env://META_APP_SECRET # optional appsecret_proof
        approval:
          scopes: [ads_management]
        settings:
          pixel_id: "123456789012345"
```

Production access requires the Meta app, Business, dataset, token subject, and
`ads_management` permission to be correctly connected. If
`approval.scopes` is present, the adapter fails locally with
`approval_required` when it does not include `ads_management`. An empty scope
list means approval state is managed externally.

## Usage

Importing the package registers the adapter. The typed workflow is available
from `Client.Conversions()`:

```go
import (
    "context"
    "time"

    conversions "social-hub/adapters/facebook/conversions"
    "social-hub/pkg/socialhub"
)

func sendPurchase(ctx context.Context, common socialhub.Client) error {
    client := common.(*conversions.Client)
    _, err := client.Conversions().SubmitEvents(ctx, conversions.SubmitEventsRequest{
        Events: []conversions.ServerEvent{{
            EventName:      "Purchase",
            EventTime:      time.Now().Unix(),
            EventID:        "order-1001",
            ActionSource:   conversions.ActionSourceWebsite,
            EventSourceURL: "https://shop.example/checkout",
            UserData: conversions.UserData{
                Emails:          []string{"buyer@example.com"},
                ClientIPAddress: "192.0.2.10",
                ClientUserAgent: "example-store/1.0",
            },
            CustomData: &conversions.CustomData{
                Value:       "49.95",
                Currency:    "USD",
                ContentIDs:  []string{"sku-1"},
                OrderID:     "order-1001",
            },
        }},
        PartnerAgent: "example-store",
    })
    return err
}
```

`Decimal` values are base-10 strings in Go and JSON numbers on the wire. They
never pass through `float64`. Scalar custom properties are split into string,
number, and boolean maps; a property cannot collide with a standard CAPI field
or occur in more than one map.

## Customer data policy

The adapter accepts plaintext or an exact 64-character lowercase SHA-256 value
for hash-required fields. Plaintext is normalized and hashed in a temporary
wire copy, and input structs are not mutated. Existing SHA-256 values are not
hashed again. Uppercase digests and legacy MD5 values are rejected because
their intent is ambiguous.

| Wire fields | Treatment |
|---|---|
| `em`, `ph`, `ge`, `db`, `fn`, `ln`, `ct`, `st`, `zp`, `country`, `external_id` | Normalize, SHA-256, deduplicate |
| `f5first`, `f5last`, `fi`, `dobd`, `dobm`, `doby`, `app_user_id` | Normalize and SHA-256 |
| client IP, user agent, `fbc`, `fbp`, subscription/login/lead IDs, mobile/anonymous IDs, `ctwa_clid`, `page_id` | Validate and send without hashing |

Validation failures report only field paths. Meta's free-form Graph error text
is not retained because it can echo customer information. Numeric Graph codes,
subcodes, HTTP status, retry hints, and safe trace IDs remain available through
`socialhub.Error`.

## Event contracts

- `website` requires an absolute HTTP(S) `event_source_url`.
- `app` requires typed `AppData` and `ExtendedDeviceInfo`; the latter always
  serializes as the official fixed 16-slot `extinfo` array.
- `business_messaging` requires exactly one of `messenger`, `whatsapp`, or
  `instagram` as `messaging_channel`.
- offline fields `namespace_id`, `upload_id`, `upload_tag`, and `upload_source`
  are available in the request envelope.
- `test_event_code` targets Events Manager's test-event flow.
- LDU country/state values are accepted only when `LDU` is present in
  `data_processing_options`.

The complete batch is normalized and validated before any HTTP request. The
adapter does not invent a fixed account quota or universal event batch limit:
Meta enforces product and account limits server-side, while HTTP 429, Graph
codes `4`, `17`, `32`, and `613`, and transient Graph failures map to retryable
social-hub errors. Callers should retry only when `Retryable()` is true and
should reuse `event_id` so browser/server duplicates can be reconciled.

Successful `events_received` and `num_processed_entries` counters are optional
in the public result because ingestion paths do not always return both. A
counter, when present, may not be negative or exceed the submitted event count.
Success responses must be JSON and any returned dataset ID must match the
configured dataset. Meta's free-form `messages` content is validated but not
retained; `MessageCount` indicates whether diagnostics were present without
placing customer-derived text into application logs.

## Deliberate exclusions

- browser request-context auto-extraction from Meta's Parameter Builder;
- Advanced Measurement, `original_event_data`, and attribution passback;
- Pixel management, Custom Audiences, and Offline Event Set administration;
- automatic retry or event queueing; and
- logging raw events, customer data, access tokens, or response bodies.

The first group changes data-collection consent behavior, while the other
excluded fields have separate contracts that should be added as explicit typed
extensions. No credentialed request has been run against a production dataset;
verification is limited to the published v26 contract and local static checks.

## Official references

- <https://developers.facebook.com/docs/marketing-api/conversions-api/>
- <https://developers.facebook.com/docs/marketing-api/conversions-api/parameters/server-event>
- <https://developers.facebook.com/documentation/ads-commerce/marketing-api/marketing-api-changelog/version26.0>
- <https://github.com/facebook/facebook-nodejs-business-sdk/releases/tag/v26.0.0>
