# Google Analytics Data API v1beta adapter

Adapter name: `google/analytics-data-api-v1beta`

This adapter exposes GA4 property-bound aggregate measurement workflows for the
[Google Analytics Data API v1](https://developers.google.com/analytics/devguides/reporting/data/v1/rest).
The REST surface is `v1beta`; the contract was checked against Discovery
revision `20260804` on 2026-08-10.

## Supported workflows

- Property metadata and Core report compatibility checks.
- Core reports and batches of up to five Core reports.
- Realtime reports with up to two minute ranges.
- Pivot reports and batches of up to five Pivot reports.
- Typed dimensions, metrics, filters, ordering, comparisons, cohorts, quota
  responses, sampling metadata, and raw response preservation.

Each social-hub account binds to exactly one numeric GA4 Property ID. The
adapter always constructs the `properties/{id}` resource name and rejects
cross-property metadata. Report headers, visible metric columns, row widths,
fixed response `kind`, quota values, sampling values, batch cardinality, and
Pivot limits are validated before a response reaches the caller.

Audience Export operations are deliberately absent. They create persistent,
user-level exports that can contain device and user identifiers; this adapter
is intentionally bounded to aggregate property measurement.

## Authentication

Configure exactly one credential mode per account:

1. A caller-managed bearer token through `access_token_ref`.
2. User OAuth 2.0 Authorization Code with offline refresh through `client_id`,
   `secret_ref`, and `settings.refresh_token_ref`.
3. Service Account JWT Bearer through `settings.service_account_email` and
   `settings.private_key_ref`.

Supported scopes are:

```text
https://www.googleapis.com/auth/analytics.readonly
https://www.googleapis.com/auth/analytics
```

Managed credentials require one of those scopes in `approval.scopes`. The
read-only scope is sufficient for every workflow in this adapter. OAuth and
Service Account access tokens can be cached through `socialhub.TokenStore`.
Credential-bearing HTTP clients reject redirects.

Private keys, client secrets, refresh tokens, and access tokens are resolved
only through `socialhub.SecretResolver`; they are never accepted as plaintext
adapter settings. A Service Account must also be granted access to the target
GA4 Property. Possession of a valid signed token does not grant Property access.

User OAuth configuration:

```yaml
version: 1
platforms:
  - adapter: google/analytics-data-api-v1beta
    product: analytics-data-api
    accounts:
      - id: analytics-production
        client_id: google-oauth-client-id
        secret_ref: env://GOOGLE_OAUTH_CLIENT_SECRET
        approval:
          scopes:
            - https://www.googleapis.com/auth/analytics.readonly
        settings:
          property_id: "123456789"
          refresh_token_ref: env://GOOGLE_ANALYTICS_REFRESH_TOKEN
```

Service Account configuration:

```yaml
version: 1
platforms:
  - adapter: google/analytics-data-api-v1beta
    product: analytics-data-api
    accounts:
      - id: analytics-backend
        approval:
          scopes:
            - https://www.googleapis.com/auth/analytics.readonly
        settings:
          property_id: "123456789"
          service_account_email: reports@example-project.iam.gserviceaccount.com
          private_key_ref: env://GOOGLE_ANALYTICS_SERVICE_ACCOUNT_PRIVATE_KEY
```

## Usage

Import the package for registration:

```go
import _ "social-hub/adapters/analyticsdata"
```

Run a bounded Core report:

```go
client := common.(*analyticsdata.Client)

report, err := client.RunReport(ctx, analyticsdata.RunReportRequest{
    DateRanges: []analyticsdata.DateRange{{
        StartDate: "7daysAgo",
        EndDate:   "yesterday",
    }},
    Dimensions: []analyticsdata.Dimension{{Name: "country"}},
    Metrics:    []analyticsdata.Metric{{Name: "activeUsers"}},
    Limit:      1000,
    ReturnPropertyQuota: true,
})
```

`Limit: 0` uses Google's 10,000-row default. Any explicit limit is capped at
250,000. Batch methods accept one to five requests. Pivot limits are required,
and the product of all limits in one Pivot request cannot exceed 250,000.
Filters are bounded to eight levels and 100 expression nodes.

## Published standard-Property quotas

| Quota | Published limit |
|---|---:|
| Core tokens per Property per day | 200,000 |
| Core tokens per Property per hour | 40,000 |
| Core tokens per project/Property per hour | 14,000 |
| Realtime tokens per Property per day | 200,000 |
| Realtime tokens per Property per hour | 40,000 |
| Realtime tokens per project/Property per hour | 14,000 |
| Concurrent requests per Property | 10 |
| Server errors per project/Property per hour | 10 |
| Potentially thresholded requests per Property per hour | 120 |

Analytics 360 properties have higher token, concurrency, and server-error
quotas. Runtime `PropertyQuota`, `Retry-After`, `RESOURCE_EXHAUSTED`, Google
Cloud quota assignments, and Property tier remain authoritative. See Google's
[quota guide](https://developers.google.com/analytics/devguides/reporting/data/v1/quotas),
[report basics](https://developers.google.com/analytics/devguides/reporting/data/v1/basics),
and [Pivot guide](https://developers.google.com/analytics/devguides/reporting/data/v1/pivots).
