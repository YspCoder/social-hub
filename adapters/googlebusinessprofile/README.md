# Google Business Profile v4 adapter

`google-business-profile/v4` implements the official My Business API v4 Local
Posts and Reviews resources for one configured business location.

## Supported contracts

- Google OAuth2 web-server authorization-code flow with offline access and
  refresh tokens;
- common `Publisher` for text-only `STANDARD` Local Posts, status reads, and
  deletion;
- common `Fetcher` for the configured Location and its Local Posts;
- typed `LocalPostWorkflow` for `STANDARD`, `EVENT`, and `OFFER` posts, CTA,
  scheduling, public `sourceUrl` media, and PATCH updates with inferred
  `updateMask`;
- typed `ReviewWorkflow` for review get/list and owner reply upsert/delete;
- canonical Google error details, request IDs, rate limits, and retry hints.

The adapter does not expose common media upload, reactions, messaging, or
webhooks. Business Profile notifications use Google Cloud Pub/Sub and do not
define the account-scoped signed HTTP callback contract required by
`socialhub.WebhookHandler`.

## Platform gates

Google Business Profile APIs require approved API access. A project quota of
zero means access has not been granted; submit Google's Basic API Access
application rather than requesting a quota increase. Every implemented request
uses this scope:

```text
https://www.googleapis.com/auth/business.manage
```

Review reads and owner replies are valid only for a verified location. These
platform gates are surfaced through capability metadata and sanitized
`socialhub.Error` values; the SDK does not claim that configuration alone grants
Google approval.

## Configuration

Each SDK account is deliberately bound to one Google Account and one Location.
Use multiple SDK accounts to manage multiple locations without weakening
resource-ownership checks.

```yaml
version: 1
platforms:
  - adapter: google-business-profile/v4
    accounts:
      - id: downtown-store
        client_id: 123456.apps.googleusercontent.com
        secret_ref: env://GBP_CLIENT_SECRET
        access_token_ref: vault://social/gbp/downtown/access-token
        approval:
          scopes:
            - https://www.googleapis.com/auth/business.manage
        settings:
          google_account_id: "100000000000000000000"
          location_id: "200000000000000000000"
          language_code: en-US
```

Credential values remain outside configuration. `access_token_ref` is resolved
at client creation; applications should persist refreshed OAuth tokens in an
encrypted token store.

## OAuth

```go
oauth, err := adapter.OAuth(ctx, "downtown-store")
if err != nil {
	return err
}

authorizationURL, err := oauth.AuthorizationURL(
	"https://app.example.com/oauth/google/callback",
	state,
)
if err != nil {
	return err
}

token, err := oauth.Exchange(
	ctx,
	code,
	"https://app.example.com/oauth/google/callback",
)
```

The authorization URL requests `access_type=offline`, incremental scopes, and
explicit consent so that the initial exchange can return a refresh token.
Google normally omits the refresh token from refresh responses; `Refresh`
preserves the caller's existing refresh token in the returned bundle.

## Typed workflows

```go
workflow := client.LocalPostWorkflow()

post, err := workflow.CreateLocalPost(ctx,
	googlebusinessprofile.LocalPostCreateRequest{
		Summary:   "Weekend tasting event",
		TopicType: googlebusinessprofile.LocalPostEvent,
		Event: &googlebusinessprofile.LocalPostEventDetails{
			Title: "Coffee tasting",
			Schedule: googlebusinessprofile.TimeInterval{
				StartDate: googlebusinessprofile.Date{Year: 2026, Month: 8, Day: 8},
				StartTime: googlebusinessprofile.TimeOfDay{Hours: 10},
				EndDate:   googlebusinessprofile.Date{Year: 2026, Month: 8, Day: 8},
				EndTime:   googlebusinessprofile.TimeOfDay{Hours: 12},
			},
		},
	},
)
```

Google documents `ALERT`, but current alert authoring is not generally
available. The SDK preserves `ALERT` in response models while rejecting new
alert creation instead of presenting it as a reliable capability.

Review replies use the typed review workflow:

```go
reviews, err := client.ReviewWorkflow().ListReviews(ctx,
	googlebusinessprofile.ReviewListRequest{MaxResults: 50},
)
if err != nil {
	return err
}

reply, err := client.ReviewWorkflow().UpdateReviewReply(
	ctx,
	reviews.Items[0].ID,
	"Thank you for visiting.",
)
```

## Verification status

The adapter is covered by deterministic local contract tests. It has not been
validated with a credentialed production Business Profile account.

Official references:

- https://developers.google.com/my-business/reference/rest/
- https://developers.google.com/my-business/reference/rest/v4/accounts.locations.localPosts
- https://developers.google.com/my-business/reference/rest/v4/accounts.locations.reviews
- https://developers.google.com/my-business/content/implement-oauth
- https://developers.google.com/my-business/content/limits
