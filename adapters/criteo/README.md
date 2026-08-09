# Criteo Marketing Solutions adapter

`adapters/criteo` implements the stable Criteo Marketing Solutions API
`2026-01` contract. It is advertiser-scoped and intentionally separate from
social-hub's organic publishing interfaces.

Official contract:

- API and changelog: <https://developers.criteo.com/marketing-solutions/>
- OpenAPI: <https://api.criteo.com/2026-01/marketingsolutions/open-api-specifications.json>
- OAuth credentials: <https://developers.criteo.com/marketing-solutions/docs/get-api-credentials>
- Campaigns: <https://developers.criteo.com/marketing-solutions/docs/campaign>
- Ad Sets: <https://developers.criteo.com/marketing-solutions/docs/ad-set>
- Analytics: <https://developers.criteo.com/marketing-solutions/docs/analytics>
- Rate limits: <https://developers.criteo.com/marketing-solutions/docs/rate-limits>

## Implemented workflows

- Advertiser portfolio lookup and configured-advertiser validation
- Campaign get/search/create and the supported spend-limit, scheduled-limit,
  and budget-automation PATCH surface
- Ad Set get/search/create, supported PATCH fields, and start/stop
- Advertiser-scoped Statistics reports in the API's default JSON format
- Static bearer tokens and managed OAuth 2.0 Client Credentials
- Typed Criteo problems, partial-error rejection, request IDs, retry hints, and
  `x-ratelimit-*` metadata

Creative, audience, catalog, and non-JSON report endpoints are not part of this
adapter version. Organic publish/fetch/media/reaction/message/webhook
capabilities are explicitly unsupported.

## Access requirements

Criteo must provision API credentials and the required application scopes:

- `MarketingSolutions_Campaign_Read`
- `MarketingSolutions_Campaign_Manage`
- `MarketingSolutions_Analytics_Read`

The configured `advertiser_id` must appear in `GET /advertisers/me`. The SDK
also verifies advertiser and Campaign ownership before mutations. Criteo access
approval, advertiser permissions, billing readiness, datasets, and eligible
objectives remain external prerequisites.

## Configuration

Managed Client Credentials obtains a token from
`https://api.criteo.com/oauth2/token`, caches it, serializes concurrent refresh,
and refreshes about two minutes before expiry. Criteo tokens commonly last 900
seconds and do not use refresh tokens for this grant.

```yaml
version: 1
platforms:
  - adapter: criteo/marketing-solutions-api-2026-01
    product: marketing-solutions-api
    accounts:
      - id: criteo-primary
        client_id: ${CRITEO_CLIENT_ID}
        secret_ref: env://CRITEO_CLIENT_SECRET
        settings:
          advertiser_id: "12345"
```

For a caller-managed Authorization Code token, replace `client_id` and
`secret_ref` with `access_token_ref`. Credential values are always resolved via
`socialhub.SecretResolver`; configuration contains references only.

## Usage

Import the adapter for registration, initialize it through `socialhub.Open`,
then use the typed client workflows:

```go
import (
    "context"

    "social-hub/adapters/criteo"
    "social-hub/pkg/socialhub"
)

func createCampaign(ctx context.Context, adapter socialhub.Adapter) error {
    common, err := adapter.Client(ctx, "criteo-primary")
    if err != nil {
        return err
    }
    client := common.(*criteo.Client)

    amount := 100.0
    _, err = client.CreateCampaign(ctx, criteo.CreateCampaignRequest{
        Name: "Acquisition", Goal: criteo.GoalAcquisition,
        SpendLimit: criteo.CreateCampaignSpendLimit{
            Type: criteo.SpendLimitCapped,
            Amount: &amount,
            Renewal: criteo.RenewalMonthly,
        },
    })
    return err
}
```

`CreateAdSet` verifies the parent Campaign belongs to the configured advertiser.
It only accepts a create response whose `activationStatus` is `off` and whose
`deliveryStatus` is `draft`. `StartAdSet` rejects ended or archived resources,
then reads the resource again to prove activation; `StopAdSet` similarly proves
the final `off` state.

Statistics dimensions and metrics produce dynamic columns. `Report` therefore
returns validated `json.RawMessage` instead of inventing a fixed row schema.
CSV, XML, and XLSX are deliberately not exposed by this version.

## Rate limits and retries

Criteo documents 250 calls per minute per application for Client Credentials
and 10 calls per minute per account for Authorization Code tokens. Responses
may include `x-ratelimit-limit`, `x-ratelimit-remaining`, and
`x-ratelimit-reset`; `APIError` preserves these values.

For `429`, Criteo recommends short exponential delays such as 1 and 2 seconds.
For `500`/`503`, the documented sequence is 10, 20, and 40 seconds. Use the
shared social-hub retry layer only for errors where `Retryable()` is true, cap
attempts, add jitter, and keep mutation idempotency under caller control.
