# Quora Ads Conversion API v0

Package `social-hub/adapters/quoraconversions` submits Website, App Install,
and Offline conversion events to Quora. Importing the package registers
`quora/conversion-api-v0`.

This adapter implements only the two operations in Quora's current public
OpenAPI 0.1.0 contract:

- `POST /ads/v0/conversion` for one event;
- `POST /ads/v0/conversions` for an independently processed batch of 1-1000
  events.

It does not implement accounts, Campaigns, Ad Sets, Ads, or reporting. The old
Ads Management documentation and `/ads/v0/accounts` route are not a current
public contract even though Conversion API still uses the `/ads/v0` base path.

The API origin is fixed to `https://api.quora.com/ads/v0`. Redirects are
rejected and the supplied HTTP client's cookie jar is removed so the bearer
token and ambient cookies cannot cross origins.

## Configuration

Generate a token in the **Conversion API** tab of Quora Ads Manager. Quora
documents the token as non-expiring and bound to one ad account.

```yaml
version: 1
platforms:
  - adapter: quora/conversion-api-v0
    product: conversion-api
    accounts:
      - id: quora-main
        access_token_ref: secret://quora/main/conversion-token
        settings:
          ad_account_id: 527745581653587
```

Adapter-level endpoint overrides, OAuth client settings, token stores,
webhooks, approval scopes, and unrelated account credentials are rejected.

## Submission

```go
import (
    "context"
    "time"

    "social-hub/adapters/quoraconversions"
    "social-hub/pkg/socialhub"
)

func send(ctx context.Context, adapter socialhub.Adapter) error {
    common, err := adapter.Client(ctx, "quora-main")
    if err != nil {
        return err
    }
    client := common.(*quoraconversions.Client)

    email, err := quoraconversions.HashEmail("John.Doe+checkout@gmail.com")
    if err != nil {
        return err
    }
    timestamp := quoraconversions.Microseconds(time.Now())
    result, err := client.Conversions().SubmitEvent(ctx, quoraconversions.SubmitEventRequest{
        Debug: true,
        Event: quoraconversions.ConversionEvent{
            User: quoraconversions.User{Email: email},
            Device: quoraconversions.Device{UserAgent: "Mozilla/5.0"},
            Conversion: quoraconversions.Conversion{
                EventName: quoraconversions.EventPurchase,
                Timestamp: &timestamp,
                ClickID:   "0|212106239366182|0",
                EventID:   "order-123",
                Value:     quoraconversions.Decimal("5.99"),
            },
        },
    })
    if err != nil {
        return err
    }
    for _, warning := range result.Warnings {
        // Treat warnings as integration defects, not retryable failures.
        _ = warning.Code
    }
    return nil
}
```

A caller may send plaintext email. Using `HashEmail` applies Quora's documented
normalization before hashing and keeps that plaintext out of the request.

## Contract boundaries

- `click_id` is the most recent landing-page `qclid`, either URL encoded or
  decoded. It is optional for API acceptance but required for attribution.
- `event_id` is optional but strongly recommended for deduplication against
  Quora Pixel events.
- Quora Pixel stores the latest `qclid` in the first-party `quora_qclid` cookie
  for 90 days.
- `timestamp` is Unix microseconds. The public contract does not state an event
  age window, so the adapter does not invent one.
- With `debug: true`, warnings include missing/invalid click IDs and missing
  event IDs. Debug changes response detail only, not event processing.
- A non-debug single submission accepts only HTTP `200` with the official
  `text/plain` representation. Debug single submissions and all batch
  submissions require HTTP `200` JSON responses with internally consistent
  statuses, indices, counts, and warning codes.
- Batch HTTP 200 means the request was accepted; inspect `EventsErrored` and
  each `EventResult` because individual items may fail independently.
- The account limit is 1,000 events per minute. Each batch item consumes one
  event, and a batch that would exceed the limit is rejected as a whole with
  HTTP 429.
- HTTP error bodies have no published schema and may echo conversion data, so
  the adapter maps the status and request ID without exposing the raw body.
- Successful per-item error and warning messages are also free text and may
  echo customer data. They are validated and discarded; callers receive the
  documented warning enum, a bounded identifier-like error code, and
  `HasErrorMessage` without the message itself.

## Request and retry safety

Only `socialhub.WithCallTimeout` is accepted per submission. Caller request
IDs, idempotency-key headers, and generic field selection are not in Quora's
contract and are rejected. Encoded request bodies are limited to 8 MiB, and
individual text fields are limited to 4,096 Unicode characters as a local
resource boundary; Quora's OpenAPI does not publish smaller field limits.

Transport failures, timeouts, and HTTP 5xx responses can occur after Quora has
accepted an event. They remain retryable only when every affected event has a
stable `event_id`; otherwise the adapter returns a user-action classification
so the caller reconciles before resubmitting. HTTP 429 remains retryable because
Quora documents that a rate-limited batch is rejected as a whole.

The token, request bodies, HTTP error bodies, and free-form success diagnostics
are never included in social-hub errors. Response request IDs and retry timing
are bounded before exposure.

## Official sources

- <https://www.quora.com/ads/conversion_api_doc>
- <https://www.quora.com/openapicapi1684b8ed77ea8ef481af8c65e191b49a2522f5dfcbd0fa18f7df8826.yaml>
- <https://quoraadsupport.zendesk.com/hc/en-us/articles/23065751885069-Conversion-API-Overview>
- <https://www.quora.com/about/conversion_api_tos>

The contract was reviewed on 2026-08-26. The official documentation entry
returned HTTP 200 with SHA-256
`3612D912A02E4ED61431896C28B6DF2490031B8D5092B3EA57576099381BD835`
and referenced the exact OpenAPI source path listed above. The normalized YAML
review capture had SHA-256
`F40BF89EDE2B8A0F50C298334C9CE65A258B266D7CA58FFB1281D5E2BFF8D18E`.
No credentialed request was sent to a production Quora Ads account.
