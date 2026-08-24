# Amazon Ads Sponsored Products v3 adapter

Package `social-hub/adapters/amazonads` implements profile-scoped Amazon Ads
Sponsored Products v3 and Reporting v3 workflows. Campaigns, Ad Groups,
Product Ads, Keywords, and reports intentionally remain separate from common
social `Post`, `Fetcher`, and `MediaUploader` operations.

## Configuration

Import the package for registration and configure one social-hub account per
Amazon Ads profile:

```go
import _ "social-hub/adapters/amazonads"
```

```yaml
adapter: amazonads/sponsored-products-v3
product: sponsored-products
accounts:
  - id: retail-us
    client_id: amzn1.application-oa2-client.example
    secret_ref: secret://amazonads/client-secret
    access_token_ref: secret://amazonads/access-token
    approval:
      scopes:
        - advertising::campaign_management
    settings:
      profile_id: "1234567890"
      region: NA
```

`region` selects the production API origin:

| Region | Origin |
|---|---|
| `NA` | `https://advertising-api.amazon.com` |
| `EU` | `https://advertising-api-eu.amazon.com` |
| `FE` | `https://advertising-api-fe.amazon.com` |

The client sends `Authorization` and `Amazon-Advertising-API-ClientId` on every
API call. It sends `Amazon-Advertising-API-Scope` on profile-scoped SP and
Reporting requests, but deliberately omits that header from `/v2/profiles`.
Redirects are rejected so bearer tokens and Amazon advertising headers cannot
be forwarded to another origin. Adapter-level `base_url` is reserved for tests
and controlled gateways.

## Access and OAuth

Amazon Ads API access requires approved developer onboarding and profile
authorization. `Adapter.OAuth` implements Login with Amazon's OAuth 2.0
authorization-code flow for `advertising::campaign_management` and refresh
tokens. Applications remain responsible for encrypted token storage and
refresh before expiry.

`ListProfiles` lists profiles visible to the OAuth token. `GetProfile` checks
that the configured `profile_id` occurs in that list; it does not accept an
arbitrary profile ID from a call site.

## Supported workflows

- OAuth-accessible Profiles and configured Profile lookup.
- Sponsored Products Campaign list, paused create, update, explicit state
  change, and archive.
- Ad Group list, paused create, update, explicit state change, and archive.
- Product Ad list, paused ASIN-or-SKU create, explicit state change, and
  archive.
- Keyword list, paused create, bid update, explicit state change, and archive.
- Reporting v3 asynchronous report create and status lookup.

Campaign, Ad Group, Product Ad, and Keyword creation always sends
`state=PAUSED`. Enabling delivery requires a separate explicit state call.
Currency amounts use the string-backed `Decimal` type but are encoded as JSON
numbers, avoiding `float64` rounding without changing Amazon's wire contract.

Every v3 mutation sends `Prefer: return=representation` and validates the
returned resource ID. Amazon may return HTTP 207 for a batch with item-level
failures; the adapter inspects the mutation envelope and returns a typed
`socialhub.Error` instead of treating every 2xx response as success.

Reporting returns Amazon's pre-signed download URL in `Report.URL` after the
report reaches a completed state. The SDK does not download or follow that
external URL because it is credential-bearing, short-lived, and outside the
configured Amazon Ads origin.

The initial adapter does not implement Sponsored Brands, Sponsored Display,
DSP, targeting clauses, negative keywords, budget rules, recommendations,
exports, bulk sheets, Amazon Marketing Stream, or Unified API v1. Those
surfaces require separately reviewed typed workflows.

## Versions, migration, and quotas

This adapter targets the product-specific Sponsored Products v3 endpoints
used by the current official Amazon Ads Postman collection. Amazon Ads Unified
API v1 is the newer cross-product model and should be evaluated for new
integrations; this package exists for applications that still require the SP
v3 contract.

Amazon is also consolidating reporting into Unified Reporting. The legacy
Sponsored Ads and DSP report centers are scheduled to close on December 31,
2026. Reporting v3 create/status remains implemented here for compatibility,
but consumers should plan and test their migration rather than treating this
surface as a permanent reporting architecture.

Amazon applies endpoint-specific token-bucket limits. Runtime response headers
and errors are authoritative: `x-amzn-RateLimit-Limit` communicates the current
limit and `Retry-After` accompanies throttling when supplied. HTTP 429 and
throttling platform codes map to retryable `socialhub.ErrRateLimited` errors.
The shared transport retains an 8 MiB bounded-response limit.

## Contract sources

The official [Amazon Ads advanced tools center](https://advertising.amazon.com/API/docs/en-us/index),
[Sponsored Products reference](https://advertising.amazon.com/API/docs/en-us/reference/3/sponsored-products),
[Login with Amazon authorization guide](https://developer.amazon.com/docs/login-with-amazon/authorization-code-grant.html),
and Amazon's [official Postman collections](https://github.com/amzn/ads-advanced-tools-docs/tree/main/postman)
are the source of truth.

Several community Go clients were reviewed for naming and integration patterns,
but none provides a sufficiently current, broadly adopted SP v3 dependency.
This package therefore uses social-hub's bounded shared transport and the
official wire contract without adding an Amazon-specific third-party module.
