# Ocean Engine Marketing API v3 adapter

`oceanengine/marketing-api-v3` provides an advertiser-scoped client for the
Ocean Engine Marketing API. It models v3 `Project`, `Promotion`, and synchronous
custom-report workflows without treating paid-media resources as organic social
posts.

## Contract

- API origin: `https://api.oceanengine.com`
- API version: v3.0 resource paths
- Authentication: `Access-Token` request header
- Business response: `code`, `message`, `data`, and `request_id`
- Account boundary: one configured numeric `advertiser_id` per client
- Creation safety: `CreateProject` and `CreatePromotion` always send
  `operation=DISABLE`; activation requires an explicit status update
- Transport safety: bounded responses, relative paths, rejected redirects, and
  sanitized errors

The provider has a large set of goal-dependent request fields. Stable required
fields are typed. Conditional fields use each request's `Fields` map. The
adapter rejects invalid field names and prevents overrides of `advertiser_id`,
resource IDs, `operation`, tokens, and secrets.

## Configuration

```yaml
version: 1
platforms:
  - adapter: oceanengine/marketing-api-v3
    product: marketing-api
    accounts:
      - id: cn-ads-primary
        app_id: "123456789"
        secret_ref: env://OCEANENGINE_APP_SECRET
        access_token_ref: env://OCEANENGINE_ACCESS_TOKEN
        settings:
          advertiser_id: 987654321
```

`app_id` and `secret_ref` are needed only by `Adapter.OAuth`. A client using a
caller-managed token still requires `access_token_ref` and `advertiser_id`.

## Usage

Import the package for registration, open the configured adapter, then type
assert the common client when using advertising workflows:

```go
import (
    "context"

    "social-hub/adapters/oceanengine"
    "social-hub/pkg/socialhub"
)

func createProject(
    ctx context.Context,
    adapter *oceanengine.Adapter,
) error {
    common, err := adapter.Client(ctx, "cn-ads-primary")
    if err != nil {
        return err
    }
    client := common.(*oceanengine.Client)

    project, err := client.Projects().CreateProject(ctx, oceanengine.CreateProjectRequest{
        Name:          "Summer launch",
        AdType:        oceanengine.AdTypeAll,
        LandingType:   oceanengine.LandingTypeLink,
        MarketingGoal: oceanengine.MarketingGoalVideoAndImage,
        DeliveryRange: oceanengine.DeliveryRange{
            InventoryCatalog: oceanengine.InventoryCatalogManual,
        },
        DeliverySetting: oceanengine.DeliverySetting{
            BidType:    oceanengine.BidTypeCustom,
            BudgetMode: oceanengine.BudgetModeDay,
        },
        Fields: map[string]any{
            "audience": map[string]any{"district": "ALL"},
        },
    })
    if err != nil {
        return err
    }

    // Creation is paused. Enabling is intentionally separate and explicit.
    return client.Projects().SetProjectStatus(
        ctx, project.ID, oceanengine.OperationEnable,
    )
}
```

Available typed workflows:

- `Projects`: list, create, update, enable, and disable v3 Projects
- `Promotions`: list, create, update, enable, and disable v3 Promotions
- `Reports`: paged synchronous `/open_api/v3.0/report/custom/get/` reads
- `Adapter.OAuth`: exchange authorization code, refresh, renew, and obtain an
  app token

The customer authorization URL is issued/configured through the Ocean Engine
developer console. The SDK does not guess or synthesize that URL.

## Permissions and limits

Production use requires a registered Ocean Engine developer application, the
relevant application capability groups, and advertiser authorization. These
permissions are external to the token and cannot be inferred reliably from a
request, so capabilities report approval as `unknown` until an API call proves
access.

QPS is managed per application and endpoint in the Ocean Engine developer
console. Preserve `request_id` when escalating provider failures and apply the
shared social-hub retry/limiter middleware according to the granted quota.

## Verification basis

The adapter contract was checked on 2026-08-09 against:

- [Ocean Engine Marketing API documentation](https://open.oceanengine.com/labels/7)
- [Ocean Engine developer QPS console](https://open.oceanengine.com/developer/admin/qps)
- [Official Go SDK v1.1.92](https://github.com/oceanengine/ad_open_sdk_go/tree/v1.1.92)

The official generated SDK is used as a contract reference rather than a Go
dependency. Its generated surface is much larger than this adapter and uses an
unbounded response read; social-hub keeps its existing bounded transport and
error model.
