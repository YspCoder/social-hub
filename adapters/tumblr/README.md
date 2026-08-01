# Tumblr API v2 adapter

Package `social-hub/adapters/tumblr` implements Tumblr's official API v2 at
`https://api.tumblr.com/v2`. It uses documented API-key and OAuth2 endpoints
only. It does not use cookies, browser automation, scraping, legacy private
mobile endpoints, or undocumented posting routes.

Implemented contracts:

- API-key reads for blog profiles, published posts, tagged discovery, and post
  notes;
- OAuth2 authorization-code, refresh-token, and client-credentials grants;
- OAuth2 dashboard reads and typed like, unlike, follow, and unfollow actions;
- current Neue Post Format (NPF) create, reblog, edit, fetch, queue, draft,
  private-post, backdate, and scheduling fields;
- streaming inline image, audio, and video uploads using Tumblr's NPF
  multipart convention;
- common text publishing, post reads, list reads, deletion, and conversation
  note mapping to comments.

The common `Publisher` intentionally supports only public text posts. Use
`NPFWorkflow` for structured content blocks, layouts, reblogs, non-public
states, scheduling, and media. Tumblr uploads media inline with an NPF post and
does not return reusable media IDs, so `MediaUploader` is disabled. The common
`Reactor` is also disabled because genuine likes and reblogs require a
`reblog_key` and source-post context; use `EngagementWorkflow` for likes and
follows, and `NPFWorkflow` for reblogs.

Tumblr API v2 does not expose direct messaging or a webhook signature and
payload contract, so `Messenger` and `WebhookHandler` are disabled.

## Authentication

Every configured account requires `client_id`. Tumblr uses the OAuth consumer
key as the `api_key` query parameter for public reads. `access_token_ref` is
optional: omit it for a read-only public client, or configure an OAuth2 user
access token to enable publishing, dashboard reads, and engagement actions.

OAuth2 scopes used by this adapter are:

| Scope | Purpose |
|---|---|
| `basic` | Authenticated blog and dashboard reads |
| `write` | NPF publishing, deletion, likes, and follows |
| `offline_access` | Issue a refresh token |

When `approval.scopes` is configured, missing scopes are rejected before
network I/O. An empty scope list means the grant is unknown, leaving Tumblr as
the authority. The OAuth helper returns refreshed tokens to the caller but does
not persist them.

Example configuration:

```yaml
adapter: tumblr/v2
accounts:
  - id: tumblr-main
    client_id: ${TUMBLR_CONSUMER_KEY}
    secret_ref: env://TUMBLR_CONSUMER_SECRET
    access_token_ref: env://TUMBLR_ACCESS_TOKEN
    approval:
      scopes:
        - basic
        - write
        - offline_access
    settings:
      blog_identifier: example.tumblr.com
```

`blog_identifier` may be a blog name, hostname, or Tumblr blog UUID. Use one
account entry per credential/default-blog pair. Individual typed calls that
accept a blog parameter may select another blog authorized by the same user.

## NPF and media

`NPFPostRequest.Content` and `Layout`, plus fetched trail items, preserve raw
JSON because Tumblr evolves NPF independently of this SDK. For an inline
upload, add an image, audio, or video content block and bind an
`NPFMediaUpload` to its zero-based block index. The adapter streams the file and
replaces that block's `media` field with Tumblr's multipart identifier.

Local validation enforces Tumblr's documented stored-content limits: 1 MB NPF
JSON, 1,000 content blocks, 4,096 Unicode code points per text block, 30 image
blocks, 10 video blocks, 10 audio blocks, 10 link blocks, and one native video
upload per post. Tumblr still performs authoritative MIME, codec, duration,
dimension, and policy validation.

`Tagged` returns a timestamp cursor derived from `featured_timestamp` for
featured tags and the normal post timestamp otherwise. `Notes` preserves the
server's `before_timestamp` cursor as text so conversation-mode microsecond
precision is not lost.

## Limits and verification status

Tumblr currently documents both IP and consumer-key limits: 300 calls per
minute, 18,000 per hour, and 432,000 per day per IP; 1,000 calls per hour and
5,000 per day per consumer key. Feature limits include 250 published posts,
250 uploaded images, 200 follows, 1,000 likes, 20 uploaded videos, and 60 total
video minutes per user per day. A blog can follow at most 5,000 blogs and hold
at most 1,000 queued posts.

Treat HTTP 429, `Retry-After`, and Tumblr's response message as the source of
truth. The adapter maps these responses to retryable `socialhub.Error` values
but does not guess which quota was exhausted.

All current tests use deterministic local HTTP fixtures, including streaming
multipart requests. The adapter has not been validated with a real Tumblr app,
user, or blog.

Official resources:

- <https://www.tumblr.com/docs/en/api/v2>
- <https://github.com/tumblr/docs/blob/master/api.md>
- <https://github.com/tumblr/docs/blob/master/npf-spec.md>
- <https://github.com/tumblr/tumblr.js>
