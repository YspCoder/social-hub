# Microsoft Advertising Conversions API v1 adapter

Package `social-hub/adapters/microsoftads/conversions` submits typed server-side
events to one configured Universal Event Tracking (UET) tag. It is separate
from `social-hub/adapters/microsoftads`, which manages Microsoft Advertising
campaigns and reports through API v13.

## Configuration

Import the package for registration and typed event access:

```go
import conversions "social-hub/adapters/microsoftads/conversions"
```

Configure one social-hub account per UET tag. A call cannot override the tag ID
or redirect events to a different tag.

```yaml
adapter: microsoftads/conversions-api-v1
product: conversions-api
accounts:
  - id: storefront-purchases
    access_token_ref: secret://microsoft-uet/storefront/capi-token
    settings:
      uet_tag_id: "123456789"
```

`access_token_ref` must resolve to the tag-specific Conversions API token. This
is not the Microsoft identity OAuth access token used by Advertising API v13,
and the CAPI request does not carry a developer token, Customer ID, or Account
ID. The token is sent only as `Authorization: Bearer <token>`. The API origin
is fixed to `https://capi.uet.microsoft.com`; HTTP redirects are rejected and
the supplied client's cookie jar is disabled so credentials and ambient
cookies cannot cross or attach to the origin.

The UET tag and its conversion goals must already exist. An owner can retrieve
or generate the tag token in the Microsoft Advertising UI or through the v13
Campaign Management `UetTagAuthKey/Query` operation.

## Submit events

```go
result, err := client.Events().SubmitEvents(ctx, conversions.SubmitEventsRequest{
    DataProvider: "social-hub",
    Events: []conversions.ConversionEvent{
        {
            EventType:      conversions.EventTypePageLoad,
            EventTime:      time.Now().Unix(),
            EventSourceURL: "https://shop.example/checkout",
            PageLoadID:     pageLoadID,
            UserData: conversions.UserData{
                AnonymousID: visitorID,
            },
        },
        {
            EventType:  conversions.EventTypeCustom,
            EventTime:  time.Now().Unix(),
            EventID:    order.EventID,
            EventName:  "purchase",
            PageLoadID: pageLoadID,
            UserData: conversions.UserData{
                MicrosoftClickID: msclkid,
                Email:            customer.Email,
                Phone:            customer.E164Phone,
                AnonymousID:      visitorID,
                ClientUserAgent:  userAgent,
                ClientIPAddress:  clientIP,
            },
            CustomData: &conversions.CustomData{
                TransactionID: order.ID,
                Value:         "125.40",
                Currency:      "USD",
                PageType:      conversions.PageTypePurchase,
            },
        },
    },
})
```

`eventTime` is Unix seconds and must fall within the previous seven days. Page
Load events require an absolute HTTP(S) `eventSourceUrl`. Every event requires
at least one of `anonymousId`, `externalId`, `em`, `ph`, `msclkid`, `idfa`, or
`gaid`. Use a shared stable `eventId`, `eventName`, UET tag, and `pageLoadId`
when corresponding UET JavaScript and CAPI events must be deduplicated.

The adapter validates an entire batch before sending it and accepts 1-1,000
events. It intentionally leaves `continueOnValidationError` at its default
`false`, preserving atomic processing: a locally invalid event prevents the
request, and a Microsoft validation error rejects the remote batch. Split
larger sets and preserve event IDs when retrying transient failures. The SDK
marks a transient submission error retryable only when every event in the batch
has a stable `eventId`; it never retries automatically.

## Matching, ecommerce, and privacy

The typed model covers Page Load and Custom events, ecommerce items, dynamic
remarketing page types, hotel fields, mobile advertising IDs, and explicit
`G`/`D` ad-storage consent. Decimal strings are validated and encoded as exact
JSON numbers without a `float64` conversion.

Email and phone can be supplied as plaintext or an exact lowercase SHA-256
digest. Plaintext is normalized and hashed in a temporary request copy:

- Email: trim, lowercase, remove dots and a `+alias` from the local part, then
  SHA-256 hash.
- Phone: require caller-normalized E.164 including the leading `+`, then
  SHA-256 hash.

Uppercase SHA-256 and MD5-looking inputs are rejected to prevent silent double
hashing. Inputs are never mutated. Platform response messages can echo rejected
identifiers, so the adapter discards all free text and exposes only a safe
platform code, HTTP status, request ID, retryability, and bounded `Retry-After`.

HTTP 200 responses can still contain warnings for optional fields Microsoft
removed. `SubmitResult.HasWarnings` reports that partial data loss without
exposing the attempted value. Because partial-mode submission is disabled, when
Microsoft includes `eventsReceived` it must equal the submitted batch size or
the response is treated as a platform contract error.

## Protocol and source

The endpoint is pinned to:

```text
POST https://capi.uet.microsoft.com/v1/{tagId}/events
```

The public documentation does not state a fixed QPS quota. Live HTTP 429 and
`Retry-After` responses are authoritative and are classified as retryable.
The official contract does not define caller request IDs, an idempotency-key
header, or response field selection, so those generic `CallOption`s are
rejected. `WithCallTimeout` remains available as a local deadline.

The official [Microsoft Advertising Conversions API integration guide](https://learn.microsoft.com/en-us/advertising/guides/uet-conversion-api-integration?view=bingads-13)
was verified on 2026-08-25. The current contract documents the endpoint,
payload, hashing rules, batch behavior, validation warnings, and troubleshooting
responses in one continuously updated guide.
