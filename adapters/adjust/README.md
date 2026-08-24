# Adjust S2S API adapter

`social-hub/adapters/adjust` submits typed event, session, and publisher ad
revenue measurements to Adjust. It is registered as `adjust/s2s-api` and does
not expose the organic `Publisher`, `Fetcher`, or messaging interfaces.

The public, continuously versioned contract was verified on 2026-08-25.

## Configuration

Configure one social-hub account per Adjust app. The app token is routing data
and belongs in account settings; the S2S Security token is a credential and
must be resolved through `access_token_ref`.

```yaml
version: 1
platforms:
  - adapter: adjust/s2s-api
    product: s2s-api
    accounts:
      - id: mobile-production
        access_token_ref: env://ADJUST_S2S_SECURITY_TOKEN
        approval:
          scopes: [events, sessions, ad_revenue]
        settings:
          app_token: your-adjust-app-token
          session_measurement_enabled: true
          ad_revenue_package_enabled: true
```

`session_measurement_enabled` is an explicit assertion that Adjust or the
account's Technical Account Manager enabled S2S sessions. CTV and PC/console
products may receive that entitlement during platform setup. Likewise,
`ad_revenue_package_enabled` asserts that the paid ad revenue package is
available. Calls fail locally with `approval_required` while the applicable
flag is false.

`approval.scopes` is optional for externally managed tokens. When present, it
must contain only `events`, `sessions`, and/or `ad_revenue`, and the adapter
rejects a call whose scope is absent before network I/O.
`approval.account_type` has no Adjust S2S meaning and is rejected.

The S2S token is sent only as `Authorization: Bearer <token>`. Redirects are
rejected, the HTTP cookie jar is disabled, and the API origin is fixed to
`https://s2s.adjust.com` so credentials cannot be redirected through
configuration.
S2S Security tokens are not compatible with Google Tag Manager event
measurement under Adjust's current contract.

## Typed workflows

```go
import "social-hub/adapters/adjust"

event, err := client.S2S().TrackEvent(ctx, adjust.EventRequest{
    EventToken: "purchase-event-token",
    Device: adjust.DeviceIdentifiers{
        GPSADID: device.GoogleAdvertisingID,
    },
    CreatedAt: &occurredAt,
    Environment: adjust.EnvironmentProduction,
    Revenue: "19.95",
    Currency: "USD",
    CallbackParams: map[string]string{
        "order_id": order.ID,
    },
})

session, err := client.S2S().TrackSession(ctx, adjust.SessionRequest{
    Device: adjust.DeviceIdentifiers{IDFA: device.IDFA},
    OSName: adjust.OSIOS,
    CreatedAt: &openedAt,
    ForwardedFor: device.IPv4,
})

adRevenue, err := client.S2S().TrackAdRevenue(ctx, adjust.AdRevenueRequest{
    Device: adjust.DeviceIdentifiers{ADID: device.AdjustID},
    Revenue: "0.0042",
    Currency: "USD",
    AdImpressionsCount: 1,
    CreatedAt: &impressionAt,
    Network: "example-network",
    Unit: "rewarded_video",
    Placement: "level_complete",
})
```

Money is represented by `Decimal`, a base-10 string, so values do not pass
through `float64`. Event revenue must be at least `0.001`; publisher ad revenue
must be non-negative. Adjust does not document an upper revenue bound, while
the adapter limits decimal input to 128 characters for local resource safety.
Revenue always requires an uppercase three-letter currency. Every request
requires at least one public Adjust device identifier.

The adapter emits `application/x-www-form-urlencoded` POST bodies, always sets
`s2s=1`, and applies a local 1 MiB encoded-request safety limit. Callback and
partner parameters are `map[string]string` values encoded as flat JSON
objects.

## Ordering, timestamps, and consent

`CreatedAt` is encoded as `created_at_unix` in Unix seconds. Events must arrive
in chronological order for each device/event-token pair and cannot be older
than 58 days. Ad revenue can arrive out of order but cannot be older than 28
days. These age limits are checked locally; per-device ordering remains the
caller's responsibility because the adapter keeps no global device state.

Adjust accepts a new Session after the documented inactivity interval and
requires timestamped successful sessions to remain in order. `SentAt`, when
provided, uses Adjust's timezone-bearing ISO-8601 `sent_at` form. Both
`IPAddress` and `X-Adjust-Forwarded-For` are restricted to IPv4 by the current
contract.

Google DMA values use optional boolean pointers so an explicit `false` is sent
as `0`. `AmazonDMAConsent` requires both `AdUserData` and `AdStorage`; the
adapter encodes them in the required nested `dma` JSON object.

## Errors and retries

Only HTTP 200 is accepted as success, including the documented plain-text `OK`
Event response. Adjust unusually uses HTTP 202 for a missing or unrecognized
S2S token; the adapter maps that response to `unauthenticated`, not success.
Adjust's native HTTP 401 `Session failed` response instead means the S2S token
lacks the required scope and is mapped to `permission_denied`.
Session responses are decoded into `ADID`,
`Timestamp`, `Message`, and `AskIn`; the response app token is deliberately not
exposed.

HTTP 429 and 5xx failures are retryable. HTTP 400/413/422 are permanent input
errors, while 401/403/451 require a scope, permission, or privacy action.
Platform response bodies may echo app tokens and device data, so the adapter
discards their free text and exposes only a generic message, HTTP status,
bounded request ID, and `Retry-After`.

Adjust's public S2S documentation does not state one global numeric request
quota. Applications should throttle independently per Adjust app and endpoint
and honor bounded `Retry-After` guidance when the service returns HTTP 429.
Of the shared call options, only `WithCallTimeout` is accepted. Adjust does not
document request-ID, idempotency-key, or response-field-selection semantics.

Official references:

- S2S overview: <https://dev.adjust.com/en/api/s2s-api/>
- Events: <https://dev.adjust.com/en/api/s2s-api/events/>
- Sessions: <https://dev.adjust.com/en/api/s2s-api/sessions/>
- Ad revenue: <https://dev.adjust.com/en/api/s2s-api/ad-revenue/>
- S2S Security: <https://dev.adjust.com/en/api/s2s-api/security/>
- Full S2S guide: <https://dev.adjust.com/en/api/s2s-api/s2s-developer-guide/>
