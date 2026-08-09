# Display & Video 360 API v4 adapter

Package `social-hub/adapters/dv360` implements advertiser-scoped paid-media
workflows for Display & Video 360 API v4. These resources intentionally remain
separate from the common organic `Publisher`, `Fetcher`, and messaging
interfaces.

## Official contract

- REST v4 reference: <https://developers.google.com/display-video/api/reference/rest/v4>
- Release notes: <https://developers.google.com/display-video/api/release-notes>
- Resource hierarchy: <https://developers.google.com/display-video/api/guides/managing-line-items/resources>
- Errors and warnings: <https://developers.google.com/display-video/api/guides/concepts/general/errors-warnings>
- Quotas: <https://developers.google.com/display-video/api/limits>
- Service accounts: <https://developers.google.com/display-video/api/guides/concepts/general/service-accounts>

The implementation was checked against the v4 Discovery document revision
`20260805` on 2026-08-09. The API root is
`https://displayvideo.googleapis.com/v4`.

## Implemented workflows

- Get the configured Advertiser and list Advertisers under a configured Partner.
- Campaign get/list, paused-first create, and typed patch.
- Insertion Order get/list, draft-first create, and typed patch.
- Standard Display, Video, and Audio RTB Line Item get/list, draft-first create,
  typed patch, and duplicate.
- Static bearer tokens or managed Google OAuth 2.0 refresh tokens.
- Google RPC errors, request IDs, retry hints, and a typed default quota policy.

All lists preserve `nextPageToken`, validate documented page sizes, cap filters
at 500 characters, and restrict `orderBy` to documented fields. Patch methods
derive an allowlisted `updateMask`; callers cannot inject arbitrary field paths.

Creation is deliberately off-first: Campaigns are sent as
`ENTITY_STATUS_PAUSED`, and Insertion Orders and Line Items are sent as
`ENTITY_STATUS_DRAFT`. The adapter rejects a response that does not preserve
that state. Child creation reads the parent resources first and checks
Advertiser, Campaign, and Insertion Order ownership. Activation similarly
requires active parents.

## Configuration

Import the package for registration:

```go
import _ "social-hub/adapters/dv360"
```

Use a caller-managed access token:

```yaml
adapter: google/display-video-360-api-v4
product: display-video-360-api
accounts:
  - id: dv360-brand
    access_token_ref: secret://dv360/access-token
    settings:
      advertiser_id: "123456789"
      partner_id: "987654321"
```

Or let the adapter refresh and cache access tokens:

```yaml
adapter: google/display-video-360-api-v4
product: display-video-360-api
accounts:
  - id: dv360-brand
    client_id: 000000000000-example.apps.googleusercontent.com
    secret_ref: secret://dv360/client-secret
    settings:
      advertiser_id: "123456789"
      partner_id: "987654321"
      refresh_token_ref: secret://dv360/refresh-token
```

`partner_id` is optional for advertiser-scoped workflows but required by
`ListAdvertisers`. Credential values are always resolved through
`socialhub.SecretResolver`. Managed tokens are serialized during refresh,
cached in memory, optionally persisted through `socialhub.TokenStore`, and
refreshed two minutes before expiry.

The required scope is:

```text
https://www.googleapis.com/auth/display-video
```

The configured Google user must have access to the Partner and Advertiser.
Advertiser billing, inventory, Floodlight, creative approval, and product
allowlists remain external prerequisites.

## Usage

```go
func createCampaign(ctx context.Context, adapter socialhub.Adapter) error {
    common, err := adapter.Client(ctx, "dv360-brand")
    if err != nil {
        return err
    }
    client := common.(*dv360.Client)

    end := dv360.Date{Year: 2026, Month: 12, Day: 31}
    _, err = client.CreateCampaign(ctx, dv360.CreateCampaignRequest{
        DisplayName: "Brand awareness",
        CampaignGoal: dv360.CampaignGoal{
            Type: dv360.CampaignGoalBrandAwareness,
            PerformanceGoal: dv360.PerformanceGoal{
                Type: dv360.PerformanceGoalCPM, AmountMicros: "2000000",
            },
        },
        CampaignFlight: dv360.CampaignFlight{
            PlannedSpendAmountMicros: "500000000",
            PlannedDates: dv360.DateRange{
                StartDate: dv360.Date{Year: 2026, Month: 9, Day: 1},
                EndDate: &end,
            },
        },
        FrequencyCap: dv360.FrequencyCap{Unlimited: true},
    })
    return err
}
```

## Scope boundaries

The initial adapter does not implement Creative, targeting, inventory-source,
asset-upload, YouTube & Partners, or Demand Gen mutation surfaces. YouTube and
Demand Gen mutations have product and allowlist restrictions; attempting to
update or duplicate those Line Item types returns `socialhub.ErrUnsupported`.

Reporting is not part of Display & Video 360 API v4. Bid Manager reporting must
be implemented as a separate product adapter instead of being presented as a
DV360 v4 feature. The removed `generateDefault` Line Item method is not in the
current v4 Discovery contract and is not exposed.

Service-account JWT and domain-wide delegation are also outside this initial
version. Use a caller-managed service-account access token when needed, or the
implemented user refresh-token flow. Organic publishing, media, reactions,
messaging, and webhooks are explicitly unsupported.

## Quotas and retries

The documented default per-minute quotas are:

| Scope | Total requests | Write requests |
|---|---:|---:|
| Google Cloud project | 1,500 | 700 |
| Advertiser and project | 300 | 150 |

Write-intensive endpoints count as five write requests. `DefaultQuotaPolicy`
exposes these defaults for a caller-side limiter, but Google can grant
project-specific adjustments, so runtime limits should remain configurable.

HTTP 429 / `RESOURCE_EXHAUSTED` maps to retryable
`socialhub.ErrRateLimited`. `Retry-After` and Google `RetryInfo.retryDelay` are
preserved when present. Use bounded exponential backoff with jitter and keep
mutation idempotency under caller control.
