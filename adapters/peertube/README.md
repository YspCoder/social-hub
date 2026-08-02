# PeerTube REST API v1 adapter

`peertube/rest-v1` targets the official per-instance REST API shipped by
PeerTube. The contract was checked against the `v8.2.3` source tag and its
OpenAPI document on 2026-08-02. The tagged OpenAPI file still reports
`info.version: 8.1.0`; this adapter uses the `v8.2.3` tag as the authoritative
contract boundary.

## Configuration

Each account selects its own instance because PeerTube is federated:

```yaml
adapter: peertube/rest-v1
accounts:
  - id: videos-main
    access_token_ref: env://PEERTUBE_ACCESS_TOKEN
    settings:
      instance_url: https://video.example.com
      account_name: creator
    approval:
      scopes: [user]
```

An access token can be acquired with `Adapter.OAuth`, which discovers the
instance's local OAuth client before running the password grant. Access tokens
are normally valid for one day and refresh tokens for two weeks. PeerTube
currently permits only one active access token per account, so applications
should refresh and persist the returned bundle instead of logging in per call.
For first-time bootstrap, `access_token_ref` may be omitted while calling
`Adapter.OAuth`; it is required only when creating an API `Client`.

## Capability boundary

- Common `Fetcher`: accounts, videos, global/account listing, and top-level
  comment threads. `ListComments` intentionally does not issue one request per
  thread to flatten all replies; use `CommentWorkflow.GetCommentThread` for the
  recursive tree.
- Common `Reactor`: like/unlike and comment/reply creation. PeerTube's deletion
  path requires both video and comment IDs, so use
  `CommentWorkflow.DeleteVideoComment`; common `DeleteComment` returns
  `unsupported` rather than guessing a video ID.
- `VideoWorkflow`: reads, single-request streaming multipart upload, metadata
  update, and deletion. Resumable upload is not exposed by this first adapter.
- `ChannelWorkflow`: list and get public video channels.
- Common `Publisher` and `MediaUploader` are deliberately unavailable: upload
  creates a video and requires channel-specific publication fields.
- Messaging and signed webhooks are not exposed because the public account API
  does not define those contracts.

The server administrator controls actual limits. PeerTube defaults to 50 API
calls per 10 seconds and 15 token requests per 5 minutes, returning HTTP 429
with `X-RateLimit-*` and `Retry-After` headers.

## Rating contract note

The `v8.2.3` OpenAPI request schema lists only `like` and `dislike`, but the
official `videoUpdateRateValidator` source accepts `none`; the adapter uses
`none` to implement `RemoveReaction`.

Tests use deterministic local HTTP fixtures only. No real PeerTube account or
instance has been exercised yet.
