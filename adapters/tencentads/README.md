# Tencent Ads Marketing API v1.3 adapter

`tencentads/marketing-api-v1.3` provides an advertiser-scoped client for the
Tencent Ads Marketing API. It models paid `Advertiser`, `Campaign`, `AdGroup`,
`AdCreative`, and synchronous report workflows without treating advertising
resources as organic social posts.

## Contract

- Production API: `https://api.e.qq.com/v1.3`
- Sandbox API: `https://sandbox-api.e.qq.com/v1.3`
- API version: v1.3
- Business authentication: `access_token`, Unix `timestamp`, and random
  `nonce` query parameters on every request
- OAuth authorization: `https://developers.e.qq.com/oauth/authorize`
- OAuth token exchange: `https://api.e.qq.com/oauth/token` (no sandbox)
- Business response: `code`, `message`, optional `message_cn`, `errors`, and
  `data`
- Account boundary: one configured numeric `account_id` per client
- Creation safety: Campaign and AdGroup creation always sends
  `configured_status=AD_STATUS_SUSPEND`; activation is an explicit operation
- Transport safety: bounded responses, relative API paths, rejected redirects,
  sanitized errors, and `X-TSA-Trace-Id` preservation

Stable request fields are typed. Provider fields that depend on the campaign
goal, inventory, targeting, or creative template use each request's `Fields`
map. The adapter rejects invalid field names and prevents overrides of account
IDs, resource IDs, creation status, tokens, timestamps, nonces, and secrets.

## Configuration

```yaml
version: 1
platforms:
  - adapter: tencentads/marketing-api-v1.3
    product: marketing-api
    accounts:
      - id: cn-ads-primary
        app_id: "123456789"
        secret_ref: env://TENCENT_ADS_CLIENT_SECRET
        access_token_ref: env://TENCENT_ADS_ACCESS_TOKEN
        settings:
          account_id: 987654321
```

`app_id` and `secret_ref` are needed only by `Adapter.OAuth`. A client using a
caller-managed token still requires `access_token_ref` and `account_id`.

For sandbox business calls, set
`base_url: https://sandbox-api.e.qq.com/v1.3`. Do not point `token_base_url` at
the sandbox because Tencent Ads does not provide a sandbox OAuth token endpoint.

## Usage

```go
import (
    "context"

    "social-hub/adapters/tencentads"
)

func createCampaign(ctx context.Context, adapter *tencentads.Adapter) error {
    common, err := adapter.Client(ctx, "cn-ads-primary")
    if err != nil {
        return err
    }
    client := common.(*tencentads.Client)

    campaign, err := client.Campaigns().CreateCampaign(ctx, tencentads.CreateCampaignRequest{
        Name:               "Summer launch",
        CampaignType:       tencentads.CampaignTypeNormal,
        PromotedObjectType: tencentads.PromotedObjectLink,
        DailyBudget:        100000,
        Fields: map[string]any{
            "speed_mode": "SPEED_MODE_FAST",
        },
    })
    if err != nil {
        return err
    }

    // Creation is paused. Enabling is intentionally separate and explicit.
    return client.Campaigns().SetCampaignStatus(
        ctx, campaign.ID, tencentads.ConfiguredStatusNormal,
    )
}
```

Available typed workflows:

- `Advertisers`: read the configured advertiser account
- `Campaigns`: list, create paused, update, enable, and suspend
- `AdGroups`: list, create paused, update, enable, and suspend
- `AdCreatives`: list, create from a template, and update
- `Reports`: paged daily and hourly report reads
- `Adapter.OAuth`: build authorization URLs, exchange authorization codes, and
  refresh tokens

## Permissions and limits

Production use requires a Tencent Ads developer application, the relevant
Account Management, Ads Management, or Ads Insights scopes, and authorization
for the advertiser account. Capabilities report approval as `unknown` because
the granted scopes and advertiser relationship cannot be inferred reliably
from local configuration.

Tencent Ads applies application- and endpoint-specific request quotas. The
`X-RateLimit-Remaining` response header reports remaining daily and minute
quota percentages. Documented rate-limit business codes are mapped to
retryable `rate_limited` errors; minute and daily codes expose a conservative
retry delay when the provider defines one.

## Verification basis

The adapter contract was checked on 2026-08-09 against:

- [Tencent Ads request contract](https://s.apifox.cn/apidoc/docs-site/3515798/doc-3176251)
- [Tencent Ads response contract](https://s.apifox.cn/apidoc/docs-site/3515798/doc-3176255)
- [Tencent Ads API index](https://s.apifox.cn/apidoc/docs-site/3515798/doc-3176269)
- [Tencent Ads return codes](https://developers.e.qq.com/docs/reference/errorcode)
- [Official Go SDK v1.7.85](https://github.com/tencentad/marketing-api-go-sdk/tree/v1.7.85)

The official generated SDK is a contract reference rather than a Go dependency.
Its generated surface is much larger than this adapter and contains unbounded
response reads. social-hub keeps its existing bounded transport, redirect
policy, error model, and capability conventions.
