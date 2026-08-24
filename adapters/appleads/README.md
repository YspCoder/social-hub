# Apple Ads Campaign Management API 5 adapter

Package `social-hub/adapters/appleads` implements organization-scoped paid-media
workflows for Apple Ads Campaign Management API 5. Campaigns, Ad Groups,
targeting Keywords, Creatives, Ads, and reports intentionally remain separate
from the common organic `Publisher`, `Fetcher`, and `MediaUploader` interfaces.

## Configuration

Import the package for registration and configure one social-hub account per
Apple Ads organization:

```go
import _ "social-hub/adapters/appleads"
```

Use a caller-managed access token:

```yaml
adapter: appleads/campaign-management-api-v5
product: campaign-management-api
accounts:
  - id: search-us
    access_token_ref: secret://appleads/access-token
    settings:
      org_id: 1234567
```

Or let the adapter obtain and cache access tokens with Apple's OAuth 2.0
client-credentials flow:

```yaml
adapter: appleads/campaign-management-api-v5
product: campaign-management-api
accounts:
  - id: search-us
    client_id: SEARCHADS.01234567-89ab-cdef-0123-456789abcdef
    settings:
      org_id: 1234567
      team_id: SEARCHADS.01234567-89ab-cdef-0123-456789abcdef
      key_id: 01234567-89ab-cdef-0123-456789abcdef
      private_key_ref: secret://appleads/private-key
```

Each account must configure exactly one credential mode. `private_key_ref`
must resolve to a P-256 ECDSA private key in PKCS#8 or SEC1 PEM form. Endpoint
overrides are intended only for tests and controlled gateways. API and OAuth
redirects are rejected so credentials cannot be forwarded to another origin.

## Access and OAuth

Apple Ads Advanced account access and an API user assigned to the organization
are required. OAuth client-secret JWTs use ES256 with the configured `team_id`,
`key_id`, and `client_id`; access-token requests use the `searchadsorg` scope.
API calls send the resulting Bearer token and `X-AP-Context: orgId=<org_id>`.

Managed tokens are cached in memory and optionally in `socialhub.TokenStore`.
Concurrent refreshes are serialized, and a token is refreshed five minutes
before expiry. The adapter never exposes private keys, JWTs, access tokens, or
organization IDs in errors.

## Supported workflows

- Organization ACL listing.
- Campaign list/find/get, paused create, typed update, explicit status change,
  and delete.
- Ad Group list/get, paused create, typed update, explicit status change, and
  delete.
- Targeting Keyword list/get, paused bulk create, typed bulk update, and delete.
- Custom and default product-page Creative list/get and create.
- Ad list/get, paused create, typed update, explicit status change, and delete.
- Campaign, Ad Group, targeting Keyword, and Ad performance reports.

Campaign, Ad Group, Keyword, and Ad creation always requests a paused state.
Enabling a Campaign requires at least one undeleted, paused Ad Group, and the
adapter refuses the transition when more than one 1000-item page would be
needed to prove every child state. Ad Groups can be enabled only under an
enabled Campaign. Keywords and Ads can be enabled only after both parents are
enabled.

Ad creation additionally requires an undeleted, paused Ad Group and a `VALID`
Creative from the configured organization and the same App Store app as the
Campaign. Deleting Campaigns, Ad Groups, Keywords, or Ads requires the resource
to be paused first. These checks make delivery a deliberate parent-to-child
sequence and prevent generic field updates from bypassing activation controls.

All returned organization, Campaign, Ad Group, Keyword, Creative, and Ad
identifiers are checked against the requested resource hierarchy. Apple can
return HTTP 200 with a business error in the response envelope; the adapter
checks that envelope before accepting any data.

The initial adapter does not implement negative keywords, Budget Orders,
geolocation search, app eligibility, Custom Product Page inspection, rejection
reason lookup, impression-share reports, or organic social capabilities. Those
surfaces require separately reviewed typed workflows.

## Version, pagination, and limits

The adapter uses `https://api.searchads.apple.com/api/v5`. Apple documents
Campaign Management API 5.2 behavior, including default product-page Ads,
under the same `/api/v5` path rather than a separate `/v5.2` base path.

Collection and report selectors require explicit bounded `offset`/`limit`
pagination; the adapter caps a page at 1000. Apple documents up to 5000
targeting Keywords per Campaign and Ad Group. No universal fixed request quota
is published: endpoint and account limits remain authoritative. HTTP 429 maps
to retryable `socialhub.ErrRateLimited`, preserves `Retry-After` when present,
and is suitable for Apple's documented 2, 4, 8, then 16 second backoff sequence.

## Contract sources

The official [Apple Ads reference](https://developer.apple.com/documentation/apple_ads),
[OAuth guide](https://developer.apple.com/documentation/apple_ads/implementing-oauth-for-the-apple-search-ads-api),
[API calling and rate-limit guide](https://developer.apple.com/documentation/apple_ads/calling-the-apple-search-ads-api),
[Campaigns](https://developer.apple.com/documentation/apple_ads/campaigns),
[Ad Groups](https://developer.apple.com/documentation/apple_ads/ad-groups),
[Keywords](https://developer.apple.com/documentation/apple_ads/keyword),
[Creatives](https://developer.apple.com/documentation/apple_ads/creatives),
[Ads](https://developer.apple.com/documentation/apple_ads/ads), and
[Reports](https://developer.apple.com/documentation/apple_ads/reports) are the
wire-contract source of truth and were verified on 2026-08-09.

No third-party Apple Ads SDK is required. The adapter uses Go's standard
library for ES256 and social-hub's bounded shared HTTP transport.
