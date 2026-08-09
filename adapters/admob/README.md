# Google AdMob API v1 adapter

`adapters/admob` implements the complete stable AdMob API v1 REST surface verified
against Discovery revision `20260731`:

| Workflow | REST methods |
|---|---|
| Accounts | `accounts.list`, `accounts.get` |
| Inventory | `accounts.apps.list`, `accounts.adUnits.list` |
| Reporting | `accounts.networkReport.generate`, `accounts.mediationReport.generate` |

The adapter is registered as `google/admob-api-v1`. It intentionally excludes
`v1beta` application creation, ad-source mappings, Mediation Groups, experiments,
and Campaign Reports.

## Authorization and account binding

AdMob documents OAuth2 Desktop and Web application flows that act on behalf of an
AdMob user. This adapter supports either a caller-managed bearer token or the
Google authorization-code flow with a refresh token and optional
`socialhub.TokenStore`. It does not implement Service Account JWT credentials.

Each social-hub account is bound to one AdMob publisher resource through
`publisher_id`. This prevents inventory and report responses from being accepted
for a different publisher. `ListAccounts` is the stable-v1 discovery exception:
it explicitly returns every publisher account visible to the configured OAuth
credential.

```go
config := socialhub.AdapterConfig{
    Adapter: "google/admob-api-v1",
    Product: "admob-api",
    Accounts: []socialhub.AccountConfig{{
        ID:        "mobile-publisher",
        ClientID:  os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
        SecretRef: "env://GOOGLE_OAUTH_CLIENT_SECRET",
        Approval: socialhub.ApprovalConfig{Scopes: []string{
            "https://www.googleapis.com/auth/admob.readonly",
        }},
        Settings: map[string]any{
            "publisher_id":     "pub-1234567890123456",
            "refresh_token_ref": "env://GOOGLE_ADMOB_REFRESH_TOKEN",
        },
    }},
}

adapter := &admob.Adapter{}
if err := adapter.Init(ctx, config,
    socialhub.WithSecretResolver(secretResolver),
    socialhub.WithTokenStore(tokenStore),
); err != nil {
    return err
}
common, err := adapter.Client(ctx, "mobile-publisher")
if err != nil {
    return err
}
client := common.(*admob.Client)
```

Use `adapter.OAuth(ctx, accountID)` to build an authorization URL, exchange the
authorization code, and obtain the offline refresh token referenced by the
configuration. The narrower `admob.report` scope authorizes account metadata and
reports only; app and ad-unit inventory requires `admob.readonly`.

## Inventory and reports

```go
apps, err := client.ListApps(ctx, admob.ListRequest{PageSize: 100})
if err != nil {
    return err
}

report, err := client.GenerateNetworkReport(ctx, admob.NetworkReportSpec{
    DateRange: admob.DateRange{
        StartDate: admob.Date{Year: 2026, Month: 8, Day: 1},
        EndDate:   admob.Date{Year: 2026, Month: 8, Day: 7},
    },
    Dimensions: []admob.Dimension{admob.DimensionDate, admob.DimensionApp},
    Metrics: []admob.Metric{
        admob.MetricImpressions,
        admob.MetricEstimatedEarnings,
    },
    LocalizationSettings: &admob.LocalizationSettings{
        CurrencyCode: "USD",
        LanguageCode: "en-US",
    },
    MaxReportRows: 10_000,
})
if err != nil {
    return err
}
```

Network reports model AdMob Network performance. Mediation reports additionally
support third-party ad-source, ad-source-instance, and Mediation Group dimensions,
plus `OBSERVED_ECPM`. Typed validation rejects dimensions, metrics, filters, sort
conditions, and documented incompatible combinations before a network request.

The API returns reports as a top-level JSON array containing one header, zero or
more rows, and one footer. The adapter decodes that stream incrementally with a
bounded reader, accepts at most 100,000 rows, validates the response order and
requested field sets, and preserves metric values as an integer, micros, or
double oneof. `footer.matchingRowCount` is exposed but is not assumed to equal the
number of returned rows, as the API can truncate a report.

## Published quota policy

`DefaultQuotaPolicy()` exposes the documented limits:

| Quota category | Limit |
|---|---:|
| Account reads | 900 requests/minute/project |
| Inventory reads | 120 requests/minute/project |
| Inventory reads | 172,800 requests/day/project |
| Reporting reads | 900 requests/minute/project |
| Apps/ad units default page | 10,000 resources |
| Apps/ad units maximum page | 20,000 resources |
| Report rows | 100,000 rows |

AdMob can also enforce dynamic back-end processing cost caps. Treat
`RESOURCE_EXHAUSTED` as retryable, rate-limit each quota category separately, and
reduce report date ranges or dimensions when a report exceeds processing limits.

Official references:

- <https://developers.google.com/admob/api/reference/rest>
- <https://developers.google.com/admob/api/v1/auth>
- <https://developers.google.com/admob/api/v1/report-overview>
- <https://developers.google.com/admob/api/v1/report-metrics-dimensions>
- <https://developers.google.com/admob/api/quotas>
