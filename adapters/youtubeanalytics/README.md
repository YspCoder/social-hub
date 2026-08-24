# YouTube Analytics API v2 adapter

Adapter name: `google/youtube-analytics-api-v2`

This adapter exposes account-bound targeted reports and private Analytics group
management for the [YouTube Analytics API v2](https://developers.google.com/youtube/analytics/reference).
The contract was checked against Discovery revision `20260805` on 2026-08-10.

The package is intentionally separate from `adapters/youtube`. The existing
YouTube Data API v3 adapter owns videos, uploads, comments, and other content
workflows. This package owns channel/content-owner measurement, Analytics
groups, and Analytics group items. It has independent scopes, quota behavior,
and response contracts.

## Supported workflows

- Typed `reports.query` requests with dimensions, metrics, filters, sorting,
  currency conversion, one-based pagination, and content-owner historical data.
- Channel and YouTube CMS content-owner account bindings. The adapter derives
  `ids` and `onBehalfOfContentOwner`; callers cannot query another account.
- Group list/create/rename/delete.
- Group-item list/add/remove, including typed handling of the API's `204 No
  Content` response when an inserted resource is already present.
- Exact report header order, column kind, cell type, row width, result kind,
  embedded error, group ownership shape, and raw JSON preservation checks.

This package covers all eight methods in the v2 Discovery surface. It does not
include the YouTube Reporting API bulk-report workflow, which is a separate API
with asynchronous jobs and downloadable files.

## Authentication and account binding

Configure exactly one credential mode per account:

1. A caller-managed bearer token through `access_token_ref`.
2. User OAuth 2.0 Authorization Code with offline refresh through `client_id`,
   `secret_ref`, and `settings.refresh_token_ref`.

The YouTube Analytics API does **not** support Service Account authentication.
OAuth access and refresh tokens can be cached through `socialhub.TokenStore`.
Credential-bearing HTTP clients reject redirects.

Each account must set exactly one of:

```yaml
settings:
  channel_id: UCxxxxxxxxxxxxxxxxxxxxxx
  refresh_token_ref: env://YOUTUBE_ANALYTICS_REFRESH_TOKEN
```

```yaml
settings:
  content_owner_id: cms-owner-id
  refresh_token_ref: env://YOUTUBE_ANALYTICS_REFRESH_TOKEN
```

`channel_id: MINE` is accepted for the authenticated user's channel. Content
owner reports require a YouTube content partner account linked to the
authenticated user.

## OAuth scopes

Managed OAuth defaults to the read-only report and group-read combination:

```text
https://www.googleapis.com/auth/youtube.readonly
https://www.googleapis.com/auth/yt-analytics.readonly
```

Supported scopes are:

```text
https://www.googleapis.com/auth/youtube
https://www.googleapis.com/auth/youtube.readonly
https://www.googleapis.com/auth/youtubepartner
https://www.googleapis.com/auth/yt-analytics.readonly
https://www.googleapis.com/auth/yt-analytics-monetary.readonly
```

Workflow gates are conservative:

| Workflow | Recommended scopes |
|---|---|
| Non-monetary reports | `youtube.readonly` + `yt-analytics.readonly` |
| Revenue/ad reports | `youtube.readonly` + `yt-analytics-monetary.readonly` |
| Read groups/items | `youtube.readonly` + `yt-analytics.readonly` |
| Mutate channel groups/items | `youtube` |
| Mutate content-owner groups/items | `youtubepartner` |

Known revenue and ad-performance metrics automatically select the monetary
scope gate. Set `ReportQuery.Monetary` for a new monetary metric that is not yet
in the adapter's known set.

## Configuration

```yaml
version: 1
platforms:
  - adapter: google/youtube-analytics-api-v2
    product: youtube-analytics-api
    accounts:
      - id: youtube-measurement
        client_id: google-oauth-client-id
        secret_ref: env://GOOGLE_OAUTH_CLIENT_SECRET
        approval:
          scopes:
            - https://www.googleapis.com/auth/youtube.readonly
            - https://www.googleapis.com/auth/yt-analytics.readonly
        settings:
          channel_id: UCxxxxxxxxxxxxxxxxxxxxxx
          refresh_token_ref: env://YOUTUBE_ANALYTICS_REFRESH_TOKEN
```

Import the package for opt-in registration:

```go
import _ "social-hub/adapters/youtubeanalytics"
```

## Usage

Query a bounded daily video report:

```go
client := common.(*youtubeanalytics.Client)

report, err := client.QueryReport(ctx, youtubeanalytics.ReportQuery{
    StartDate:  "2026-08-01",
    EndDate:    "2026-08-09",
    Dimensions: []youtubeanalytics.Dimension{youtubeanalytics.DimensionDay},
    Metrics:    []youtubeanalytics.Metric{youtubeanalytics.MetricViews},
    Filters: []youtubeanalytics.Filter{{
        Dimension: youtubeanalytics.DimensionVideo,
        Values:    []string{"video-id-1", "video-id-2"},
    }},
    Sort:       []youtubeanalytics.Sort{{Name: "views", Descending: true}},
    MaxResults: 100,
})
```

Create and populate a private Analytics group:

```go
group, err := client.CreateGroup(ctx, youtubeanalytics.CreateGroupInput{
    Title:    "Campaign videos",
    ItemType: youtubeanalytics.ResourceVideo,
})
if err != nil {
    return err
}

added, err := client.AddGroupItem(ctx, youtubeanalytics.AddGroupItemInput{
    GroupID:    group.ID,
    ResourceID: "video-id-1",
    Kind:       youtubeanalytics.ResourceVideo,
})
if err != nil {
    return err
}
if added.AlreadyPresent {
    // The resource was already a member; the API returned 204.
}
```

Channel owners can create video and playlist groups. Content owners can also
create channel and `youtubePartner#asset` groups.

## Quotas and limits

| Constraint | Adapter behavior |
|---|---|
| Items in one Analytics group | Maximum 500 |
| IDs in `video`, `playlist`, or `channel` filters | Maximum 500 |
| Traffic-source query cost | `queried videos * date-range days <= 50,000` |
| Targeted-query quota | Dynamic per-query cost; runtime API/Cloud quota errors are authoritative |

Google does not publish one stable numeric daily quota for every targeted
query. The adapter therefore does not invent a token bucket from a nominal
daily value. It preserves quota errors and `Retry-After` for the shared retry
and rate-limit layers. For large data sets, use the
[YouTube Reporting API bulk reports](https://developers.google.com/youtube/reporting/v1/reports)
instead of splitting an unbounded targeted query inside this adapter.

See the official [report query reference](https://developers.google.com/youtube/analytics/reference/reports/query),
[groups reference](https://developers.google.com/youtube/analytics/reference/groups),
[group-items reference](https://developers.google.com/youtube/analytics/reference/groupItems),
[OAuth guide](https://developers.google.com/youtube/reporting/guides/authorization/server-side-web-apps),
and [revision history](https://developers.google.com/youtube/analytics/revision_history).
