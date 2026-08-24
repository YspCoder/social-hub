# Airbridge S2S Events API v2 adapter

`social-hub/adapters/airbridge` submits typed mobile-app and web attribution
events to Airbridge. It is registered as `airbridge/s2s-events-api-v2` and
exposes an `Events()` workflow rather than the organic social interfaces.

The contract was verified on 2026-08-25 against Airbridge's current S2S Event,
standard-event, semantic-attribute, and taxonomy-limit documentation. No
mature, maintained Go S2S SDK was available to reuse, so this adapter implements
the official HTTP contract directly.

## Authentication and configuration

Create one social-hub account per Airbridge app. `app_name` is the lowercase
alphanumeric Airbridge App Name, not a display name. Store the app's API token
behind `access_token_ref`; it is resolved only at runtime and sent as a Bearer
token. Requests always target `https://api.airbridge.io`; endpoint overrides,
redirects, and cookie jars are disabled so credentials and attribution
identifiers cannot cross origins.

```yaml
version: 1
platforms:
  - adapter: airbridge/s2s-events-api-v2
    product: s2s-events-api
    accounts:
      - id: mobile-production
        access_token_ref: env://AIRBRIDGE_API_TOKEN
        settings:
          app_name: exampleapp
```

Airbridge API access and the API token must be provisioned for the target app.
The public S2S contract documents a limit of 1,000 requests per minute for each
Mobile and Web endpoint. This adapter reports that limit in capability metadata
but does not create process-local quota state; apply the shared social-hub rate
limiter across all workers using the same app.

## Mobile events

```go
import "social-hub/adapters/airbridge"

result, err := client.Events().SendMobileEvent(ctx, airbridge.MobileEventRequest{
    EventUUID:      "9b4b3e4e-2162-4ae6-8986-91ee84644262",
    EventTimestamp: &occurredAt,
    User: airbridge.User{
        ExternalUserID: "customer-123",
    },
    Device: airbridge.Device{
        DeviceUUID:      device.AirbridgeUUID,
        GAID:            device.GAID,
        OSName:          airbridge.OSAndroid,
        OSVersion:       device.OSVersion,
        LimitAdTracking: &device.LimitAdTracking,
    },
    App: airbridge.App{
        PackageName: "com.example.app",
        Version:     "1.2.3",
    },
    Goal: airbridge.Goal{
        Category: airbridge.EventOrderCompleted,
        Value:    airbridge.Decimal("19.95"),
        SemanticAttributes: airbridge.SemanticAttributes{
            TransactionID: "order-123",
            Currency:      "USD",
            Products: []airbridge.Product{{
                ProductID: "sku-123",
                Price:     airbridge.Decimal("19.95"),
                Currency:  "USD",
                Quantity:  &quantity,
            }},
        },
    },
    ForwardedFor: requestIP,
})
```

A Mobile event requires `user.externalUserID` or `device.deviceUUID`. A supplied
Device UUID also requires `osName` and `osVersion`. The adapter models Android
and Apple identifiers separately and validates ATT values before network I/O.

Send the client IP by exactly one route:

- set `ForwardedFor` to emit `X-Forwarded-For`; or
- set `Device.ClientIP`, which also emits `x-airbridge-use-client-ip: 1`.

Both IPv4 and IPv6 are accepted. DMA consent is exposed as booleans and encoded
to Airbridge's documented `device.alias` string values (`"0"` or `"1"`).

## Web events

```go
result, err := client.Events().SendWebEvent(ctx, airbridge.WebEventRequest{
    Browser: airbridge.Browser{
        ClientID:  browser.AirbridgeClientID,
        UserAgent: browser.UserAgent,
    },
    ShortID: browser.AirbridgeShortID,
    Tracking: &airbridge.TrackingData{
        Channel: browser.AirbridgeChannel,
        Params: airbridge.Properties{
            Strings: browser.AirbridgeTrackingParams,
        },
    },
    Goal: airbridge.Goal{
        Category: airbridge.EventProductViewed,
    },
    ForwardedFor: requestIP,
})
```

A Web event requires `user.externalUserID` or `browser.clientID`. Without an
External User ID, `shortID`, tracking channel, and the tracking params object
must be supplied together so browser attribution is not silently degraded.
`ForwardedFor` is required for Web events.

## Event and value contract

The adapter provides constants for all 25 current Airbridge Standard Event
categories while keeping `EventCategory` open for custom categories. Category,
action, and label values are limited to 128 characters. The semantic model covers
the documented commerce, subscription, rating, achievement, sharing, schedule,
gaming, ad-revenue, and first-event attributes, including typed Product values
and nested `adPartners` data.

`Decimal` writes an exact plain JSON number and rejects exponent notation,
NaN, and infinity. `Properties` allows flat string, exact-number, and boolean
objects and rejects keys duplicated across those maps. Encoded
`customAttributes` cannot exceed Airbridge's documented 2,048-byte limit.

`eventUUID` is optional; when present it must be UUIDv4 and provides Airbridge's
documented duplicate-event identity. `eventTimestamp` is encoded as Unix
milliseconds and must be no more than 24 hours old. `WithIdempotencyKey` is
rejected because Airbridge documents body-level `eventUUID`, not an idempotency
header. Generate and persist one UUIDv4 per logical event before retrying an
ambiguous delivery.
Of the shared call options, only `WithCallTimeout` is accepted; caller request
IDs and generic field selectors have no documented S2S Events API meaning.

## Responses and errors

Only HTTP 200 with the documented non-empty `at` and `data` response fields is
accepted. HTTP 400 is a permanent input error, 401 requires credential action,
403 requires permission action, 404 is permanent, and 429 plus 5xx responses
are retryable. Error response bodies are discarded because they may contain
credentials or user identifiers; callers receive only a generic message,
status, bounded request/correlation ID, and bounded `Retry-After`.

Official references:

- S2S Event API: <https://help.airbridge.io/en/references/s2s-event>
- API introduction and token authentication: <https://help.airbridge.io/en/references/introduction>
- Airbridge App Name rules: <https://help.airbridge.io/en/guides/register-a-new-app>
- Standard Events and Semantic Attributes: <https://help.airbridge.io/en/developers/standard-events-and-semantic-attributes>
- Event Taxonomy Limitations: <https://help.airbridge.io/en/developers/event-taxonomy-limitations>
