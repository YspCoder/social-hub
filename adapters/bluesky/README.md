# Bluesky / AT Protocol XRPC adapter

Package `social-hub/adapters/bluesky` implements the official Bluesky
application Lexicons over AT Protocol XRPC. Each account supplies its own PDS
`service_url`; the adapter never assumes `bsky.social` or any other hosted
provider.

Implemented contracts:

- legacy PDS session creation, refresh, revocation, optional second-factor
  tokens, and JWT expiry extraction for headless integrations and bots;
- profile and post lookup, author feeds, thread replies, and the authenticated
  home timeline;
- public post records, replies with inherited thread roots, quotes, deletion,
  deterministic record keys, languages, image alt text, and aspect ratios;
- `com.atproto.repo.uploadBlob` for images and simple MP4 blobs, including
  image, video, and `recordWithMedia` embeds;
- idempotent Like and Repost record creation/removal using viewer state and
  content-addressed strong references;
- AT URI, cursor, XRPC error, request ID, and rate-limit header mapping.

Bluesky repository records are public. Non-public visibility is rejected
instead of silently widening access. Feed repost reasons are represented as a
wrapper `Post` whose ID and author belong to the repost record, with a
`RelationRepost` pointing to the original post; the content and metrics remain
those of the target post.

The common `Messenger` and `WebhookHandler` are disabled. Bluesky chat is a
separately authorized proxied service, while repository changes are delivered
through event streams/firehose rather than signed HTTP webhooks. Neither is
represented as a weaker common contract in this adapter.

## Authentication boundary

`Adapter.Session` implements the official legacy `createSession`,
`refreshSession`, and `deleteSession` procedures. Configure `secret_ref` as an
app password and persist each returned access/refresh JWT rotation in the
application's secret/token store. `access_token_ref` must resolve to the
current access JWT when constructing a common client; this adapter does not
mutate external secrets automatically.

Full AT Protocol OAuth is intentionally not reimplemented here. It requires
OAuth 2.1 authorization-server discovery, PKCE, PAR, DPoP, and identity/PDS
resolution. Applications that require interactive OAuth should use the
official [`bluesky-social/indigo`](https://github.com/bluesky-social/indigo)
OAuth implementation to obtain and rotate tokens, then expose the current
access token through social-hub's `SecretResolver`.

## Blob lifecycle and video

`MediaUploader` accepts exactly one part. Images are limited to 2,000,000
bytes; MP4 inputs are limited to the Lexicon's 100,000,000-byte maximum. A PDS
or hosting provider may enforce a lower generic blob limit. In particular, the
hosted Bluesky PDS currently documents a 52,428,800-byte individual blob cap,
so callers must also honor 413 responses and provider metadata.

Completed Blob CIDs are retained only by the `Client` that uploaded them.
`Publish` rejects arbitrary or cross-client CIDs because it cannot recover the
required typed Blob reference from a CID alone. Unreferenced blobs are
temporary repository objects; publish immediately after completion. For video
that requires transcoding, captions, or service-side processing, use the
official video service workflow before creating a post record. The common
uploader only advertises direct, already-compatible MP4 blobs.

## Rate limits

Current hosted-service guidance documents:

- repository writes: 5,000 points per hour and 35,000 per day, where CREATE
  costs 3 points, UPDATE 2, and DELETE 1;
- overall hosted PDS traffic: 3,000 requests per five minutes per IP;
- `createSession`: 30 requests per five minutes and 300 per day per account.

These limits are provider-specific and may change. Consume rate headers,
`Retry-After`, `RateLimit-Reset`, and HTTP 429 instead of hard-coding hosted
defaults.

Example account settings:

```yaml
adapter: bluesky/atproto
accounts:
  - id: bluesky-main
    secret_ref: env://BLUESKY_APP_PASSWORD
    access_token_ref: env://BLUESKY_ACCESS_JWT
    settings:
      service_url: https://pds.example
      repo: did:plc:replace-with-account-did
      identifier: alice.example
```

Official documentation:

- <https://atproto.com/specs/oauth>
- <https://atproto.com/specs/at-uri-scheme>
- <https://atproto.com/specs/record-key>
- <https://docs.bsky.app/docs/advanced-guides/api-directory>
- <https://docs.bsky.app/docs/advanced-guides/oauth-client>
- <https://docs.bsky.app/docs/advanced-guides/rate-limits>
- <https://docs.bsky.app/docs/tutorials/creating-a-post>
- <https://docs.bsky.app/docs/tutorials/like-repost>
- <https://docs.bsky.app/docs/tutorials/video>
