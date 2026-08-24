# Branch Events API v2 adapter

`adapters/branch` implements Branch's Events API v2 for real-time Standard and
Custom mobile attribution events. It is registered as
`branch/events-api-v2` and exposes a typed `Events()` workflow rather than the
organic social interfaces.

The contract was verified on 2026-08-25 against Branch's current Events API
overview (updated 2026-08-20) and the Standard/Custom endpoint OpenAPI 1.0.1
definitions.

## Authentication and account binding

Create one social-hub account per Branch app. Store the Branch Live or Test Key
behind `access_token_ref`; the adapter resolves `key_live_...` and
`key_test_...` values at runtime. Branch requires the key in the JSON
`branch_key` property, so the adapter deliberately sends no `Authorization`
header. Requests always target `https://api2.branch.io`; endpoint overrides,
redirects, and cookie jars are disabled to prevent the body credential and
attribution identifiers from crossing origins.

```yaml
version: 1
platforms:
  - adapter: branch/events-api-v2
    product: events-api
    accounts:
      - id: mobile-production
        access_token_ref: env://BRANCH_KEY
        settings:
          ip_override_enabled: false
```

`ip_override_enabled` is an explicit record that Branch Support has allowlisted
the app for `X-IP-Override`. Keep it `false` until Branch confirms the
entitlement. An override must be a public IPv4 address and exactly match
`user_data.ip`; otherwise the call fails before network I/O.

## Typed event submission

```go
package main

import (
    "context"

    "social-hub/adapters/branch"
)

func sendPurchase(ctx context.Context, client *branch.Client) error {
    result, err := client.Events().TrackStandardEvent(ctx, branch.StandardEventRequest{
        Name: branch.EventPurchase,
        UserData: branch.UserData{
            OS:                  branch.OSAndroid,
            OSVersion:           "15",
            AAID:                "abcdabcd-0123-0123-00f0-000000000000",
            DeveloperIdentity:   "customer-123",
        },
        EventData: &branch.EventData{
            TransactionID: "order-123",
            Revenue:       branch.Decimal("19.95"),
            Currency:      "USD",
        },
        ContentItems: []branch.ContentItem{{
            Schema:              branch.SchemaCommerceProduct,
            CanonicalIdentifier: "sku-123",
            ProductName:         "Running shoes",
            Price:               branch.Decimal("19.95"),
            Quantity:            branch.Decimal("1"),
        }},
        CustomData: branch.Properties{
            Strings: map[string]string{"warehouse_id": "wh-1"},
        },
    })
    if err != nil {
        return err
    }
    _ = result
    return nil
}
```

The adapter accepts all 24 current Standard Event names across Commerce,
Content, and Lifecycle categories. `content_items` is restricted to Commerce
and Content events. Custom event names cannot reuse a Standard Event name or
Branch's reserved `custom event` label.

`Decimal` preserves exact JSON numbers without `float64` rounding. `Properties`
provides flat string, exact-number, and boolean values for `custom_data`,
`meta_data`, and `$custom_fields`, and rejects duplicate keys across value
types. Standard events support `custom_data`; Custom events additionally
support Branch's exact `meta_data` wire property.

Every event must include at least one documented identity combination:

- `developer_identity`;
- `browser_fingerprint_id`;
- `os=iOS` with `idfa` or `idfv`; or
- `os=Android` with `aaid` or `android_id`.

`os_version` is strongly recommended for paid traffic and required for Meta
campaigns on iOS. `anon_id` is additionally required for Meta Aggregated Event
Measurement on iOS. Those campaign-specific facts are not inferable from an
event request, so callers must enforce them in their campaign pipeline. When
`dma_eea=true`, both `dma_ad_personalization` and `dma_ad_user_data` are
required locally.

Identity, advertising, IP, consent, event, and content fields are sent only
when the caller supplies them. The adapter performs no device enrichment and
does not log request or response bodies.

## Delivery and errors

Events must be sent in real time; Branch does not backfill historical events.
Only HTTP 200 is accepted. Successful responses expose the documented SKAN
fields `ascending_only`, `coarse_key`, `locked`, and
`update_conversion_value`.

Branch documents HTTP 400 as authentication failure and HTTP 429 as rate
limiting, but the current public Events API documentation does not publish a
numeric QPS or window. Treat limits as app- and contract-specific. The adapter
maps 429 and 5xx responses to retryable errors, preserves a valid
`Retry-After` header, and discards all error response bodies because they can
echo the body credential or attribution identifiers. Branch does not document
an idempotency-key contract for Events API v2, so `WithIdempotencyKey` is
rejected rather than creating a false delivery guarantee. Callers should
durably deduplicate events before retrying an ambiguous timeout or 5xx
response.

Official references:

- Events API overview: <https://help.branch.io/apidocs/events-api>
- Standard Events endpoint: <https://help.branch.io/apidocs/logstandardevents>
- Custom Events endpoint: <https://help.branch.io/apidocs/logcustomevents>
- Event tracking guide: <https://help.branch.io/developer-hub/docs/track-branch-events>
- Event ontology: <https://help.branch.io/developer-hub/docs/branch-event-ontology>
