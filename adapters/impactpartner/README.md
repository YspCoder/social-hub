# impact.com Partner API v16 adapter

Registration name: `impact/partner-api-v16`

This adapter implements a bounded commerce and attribution surface from the
impact.com Partner API. It is for media partners that have an impact.com
AccountSID and AuthToken; it does not expose the brand, agency, or organic
social products.

## Implemented workflows

| social-hub method | impact.com operation | Notes |
|---|---|---|
| `Partner().ListPrograms` | `GET /Mediapartners/{AccountSID}/Campaigns` | joined programs, optionally filtered by active or expired insertion order |
| `Partner().SearchCatalogItems` | `GET /Mediapartners/{AccountSID}/Catalogs/ItemSearch` | cross-catalog keyword search with page/page-size metadata |
| `Partner().GetCatalogItem` | `GET /Mediapartners/{AccountSID}/Catalogs/{CatalogId}/Items/{Id}` | typed item detail with the complete provider object in `Raw` |
| `Partner().CreateTrackingLink` | `POST /Mediapartners/{AccountSID}/Programs/{ProgramId}/TrackingLinks` | regular, vanity, ad, deep-link, media-property, sub-ID, and shared-ID parameters |
| `Partner().ListActions` | `GET /Mediapartners/{AccountSID}/Actions` | attributed actions and commissions with state, event/update/locking date, and pagination filters |

The initial surface intentionally excludes campaign applications, catalog feed
downloads, ads and promotions, action items/updates, invoices, lifecycle
postbacks, and brand-side workflows. Those APIs have separate authorization,
lifecycle, or streaming contracts and should be added as separate typed
workflows.

## Configuration

impact.com authenticates Partner API calls with HTTP Basic authentication:
`AccountSID` is the username and a UI-generated AuthToken is the password. The
AuthToken is static and scope-bound. There is no authorization-code or refresh
flow, so rotation is handled by replacing the referenced secret.

```yaml
version: 1
platforms:
  - adapter: impact/partner-api-v16
    product: partner-api
    accounts:
      - id: primary-partner
        access_token_ref: env://IMPACT_AUTH_TOKEN
        approval:
          account_type: approved-impact-media-partner
        settings:
          account_sid: IR1234567
```

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/impactpartner"
```

Every request sends `IR-Version: 16` and `Accept: application/json`. The
optional adapter-level `base_url` override is intended only for a controlled
contract-verification gateway. Redirects are rejected so Basic credentials
cannot move to another origin.

## Request and data guarantees

- Tracking-link creation is a `POST` with no body. `Type`, `CustomPath`,
  `AdId`, `DeepLink`, `MediaPartnerPropertyId`, `subId1`-`subId3`, and
  `sharedId` are encoded only as query parameters. The v16 description is
  contradictory about whether a vanity `CustomPath` is mandatory, so the SDK
  leaves provider-side defaulting and validation intact.
- `StartDate`/`EndDate` and `LockingDateStart`/`LockingDateEnd` must be supplied
  as pairs and span no more than 45 days. `StartDate` cannot be more than three
  years old. The list-actions OpenAPI permits a one-sided `ActionDate` filter;
  when both bounds are supplied, the SDK validates ordering and the 45-day
  window. Without update-date bounds, impact.com defaults to the latest seven
  days.
- Search and action pagination are 1-based when supplied; catalog search uses
  the provider's default page size of 100 when omitted. Numeric pagination
  values are retained as `ExactValue` so a provider string or JSON number is
  not coerced through `float64`. The response decoder accepts the documented
  `@firstpageuri`, `@previouspageuri`, and `@lastpageuri` fields as well as the
  provider documentation's `...pageruri` misspellings.
- Programs, catalog items, actions, list envelopes, and tracking links retain
  their complete successful provider object in `Raw`. Catalog prices and
  custom numeric/money fields remain provider strings, matching v16.
- All five workflows require the documented HTTP `200` JSON response. Program,
  catalog-search, and action collections must be present; their resource IDs
  must be non-empty and duplicate resources are rejected. Catalog detail IDs
  must match the request. Validation errors return the decoded value with its
  `Raw` object and response metadata intact.
- A successful tracking-link response must contain a valid HTTP(S)
  `TrackingURL` without credentials. impact.com documents no idempotency key
  for this operation, so the SDK rejects that call option. Transport failures,
  HTTP 408/5xx responses, unexpected successful statuses, and malformed or
  invalid successful responses wrap `ErrOutcomeUnknown` as a `conflict` /
  `user_action` error. Reconcile account state before retrying; definitive 4xx
  provider errors retain their normal classification.
- Both documented error shapes are decoded. The resulting `APIError` wraps a
  `socialhub.Error`, retains field validation details or tracking-service
  details, redacts credential-like values, and exposes current hourly quota
  headers.

## Access and rate limits

An impact.com partner account, AccountSID, scoped AuthToken, joined program,
and resource-specific access are required. Catalog visibility and link
generation depend on the relevant program relationship. Vanity links are
limited to 5,000 per account; provider-side resource or entitlement failures
remain typed permission errors.

The published hourly limits differ by endpoint family:

| Endpoint family | Published limit |
|---|---:|
| Programs and Tracking Links | 1,000 requests/hour |
| Catalog search and item detail | 3,000 requests/hour |
| Actions | 500 requests/hour |

The adapter exposes `X-RateLimit-Limit-hour`,
`X-RateLimit-Remaining-hour`, and `RateLimit-Reset`. HTTP 429 maps to a
retryable rate-limit error and `Retry-After` is parsed and capped at 24 hours
when present. A limiter should be keyed by credential/account and endpoint
family rather than applying one global impact.com quota.

Official v16 contracts reviewed 2026-08-26. OpenAPI snapshot hashes:

- [Campaigns v16](https://openapi.gitbook.com/o/0mbgBjArWXoMupyWdFkH/spec/partner-campaigns-v16.yaml) — SHA-256 `D23ADD893318451A456987D1CFD680342D74EA4AEA6F74D46D03A621DB925403`
- [Catalogs v16](https://openapi.gitbook.com/o/0mbgBjArWXoMupyWdFkH/spec/partner-catalogs-v16.yaml) — SHA-256 `649DBC9D56EBBF3087E1CD8C5BAAE5E7F244FCCC51C021A4A097016272F9BC51`
- [Tracking Links v16](https://openapi.gitbook.com/o/0mbgBjArWXoMupyWdFkH/spec/partner-trackinglinks-v16.yaml) — SHA-256 `82D359A73B425F922546573EEE56602BA0A24BA2BB398E67651F70DD953FED78`
- [Actions v16](https://openapi.gitbook.com/o/0mbgBjArWXoMupyWdFkH/spec/partner-actions-v16.yaml) — SHA-256 `197F89E6DAEBC2E349BC7B9D1F879332D5EAA658D13CF509EB3AE6767CF538AB`

Official documentation reviewed with those snapshots:

- [Partner API reference](https://integrations.impact.com/partner-api-reference/readme.md)
- [Authentication](https://integrations.impact.com/partner-api-reference/readme/authentication.md)
- [Requests](https://integrations.impact.com/partner-api-reference/readme/requests.md)
- [Responses](https://integrations.impact.com/partner-api-reference/readme/responses.md)
- [Status codes and errors](https://integrations.impact.com/partner-api-reference/readme/status-codes-and-errors.md)
- [Pagination](https://integrations.impact.com/partner-api-reference/readme/pagination.md)
- [Rate limits](https://integrations.impact.com/partner-api-reference/readme/rate-limits.md)
- [Versioning](https://integrations.impact.com/partner-api-reference/readme/versioning.md)
- [Programs](https://integrations.impact.com/partner-api-reference/reference/programs/programs.md)
- [Catalogs](https://integrations.impact.com/partner-api-reference/reference/catalogs/catalogs.md)
- [Tracking links](https://integrations.impact.com/partner-api-reference/reference/tracking-links/tracking-links.md)
- [Actions](https://integrations.impact.com/partner-api-reference/reference/actions/actions.md)
- [Action models](https://integrations.impact.com/partner-api-reference/reference/actions/models.md)
