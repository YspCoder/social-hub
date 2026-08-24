# AppLovin Growth Campaign Management API v1 adapter

`adapters/applovinads` implements AppLovin Axon Campaign Management API v1 for
paid user acquisition. It is registered as
`applovin/growth-campaign-management-api-v1` and is intentionally separate from
`applovin/max-reporting-apis`: Growth manages advertiser campaigns and creative
assets, while MAX reports publisher mediation revenue.

The contract was verified on 2026-08-25 against AppLovin's current APP and WEB
documentation and the official MIT-licensed `applovin-mcp@0.0.8` npm package.
The published MCP package was used as a typed contract reference only; it is not
a Go dependency. This adapter also covers the current documented association
endpoints.

## Implemented workflows

All 18 current management endpoints are implemented:

| Workflow | Operations |
|---|---|
| Campaigns | list, create, update, and WEB catalog/variant-set discovery |
| Creative Sets | list, list by Campaign, create, update, clone, add to Campaigns, remove from selected Campaigns, remove from all Campaigns |
| Assets | list, bounded streaming multipart upload, upload-result polling, add to Creative Sets, remove from selected Creative Sets, remove from all Creative Sets |

APP and WEB use closed create/update request variants. The client injects the
configured account type into every mutation and rejects cross-type inputs before
network I/O. Campaign `status` is ignored by AppLovin during creation, so it is
not present in create models. Creative Set cloning always sends `PAUSED`; use an
explicit update after review to make the clone live.

The six current cross-Campaign/Creative Set association methods are based on the
latest official documentation:

- `POST /creative_set/add-to-campaigns`
- `POST /creative_set/remove-from-campaigns`
- `POST /creative_set/remove-from-all-campaigns`
- `POST /asset/add-to-creative-sets`
- `POST /asset/remove-from-creative-sets`
- `POST /asset/remove-from-all-creative-sets`

## Access and authentication

Campaign Management API access is whitelist-only. Contact the AppLovin account
team, then obtain the Campaign Management API key from Ads Manager. Only account
Admins can view keys. This key is different from the MAX Report Key, MAX SDK Key,
Ad Unit Management Key, and Event Key.

The adapter resolves `secret_ref` at runtime and sends the key verbatim in the
`Authorization` header. It adds the numeric Ads Manager `account_id` query
parameter to every request. The API origin is fixed to
`https://api.ads.axon.ai/manage/v1`; redirects and caller cookie jars are not
used for authenticated requests. Record the externally granted entitlement as
`campaign_management_api` in `approval.scopes`; without that declaration,
capabilities report `ApprovalRequired` and calls fail locally without consuming
AppLovin's error budget.

APP and WEB accounts have materially different Campaign, goal, targeting, and
Creative Set rules. Configure each as a separate social-hub account:

```yaml
version: 1
platforms:
  - adapter: applovin/growth-campaign-management-api-v1
    product: growth-campaign-management-api
    accounts:
      - id: applovin-app-acquisition
        secret_ref: env://APPLOVIN_CAMPAIGN_MANAGEMENT_API_KEY
        approval:
          scopes: [campaign_management_api]
        settings:
          account_id: "123456"
          account_type: APP
      - id: applovin-web-acquisition
        secret_ref: env://APPLOVIN_WEB_CAMPAIGN_MANAGEMENT_API_KEY
        approval:
          scopes: [campaign_management_api]
        settings:
          account_id: "654321"
          account_type: WEB
```

## Typed usage

```go
package main

import (
    "context"

    "social-hub/adapters/applovinads"
    "social-hub/pkg/socialhub"
)

func createCampaign(ctx context.Context, config socialhub.AdapterConfig) error {
    adapter, err := socialhub.Open(ctx, "applovin/growth-campaign-management-api-v1", config)
    if err != nil {
        return err
    }
    defer adapter.Close()

    common, err := adapter.Client(ctx, "applovin-app-acquisition")
    if err != nil {
        return err
    }
    client := common.(*applovinads.Client)

    _, err = client.Campaigns().CreateCampaign(ctx, applovinads.AppCampaignCreateRequest{
        Name:            "US launch",
        StartDate:       "2026-08-15T00:00:00Z",
        EndDate:         "2026-08-31T00:00:00Z",
        BiddingStrategy: applovinads.BiddingTargetGoalCPI,
        Platform:        applovinads.PlatformIOS,
        PackageName:     "com.example.app",
        ITunesID:        "123456789",
        Budget: applovinads.Budget{
            DailyBudgetForAllCountries: "5000",
        },
        Goal: applovinads.Goal{
            GoalType:                 applovinads.GoalCPI,
            GoalValueForAllCountries: "10",
        },
        Targeting: []applovinads.Targeting{{CountryCode: "US"}},
        Tracking: applovinads.Tracking{
            TrackingMethod: applovinads.TrackingAppsFlyer,
            ImpressionURL:  "https://impression.example/path",
            ClickURL:       "https://click.example/path",
        },
    })
    return err
}
```

Currency, bid, budget, and ROAS values stay as decimal strings. This avoids
binary floating-point changes on the wire. ROAS values are multipliers: `1.0`
means 100%, not 1%.

Asset upload accepts one to 40 caller-owned `io.Reader` values, streams them
without whole-file buffering, and validates the documented one-GiB per-file and
10-GiB batch limits. Filenames must be unique. Allowed media types are
`text/html`, GIF/JPEG/PNG, MP4, and QuickTime. Poll `GetAssetUploadStatus` until
`upload_status` is `FINISHED`; each failed asset counts separately toward the
account error threshold.

## Limits, errors, and retries

`DefaultQuotaPolicy` records the current account/key policy:

| Policy | Limit | Penalty |
|---|---:|---:|
| Request rate | 1,000 requests / 60 seconds | HTTP 429 for 10 minutes |
| Failed-request budget | 100 errors / 5 minutes | HTTP 429 for 24 hours; `x-al-error-code: 100007` |

`APIError` preserves a numeric `x-al-error-code`, a control-character-free
`X-TRACE-ID`, and `Retry-After`. Provider-supplied error messages and response
bodies are not exposed; the public message is derived from the HTTP status.
When AppLovin omits `Retry-After`, the adapter reports the documented 10-minute
penalty, or 24 hours for error code `100007`.

Coordinate limits through the shared social-hub limiter across all processes
using the same key. Retry only failures whose `Retryable()` method returns true.
The current contract does not publish idempotency semantics for mutations, so
do not automatically replay Campaign, Creative Set, association, or Asset upload
writes after an ambiguous transport failure.

Official references:

- APP Campaign Management API: <https://support.applovin.com/en/growth/promoting-your-apps/api/axon-campaign-management-api>
- WEB Campaign Management API: <https://support.applovin.com/en/growth/promoting-your-websites/api/axon-campaign-management-api-web>
- AppLovin MCP setup and whitelist note: <https://support.applovin.com/en/growth/introduction/applovin-mcp>
- Official npm reference package: <https://www.npmjs.com/package/applovin-mcp>

The npm package metadata points to
`github.com/AppLovin/Campaign-Management-MCP`, but that repository was still not
publicly resolvable on 2026-08-25. Use the npm integrity-pinned artifact when
independently auditing the reference implementation.
