# Mixcloud adapter

`adapters/mixcloud` implements Mixcloud's official, unversioned creator API as
`mixcloud/api`.

## Supported contract

- OAuth 2 browser authorization-code flow and form-encoded token exchange.
- Authorized-user and public-user profiles.
- Cloudcast detail, per-user Cloudcast lists, and public comments.
- Search for Cloudcasts, users, and tags.
- Favourite, Repost, Listen Later, and Follow toggles.
- Streaming single-request MP3 upload with optional artwork, tags, sections,
  scheduling, visibility, statistics controls, and co-hosts.
- Cloudcast metadata and artwork editing.

The common `Fetcher` maps users, Cloudcasts, and comments. The common
`Reactor` maps `LIKE` to Favourite and `REPOST` to Repost. Upload is not
exposed as `Publisher` or `MediaUploader`: Mixcloud creates the Cloudcast and
its audio in one multipart request, so `UploadWorkflow` preserves that
required shape.

Mixcloud does not expose audio stream URLs through this API, direct messages,
signed webhooks, independent media resources, or documented comment writes.
The adapter therefore does not synthesize those capabilities. The old
documented `/popular/`, `/popular/hot/`, and `/new/` list examples no longer
returned list resources when rechecked on 2026-08-02, so discovery uses the
working `/search/` contract.

## Configuration

```yaml
version: 1
platforms:
  - adapter: mixcloud/api
    product: api
    accounts:
      - id: creator
        client_id: ${MIXCLOUD_CLIENT_ID}
        secret_ref: env://MIXCLOUD_CLIENT_SECRET
        access_token_ref: vault://social/mixcloud/access-token
        approval:
          account_type: pro
        settings:
          username: sample-dj
```

`account_type: pro` lets the SDK confirm that scheduling,
`disable_comments`, `hide_stats`, and co-host fields are eligible before a
large upload starts. When account type is omitted, Mixcloud remains the source
of truth. A known non-Pro account receives `socialhub.ErrApprovalRequired` for
those fields.

## OAuth

```go
oauth, err := adapter.OAuth(ctx, "creator")
if err != nil {
	return err
}

authorizeURL, err := oauth.AuthorizationURL(
	"https://app.example/oauth/callback",
	"application-generated-state",
)
if err != nil {
	return err
}

token, err := oauth.Exchange(
	ctx,
	authorizationCode,
	"https://app.example/oauth/callback",
)
```

Mixcloud's official contract places both token-exchange credentials and API
access tokens in query parameters. The adapter follows that contract, rejects
redirects that could forward a token, sanitizes transport errors, and removes
`access_token` from returned paging links. Mixcloud does not document refresh
tokens or an expiry; revoked tokens require browser re-authorization.

## Streaming upload

```go
result, err := client.UploadWorkflow().Upload(
	ctx,
	mixcloud.UploadRequest{
		Name:          "Night Radio",
		AudioFilename: "night-radio.mp3",
		AudioSize:     audioSize,
		Description:   "Recorded live",
		Tags:          []string{"house", "radio"},
		Sections: []mixcloud.Section{
			{Chapter: "Intro", StartTime: 0},
			{Artist: "Artist", Song: "Track", StartTime: 45},
		},
	},
	audioReader,
	nil,
)
```

The MP3 is streamed through `io.Pipe`; it is never buffered with `io.ReadAll`.
The declared byte count must exactly match the reader. The documented limits
are 4,294,967,296 bytes for MP3 data, 10,485,760 bytes for artwork, five tags,
and two Pro co-hosts.

## Operational notes

- Mixcloud publishes no fixed rate quota. A limit response is HTTP `403` with
  `error.type=RateLimitException`; the adapter classifies it as retryable and
  preserves `Retry-After` or `error.retry_after`.
- Public reads are available without authorization on the platform, but a
  social-hub `Client` represents one configured account and therefore requires
  `access_token_ref` for a consistent read/write contract.
- This package is verified with deterministic local HTTP contract tests only;
  it has not been exercised against a real Mixcloud account.

Official documentation: <https://www.mixcloud.com/developers/>
