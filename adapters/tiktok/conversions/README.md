# TikTok Events API 2.0 adapter

Package `social-hub/adapters/tiktok/conversions` submits typed server events to
one configured TikTok Pixel, TikTok App, Offline Event Set, or CRM Event Set.
It uses the current Events API 2.0 route under Business API v1.3 and is separate
from both organic TikTok APIs and paid-media Campaign management.

## Configuration

Import the package for registration:

```go
import conversions "social-hub/adapters/tiktok/conversions"
```

Configure one social-hub account per event source. This prevents an event batch
from selecting or overriding another Pixel, App, or Event Set at call time.

```yaml
adapter: tiktok/events-api-v2
product: events-api
accounts:
  - id: web-storefront
    access_token_ref: secret://tiktok-events/web-storefront/access-token
    approval:
      scopes:
        - "Measurement > Report Conversion Event"
    settings:
      event_source: web
      event_source_id: C123PIXEL456
```

`event_source` is one of `web`, `app`, `offline`, or `crm`.
`access_token_ref` is resolved only at runtime and sent in the `Access-Token`
header. The API origin is fixed at
`https://business-api.tiktok.com/open_api/v1.3`, HTTP redirects are rejected,
and caller cookie jars are ignored so the credential cannot be forwarded to a
different origin.

An Events Manager token is limited to event sources created under the same ad
account. A Developer App token can cover event sources across authorized ad
accounts and requires the `Measurement > Report Conversion Event` permission.
For an Events Manager token, `approval.scopes` may be omitted; social-hub then
reports approval as unknown and lets TikTok enforce event-source ownership.

## Submit events

```go
client := common.(*conversions.Client)
result, err := client.Events().SubmitEvents(ctx, conversions.SubmitEventsRequest{
    Events: []conversions.ConversionEvent{{
        Event:     conversions.EventPurchase,
        EventTime: time.Now().Unix(),
        EventID:   order.EventID,
        User: &conversions.User{
            Emails:    []string{customer.Email},
            Phones:    []string{customer.E164Phone},
            IP:        requestIP,
            UserAgent: requestUserAgent,
        },
        Page: &conversions.Page{URL: checkoutURL},
        Properties: &conversions.Properties{
            ContentIDs: []string{"sku-1"},
            ContentType: conversions.ContentTypeProduct,
            Currency: "USD",
            Value: "46.00",
            OrderID: order.ID,
        },
    }},
})
```

`Decimal` values are validated non-negative base-10 strings and encoded as JSON
numbers without conversion through `float64`. A batch contains 1-1,000 events.
Set `TestEventCode` only while using the code copied from Events Manager's Test
Events view, and remove it for production traffic.

## Source-specific contract

| Source | Required object | Source-specific fields and constraints |
|---|---|---|
| Web | `page.url` | `ttclid`, `ttp`, names, address fields, `external_id`, `ad`, and LDU are supported |
| App | `app.app_id` | App Events API reporting is allowlist-only; IDFA, IDFV, GAID, ATT status, `ad`, and LDU are supported |
| Offline | None | Only Web/Offline Standard Event names are accepted; Web/App-only matching and attribution fields are rejected |
| CRM | `lead.lead_id` | Email, phone, and `external_id` matching are supported; Web/App-only context is rejected |

The Parameters page says CRM accepts only Custom Events, while the current
Supported Events page says CRM supports the same Standard Events as Web and
shows standard CRM examples. The adapter does not reject CRM Standard Events;
TikTok's live contract remains authoritative until the documentation converges.

For Web and App, `limited_data_use=true` requires a non-hashed public `user.ip`,
as required by TikTok's LDU documentation. It is rejected for Offline and CRM.

## PII and response privacy

The adapter accepts plaintext or exact lowercase SHA-256 values for email,
phone, external ID, first name, last name, and ZIP. Plaintext is normalized and
hashed in a temporary request copy:

- Email: trim and lowercase.
- Phone: caller-supplied E.164, including the leading `+`.
- Names: trim, lowercase, and remove Unicode punctuation.
- ZIP: lowercase and remove spaces/hyphens; US values use the first five digits.
- External ID: trim, then hash.

Uppercase SHA-256 and MD5-looking values are rejected instead of being hashed a
second time. City, state, country, public IP, User-Agent, cookie/click IDs, and
mobile advertising identifiers follow TikTok's documented cleartext or raw-ID
formats. Inputs are never mutated.

TikTok validation messages can include the first invalid event index and field
value. The adapter therefore discards all free-text platform messages and only
returns the numeric platform code, HTTP status, request ID, retryability, and
bounded `Retry-After`. A successful result contains no platform message or
event payload.

## Versions, quotas, and sources

The endpoint is pinned to:

```text
POST /open_api/v1.3/event/track/
```

The official endpoint limit is 1,000 QPS and a request can contain at most
1,000 events. Return code `40100` is classified as retryable rate limiting;
assigned account quotas and live responses remain authoritative.

The official [Events API 2.0 endpoint](https://business-api.tiktok.com/portal/docs?id=1771101303285761),
[authentication guide](https://business-api.tiktok.com/portal/docs?id=1771101130925058),
[parameter contract](https://business-api.tiktok.com/portal/docs?id=1771101151059969),
[supported events](https://business-api.tiktok.com/portal/docs?id=1771101186666498),
[test-event guide](https://business-api.tiktok.com/portal/docs?id=1771100984456193), and
[Limited Data Use guide](https://business-api.tiktok.com/portal/docs?id=1771101204435970)
were verified on 2026-08-10. TikTok's current official Business SDK still
models the older `/pixel/track/` and `/pixel/batch/` endpoints, so it was used
only as an authentication and repository-convention reference; this adapter's
wire contract follows the current official Events API 2.0 documentation.
