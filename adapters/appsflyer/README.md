# AppsFlyer Mobile S2S Events API v3 adapter

`adapters/appsflyer` implements AppsFlyer's Mobile S2S Events API v3. It is
registered as `appsflyer/mobile-s2s-events-api3` and intentionally exposes a
typed `Events()` workflow rather than the organic social `Publisher` contract.

The contract was verified on 2026-08-25 against the current API reference,
Mobile S2S guide, and token-management documentation.

## Authentication and account binding

Create an S2S token in AppsFlyer and store its secret reference in
`access_token_ref`. One social-hub account binds one AppsFlyer dashboard app.
The adapter sends the resolved key in the raw `authentication` header and
always posts to `https://api3.appsflyer.com`; endpoint overrides, redirects,
and cookie jars are disabled so the token cannot cross origins. The production
API requires TLS 1.2 or newer.

```yaml
version: 1
platforms:
  - adapter: appsflyer/mobile-s2s-events-api3
    product: mobile-s2s-events-api
    accounts:
      - id: ios-production
        access_token_ref: env://APPSFLYER_S2S_KEY
        settings:
          app_id: id123456789
          platform: ios
          bundle_identifier: com.example.app
```

Use the app ID exactly as shown in the AppsFlyer dashboard. iOS IDs must keep
their `id` prefix; AppsFlyer can otherwise return HTTP 200 without recording
the event. Android IDs are package names. `bundle_identifier` is optional but
recommended because integrated ad networks may use it for optimization.

AppsFlyer does not use OAuth for this API. Only account admins can view and
manage S2S tokens. AppsFlyer currently allows at most two tokens of each type,
recommends rotation every 180 days, and notes that a new S2S token can remain
pending for up to 30 minutes. For most ad networks, S2S token generation is no
longer self-service and is available only to specific managed partners.

## Platform-specific fields

| App platform | Supported device fields | Additional rules |
| --- | --- | --- |
| Android | `advertising_id`, `oaid`, `amazon_aid`, `imei`, `fb_login_id` | `app_set_id` is Android-only; `aie` carries the advertising-ID sharing signal. |
| iOS | `idfa`, `idfv`, `fb_login_id` | `os` is required by this adapter; use `att` for iOS 14.5+ and `aie` only for older iOS versions; `app_type=app_clip` is iOS-only. |
| Windows | `fb_login_id` and common event/customer fields | The current request schema documents mobile advertising identifiers and consent only for Android/iOS, so this adapter does not send them for Windows apps. |

Cross-platform advertising identifiers are rejected before network I/O to
avoid disclosing irrelevant device data. AppsFlyer ID, device identifiers, IP,
hashed customer data, and custom data are sent only when supplied by the
caller; the adapter does not enrich them.

## Typed event submission

```go
package main

import (
    "context"
    "time"

    "social-hub/adapters/appsflyer"
)

func sendPurchase(ctx context.Context, client *appsflyer.Client) error {
    consent := true
    result, err := client.Events().SendEvent(ctx, appsflyer.EventRequest{
        AppsFlyerID:   "1234567890123-1234567",
        EventName:     "af_purchase",
        EventValue: appsflyer.EventValues{
            "af_revenue": "19.95",
            "af_content_id": "sku-123",
        },
        EventCurrency: "USD",
        EventTime:     timePointer(time.Now()),
        OS:            "17.6",
        Device:        appsflyer.DeviceIdentifiers{IDFA: "00000000-0000-4000-8000-000000000000"},
        Consent: &appsflyer.ConsentData{Manual: &appsflyer.ManualConsent{
            AdUserDataEnabled: &consent,
        }},
    })
    if err != nil {
        return err
    }
    _ = result
    return nil
}

func timePointer(value time.Time) *time.Time { return &value }
```

`eventValue` is required by AppsFlyer. The adapter stringifies non-empty
`EventValues` as JSON and sends an empty map as `"eventValue":""`. Monetary
values in `eventValue` remain strings, matching AppsFlyer's wire examples.
`CustomData` supports typed strings, exact decimal JSON numbers, booleans, and
nested objects without allowing arbitrary `float64` values. Like `eventValue`,
it is serialized as a JSON string on the wire, as required by the current
OpenAPI schema.

Hashed customer fields accept only already normalized lowercase SHA-256
digests. Manual and TCF consent are mutually exclusive. For Amazon Ads' EU
consent policy effective 2026-06-30, send at least `ad_user_data_enabled` when
the manual flow applies; consult your privacy team for jurisdictional policy.

## Delivery semantics and retries

Each request contains exactly one event and the final JSON body is limited to
1024 bytes before network I/O. Only HTTP 200 is accepted. AppsFlyer performs
minimum validation before returning 200, so success does not prove that the
event was fully recorded. The adapter does not log request or response bodies.

The current public Mobile S2S documentation does not publish a numeric QPS or
daily quota. Treat limits as account- and contract-specific. HTTP 429 is
reported as a retryable rate-limit error and a valid `Retry-After` header is
preserved. Provider response text is used only for internal classification and
is never returned because it can contain app or device identifiers.

The current contract exposes no idempotency key or deduplication guarantee.
The caller must provide its own durable idempotency record before retrying an
ambiguous timeout or 5xx response. Sending an event within roughly 20-30
seconds of a new install can also precede install processing and cause organic
classification.

Official references:

- API reference: <https://dev.appsflyer.com/hc/reference/s2s-events-api3-post>
- API overview: <https://dev.appsflyer.com/hc/docs/s2s-events-api3-overview>
- Mobile S2S guide: <https://support.appsflyer.com/hc/en-us/articles/207034486-Server-to-server-events-API-for-mobile-S2S-mobile>
- Token management: <https://support.appsflyer.com/hc/en-us/articles/360004562377-Managing-AppsFlyer-tokens>
- Amazon Ads EU consent policy: <https://support.appsflyer.com/hc/en-us/articles/11904659502737-Amazon-Ads-campaign-configuration>
