# AppLovin Growth Reporting APIs adapter

`adapters/applovinreporting` implements AppLovin Axon advertiser reporting. It
is registered as `applovin/growth-reporting-apis` and is intentionally separate
from both `applovin/growth-campaign-management-api-v1` and
`applovin/max-reporting-apis`:

- Growth Reporting reads advertiser Campaign and creative performance.
- Growth Campaign Management mutates Campaigns, Creative Sets, and Assets.
- MAX Reporting reads publisher mediation revenue, user-level revenue, and
  cohorts from the separate `/max*` endpoint family.

The contract was verified on 2026-08-25 against the current APP, WEB, Asset,
and HTML Metrics documentation and the official MIT-licensed
`applovin-mcp@0.0.8` package. The npm artifact was used as a typed contract
reference only and is not a Go dependency.

## Implemented endpoints

| Endpoint | Account | Window | SDK methods |
|---|---|---:|---|
| `GET /accountInfo` | APP/WEB | n/a | `AccountInfo` |
| `GET /report?report_type=advertiser` | APP | 45 days | `CampaignReport`, `DownloadCampaignCSV` |
| `GET /report?report_type=advertiser` | WEB | 90 days | `CampaignReport`, `DownloadCampaignCSV` |
| `GET /report?report_type=publisher` | APP only | 45 days | `CampaignReport`, `DownloadCampaignCSV` |
| `GET /assetReport` | APP/WEB | named range | `AssetReport`, `DownloadAssetCSV` |
| `GET /assetAnalyticsReport` | APP/WEB | 45 days | `AssetReport`, `DownloadAssetCSV` |
| `GET /playableMetrics` | APP only | 45 days | `PlayableReport`, `DownloadPlayableCSV` |

Publisher `/report` data is a separate business domain from advertiser spend.
Do not select `ReportPublisher` for normal Campaign, ROAS, spend, or creative
analysis. Use it only when the account is also acting as an AppLovin publisher
and publisher revenue was explicitly requested.

## Authentication and account binding

Use the **Reporting API Key** from Ads Manager Account > API keys. Only account
Admins can view keys. Configure it as `secret_ref`; the adapter resolves it at
runtime and sends it only as the documented `api_key` query parameter. Its HTTP
client always targets `https://r.applovin.com`, does not follow redirects, and
does not use a cookie jar, so configuration cannot forward the query key to
another origin.

APP and WEB accounts have different columns and request windows. Configure each
key as a separate social-hub account:

```yaml
version: 1
platforms:
  - adapter: applovin/growth-reporting-apis
    product: growth-reporting-apis
    accounts:
      - id: applovin-growth-app
        secret_ref: env://APPLOVIN_APP_REPORTING_API_KEY
        settings:
          account_id: "123456"
          account_type: APP
      - id: applovin-growth-web
        secret_ref: env://APPLOVIN_WEB_REPORTING_API_KEY
        settings:
          account_id: "654321"
          account_type: WEB
```

Call `AccountInfo` during credential setup or health checking. The endpoint
returns the numeric ID for WEB keys, so the adapter verifies both type and ID.
For APP keys it returns the documented-by-official-MCP `not a web account`
response; the adapter can verify APP type but reports `AccountIDVerified=false`
because AppLovin does not disclose the numeric ID in that response.

## Typed usage

```go
report, err := client.Reports().CampaignReport(ctx,
    applovinreporting.CampaignReportRequest{
        Type:  applovinreporting.ReportAdvertiser,
        Start: "2026-08-01",
        End:   "2026-08-07",
        Columns: []applovinreporting.CampaignColumn{
            applovinreporting.CampaignColumnDay,
            applovinreporting.CampaignColumnCampaign,
            applovinreporting.CampaignColumnCost,
        },
        Filters: []applovinreporting.CampaignFilter{{
            Column: applovinreporting.CampaignColumnCountry,
            Values: []string{"US", "CA"},
        }},
        Sorts: []applovinreporting.CampaignSort{{
            Column: applovinreporting.CampaignColumnCost,
            Order:  applovinreporting.SortDescending,
        }},
        Attribution: applovinreporting.AttributionCohort,
    },
)
```

The zero report type defaults to advertiser. The zero attribution mode defaults
to cohort reporting, which sends `day_column=day`; select
`AttributionRealtime` for event-day Campaign metrics. Playable reporting uses
the inverse endpoint convention and sends `day_column=event_day` only for
realtime queries.

Use `CampaignColumns(accountType, reportType)`, `AssetColumns(accountType)`, and
`PlayableColumns()` to discover the complete typed column contract. Validation
rejects columns belonging to another account or report type before network
I/O. This includes the current WEB D0/D7 new-customer fields and the checkout
columns verified by the official MCP reference package.

`ReportValue` preserves numeric text exactly and distinguishes JSON `null`.
Money, ratios, ROAS, and large counts are never coerced through `float64`.

## CSV streaming and errors

JSON calls are bounded by the shared 8 MiB transport and a maximum page size of
500 rows. CSV methods stream into a caller-provided `io.Writer`, default to a
256 MiB bound, and validate the header order and every record while streaming.
Use pagination for larger result sets.

Streaming validation is not transactional: malformed input or a later writer
failure can leave partial bytes in the destination. When an atomic file is
required, write to a temporary file and rename it only after the method returns
successfully.

`APIError` preserves only a fixed platform message, Trace ID, HTTP status, and
`Retry-After`. Report Keys and provider free-form error text never appear in
`Error()`. AppLovin does not publish a stable global request-per-second limit
for Growth Reporting; coordinate calls through the shared limiter and honor
HTTP `429` rather than inventing a quota. Of the shared call options, only
`socialhub.WithCallTimeout` is accepted; caller request IDs, idempotency keys,
and generic field selectors have no documented meaning for these endpoints.

Official references:

- APP Reporting API: <https://support.applovin.com/en/growth/promoting-your-apps/api/reporting-api>
- WEB Reporting API: <https://support.applovin.com/en/growth/promoting-your-websites/api/reporting-api>
- Asset Reporting API: <https://support.applovin.com/en/growth/promoting-your-apps/api/asset-reporting-api>
- HTML Metrics API: <https://support.applovin.com/en/growth/promoting-your-apps/api/html-metrics-api>
- AppLovin MCP setup: <https://support.applovin.com/en/growth/introduction/applovin-mcp>
- Official npm reference package: <https://www.npmjs.com/package/applovin-mcp>
