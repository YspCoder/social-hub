# Snapchat Marketing API v1 adapter

Package `social-hub/adapters/snapchat/marketing` implements Ad Account scoped
paid-media workflows for Snapchat Marketing API v1. Campaigns, Ad Squads, Ads,
and Stats intentionally remain separate from common social `Post`, `Fetcher`,
and `MediaUploader` operations and from the organic
`snapchat/public-profile-v1` adapter.

## Configuration

Import the package for registration and configure one social-hub account per
Snapchat Ad Account:

```go
import _ "social-hub/adapters/snapchat/marketing"
```

```yaml
adapter: snapchat/marketing-api-v1
product: marketing-api
accounts:
  - id: paid-social-us
    client_id: snapchat-oauth-client-id
    secret_ref: secret://snapchat/client-secret
    access_token_ref: secret://snapchat/access-token
    approval:
      scopes:
        - snapchat-marketing-api
    settings:
      ad_account_id: "11111111-1111-4111-8111-111111111111"
```

Access tokens and client secrets are always resolved through secret references.
Redirects are rejected so bearer credentials cannot be forwarded to another
origin. Endpoint overrides are intended only for tests and controlled gateways.

## Access and OAuth

The Marketing API requires a Business Manager OAuth application created by an
Organization Admin. The authorized user must retain the required Organization
and Ad Account roles; a valid token alone does not grant access to every
resource.

`Adapter.OAuth` supports the OAuth 2.0 authorization-code and refresh-token
grants for `snapchat-marketing-api`. Access tokens normally expire after about
one hour. Refresh tokens generally remain valid until authorization is revoked;
the helper preserves the existing refresh token when Snapchat omits a
replacement. Applications remain responsible for encrypted token storage and
proactive refresh.

## Supported workflows

- Configured Ad Account lookup.
- Account-scoped Campaign list, direct get, paused create, and name/status JSON
  Patch updates.
- Account-scoped Ad Squad list, direct get, paused create, and name/status JSON
  Patch updates.
- Account-scoped Ad list, direct get, paused create from an existing Creative
  ID, and name/status JSON Patch updates.
- Synchronous configured-account Stats for `TOTAL`, `DAY`, `HOUR`, and
  `LIFETIME` granularities. Metric names remain caller selected and metric
  values retain their raw JSON representation.

Campaign, Ad Squad, and Ad creation always sends `PAUSED`; enabling delivery
requires a separate explicit status call. Initial Ad Squad creation uses
`SNAP_ADS`, automatic placement, impression optimization/billing,
`LOWEST_COST_WITH_MAX_BID`, daily budget delivery, and country targeting. This
avoids pinning the SDK to a stale placement list as Snapchat expands placements,
including Sponsored Snaps.

Before an Ad Squad mutation, the adapter reads its Campaign and verifies that
the Campaign belongs to the configured Ad Account. Ad mutations also walk the
Ad Squad-to-Campaign parent chain. A cross-account parent is rejected before a
write request. Mutation responses must contain exactly one successful
subrequest and are read back before being returned.

Collection responses expose `paging.next_link`. The adapter never follows that
URL directly: it validates the origin and expected account-scoped path, extracts
one bounded cursor, and rebuilds the next request against the configured base
URL. Entity list limits are 50-1,000; synchronous Stats limits are capped at 200.

## Limits and exclusions

Snapchat documents average limits of 20 requests per second per application and
10 requests per second per access token. Account-specific responses remain
authoritative. HTTP 429 and throttling failures map to the shared retryable
error contract, including `Retry-After` when supplied.

The initial adapter does not implement Creative creation, Media upload,
Lens/Filter workflows, Custom Audiences, Conversions API, asynchronous reports,
deletion, arbitrary JSON Patch paths, arbitrary targeting JSON, or advertising
webhooks. These surfaces need separately reviewed typed contracts rather than
unvalidated passthrough maps.

## Contract sources

The official [Introduction](https://developers.snap.com/marketing-api/Ads-API/introduction),
[Authentication](https://developers.snap.com/marketing-api/Ads-API/authentication),
[API Patterns](https://developers.snap.com/marketing-api/Ads-API/api-patterns),
[Campaigns](https://developers.snap.com/marketing-api/Ads-API/campaigns),
[Ad Squads](https://developers.snap.com/marketing-api/Ads-API/ad-squads),
[Ads](https://developers.snap.com/marketing-api/Ads-API/ads),
[Measurement](https://developers.snap.com/marketing-api/Ads-API/measurement), and
[Rate Limits](https://developers.snap.com/marketing-api/Ads-API/rate-limits)
documentation are the source of truth.

There is no comparably current official or mature community Go SDK for typed
mutation workflows. This package therefore uses social-hub's bounded shared
transport and the official wire contract without adding a Snapchat-specific
third-party dependency.
