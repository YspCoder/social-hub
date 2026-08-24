# Campaign Manager 360 API v5 adapter

`adapters/cm360` implements advertiser-bound Campaign Manager 360 (CM360)
trafficking and reporting workflows. It is separate from `adapters/dv360`:
CM360 manages ad serving, placements, ads, Floodlight-backed reporting, and
Report Builder files; DV360 manages programmatic buying resources.

Adapter identifier:

```text
google/campaign-manager-360-api-v5
```

## Contract baseline

The adapter was verified on 2026-08-09 against:

- [Campaign Manager 360 REST API v5](https://developers.google.com/doubleclick-advertisers/rest/v5)
- [Discovery document revision 20260721](https://dfareporting.googleapis.com/$discovery/rest?version=v5)
- [Google API Go client](https://github.com/googleapis/google-api-go-client/tree/68555327f8bf789f35a7e0d0ebfe9fac16530a68/dfareporting/v5)
- [OAuth2 authorization guide](https://developers.google.com/doubleclick-advertisers/authorizing)
- [ReportData query guide](https://developers.google.com/doubleclick-advertisers/guides/query_report_data)
- [Report execution guide](https://developers.google.com/doubleclick-advertisers/guides/run_reports)
- [Report file download guide](https://developers.google.com/doubleclick-advertisers/guides/download_reports)
- [Published quota defaults](https://developers.google.com/doubleclick-advertisers/quotas)

The official generated client is used as a contract reference, not a runtime
dependency. This keeps social-hub's opt-in adapter dependency surface small.

## Supported workflows

| Resource | Operations | Scope |
|---|---|---|
| User Profile | Get the configured profile | Any CM360 OAuth scope |
| Advertiser | Get the configured advertiser | `dfatrafficking` |
| Campaign | Get, list, archived-first create, patch | `dfatrafficking` |
| Placement | Get and advertiser-filtered list | `dfatrafficking` |
| Ad | Get and advertiser-filtered list | `dfatrafficking` |
| ReportData | Typed, paginated synchronous query | `dfareporting` |
| Report | Get, list, run existing reports | `dfareporting` |
| Report File | Get/list metadata and bounded Range download | `dfareporting` |

Placement and Ad creation are deliberately not exposed in this first contract.
Those resources require large, type-specific nested trafficking payloads. A
partial pass-through model would undermine type safety and could accidentally
serve an incomplete ad.

## Access requirements

The caller needs:

1. A Campaign Manager 360 account with API access enabled.
2. A CM360 user profile with the required UI permissions.
3. A Google Cloud project with the Campaign Manager 360 API enabled.
4. OAuth2 credentials and user consent for the required scopes.

CM360 has no separate API permission layer: API access follows the configured
user profile's CM360 UI permissions.

Supported scopes are:

```text
https://www.googleapis.com/auth/dfatrafficking
https://www.googleapis.com/auth/dfareporting
https://www.googleapis.com/auth/ddmconversions
```

The adapter currently implements the first two capability groups. The
conversion scope is accepted for profile access and future offline conversion
workflows, but no conversion mutation is exposed yet.

## Configuration

One social-hub account binds one CM360 user profile to one advertiser. Use
multiple account entries when one application manages multiple profiles or
advertisers.

```yaml
adapters:
  - adapter: google/campaign-manager-360-api-v5
    product: campaign-manager-360-api
    accounts:
      - id: agency-brand-a
        client_id: ${GOOGLE_CLIENT_ID}
        secret_ref: env://GOOGLE_CLIENT_SECRET
        approval:
          scopes:
            - https://www.googleapis.com/auth/dfatrafficking
            - https://www.googleapis.com/auth/dfareporting
        settings:
          profile_id: "1234567"
          advertiser_id: "7654321"
          refresh_token_ref: env://CM360_REFRESH_TOKEN
```

For caller-managed token rotation, set `access_token_ref` and omit
`refresh_token_ref`. `client_id` and `secret_ref` may remain configured together
when the application also uses `Adapter.OAuth` to obtain replacement consent.

Settings overrides are available for controlled gateways and tests:

```yaml
settings:
  base_url: https://dfareporting.googleapis.com/dfareporting/v5
  auth_url: https://accounts.google.com/o/oauth2/v2/auth
  token_url: https://oauth2.googleapis.com/token
```

## Safety invariants

- `CreateCampaign` always sends `archived: true`; callers cannot override it.
- Every Campaign, Placement, and Ad response must match the configured
  `advertiser_id`, or the adapter returns `permission_denied`.
- Every `QueryReportData` request contains an exact `advertiser` filter for
  the configured advertiser. Conflicting filters are rejected before network
  I/O.
- List operations are bounded to 1,000 resources per request and return the
  platform page token instead of auto-draining an account.
- Report downloads require an inclusive byte range of at most 8 MiB. Callers
  can assemble large files without allowing an unbounded allocation.
- Redirects are not followed by the adapter's credential-bearing HTTP client.

Unarchiving an existing Campaign is an explicit `UpdateCampaign` operation. It
may make downstream trafficking eligible to serve, so production applications
should place an approval step before setting `Archived` to `false`.

## Usage

Import the adapter for registration:

```go
import _ "social-hub/adapters/cm360"
```

After opening the configured adapter, use its typed client:

```go
client := common.(*cm360.Client)

campaign, err := client.CreateCampaign(ctx, cm360.CreateCampaignRequest{
    Name:      "Autumn launch",
    StartDate: "2026-09-01",
    EndDate:   "2026-10-31",
})
// campaign.Archived is guaranteed true on a successful return.
```

Query report data without exposing other advertisers accessible to the same
profile:

```go
page, err := client.QueryReportData(ctx, cm360.ReportDataQueryRequest{
    DateRange:      cm360.DateRange{RelativeDateRange: "LAST_7_DAYS"},
    DimensionNames: []string{"campaign"},
    MetricNames:    []string{"impressions", "clicks"},
    MaxResults:     500,
})
```

Download a report file in bounded chunks:

```go
result, err := client.DownloadReportFileRange(
    ctx,
    reportID,
    fileID,
    cm360.ByteRange{Start: 0, End: (8 << 20) - 1},
    destination,
)
```

`RunReport(..., synchronous=true)` is only a request for synchronous execution.
CM360 can move a long-running report into the asynchronous queue. Always inspect
the returned `ReportFile.Status` and poll with exponential backoff until it is
`REPORT_AVAILABLE`.

## Quotas

`DefaultQuotaPolicy` exposes the published defaults:

| Limit | Default |
|---|---:|
| Requests per project per day | 50,000 |
| Queries per project per second | 1 |
| Maximum configurable project QPS | 10 |
| ReportData queries per user per minute | 120 |
| ReportData queries per project per day | 10,000 |
| ReportData execution timeout | 60 seconds |
| Recommended concurrent writes | 1 |

Report Builder also has account-specific quotas. Treat these values as initial
limiter settings and honor `Retry-After` plus Google reasons such as
`userRateLimitExceeded`, `quotaExceeded`, and `dailyLimitExceeded`.

## Errors

Google's legacy `error.errors[].reason` envelope and canonical RPC `status` are
mapped to `socialhub.Error`. The sanitized original envelope remains available
through `*cm360.APIError`. Authentication, approval, invalid argument,
not-found, quota, and transient backend failures retain their retry/user-action
classification.
