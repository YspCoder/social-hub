# Tenjin S2S API v0 adapter

`social-hub/adapters/tenjin` sends typed app opens, custom events, purchases,
and impression-level ad revenue to Tenjin. It is registered as
`tenjin/s2s-api-v0` and exposes an `S2S()` workflow rather than the organic
social interfaces.

The contract was verified on 2026-08-26 against Tenjin's current S2S setup,
no-SDK tracking, and generic Impression-Level Revenue Data (ILRD) guides. The
official Tenjin GitHub organization publishes actively maintained Android,
iOS, Unity, Flutter, and React Native SDKs, but no Go S2S client is available.
This adapter therefore implements the official form contract directly without
adding a third-party dependency. Campaign Management, Reporting Metrics, Raw
Data Exports, LiveOps, and S2S Callback configuration remain separate APIs.

## Provisioning and configuration

Create one social-hub account per Tenjin app. Set `app_id` to the app's Bundle
ID and `account.settings.platform` to `ios`, `android`, `amazon`, or
`android_other`. The last value covers Android distribution without Google
Play, including Chinese app stores. Store the app-specific Tenjin SDK Key
behind `access_token_ref`.

The current public S2S and ILRD documents do not define a plan tier, approval
scope, or whitelist prerequisite. The adapter therefore reports these typed
capabilities with `ApprovalUnknown` and rejects approval fields instead of
turning undocumented historical restrictions into runtime gates.

```yaml
version: 1
platforms:
  - adapter: tenjin/s2s-api-v0
    product: s2s-api
    accounts:
      - id: android-cn-production
        app_id: com.example.game
        access_token_ref: env://TENJIN_SDK_KEY
        settings:
          platform: android_other
          google_ads: false
          meta_aem: false
```

The adapter uses HTTP Basic authentication with the SDK Key as the username
and an empty password. It never places the key in a URL or form body. The API
origin is fixed to `https://track.tenjin.com`, redirects are rejected, and the
supplied HTTP client's cookie jar is removed so credentials, cookies, and
device identifiers cannot cross origins.

## App opens and custom events

Generate and persist one Analytics Installation ID for each app installation,
then reuse it for every subsequent open, custom event, and purchase. The
adapter accepts a hyphenated UUID or a 32-hex-character UUID and sends Tenjin's
canonical lowercase, no-hyphen form.

```go
import "social-hub/adapters/tenjin"

identity := tenjin.DeviceIdentity{
    AnalyticsInstallationID: installationID,
    AdvertisingID:           device.OAID,
}

result, err := client.S2S().TrackOpen(ctx, tenjin.OpenRequest{
    Context: tenjin.EventContext{
        Identity:  identity,
        OSVersion: device.AndroidAPILevel,
        AppVersion: appVersion,
        IPAddress: requestIP,
        Country:   "CN",
    },
    Referrer: installReferrer,
})

value := int64(10)
result, err = client.S2S().TrackCustomEvent(ctx, tenjin.CustomEventRequest{
    Context: tenjin.EventContext{
        Identity:  identity,
        OSVersion: device.AndroidAPILevel,
        AppVersion: appVersion,
        IPAddress: requestIP,
    },
    Name:  "level_complete",
    Value: &value,
})
```

The first open received for an installation establishes the install; later
calls to the same endpoint are app opens. `advertising_id` remains present even
when empty or all-zero, as Tenjin requires. iOS requests require an IDFV in
`developer_device_id`. When the advertising ID is unavailable, the adapter
requires the actual user IP instead of silently letting Tenjin attribute using
the backend server's IP.

## Purchases and channel-specific fields

Purchase amounts use exact plain `Decimal` values and reject exponent notation
or negative values. Send gross unit revenue unless `AfterPlatformCut` is
explicitly true.

```go
result, err := client.S2S().TrackPurchase(ctx, tenjin.PurchaseRequest{
    Context: tenjin.EventContext{
        Identity:  identity,
        OSVersion: device.AndroidAPILevel,
        AppVersion: appVersion,
        IPAddress: requestIP,
    },
    ProductID: "coins-100",
    Price:     "0.99",
    Quantity:  1,
    Currency:  "USD",
})
```

Set `account.settings.google_ads: true` when this app is measured through
Google Ads. The adapter will then require `os_version_release`, `build_id`,
`locale`, and `device_model`; custom events and purchases also require
`app_version`. For iOS Meta AEM campaigns, set `meta_aem: true` and provide a
typed `TrackingStatus` value from 0 through 3 on custom events and purchases.
Google DMA consent fields are omitted when their pointers are nil and encoded
as `1` or `0` only when a signal is known.

## Impression-level ad revenue

Use direct ILRD only when the Tenjin account accepts this ingestion path and
when the mediation integration is not already forwarding the same impressions.
Sending both paths creates duplicate revenue.

```go
result, err := client.S2S().TrackAdImpression(ctx, tenjin.AdImpressionRequest{
    Context: tenjin.AdImpressionContext{
        Identity: tenjin.DeviceIdentity{
            AnalyticsInstallationID: installationID,
            AdvertisingID:           device.OAID,
        },
        AppVersion: appVersion,
        IPAddress:  requestIP,
        OSVersion:  device.AndroidAPILevel,
        SentAt:     &impressionTime,
    },
    Mediation:        tenjin.MediationTopOn,
    NetworkName:      "mintegral",
    Currency:         "USD",
    RevenueDecimal:   "0.00125",
    MediationCountry: "CN",
    Format:           tenjin.AdFormatRewarded,
    AdUnitID:         ad.UnitID,
    Placement:        ad.Placement,
    AuctionID:        ad.AuctionID,
})
```

The generic model covers MAX, ironSource/LevelPlay, AdMob, TopOn, CAS,
TradPlus, and custom mediation. It accepts either per-impression revenue or
CPM; if both are supplied, Tenjin prioritizes `revenue_decimal`. iOS without a
usable IDFA requires both IDFV and Analytics Installation ID.

## Delivery and errors

All operations use POST with `application/x-www-form-urlencoded`. Core open,
event, and purchase calls require HTTP 200 plus Tenjin's documented
`{"code":200}` acknowledgement. The ILRD guide documents only HTTP acceptance,
so ILRD requires HTTP 200 without assuming an undocumented response body.

Only `WithCallTimeout` is accepted. Tenjin does not document request IDs,
response field selection, or an idempotency/deduplication key, so those call
options are rejected and never reach the transport. Encoded form bodies are
limited to 1 MiB. Callers must persist a logical event identity and suppress
duplicates around ambiguous network failures. HTTP 400/413/422 failures are
permanent input errors, 401/403 require credential or permission action, and
429 plus 5xx responses are retryable. Free-form response bodies are discarded
because they may echo SDK keys, Bundle IDs, or device identifiers. Returned
request IDs and retry headers are bounded and credential-filtered.

Official references:

- S2S event and purchase contract: <https://tenjin.com/docs/server-to-server-s2s-setup/>
- No-SDK S2S workflow: <https://tenjin.com/docs/implementing-s2s-tracking-a-no-sdk-guide/>
- Generic ILRD API: <https://tenjin.com/docs/impression-level-revenue-data-api-s2s/>
- Callback macro semantics: <https://tenjin.com/docs/callback-macros/>
- Tenjin Campaign/Reporting/Data Export APIs: <https://api-docs.tenjin.com/>
- Official SDK repositories: <https://github.com/tenjin>

The public documentation responses returned HTTP 200 on 2026-08-26 with these
SHA-256 digests:

| Reference | SHA-256 |
|---|---|
| S2S setup | `B526A9D470CD1021000C2BC07D30F16B362D81A851B80A6228B3DA111CF3B2AD` |
| No-SDK guide | `3F43BB812213B966A25199E61CEAB1409D480D4FC3387A8390F8E8802C41F8D9` |
| Generic ILRD | `34026F331EBDD3E8316EE8D4A8E129888BD109463DE17EA1F86957D8DC7B901E` |
| Callback macros | `DD4327EFB349FD40D4FAEC5CA753A5F2BE7602AF59083F9FD73191577CB974EA` |
| API documentation entry | `5E4C548104D5E6A986182476CA4265F79E140ED432B2D5CCBCC36E2C22BD1F26` |
| GitHub organization | `B56B0423BE6E8AF17970D429F00D3B841DDC520826E010645724E728FEA71871` |

No credentialed production request was sent during verification.
