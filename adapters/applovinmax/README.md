# AppLovin MAX Reporting APIs adapter

`adapters/applovinmax` implements the current, unversioned AppLovin MAX
publisher reporting surface verified on 2026-08-25. It is registered as
`applovin/max-reporting-apis` and is read-only: this product reports mediation
and monetization data; it does not manage AppLovin Ads campaigns or MAX ad
units.

## Implemented workflows

| Workflow | Endpoint | SDK methods |
|---|---|---|
| Aggregated revenue | `GET /maxReport` | `RevenueReport`, `DownloadRevenueReport` |
| User-level revenue | `GET /max/userAdRevenueReport` | `RequestUserLevelReport`, `DownloadUserLevelReport` |
| Revenue cohort | `GET /maxCohort` | `CohortReport`, `DownloadCohortReport` |
| Impression cohort | `GET /maxCohort/imp` | `CohortReport`, `DownloadCohortReport` |
| Session cohort | `GET /maxCohort/session` | `CohortReport`, `DownloadCohortReport` |

JSON methods return typed rows whose values preserve AppLovin's exact decimal
text. CSV methods stream into a caller-provided `io.Writer` and enforce a
configurable decompressed-byte bound. JSON calls default to 1,000 rows and are
bounded at 10,000 rows; use the CSV download methods for larger reports.

## Authentication and configuration

Use the MAX **Report Key** from Account > General > Keys. The Report Key is not
the SDK Key, Management Key, Event Key, or Ad Review Key. Configure only a
`secret_ref`; the adapter resolves the value at runtime and sends it as the
documented `api_key` query parameter.

The API origin is fixed to `https://r.applovin.com`. Adapter-level endpoint
settings are rejected so the Report Key cannot be redirected to another host.
Credential-bearing clients reject redirects and discard cookie jars. Report
Key resolution errors expose neither resolver text, the secret reference, nor
any resolved value.

```yaml
version: 1
platforms:
  - adapter: applovin/max-reporting-apis
    product: max-reporting-apis
    accounts:
      - id: max-publisher-primary
        secret_ref: env://APPLOVIN_MAX_REPORT_KEY
```

```go
common, err := adapter.Client(ctx, "max-publisher-primary")
if err != nil {
    return err
}
client := common.(*applovinmax.Client)

report, err := client.RevenueReport(ctx, applovinmax.RevenueReportRequest{
    Start:   applovinmax.Date{Year: 2026, Month: time.August, Day: 1},
    End:     applovinmax.Date{Year: 2026, Month: time.August, Day: 7},
    Columns: []applovinmax.RevenueColumn{
        applovinmax.RevenueDay,
        applovinmax.RevenueApplication,
        applovinmax.RevenueImpressions,
        applovinmax.RevenueEstimatedRevenue,
    },
})
```

All MAX report dates are UTC. The adapter rejects dates outside the documented
rolling 45-day window. Hourly revenue columns are restricted to the latest 30
days. It also validates the documented incompatible `requests`, `attempts`,
`responses`, `fill_rate`, network, and placement combinations before sending a
request.

## User-level report safety

The user-level endpoint returns pre-signed CSV URLs. The production adapter
accepts downloads only from AppLovin's documented external-report S3 origin,
`https://applovin-externalreports.s3.amazonaws.com`, does not attach the Report
Key or cookies, and never follows redirects. Neither the API origin nor the
download origin is configurable.

User-level files contain advertising identifiers and optional internal user
IDs. Applications must apply access control, retention, regional privacy, and
log-redaction policies outside the SDK. The adapter never places a signed URL or
report body in an error message.

## API boundaries and limits

AppLovin documents a 45-day request window but does not publish a stable global
request-per-second quota for these endpoints. Use the shared rate limiter,
honor HTTP `429` and `Retry-After`, paginate with `limit`/`offset`, and do not
invent a fixed quota. Aggregated data from the latest one to two hours may be
incomplete; user-level data becomes available about eight hours after the UTC
day ends.

Only `socialhub.WithCallTimeout` is supported. Caller request IDs, idempotency
keys, and generic field selectors are rejected because these endpoints do not
document them; use the typed `Columns` fields for report selection. Provider
free-form error text and signed URLs are not exposed through SDK errors.

Official references:

- <https://support.applovin.com/en/max/max-dashboard/reports/reporting-apis>
- <https://support.applovin.com/en/max/reporting-apis/revenue-reporting-api>
- <https://support.applovin.com/en/max/reporting-apis/user-level-ad-revenue-api>
- <https://support.applovin.com/en/max/reporting-apis/cohort-api>
- <https://support.applovin.com/en/max/max-dashboard/account/account-info>
