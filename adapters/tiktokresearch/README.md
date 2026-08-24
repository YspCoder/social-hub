# TikTok Research API v2 adapter

Adapter name: `tiktok/research-api-v2`

This package implements a bounded, read-only surface of the official TikTok
Research API v2:

- `POST /v2/research/video/query/`;
- `POST /v2/research/user/info/`;
- `POST /v2/research/video/comment/list/` for comments and replies.

Publishing, media upload, reactions, messaging, webhooks, OAuth redirects,
client-credential exchange, and token refresh are deliberately excluded. This
adapter is separate from TikTok's consumer Content Posting and business APIs;
a token for another TikTok product cannot be assumed to authorize research
data.

## Authentication and eligibility

Every request uses the fixed official origin and an externally managed Client
Access Token:

```text
https://open.tiktokapis.com
Authorization: Bearer <client-access-token>
Scope: research.data.basic
```

TikTok issues Research API client credentials only after approving a concrete
research project. A regular developer account is not sufficient. TikTok's
current eligibility material limits Research Tools to qualifying researchers
and organizations in the United States, Europe, and Brazil under region- and
organization-specific conditions. Creators, advertisers, commercial users,
and research serving commercial interests are not eligible.

The Client Access Token is obtained from `POST /v2/oauth/token/` with
`grant_type=client_credentials` and currently expires after about two hours.
Token acquisition, rotation, and refresh remain outside this adapter. Resolve
the current token through `access_token_ref`:

```yaml
version: 1
platforms:
  - adapter: tiktok/research-api-v2
    product: research-api
    accounts:
      - id: approved-project
        access_token_ref: env://TIKTOK_RESEARCH_CLIENT_ACCESS_TOKEN
        approval:
          scopes: [research.data.basic]
```

Adding `approval.scopes` records externally granted access for capability
discovery; it does not grant TikTok approval. The HTTP client never follows
redirects, uses a cookie jar, accepts an alternate API origin, or places the
Bearer token in a URL.

## Querying videos

Response fields are mandatory and use a typed allowlist. The query must have
at least one condition in `And`, `Or`, or `Not`. Dates use `YYYYMMDD`; the end
date cannot precede the start date or be more than 30 days later.

```go
page, err := client.Research().QueryVideos(ctx, tiktokresearch.QueryVideosRequest{
	Query: tiktokresearch.Query{And: []tiktokresearch.Condition{{
		Field:       tiktokresearch.QueryFieldRegionCode,
		Operator:    tiktokresearch.OperatorEqual,
		FieldValues: []string{"US"},
	}}},
	StartDate: "20260801",
	EndDate:   "20260824",
	MaxCount:  100,
	Fields: []tiktokresearch.VideoField{
		tiktokresearch.VideoFieldID,
		tiktokresearch.VideoFieldDescription,
		tiktokresearch.VideoFieldCreateTime,
		tiktokresearch.VideoFieldViewCount,
	},
})
```

The default video page size is 20 and the maximum is 100. To continue, pass
both values from the previous page without interpreting `SearchID`:

```go
nextRequest.Cursor = page.Cursor
nextRequest.SearchID = page.SearchID
```

`SearchID` is opaque, remains in the JSON request body, and is never placed in
the URL or an error message. TikTok video, comment, music, playlist, and effect
IDs are retained as decimal strings by `ID`; decoding accepts provider JSON
numbers and strings without first converting through `float64`.

## Users and comments

`GetUserInfo` accepts a username and a typed `[]UserField`. `ListComments`
requires exactly one of `VideoID` (top-level comments) or `CommentID`
(replies). Comment pages default to 10 records and accept at most 100. TikTok
anonymizes personal information in Research API comments. Although the Comment
Object documentation describes `display_name`, the endpoint's complete field
mask does not list it, so the model preserves a returned value but the adapter
does not allow callers to request it.

## Quotas, freshness, and governance

The current general Research API quota is 1,000 requests per day and 100,000
records per day across all Research API endpoints, resetting at 00:00 UTC.
Video and comment requests return at most 100 records. The adapter exposes
HTTP `Retry-After` on rate-limit errors but does not automatically retry,
throttle, cache, or coordinate the daily project quota.

Research video search is archival rather than real time. New public videos may
take up to 48 hours to enter search, and metrics may lag by up to 10 days. The
Research API codebook describes public content from adult creators, with
coverage for the United States, Europe, and the rest of the world except
Canada.

Research data may be used only for the approved purpose and remains subject to
the current Research API Terms and the project's data-handling commitments.
Applications should implement access control, storage, sharing, retention, and
deletion policies against the current terms and approval, rather than assuming
a fixed period from this SDK. Complete successful provider envelopes remain in
bounded `Raw` fields; sanitized error `Raw` data is capped at 64 KiB.

## Official sources

Official material reviewed on 2026-08-25. The Query Videos reference reports
an update date of 2026-08-19:

- <https://developers.tiktok.com/doc/research-api-get-started>
- <https://developers.tiktok.com/doc/research-api-specs-query-videos>
- <https://developers.tiktok.com/doc/research-api-specs-query-user-info>
- <https://developers.tiktok.com/doc/research-api-specs-query-video-comments>
- <https://developers.tiktok.com/doc/client-access-token-management>
- <https://developers.tiktok.com/doc/tiktok-api-scopes>
- <https://developers.tiktok.com/doc/research-api-faq>
- <https://developers.tiktok.com/doc/research-api-codebook>
- <https://developers.tiktok.com/products/research-api/>
- <https://www.tiktok.com/legal/page/global/terms-of-service-research-api/en>

The adapter adds no third-party dependency.
