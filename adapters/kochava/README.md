# Kochava S2S Measurement adapter

`social-hub/adapters/kochava` sends typed install notifications, post-install
events, and iOS/tvOS IDFA updates to Kochava. It is registered as
`kochava/s2s-measurement` and exposes an `S2S()` workflow rather than the
organic social interfaces.

The wire contract was verified on 2026-08-25 against Kochava's current S2S
install, post-install event, no-device-UA, and Strict Authentication guides.
No mature, maintained Go SDK covers this endpoint, so the adapter implements
the official JSON contract directly. Reporting, IdentityLink, postbacks, and
network/publisher APIs remain separate product surfaces.

## Provisioning and configuration

Kochava S2S measurement is available only to paid accounts. Create one
social-hub account per Kochava App GUID and set `approval.account_type` to
`paid` only after Kochava has provisioned that app. Until then, capabilities
report `approval_required` and submissions fail locally without network I/O.

Strict Authentication is optional. When Kochava enables it, store the API Key
behind `access_token_ref` and the paired app secret behind `secret_ref`. Both
references must be present together. The adapter resolves them only when the
client is created, always targets `https://control.kochava.com`, rejects
redirects and cookie jars, escapes `/` as `\/`, signs the exact JSON bytes sent,
and places credentials only in the documented Kochava headers.

```yaml
version: 1
platforms:
  - adapter: kochava/s2s-measurement
    product: s2s-measurement
    accounts:
      - id: mobile-production
        app_id: koconversionsdemo777ea19bc63928c
        access_token_ref: env://KOCHAVA_API_KEY
        secret_ref: env://KOCHAVA_APP_SECRET
        approval:
          account_type: paid
```

Omit both credential references when Strict Authentication is not enabled for
the provisioned app. `app_id` is the Kochava App GUID, not an App Store ID or
package name.

## Install notifications

Every install requires a valid client IP and at least one device identifier.
Send the actual device User-Agent whenever available. If it is unavailable,
send `DeviceVersion` in `model-OS-version` form; the adapter retains the empty
`device_ua` key and sets HTTP `User-Agent: Unknown` as Kochava requires.

```go
import "social-hub/adapters/kochava"

result, err := client.S2S().TrackInstall(ctx, kochava.InstallRequest{
    KochavaDeviceID: installationID,
    Context: kochava.DeviceContext{
        DeviceIDs: kochava.DeviceIdentifiers{
            IDFA: device.IDFA,
            IDFV: device.IDFV,
        },
        OccurredAt:      &installedAt,
        OriginationIP:   requestIP,
        DeviceUserAgent: device.UserAgent,
        AppVersion:      appVersion,
        ATT: &kochava.AppTrackingTransparency{
            Authorized:        &attAuthorized,
            AuthorizationTime: &attTime,
            Detail:            kochava.ATTAuthorized,
        },
    },
})
```

The install model also supports Google DMA consent, legacy iAd Version 3.1
claims, current AdServices token/results, and install-referrer attribution. An
AdServices token can be forwarded by itself or with attribution results.

## Events and IDFA updates

Post-install events always include Kochava's required association keys,
including empty `kochava_device_id`, `device_ver`, and `device_ua` values when
allowed by the contract. Event data is a flat typed object with at most 16
parameters. Exact `Decimal` values avoid `float64` rounding. String lists can be
sent, but arrays are not considered for SKAdNetwork conversion processing.

```go
result, err := client.S2S().TrackEvent(ctx, kochava.EventRequest{
    KochavaDeviceID: installationID,
    Context: kochava.DeviceContext{
        DeviceIDs:       kochava.DeviceIdentifiers{ADID: device.AdvertisingID},
        OriginationIP:   requestIP,
        DeviceUserAgent: device.UserAgent,
    },
    Name:     "Purchase",
    Currency: "USD",
    Data: kochava.Properties{
        Strings: map[string]string{"content_id": "sku-123"},
        Numbers: map[string]kochava.Decimal{"price": "19.95"},
    },
})
```

After ATT makes an IDFA newly available on iOS or tvOS, associate it with an
existing Kochava Device ID:

```go
result, err := client.S2S().UpdateIDFA(ctx, kochava.UpdateIDFARequest{
    KochavaDeviceID: installationID,
    IDFA:            device.IDFA,
})
```

## Delivery and errors

Each request contains one install, event, or update and must remain under 2
MiB after JSON slash escaping. Kochava does not document an idempotency or
deduplication key, so `WithIdempotencyKey` is rejected. Callers must persist a
logical submission identity and suppress duplicates around ambiguous network
failures.
Of the shared call options, only `WithCallTimeout` is accepted; caller request
IDs and generic field selectors have no documented Kochava S2S meaning.

Any HTTP 2xx response is treated as transport acceptance. HTTP 400/413/422
failures are permanent input errors, 401/403 require credential or permission
action, and 429 plus 5xx responses are retryable. Response bodies are never
retained because they may echo app or device identifiers and authentication
inputs; errors expose only generic context, status, bounded request IDs, and a
bounded `Retry-After` value.

Official references:

- S2S overview: <https://support.kochava.com/articles/server-to-server-integration/388-server-to-server-integration-overview/>
- Install notifications: <https://support.kochava.com/articles/server-to-server-integration/179-install-notification-setup>
- Post-install events and IDFA updates: <https://support.kochava.com/articles/server-to-server-integration/185-post-install-event-setup>
- Requests without a device User-Agent: <https://support.kochava.com/articles/server-to-server-integration/18474-s2s-without-device-ua/>
- Strict Authentication: <https://support.kochava.com/articles/server-to-server-integration/15890-kochava-install-authentication-integration/>
- Standard event parameters: <https://support.kochava.com/articles/reference-information/2213-post-install-event-examples>
