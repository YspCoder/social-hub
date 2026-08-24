# ironSource Ads Advertiser Reporting API v4 adapter

`adapters/ironsourcereporting` implements the four current ironSource Ads user
acquisition reporting endpoints under `/advertisers/v4/reports`.

Official references:

- Advertiser Reporting API v4: <https://docs.unity.com/en-us/grow/is-ads/user-acquisition/apis/reporting-api-v4>
- Cost API: <https://docs.unity.com/en-us/grow/is-ads/user-acquisition/apis/cost-api>
- SKAdNetwork Reporting API v4: <https://docs.unity.com/en-us/grow/is-ads/user-acquisition/apis/skan-reporting-api>
- ironSource Ads reports overview: <https://docs.unity.com/grow/is-ads/user-acquisition/reports/index>

The contract and its Unity-hosted documentation were reviewed on 2026-08-17.
On 2026-08-25, the legacy documentation routes remained in Unity Docs navigation
but returned a content-level not-found page. The adapter therefore retains the
last verified contract date rather than claiming a newer verification.

## Implemented reports

| Workflow | Endpoint | Important meaning |
|---|---|---|
| `AdvertiserReport` | `GET /advertisers/v4/reports` | Delivery statistics and installs reported by the configured MMP |
| `CostReport` | `GET /advertisers/v4/reports/cost` | Billable installs, `billable_spend`, and `ecpi` |
| `SKANReport` | `GET /advertisers/v4/reports/skan` | SKAdNetwork impressions, store opens, installs, and spend |
| `SKANConversionValues` | `GET /advertisers/v4/reports/skan/cv` | Dedicated conversion-value buckets `0..63` |

Each workflow has a typed JSON method and a bounded CSV streaming method. The
JSON row shape depends on requested metrics and breakdowns, so `ReportRow` is a
map of column names to `ReportValue`. `ReportValue` preserves exact JSON number
text and never converts spend through `float64`. SKAN conversion values use a
typed `map[uint8]int64` and reject bucket IDs outside `0..63`.

Do not use the ordinary advertiser report as a billing ledger. Unity explicitly
documents that its installs are supplied by the MMP. Use `CostReport` for
billable installs and spend.

## Authentication and access

API v4 requires a Bearer token provisioned for the ironSource advertiser
account. Keep the credential in a secret store and configure only its reference:

```yaml
version: 1
platforms:
  - adapter: ironsource/advertiser-reporting-api-v4
    product: advertiser-reporting-api
    accounts:
      - id: ironsource-ua-production
        access_token_ref: env://IRONSOURCE_ADVERTISER_BEARER_TOKEN
```

This adapter deliberately does not call the LevelPlay publisher token endpoint
`/partners/publisher/auth` on behalf of an advertiser account. Unity documents
that endpoint in the monetization product, while this adapter targets the
separate user-acquisition advertiser product. Rotate or refresh the advertiser
Bearer token in the configured secret provider according to the account's
provisioning contract.

Credentials are never placed in query parameters. The official HTTPS origin is
fixed to `https://api.ironsrc.com` and cannot be overridden through adapter
settings. Redirects are rejected and the supplied HTTP client's cookie jar is
disabled, so credentials and ambient cookies cannot be forwarded or attached.

## Typed usage

Import the package for registration, open the adapter, and query the desired
report surface:

```go
package main

import (
	"context"

	"social-hub/adapters/ironsourcereporting"
	"social-hub/pkg/socialhub"
)

func loadCost(ctx context.Context, config socialhub.AdapterConfig) (ironsourcereporting.ReportPage, error) {
	adapter, err := socialhub.Open(
		ctx,
		"ironsource/advertiser-reporting-api-v4",
		config,
	)
	if err != nil {
		return ironsourcereporting.ReportPage{}, err
	}
	defer adapter.Close()

	common, err := adapter.Client(ctx, "ironsource-ua-production")
	if err != nil {
		return ironsourcereporting.ReportPage{}, err
	}
	client := common.(*ironsourcereporting.Client)

	return client.Reports().CostReport(ctx, ironsourcereporting.CostReportRequest{
		Start: ironsourcereporting.Date("2026-08-01"),
		End:   ironsourcereporting.Date("2026-08-07"),
		Metrics: []ironsourcereporting.CostMetric{
			ironsourcereporting.CostMetricInstalls,
			ironsourcereporting.CostMetricBillableSpend,
			ironsourcereporting.CostMetricECPI,
		},
		Breakdowns: []ironsourcereporting.CostBreakdown{
			ironsourcereporting.CostBreakdownDay,
			ironsourcereporting.CostBreakdownCampaign,
			ironsourcereporting.CostBreakdownCountry,
		},
		Filters: ironsourcereporting.ReportFilters{
			Countries: []string{"CN", "US"},
		},
	})
}
```

When `HasMore` is true, repeat the same request with `Cursor` set to
`NextCursor`. The SDK extracts only the opaque cursor from a same-origin,
same-endpoint `paging.next` URL. It never follows a server-supplied absolute URL.

For large exports, call the corresponding `Download*CSV` method with an
`io.Writer`. Downloads default to a 256 MiB output bound, validate a unique CSV
header and consistent column counts, reject content-encoding surprises, and
surface the next cursor from the same-origin `Link` header. A failed validation
can leave a partial stream in the writer; write to a temporary object and
promote it only after the method succeeds.

## Limits and contract boundaries

- The documented default page size is 10,000 and maximum is 250,000 rows for
  advertiser and Cost reports. The SDK applies the same 250,000 safety ceiling
  to the two SKAN endpoints and recommends CSV for high-cardinality output.
- Advertiser Reporting API v4 limits `endDate` to within three calendar months
  of `startDate`; the adapter enforces this locally.
- The documented request rate is 100 requests per minute per endpoint.
  `DefaultQuotaPolicy` records this contract for a shared limiter.
- HTTP 429 is classified as retryable and preserves bounded retry/quota headers.
  Authentication and permission failures require user action.
- Only `socialhub.WithCallTimeout` is supported per request. Caller request IDs,
  idempotency keys, and field-selection options are rejected because the
  reporting contract does not document them.
- Provider error messages are not exposed because they are untrusted free text.
  The adapter preserves only a bounded scalar provider code, safe request/quota
  headers, HTTP status, and a fixed platform message.
- Request filters are encoded into a GET URL. The SDK rejects encoded queries
  above 64 KiB; split very large filter sets across calls.
- This adapter is read-only. Campaign, bid, audience, country-group, and creative
  management belong in a separate advertiser management adapter rather than
  being presented as organic social publishing.
