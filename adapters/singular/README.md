# Singular S2S EVENT API v2 adapter

`social-hub/adapters/singular` submits typed attribution and revenue events to
Singular's SDID-based S2S EVENT API v2. It is registered as
`singular/s2s-events-v2` and exposes an `Events()` workflow rather than the
organic social interfaces.

The contract was verified on 2026-08-26 against Singular's current EVENT v2,
Web S2S, standard-event, standard-attribute, and response-error documentation.
No mature, maintained Go SDK for this endpoint was available to reuse, so this
adapter implements the official form-encoded wire contract directly. It does
not implement the legacy V1 EVENT or SESSION endpoints.

## Authentication and configuration

Create one social-hub account per Singular app or Web product. Put the
case-sensitive Singular app identifier in `app_id` and store the SDK Key behind
`access_token_ref`. Use the SDK Key from Developer Tools, not a Reporting API
Key. The resolved key is sent only as form parameter `a`; this API does not use
an `Authorization` header. Redirects are rejected so the key and attribution
identifiers cannot cross origins. The API origin is fixed to
`https://s2s.singular.net`, and the supplied HTTP client's cookie jar is
removed so ambient cookies cannot be sent to Singular.

```yaml
version: 1
platforms:
  - adapter: singular/s2s-events-v2
    product: s2s-events
    accounts:
      - id: mobile-production
        app_id: com.example.app
        access_token_ref: env://SINGULAR_SDK_KEY
```

All v2 events require an SDID UUIDv4. On iOS and Android, obtain the SDID from
the initialized Singular SDK. Mobile integrations must establish the SESSION
before sending events and preserve event order. On Web, persist one SDID in the
browser and reuse it across visits. For PC, consoles, Meta Quest, and CTV,
generate and persist one UUIDv4 for the app installation. The documented
console spelling is case-sensitive `PlayStation`, exposed as
`PlatformPlayStation`.

## Mobile events

```go
import "social-hub/adapters/singular"

result, err := client.Events().SendEvent(ctx, singular.EventRequest{
    Platform:   singular.PlatformAndroid,
    SDID:       device.SingularDeviceID,
    Name:       singular.EventAddToCart,
    IPAddress:  requestIP,
    OSVersion:  device.OSVersion,
    AppVersion: appVersion,
    Attributes: singular.Properties{
        Strings: map[string]string{
            string(singular.AttributeContentID): "sku-123",
        },
        Numbers: map[string]singular.Decimal{
            string(singular.AttributeItemPrice): "19.950",
        },
    },
})
```

Supply exactly one IP source: `IPAddress`, or `UseRequestIP: true` together
with an uppercase two-letter `Country`. Mobile make and model must be supplied
together. iOS always requires `ATTStatus` in the range 0-3. Typed SKAN fields
are iOS-only and validate conversion values and timestamp ordering.

`Properties` writes the `e` JSON form field and accepts strings, exact decimal
numbers, booleans, and string lists. Keys and values are bounded by Singular's
500-character attribute limit. The documented CAPI match fields (`ehash`, `phash`,
`fnamehash`, `lnamehash`, and `phashE164`) require lowercase SHA-256 values.
`GlobalProperties` is limited to five pairs and 200 Unicode characters per key
and value.

Event names must be 1-32 printable ASCII characters. The typed standard-event
constants follow the current official catalog; obsolete `sng_book`,
`sng_submit_application`, and `sng_update` constants are intentionally not
defined. Custom names remain available through `EventName`. `eventId` is a
partner-platform deduplication attribute, not a Singular ingestion dedupe key,
and `CustomUserID` must not contain personally identifiable information.

## Web attribution

The first event for a visit must use `__PAGE_VISIT__`,
`ConversionEvent: true`, a stored marketing landing URL, and attribution data.
Later engagement events use `ConversionEvent: false` and omit attribution data.

```go
result, err := client.Events().SendEvent(ctx, singular.EventRequest{
    Platform:  singular.PlatformWeb,
    SDID:      browser.SingularDeviceID,
    Name:      singular.EventPageVisit,
    IPAddress: requestIP,
    Web: &singular.WebData{
        ConversionEvent: true,
        LandingPageURL:  landingURL,
        DeviceUserAgent: browser.UserAgent,
        AttributionData: map[string]string{
            "partner_name":          "Snapchat",
            "is_attributed":         "true",
            "partner_campaign_name": "campaign-1",
        },
    },
})
```

The adapter treats `web_url` and `web_page_referrer` as distinct attribution
inputs and does not substitute event-logging-only URL fields for them.

## Revenue and ad revenue

Use `Revenue` for business revenue. Amount and currency are paired; exact
decimals avoid `float64` rounding. A revenue event without an amount is valid
only with `IsRevenueEvent: true`. App-store receipts are restricted to mobile,
and receipt signatures are Android-only.

Use `AdRevenue` for impression-level ad monetization. The adapter requires the
fixed `__ADMON_USER_LEVEL_REVENUE__` event name, writes
`is_admon_revenue=true` and `is_revenue_event=true`, and builds the documented
`ad_platform` attributes in `e`.

## Delivery and errors

Singular does not deduplicate S2S events. Only `socialhub.WithCallTimeout` is
accepted per call. Caller request IDs, idempotency-key headers, and generic
field selection are rejected because Singular documents none of them. Encoded
form bodies are limited to 1 MiB as a local resource boundary. Callers must
persist a logical event identity and prevent duplicate submission in their own
delivery pipeline, especially after ambiguous network failures.

Success requires HTTP 200 and body `{"status":"ok"}`. The adapter also checks
the body because Singular reports normal parameter failures as HTTP 200 with
`status=error`. Authentication-shaped reasons require credential action;
reasons containing `invalid`, `missing`, `should have`, or `no device id` are
permanent input errors. Unknown reasons remain retryable as Singular directs,
as do HTTP 429, timeouts, and 5xx failures. A success response must use a JSON
content type and contain `status=ok` without a reason. Free-form response
reasons, request bodies, and non-200 bodies are never retained because they can
echo SDK keys, app identifiers, and device identifiers. SDK-key references,
resolved keys, and app identifiers are also filtered from credential errors
and response request IDs.

Official references:

- EVENT endpoint: <https://support.singular.net/hc/en-us/articles/31496864868635-Server-to-Server-EVENT-Endpoint-API-Reference>
- Web S2S guide: <https://support.singular.net/hc/en-us/articles/52863243353627-Server-to-Server-Web-S2S-Implementation-Guide>
- Response codes and errors: <https://support.singular.net/hc/en-us/articles/31542603988379-Server-to-Server-API-Response-Codes-Errors>
- Standard events: <https://support.singular.net/hc/en-us/articles/7648172966299-Singular-Standard-Events-Full-List-and-Recommended-Events-by-Vertical>
- Conversion API attributes: <https://support.singular.net/hc/en-us/articles/30957145603611-Standard-Event-Attributes-for-Conversion-API-Integrations>

The official Help Center JSON responses returned HTTP 200 on 2026-08-26 with
these SHA-256 digests:

- EVENT endpoint: `3B8BA49F933C9CED6A81DA833CC2E05B90C2CC4A2220B0D4FCF245D99A2DEE28`
- Web S2S guide: `1D992ED62DCE22386F17E26FF4166429350F08C1398FEAB7E00A63F98E88F08A`
- Response codes and errors: `38B74E41A52E81C17FFF9328524FD5FD75640972834CF90A8289A6E37E4B5F32`
- Standard events: `DF79629977E13165337C8DEAC981026822D9B5A5EF22906621B4CF33E31D58F3`
- Conversion API attributes: `584A9AC06FF13F2D8F57162578678A248131A70BC55EFD53E7EDF69B1A023819`

No credentialed request was sent to a production Singular account.
