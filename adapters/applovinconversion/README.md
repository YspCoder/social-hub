# AppLovin Growth Conversion API v1 adapter

`adapters/applovinconversion` implements AppLovin's server-to-server Axon
Conversion API. It is registered as `applovin/growth-conversion-api-v1` and is
intentionally separate from the other AppLovin products:

- Growth Conversion submits conversion telemetry to `b.applovin.com`.
- Growth Campaign Management mutates Campaigns, Creative Sets, and Assets.
- Growth Reporting reads advertiser and asset performance.
- MAX Reporting reads publisher mediation and user-level revenue.

The contract was verified on 2026-08-25 against the current standard,
lead-generation, restricted lead-generation, events-and-objects, and event
deduplication documentation.

## Authentication and account policy

Configure the Event Key as `account.settings.event_key` and the Conversion API
Key as `secret_ref`. The adapter resolves the secret at runtime, sends it
verbatim in `Authorization`, and adds the Event Key as `pixel_id`. The API base
is fixed to `https://b.applovin.com/v1`; redirects are rejected and the HTTP
cookie jar is disabled so neither credential can cross origins.

Each account requires a closed policy because AppLovin's restricted lead-gen
contract prohibits every field it does not explicitly list:

| Policy | Events | Source URL | User data | Measurement data |
|---|---|---|---|---|
| `STANDARD` | All 14 current standard/mobile events | Complete HTTP(S) URL | Standard Conversion fields | Supported |
| `LEAD_GEN` | `page_view`, `generate_lead`, mobile `app_open` | Complete HTTP(S) URL | Standard lead-gen fields | Rejected |
| `RESTRICTED_LEAD_GEN` | `page_view`, `generate_lead` | Origin only; no path or query | Only `alart`, `aleid`, `esi`, `client_id`, IP, User-Agent, numeric `user_id` | Rejected |

Restricted mode also rejects email, phone, `axwrt`, advertising/device IDs,
country, OS, session, zip, and all other non-listed fields before network I/O.

```yaml
version: 1
platforms:
  - adapter: applovin/growth-conversion-api-v1
    product: growth-conversion-api
    accounts:
      - id: applovin-web-conversions
        secret_ref: env://APPLOVIN_CONVERSION_API_KEY
        settings:
          event_key: your-event-key
          policy: STANDARD
      - id: applovin-restricted-leads
        secret_ref: env://APPLOVIN_RESTRICTED_CONVERSION_API_KEY
        settings:
          event_key: your-restricted-event-key
          policy: RESTRICTED_LEAD_GEN
```

Key provisioning depends on the account and flow. The standard Conversion API
documentation directs advertisers to their Axon representative or Support;
the lead-gen documentation also exposes keys in the Ads Manager Keys section.

## Typed submission

```go
package main

import (
    "context"
    "time"

    "social-hub/adapters/applovinconversion"
)

func sendPurchase(ctx context.Context, client *applovinconversion.Client) error {
    result, err := client.Conversions().SubmitEvents(ctx, []applovinconversion.ServerEvent{{
        EventTime:      applovinconversion.UnixMilliseconds(time.Now()),
        EventSourceURL: "https://shop.example/checkout",
        Name:           applovinconversion.EventPurchase,
        UserData: applovinconversion.UserData{
            ClientID:        "stable-first-party-id",
            ClientIPAddress: "203.0.113.10",
            ClientUserAgent: "browser user agent",
            ESI:              applovinconversion.SourceWeb,
        },
        Data: &applovinconversion.PurchaseData{
            Currency:      "USD",
            Items: []applovinconversion.Item{{
                ItemID:  "sku-123",
                Price:   "19.95",
                Quantity: "2",
            }},
            Shipping:      "0",
            Tax:           "3.19",
            TransactionID: "order-123",
            Value:         "39.90",
        },
        DedupeID: "order-123",
    }})
    if err != nil {
        return err
    }
    _ = result
    return nil
}
```

`Decimal` fields are base-10 strings in Go and JSON numbers on the wire. This
preserves money and attribution values without `float64` rounding. Event data
is a closed interface implemented only by the event-specific types; raw maps
and unsupported custom objects cannot bypass validation.

For `page_view` and `app_open`, leave `Data` nil so the required JSON field is
sent as `null`. `app_open` additionally requires `user_data.esi` to be `app`.
When `event_source_url` contains an `aleid` query parameter, the adapter requires
the matching field to be present in `user_data`.

Standard Conversion API submissions use the official top-level event array.
Lead-gen and restricted lead-gen submissions use their documented
`{"events":[...]}` envelope. All policies send `application/json` to
`POST /v1/event?pixel_id=...`.

## Atomic batches, errors, and retries

`SubmitEvents` accepts one to 100 events. The complete batch is validated before
serialization and network I/O because AppLovin drops the entire request if any
event is invalid. Only HTTP 200 is accepted as success. HTTP 400 maps to a
permanent `invalid_argument` `APIError` with `BatchDropped=true`; do not retry
that batch unchanged.

AppLovin does not publish an item-count or request-byte limit for this endpoint.
For local resource safety, the adapter accepts at most 1,000 items per event and
rejects an encoded payload larger than 1 MiB.

The adapter does not log payloads and `APIError.Error()` never includes the
platform response body, Conversion API Key, Event Key, user identifiers, or
event data. Provider error bodies are discarded completely; the separate
`ConversionErrorDetails.Message` is also fixed adapter text.

The official contract does not define caller request IDs, an idempotency-key
header, or response field selection, so the corresponding generic
`CallOption`s are rejected. `WithCallTimeout` remains available because it is a
local client deadline.

Set the same `dedupe_id` in Pixel, SDK, and server events representing one user
action. AppLovin merges matching events received within five minutes and drops
a later duplicate after that window. A retry after an ambiguous transport or
5xx failure is therefore appropriate only when every event has a stable
`dedupe_id`. The adapter marks transient errors retryable only for such batches
and never retries submissions automatically. AppLovin documents only 200, 400,
and 401 responses and no global numeric quota or `Retry-After` contract; if the
service nevertheless returns 429 with `Retry-After`, the adapter applies its
bounded generic HTTP handling.

Official references:

- Standard Conversion API: <https://support.applovin.com/en/growth/promoting-your-websites/api/conversion-api>
- Lead-gen Conversion API: <https://support.applovin.com/en/growth/promoting-your-websites/api/conversion-api-for-lead-gen>
- Restricted lead-gen Conversion API: <https://support.applovin.com/en/growth/promoting-your-websites/api/restricted-lead-gen-capi>
- Events and objects: <https://support.applovin.com/en/growth/promoting-your-websites/track-and-optimize/events-and-objects>
- Event deduplication: <https://support.applovin.com/en/growth/promoting-your-websites/track-and-optimize/deduplicating-events>
