# Partnerize Partners API adapter

Registration name: `partnerize/partners-api-v1.4.99`

This package implements a bounded partner-side affiliate surface from the
Partnerize Partners API. Version `1.4.99` is the current official OpenAPI
document version, not one uniform URL version: the published contract combines
global v1-style paths, `/v2` and `/v3` products. This adapter currently uses
the partner v1 paths, v2 Tracking Links, and the partner reporting path; it does
not claim the entire document or v3 Analytics.

## Implemented workflows

| social-hub method | Partnerize operation | Notes |
|---|---|---|
| `Partnerize().GetPartner` | `GET /user/publisher/{publisher_id}` | profile, websites, and databases |
| `Partnerize().ListCampaigns` | `GET /user/publisher/{publisher_id}/campaign/{status}` | approved (`a`), pending (`p`), or rejected (`r`) relationships |
| `Partnerize().ListCreatives` | `GET /user/publisher/{publisher_id}/campaign/{campaign_id}/creative` | optional active, tags, and creative-type filters |
| `Partnerize().CreateTrackingLink` | `POST /v2/publishers/{publisher_id}/links` | campaign, destination, parameters, description, and active state |
| `Partnerize().ListConversions` | `GET /reporting/report_publisher/publisher/{publisher_id}/conversion.json` | date window, currency, pivot, status, offset, and optional payment data |

Account mutation, campaign applications, invitations, vouchers, websites,
databases, payments, payable reports, v3 participations, campaign terms,
Partnerize Tags, exports, and v3 Analytics are outside the initial surface.

## Authentication and configuration

Partnerize uses HTTP Basic authentication for these APIs:

- username: User Application Key;
- password: User API Key.

Both values are available in Partnerize Console under Account settings. They
are static API credentials, not OAuth tokens; Partnerize publishes no refresh
flow for them. The adapter maps the User Application Key to `client_id` and
resolves the User API Key from `secret_ref`.

```yaml
version: 1
platforms:
  - adapter: partnerize/partners-api-v1.4.99
    product: partners-api
    accounts:
      - id: primary-partner
        client_id: your-user-application-key
        secret_ref: env://PARTNERIZE_USER_API_KEY
        approval:
          account_type: partner
        settings:
          publisher_id: 1l1007802
```

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/partnerize"
```

Credentials are sent only through the HTTP Basic `Authorization` header.
Redirects are rejected so the header cannot move to another origin. The
adapter-level `base_url` override is intended only for a controlled
contract-verification gateway.

## Contract and data notes

- Provider identifiers and decimal-like numeric fields use `ExactValue` where
  the API may return numbers, strings, or nulls. Typed entities and response
  envelopes retain their complete `Raw` JSON.
- The official conversion example wraps each item as `conversion_data`, while
  the OpenAPI response schema declares direct conversion objects. The decoder
  accepts both official representations and exposes one `Conversion` value.
- In the same conversion contract, the example nests
  `publisher_commission` inside `conversion_value`, while the generated schema
  places it at conversion level. Both positions are typed and the original
  response remains available through `Raw`.
- Conversion `meta_data` and voucher-code structures are retained as
  `json.RawMessage` because their official schemas do not define one stable
  value shape.
- `CreateTrackingLink` does not accept `WithIdempotencyKey`: the official
  operation does not define an idempotency contract. None of the implemented
  operations defines dynamic field selection.
- All five workflows require the documented HTTP `200` JSON response. Partner,
  campaign, creative, tracking-link, and conversion identifiers are validated;
  returned resources must belong to the configured publisher or requested
  campaign, and duplicate campaign, creative, or conversion IDs are rejected.
  Post-decode validation errors return the decoded value with its complete
  `Raw` response and bounded response metadata intact.
- Tracking-link creation has no documented idempotency mechanism. Transport
  failures, HTTP 408/5xx responses, unexpected successful statuses, and invalid
  successful responses wrap `ErrOutcomeUnknown` as a `conflict` / `user_action`
  error. Reconcile the partner account before retrying. Definitive provider 4xx
  errors and local validation failures retain their normal classification.
- The conversion report's `start_date` is required. Dates are sent as RFC 3339
  values, `currency[]` and `statuses[]` retain the official parameter names,
  and multipivot keys are limited to the documented campaign, product, and
  publisher-reference pivots.
- The current document is internally inconsistent about conversion paging:
  the operation parameter table still declares `offset`, while its Cursor
  Pagination section assigns this endpoint `cursor_id` plus `limit` (maximum
  300). The adapter supports both forms, rejects mixing them, and exposes the
  returned cursor and hypermedia links. A subsequent cursor request must
  specify the desired limit.

## Rate limits and access

Partnerize may change limits dynamically. Successful and failed responses can
expose `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, and
`X-RateLimit-Retry-After`; the adapter preserves these values in response
metadata or `APIError`. HTTP 429 maps to `socialhub.CodeRateLimited` and a
retryable class.

There is no safe fixed global QPS assumption. Partnerize v3 Analytics has a
separate allowance based on endpoint, requested date span, and operation cost;
that allowance must be modeled independently when Analytics is added. Access
to campaigns, creatives, links, conversion fields, and payment data remains
subject to the partner account's permissions and campaign relationships.

Official documentation reviewed 2026-08-26:

- <https://api-docs.partnerize.com/partner/>

The reviewed HTML snapshot has SHA-256
`055AAEDFAA71772EE7C64A7A13F26A7A34407286FC74BE0385DC6ECF52D0FAC5` and
`Last-Modified: Wed, 22 Apr 2026 09:16:22 GMT`. Its embedded OpenAPI document
reports OpenAPI `3.0.1`, version `1.4.99`, and server
`https://api.partnerize.com`.
