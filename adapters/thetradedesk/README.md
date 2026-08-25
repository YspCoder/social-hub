# The Trade Desk Platform API v3 adapter

Registration name: `thetradedesk/platform-api-v3`

This package implements advertiser-scoped REST workflows from the current The
Trade Desk Platform API v3 contract. It is a paid-media adapter and does not
implement social-hub's organic publishing interfaces.

Official references reviewed on 2026-08-25:

- [Platform API overview](https://partner.thetradedesk.com/v3/portal/api/overview)
- [Account setup](https://partner.thetradedesk.com/v3/portal/api/doc/ApiPlatformGetStarted)
- [Making REST API calls](https://partner.thetradedesk.com/v3/portal/api/doc/ApiUsageGuidelines)
- [API token authentication](https://partner.thetradedesk.com/v3/portal/api/doc/PlatformAuthentication)
- [Short-lived API tokens](https://partner.thetradedesk.com/v3/portal/api/doc/AuthenticationShortLive)
- [Partner Sandbox](https://partner.thetradedesk.com/v3/portal/api/doc/PartnerSandbox)
- [Strict mode](https://partner.thetradedesk.com/v3/portal/api/doc/StrictMode)
- [REST rate limits](https://partner.thetradedesk.com/v3/portal/api/doc/RateLimits)
- [REST return codes](https://partner.thetradedesk.com/v3/portal/api/doc/ReturnCodes)
- [Create campaigns](https://partner.thetradedesk.com/v3/portal/api/doc/CampaignCreate)
- [Current REST OpenAPI document](https://partner.thetradedesk.com/v3/portalapi/openapi?portalCategory=api)

## Commercial and lifecycle boundary

Platform API access is not self-service. Onboarding requires an account,
signed contracts, API credentials, and service-specific permissions arranged
through an Account Manager or Technical Account Manager. The official account
setup guide says services typically require a setup fee, and some services
require additional paperwork, permissions, or fees. Impression budgeting is a
limited-availability feature that requires separate permission.

Set `approval.account_type: platform-api` only after that external grant is in
place. Without it, the adapter reports paid-media capabilities as approval
required. A token does not expand the entities or services granted to its API
user; HTTP `403` remains a permission or commercial-access failure and links to
the account setup guide.

The current official OpenAPI document marks the implemented Advertiser and
Campaign REST operations as legacy, recommends their GraphQL equivalents, and
sets `x-legacyDate` to `2027-01-11`. This adapter remains useful for existing
REST integrations, but new long-lived integrations should plan a GraphQL
migration rather than assuming these REST workflows are permanent.

## Implemented contract

| Workflow | Official endpoint |
|---|---|
| Generate a short-lived token | `POST /v3/authentication` |
| Read the configured Advertiser | `GET /v3/advertiser/{advertiserId}` |
| Query an Advertiser's Campaigns | `POST /v3/campaign/query/advertiser` |
| Read a Campaign | `GET /v3/campaign/{campaignId}` |
| Create one Kokai Campaign | `POST /v3/campaign` |
| Partially update a Campaign | `PUT /v3/campaign` |

Campaign creation uses the current REST requirements: `Version` must be
`Kokai`, `PrimaryChannel` is required, `PrimaryGoal` must contain exactly one
typed goal, and the implicit single flight needs a start date and exactly one
currency or impression budget. Send an empty
`CampaignConversionReportingColumns` slice for a campaign that is not
conversion-focused. Each column requires a positive, unique
`ReportingColumnId` and `TrackingTagId`. The official guide describes five
columns as the normal limit, while the OpenAPI schema says the actual maximum
is API-user-configured; the platform therefore enforces that commercial-account
limit. The SDK also rejects structured request collections above 4096 items as
an internal memory-safety boundary; this is not presented as a provider quota.

This initial surface creates one campaign per call and does not expose explicit
`CampaignFlights`. Complex goal updates, secondary and tertiary goals, Ad
Groups, Creatives, GraphQL bulk operations, My Reports, and REDS are outside
the adapter. The public OpenAPI document does not define a
`GET /v3/campaign/{campaignId}/metrics` operation, so reporting is not emulated.

## Authentication and origins

The Platform API uses a token in the `TTD-Auth` header; it is not OAuth. The
adapter accepts only these official HTTPS REST origins:

| Environment | `settings.base_url` |
|---|---|
| Production (default) | `https://api.thetradedesk.com` |
| Partner Sandbox | `https://ext-api.sb.thetradedesk.com` |

Arbitrary gateways and alternate hosts are rejected so credentials cannot be
redirected to an untrusted origin. The cloned HTTP client also disables
redirect following and discards its Cookie Jar.

For production, The Trade Desk recommends generating a long-lived token in the
OpenTTD Access Management UI. These tokens live from one week to one year and
must be revoked and replaced to rotate them. Configure the token as a secret
reference, never as a literal:

```yaml
version: 1
platforms:
  - adapter: thetradedesk/platform-api-v3
    product: platform-api
    accounts:
      - id: us-advertiser
        access_token_ref: env://TTD_API_TOKEN
        approval:
          account_type: platform-api
        settings:
          advertiser_id: advertiser123
```

For temporary or lower-risk workflows, configure the API login in `client_id`
and the password reference in `secret_ref`. The adapter calls
`POST /v3/authentication`, tracks expiry conservatively, and generates another
token before expiry. The endpoint issues no refresh token; short-lived tokens
cannot be refreshed or revoked.

```yaml
version: 1
platforms:
  - adapter: thetradedesk/platform-api-v3
    product: platform-api
    accounts:
      - id: sandbox-advertiser
        client_id: api-user@example.com
        secret_ref: env://TTD_API_PASSWORD
        approval:
          account_type: platform-api
        settings:
          advertiser_id: advertiser123
          token_expiration_minutes: 60
          strict_mode: true
    settings:
      base_url: https://ext-api.sb.thetradedesk.com
```

`token_expiration_minutes` defaults to `1440` and must be from 1 through 1440.
Supply a production encrypted `socialhub.TokenStore` to share short-lived
tokens across processes. Resolver and TokenStore causes are deliberately
replaced with generic errors so credential references, API logins, and token
values cannot escape through error unwrapping.

The Partner Sandbox is refreshed weekly and does not spend real budget. It can
accept an existing production token, but the official guide recommends a
sandbox-only key for additional security. Strict mode is off by default; when
configured, the adapter explicitly sends `TTD-Strict-Mode`. The official guide
recommends strict mode during integration development and incremental,
carefully vetted production use because it turns ignored unknown or read-only
properties into HTTP `400` failures.

## Request, errors, and rate limits

- Only `socialhub.WithCallTimeout` is supported. Caller request IDs,
  idempotency keys, and field selection are rejected because the REST contract
  does not define them. Caller CallOptions are evaluated once, and the timeout
  covers the whole operation, including the ownership preflight for updates.
- Campaign reads and writes are checked against the configured
  `advertiser_id`. Updates perform a preflight ownership read and send only the
  requested partial fields. Array fields replace the existing server value.
- Campaign queries default to a page size of 100 and are bounded to the
  documented recommended maximum of 1000. A pointer to zero requests counts
  only. A missing result array, or a result array larger than the requested
  page size, is rejected as a provider contract violation.
- Rate limits are per endpoint and time window, usually one minute, and may be
  reduced dynamically or applied across related API users. No stable numeric
  quota is published. The official guide recommends at most four concurrent
  callers per endpoint.
- HTTP `429` is retryable and honors a valid bounded `Retry-After`; `500` and
  `503` are retryable. The platform states that throttled `429` requests are
  prevented from executing.
- A Campaign mutation with a network failure, HTTP `5xx`, unreadable or invalid
  successful response, or unexpected successful status returns
  `ErrOutcomeUnknown`. Reconcile advertiser state before retrying to avoid a
  duplicate create or repeated update.
- Successful REST and authentication responses must declare
  `Content-Type: application/json` (parameters are allowed).
- Only bounded structured `Property`, `Reasons`, and `ErrorCode` fields are
  retained from an error body. Free-form bodies are not propagated. Only the
  branded `X-TTD-Request-ID` response header is retained, after strict length,
  UTF-8, control-character, credential-marker, and live credential-value
  checks. `Retry-After` is parsed only after strict header validation.
- `Client.Close` clears process-local static or short-lived token state and
  closes the request-ID filter. It does not delete a token from an external
  `TokenStore`, whose lifecycle remains owned by the caller.
