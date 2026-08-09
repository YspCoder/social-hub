# Unity Ads Publisher Manage API v2 adapter

`adapters/unitypublisher` implements the organization-bound Unity Ads
Publisher Manage API v2. Unity labels the published OpenAPI contract `1.0.0`
and serves its routes below `/ads/publisher/public/v1`; this is the current
Publisher Manage API v2 product, not the legacy Monetize Manage API v1.

Official references:

- OpenAPI 1.0.0: <https://docs.unity.com/en-us/oas-unified-platform-publisher-public/1.0.0>
- Manage API v2 migration: <https://docs.unity.com/en-us/monetization/performance-and-analytics/api/manage-api/migrate-to-the-new-dashboard-manage-api>
- Publisher migration reference: <https://docs.unity.com/en-us/monetization/performance-and-analytics/api/manage-api/publisher-api-migration-reference>
- Service Account authentication: <https://services.docs.unity.com/docs/service-account-auth/>
- Errors: <https://services.docs.unity.com/docs/errors/>
- Response headers: <https://services.docs.unity.com/docs/headers/>

## Implemented workflows

All 18 operations in the current OpenAPI contract are implemented:

- Application list, create, get, and partial update
- Application test-mode get and update
- Per-Application Placement list, create, get, full update, archive, and restore
- Organization Placement list
- Organization Test Device list, create, get, update, and delete
- Optional `dryrun=true` on every write operation declared by the current
  OpenAPI contract

The current API returns arrays directly for list operations and exposes no
pagination fields. Placement list filters support `isArchived` and repeated
`adFormat` query values.

## Access requirements and authentication

Create a Unity Organization Service Account and assign the required
organization-level Monetize roles. The adapter accepts exactly one of:

- HTTP Basic credentials: `client_id` is the Service Account Key ID and
  `secret_ref` resolves the Secret Key. The SDK Base64-encodes `KEY_ID:SECRET_KEY`
  through HTTP Basic authentication.
- A long-lived Service Account bearer token through `access_token_ref`. Unity
  documents that these tokens do not require refresh.

Credential values are always resolved through `socialhub.SecretResolver`;
configuration stores references only.

```yaml
version: 1
platforms:
  - adapter: unity/ads-publisher-manage-api-v2
    product: ads-publisher-manage-api
    accounts:
      - id: unity-publisher-primary
        client_id: ${UNITY_SERVICE_ACCOUNT_KEY_ID}
        secret_ref: env://UNITY_SERVICE_ACCOUNT_SECRET_KEY
        settings:
          organization_id: "3573617062594"
```

For long-lived Bearer authentication, replace `client_id` and `secret_ref`
with:

```yaml
        access_token_ref: env://UNITY_SERVICE_ACCOUNT_BEARER_TOKEN
```

## Usage

Import the adapter for registration, initialize it through `socialhub.Open`,
then use its typed workflows:

```go
import (
    "context"

    "social-hub/adapters/unitypublisher"
    "social-hub/pkg/socialhub"
)

func createPlacement(ctx context.Context, adapter socialhub.Adapter) error {
    common, err := adapter.Client(ctx, "unity-publisher-primary")
    if err != nil {
        return err
    }
    client := common.(*unitypublisher.Client)

    _, err = client.CreatePlacement(
        ctx,
        "5a8591dd-4039-49df-9202-96385ba3eff8",
        unitypublisher.PlacementRequest{
            Name:     "Rewarded Placement",
            AdFormat: unitypublisher.AdFormatRewarded,
            AdFormatConfigurations: unitypublisher.RewardedConfigurations{
                Name: "coins",
                Value: 100,
            },
        },
        unitypublisher.MutationOptions{DryRun: true},
    )
    return err
}
```

Use the Placement response `id` UUID for get, update, archive, and restore
paths. The response `key` is a human-readable slug and is deliberately rejected
as a path identifier. `NullablePlatform` distinguishes an omitted Test Device
patch field from an explicit JSON `null` used to clear the platform.

## Deliberate boundaries

The adapter never calls legacy `/monetize/v1` routes. Publisher Manage API v2
does not expose Ad Unit resources or eCPM Target management; do not infer or
construct routes for them. Ad-format settings now live on Placements, and
client-chosen Placement IDs are not supported.

The current OpenAPI contract explicitly declares `dryrun` on write operations,
even though older migration prose described it as removed. This adapter follows
the published contract, which Unity identifies as the source of truth.

## Rate limits and retries

`DefaultQuotaPolicy` records Unity's documented ceilings:

- All traffic per IP: 40 requests/second
- `GET`: 20 requests/second and 8,000/hour
- `POST`: 1 request/second and 60/hour
- `PATCH`, `PUT`, and `DELETE`: 1 request/second and 200/hour

`APIError` preserves sanitized Unity problem fields, `Retry-After`,
`RateLimit-Policy`, `RateLimit`, and `Unity-RateLimit`. Use the shared
social-hub limiter and retry layer only when `Retryable()` is true. Keep writes
caller-controlled because the contract does not publish idempotency guarantees.
