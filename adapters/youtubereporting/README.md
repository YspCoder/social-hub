# YouTube Reporting API v1 adapter

Adapter name: `google/youtube-reporting-api-v1`

This adapter exposes asynchronous bulk-report workflows for the
[YouTube Reporting API v1](https://developers.google.com/youtube/reporting/v1/reference/rest/).
The contract was checked against Discovery revision `20260805` on 2026-08-10.
It is intentionally separate from `youtube/data-v3` and
`google/youtube-analytics-api-v2`: those adapters publish and inspect YouTube
resources or execute synchronous targeted queries, while this package manages
daily bulk-report jobs and downloads versioned CSV files.

## Supported workflows

- List channel or Content Owner report types, including system-managed types.
- Create, get, list, and delete user-managed reporting jobs.
- List report instances with creation and data-period timestamp filters.
- Get report metadata and download the corresponding CSV.
- Preserve raw resource JSON while validating IDs, timestamps, job ownership,
  pagination tokens, duplicate resources, and response cardinality.

The package covers the complete eight-method REST surface:

```text
GET    /v1/reportTypes
POST   /v1/jobs
GET    /v1/jobs
GET    /v1/jobs/{jobId}
DELETE /v1/jobs/{jobId}
GET    /v1/jobs/{jobId}/reports
GET    /v1/jobs/{jobId}/reports/{reportId}
GET    /v1/media/{+resourceName}?alt=media
```

System-managed jobs are available only to eligible Content Owners and cannot
be created or deleted. Runtime authorization remains authoritative.

## Authentication and account binding

Supported scopes are:

```text
https://www.googleapis.com/auth/yt-analytics.readonly
https://www.googleapis.com/auth/yt-analytics-monetary.readonly
```

Configure exactly one credential mode per account:

1. A caller-managed bearer token through `access_token_ref`.
2. User OAuth 2.0 Authorization Code with offline refresh through `client_id`,
   `secret_ref`, and `settings.refresh_token_ref`.
3. Service Account JWT Bearer through `settings.service_account_email` and
   `settings.private_key_ref`.

When `settings.content_owner_id` is empty, the account is bound to the current
OAuth user's Channel. Setting it binds every API request to one YouTube CMS
Content Owner through `onBehalfOfContentOwner`. Service Accounts are accepted
only for Content Owner bindings, matching Google's documented restriction;
they cannot access a standalone Channel.

OAuth and Service Account access tokens can be cached through
`socialhub.TokenStore`. Client secrets, refresh tokens, private keys, and
access tokens are resolved only through `socialhub.SecretResolver`.
Credential-bearing HTTP clients do not follow redirects.

Channel user-OAuth configuration:

```yaml
version: 1
platforms:
  - adapter: google/youtube-reporting-api-v1
    product: youtube-reporting-api
    accounts:
      - id: youtube-channel-reports
        client_id: google-oauth-client-id
        secret_ref: env://GOOGLE_OAUTH_CLIENT_SECRET
        approval:
          scopes:
            - https://www.googleapis.com/auth/yt-analytics.readonly
        settings:
          refresh_token_ref: env://YOUTUBE_REPORTING_REFRESH_TOKEN
```

Content Owner Service Account configuration:

```yaml
version: 1
platforms:
  - adapter: google/youtube-reporting-api-v1
    product: youtube-reporting-api
    accounts:
      - id: youtube-cms-reports
        approval:
          scopes:
            - https://www.googleapis.com/auth/yt-analytics-monetary.readonly
        settings:
          content_owner_id: CONTENT_OWNER_ID
          service_account_email: reports@example-project.iam.gserviceaccount.com
          private_key_ref: env://YOUTUBE_REPORTING_SERVICE_ACCOUNT_PRIVATE_KEY
```

## Usage

Import the package for registration:

```go
import _ "social-hub/adapters/youtubereporting"
```

Create a job and retrieve its reports:

```go
client := common.(*youtubereporting.Client)

job, err := client.CreateJob(ctx, youtubereporting.CreateJobInput{
    ReportTypeID: "channel_basic_a3",
    Name:         "daily-channel-activity",
})
if err != nil {
    return err
}

reports, err := client.ListReports(ctx, job.ID, youtubereporting.ListReportsRequest{
    CreatedAfter: lastProcessedAt,
})
```

Download one CSV with an explicit decompressed-size bound:

```go
result, err := client.DownloadReport(
    ctx,
    job.ID,
    reports.Items[0].ID,
    output,
    youtubereporting.DownloadOptions{
        MaxBytes: 64 << 20,
        Gzip:     true,
    },
)
```

`DownloadReport` re-fetches report metadata and does not accept an arbitrary
URL. It requires the download URL to use the configured Reporting API origin,
the `/v1/media/` path, and only `alt=media`; redirects are rejected. Output is
streamed and bounded after optional gzip decompression. A zero `MaxBytes` uses
the 256 MiB default.

## Lifecycle and quota notes

- The first report can take up to 48 hours after job creation.
- Each normal report covers one 24-hour period and reports are updated daily.
- Normal reports remain available for 60 days; historical reports generated
  for a new job remain available for 30 days.
- Backfill reports use a new report ID and replace earlier data for the same
  `startTime` and `endTime`; consumers should retain the newest `createTime`.
- Content Owners can receive eligible system-managed revenue and metadata
  reports without creating jobs.
- YouTube resource metadata stored from related APIs must follow YouTube's
  refresh/delete policies; bulk CSV ingestion does not waive those rules.

Google does not publish a fixed `pageSize` maximum in the current REST
Discovery document. The adapter therefore validates nonnegative `int32`
values without inventing a local quota. Google Cloud quota assignments,
runtime `RESOURCE_EXHAUSTED`/`quotaExceeded` responses, and `Retry-After`
headers remain authoritative.

See Google's [bulk report guide](https://developers.google.com/youtube/reporting/v1/reports),
[authorization guide](https://developers.google.com/youtube/reporting/guides/authorization/server-side-web-apps),
and [revision history](https://developers.google.com/youtube/reporting/revision_history).
