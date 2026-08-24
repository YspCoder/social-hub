# Unity Advertising Management API v1 adapter

`adapters/unityadvertising` implements Unity's organization-bound Advertising
Management API v1 for paid user acquisition. Unity labels the current OpenAPI
3.0 contract `v1.0 latest`; production routes are below `/advertise/v1`.

Official references:

- Advertising Management API v1: <https://services.docs.unity.com/advertise/v1/>
- Campaign management overview: <https://docs.unity.com/en-us/user-acquisition/management/api-campaign-management>
- Service Account authentication: <https://services.docs.unity.com/docs/service-account-auth/>
- Manage API v2 migration: <https://docs.unity.com/en-us/monetization/performance-and-analytics/api/manage-api/migrate-to-the-new-dashboard-manage-api>

## Implemented workflows

All 42 non-deprecated operations in the current v1 contract are implemented:

- Apps (5): list, create, get, update, and delete.
- Creatives (3): list, streaming multipart create, and get. Create supports a
  square end card, portrait/landscape end-card pair, MP4 video, or HTML
  playable without buffering the entire asset in memory.
- Creative Packs (5): list, create, get, update, and delete.
- Campaigns (13): list, create, get, update, delete, Creative Pack assignment,
  Targeting get/update, Budget get/update, and SDK event-name discovery.
- Bids (16): list, replace, and patch for CPI, Source, ROAS, Retention, and
  Event Optimization bids, plus Event Optimization eligibility discovery.

Campaign creation uses closed request variants for Installs, Retention, ROAS,
Creative Testing, and Event Optimization goals. Budget updates likewise use
separate daily, per-country, and country-group variants. Patch models preserve
the difference between an omitted field and an explicit JSON `null` where the
Unity contract uses `null` to remove a value.

The deprecated `advertise-listRoasInfo` and `advertise-listRetentionInfo`
operations are deliberately omitted. Advertising Statistics API reporting is
a separate product and is not inferred from this management contract.

## Access requirements and authentication

Unity must first grant the organization access to the Advertising Management
API. Contact the organization's Unity Client Partner or
`unityads-support@unity3d.com`. An Organization Owner must then create a Unity
Organization Service Account and assign the least-privileged Growth roles
needed by the integration:

- `Advertise API Viewer`
- `Advertise API Apps Editor`
- `Advertise API Campaigns Editor`
- `Advertise API Bids Editor`
- `Advertise API Creative Packs Editor`
- `Advertise API Targeting Editor`
- `Advertise API Admin` for full read/write access

The adapter accepts exactly one authentication mode:

- HTTP Basic: `client_id` is the Service Account Key ID and `secret_ref`
  resolves the Secret Key. The SDK encodes `KEY_ID:SECRET_KEY` using HTTP Basic.
- Long-lived Bearer: `access_token_ref` resolves a Unity Service Account bearer
  token. Unity documents these tokens as not requiring refresh.

Credential values are resolved through `socialhub.SecretResolver`; configuration
stores references only. `organization_id` must be the numeric **Organization
Core ID**, not another Unity organization identifier.

```yaml
version: 1
platforms:
  - adapter: unity/advertising-management-api-v1
    product: advertising-management-api
    accounts:
      - id: unity-acquisition-primary
        client_id: ${UNITY_SERVICE_ACCOUNT_KEY_ID}
        secret_ref: env://UNITY_SERVICE_ACCOUNT_SECRET_KEY
        settings:
          organization_id: "5772562874846"
```

For long-lived Bearer authentication, replace `client_id` and `secret_ref`
with:

```yaml
        access_token_ref: env://UNITY_SERVICE_ACCOUNT_BEARER_TOKEN
```

## Typed usage

Import the package for registration, open the configured adapter, and use its
typed workflows:

```go
package main

import (
	"context"

	"social-hub/adapters/unityadvertising"
	"social-hub/pkg/socialhub"
)

func listCampaigns(ctx context.Context, config socialhub.AdapterConfig, appID string) error {
	adapter, err := socialhub.Open(ctx, "unity/advertising-management-api-v1", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	common, err := adapter.Client(ctx, "unity-acquisition-primary")
	if err != nil {
		return err
	}
	client := common.(*unityadvertising.Client)

	_, err = client.Campaigns().ListCampaigns(
		ctx,
		appID,
		unityadvertising.ListCampaignsRequest{},
	)
	return err
}
```

Unity calls an App ID a `campaignSetId` in paths. Campaign, Creative, and
Creative Pack IDs are 24-character hexadecimal IDs. Core responses retain the
sanitized typed fields and their original JSON in `Raw` for forward-compatible
inspection.

## Rate limits and retries

`DefaultQuotaPolicy` records the current documented ceilings:

| Endpoint or method group | Requests/second | Requests/30 minutes |
|---|---:|---:|
| Apps, Campaigns, Creatives, Creative Packs | 20 | 4,000 |
| Bids | 10 | 4,000 |
| `POST` | 1 | 30 |
| `PUT`, `PATCH`, `DELETE` | 1 | 100 |

The most restrictive applicable group wins. `APIError` preserves sanitized
Unity problem details, `Retry-After`, `RateLimit-Policy`, `RateLimit`, and
`Unity-RateLimit`. Coordinate enforcement through social-hub's shared limiter,
retry only errors whose `Retryable()` method returns true, and keep mutations
caller-controlled because the v1 contract does not publish idempotency keys.

## Manage API v2 migration risk

Unity's current dashboard migration guidance says User Acquisition integrations
must migrate to Manage API v2 before their organization moves to the new Unity
Dashboard, and that Manage API v1 stops working immediately after that
organization migration. As verified on 2026-08-10, the same guidance still
labels the Advertiser Public API v2 migration reference as "coming soon" and
does not link a stable public advertiser OpenAPI contract.

This adapter therefore remains pinned to the published v1 OpenAPI contract.
Treat organization migration as a deployment blocker: validate a future,
separately named Advertiser v2 adapter in a test organization before moving
production credentials or traffic. Do not silently redirect this adapter to
v2 routes when they become available.
