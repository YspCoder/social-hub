# Google Search Ads 360 Reporting API v0

Package `social-hub/adapters/searchads360` implements the complete REST surface
published by the Search Ads 360 Reporting API v0 Discovery document. Importing
the package registers `google/search-ads-360-reporting-api-v0`.

The adapter is intentionally separate from `googleads/api-v25`, DV360, and
CM360 because these products have different customer hierarchies, permissions,
resource models, and quota accounting.

## Supported operations

- `customers.listAccessibleCustomers`;
- paginated `customers.searchAds360.search` reports;
- `customers.customColumns.list` and `.get`;
- `searchAds360Fields.search` and `.get`.

`SearchStream` is available through Google's gRPC API, but is not exposed by
the v0 REST Discovery document or the generated official Go REST client. This
adapter therefore does not invent an HTTP streaming route.

## Configuration

The OAuth principal must have access to the configured customer. Both customer
IDs and `login_customer_id` values are 10 digits supplied without hyphens.

```yaml
version: 1
platforms:
  - adapter: google/search-ads-360-reporting-api-v0
    product: reporting-api
    accounts:
      - id: sa360-main
        client_id: 123456789.apps.googleusercontent.com
        secret_ref: secret://google/sa360/client-secret
        access_token_ref: secret://google/sa360/access-token
        approval:
          scopes:
            - https://www.googleapis.com/auth/doubleclicksearch
        settings:
          customer_id: "1234567890"
          login_customer_id: "9876543210"
```

`login_customer_id` is required when a manager account accesses a sub-manager
or client account. It can be omitted for direct customer access. The API does
not use a Google Ads developer token. API and OAuth origins are fixed to the
official HTTPS endpoints; adapter-level settings are rejected.

## Report query

```go
import (
    "context"

    "social-hub/adapters/searchads360"
    "social-hub/pkg/socialhub"
)

func report(ctx context.Context, adapter socialhub.Adapter) error {
    common, err := adapter.Client(ctx, "sa360-main")
    if err != nil {
        return err
    }
    client := common.(*searchads360.Client)
    page, err := client.Reports().Search(ctx, searchads360.SearchRequest{
        Query: `SELECT campaign.id, campaign.name, metrics.clicks, metrics.cost_micros
                FROM campaign
                WHERE segments.date DURING LAST_7_DAYS`,
        PageSize:                5000,
        ReturnTotalResultsCount: true,
    })
    if err != nil {
        return err
    }
    for _, row := range page.Rows {
        var campaign struct {
            ID   string `json:"id"`
            Name string `json:"name"`
        }
        if err := row.DecodeField("campaign", &campaign); err != nil {
            return err
        }
    }
    return nil
}
```

Rows retain each top-level resource, `metrics`, `segments`, and
`customColumns` value as `json.RawMessage`. This preserves int64, micros, and
decimal values exactly and avoids dropping newly added v0 fields.

## OAuth and operational boundaries

- `Adapter.OAuth` supports Google's Authorization Code flow and refresh-token
  exchange using `https://www.googleapis.com/auth/doubleclicksearch`. The
  helper returns tokens but does not persist them or rotate a configured client
  automatically; store the returned token through the application's secret
  lifecycle before creating or replacing a client.
- The OAuth helper uses `https://accounts.google.com/o/oauth2/v2/auth` and
  `https://oauth2.googleapis.com/token`, requires HTTPS callbacks except for
  loopback development callbacks, rejects redirects, and never reuses the
  caller's Cookie Jar.
- Google OAuth verification is required for applications requesting the
  Search Ads 360 Reporting scope. The Cloud project must enable this API, and
  the user still needs Search Ads 360 account access.
- REST pages contain at most 10,000 rows. A valid continuation page is free
  against the daily operation quota; an invalid or expired token is charged.
- Current published query quotas are 3,000 queries/minute/project/user, 3,000
  queries/minute/project, and 150,000 queries/day/project.
- API failures can still consume daily quota. `RESOURCE_EXHAUSTED` maps to a
  retryable rate-limit error; an explicit daily-limit code requires user
  action.
- The shared transport bounds each REST response at 8 MiB. Select fewer fields
  or reduce `page_size` if a dense page exceeds that SDK safety limit.
- Only `socialhub.WithCallTimeout` is supported. Caller request IDs,
  idempotency keys, and generic field selection are rejected; select fields in
  the typed Search Ads 360 query request.
- Report requests are read-only. Campaign mutations, conversions, and report
  delivery webhooks are outside this API product.

## Official and mature references

- [REST v0 reference](https://developers.google.com/search-ads/reporting/api/reference/rest)
- [Discovery document](https://searchads360.googleapis.com/$discovery/rest?version=v0), revision `20260820`
- [Call structure](https://developers.google.com/search-ads/reporting/concepts/call-structure)
- [Pricing, limits, and quotas](https://developers.google.com/search-ads/reporting/concepts/quotas)
- [Official generated Go REST client](https://github.com/googleapis/google-api-go-client/tree/main/searchads360/v0)
- [Google OAuth 2.0 web-server flow](https://developers.google.com/identity/protocols/oauth2/web-server)

The v0 Discovery document, call structure, quotas, and Google OAuth web-server
flow were reviewed on 2026-08-25.
