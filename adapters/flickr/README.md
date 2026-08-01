# Flickr Services API adapter

Package `social-hub/adapters/flickr` implements Flickr's official REST and
Upload APIs. It does not use cookies, scraping, browser automation, private
endpoints, or undocumented Flickr Push behavior.

## Access prerequisites

Create an API key in the Flickr App Garden. Non-commercial use is available to
registered applications. Commercial use requires prior arrangement with
Flickr/SmugMug and may be subject to fees. Applications must also follow each
photo owner's license and the Flickr API terms, including attribution, removal,
cache, bandwidth, and the limit of 30 Flickr photos displayed per page.

Flickr uses OAuth 1.0 Revision A with HMAC-SHA1. Permissions are hierarchical:
`read < write < delete`. Flickr does not issue refresh tokens or publish an
expiry for access-token credential pairs; store both the access token and token
secret securely and replace them when authorization is revoked or repeated.

The Flickr developer guide describes 3,600 requests per hour across the whole
API key as the normal operating guideline. Treat this as an API-key-wide fixed
window and keep the limit configurable because Flickr may change or contractually
override it.

## Implemented contracts

| Surface | Support |
|---|---|
| Common `Fetcher` | Member profiles, photo metadata, photostreams, normalized posts, and flat comments |
| Common `Reactor` | Favorites and flat photo comment creation/deletion |
| Typed `PhotoWorkflow` | Photo reads, metadata update, and deletion |
| Typed `PhotoUploadWorkflow` | Streaming multipart photo/video upload that creates the Flickr resource |
| Typed `AlbumWorkflow` | Photoset reads, listing, creation, and membership changes |
| Common `Publisher` / `MediaUploader` | Not exposed; Flickr upload directly creates the photo resource rather than a detached media ID |
| Messaging / webhooks | Not exposed by the documented Services API contracts used here |

Public clients can read public profiles, photos, photostreams, and comments with
only an API key. Authenticated reads are signed so private photos visible to the
authorized member can be returned. Mutations enforce their exact OAuth
permission before network I/O.

## Configuration

```yaml
adapter: flickr/services-api
accounts:
  - id: photographer
    client_id: "flickr-api-key"
    secret_ref: env://FLICKR_CONSUMER_SECRET
    access_token_ref: env://FLICKR_ACCESS_TOKEN
    approval:
      scopes: [delete]
    settings:
      user_id: "12345678@N01"
      token_secret_ref: env://FLICKR_TOKEN_SECRET
```

`client_id` is the Flickr API key. `settings.user_id` is the account's opaque
Flickr NSID. IDs remain strings throughout the adapter. `secret_ref` contains
the API consumer secret, while `access_token_ref` and `token_secret_ref` form
the OAuth token credential pair.

For public-only access, omit the three secret/token references and approval
scopes. A consumer secret may be configured without an access token when the
same account entry is used only to start `Adapter.OAuth` authorization.

## OAuth flow

```go
oauthClient, err := adapter.OAuth(ctx, "photographer")
if err != nil {
	return err
}

temporary, err := oauthClient.BeginAuthorization(
	ctx,
	"https://app.example.com/oauth/flickr/callback",
	flickr.PermissionWrite,
)
if err != nil {
	return err
}

// Redirect the member to temporary.AuthorizationURL, then exchange the
// callback oauth_verifier together with this temporary credential pair.
access, err := oauthClient.Exchange(ctx, *temporary, verifier)
```

Persist `access.Token`, `access.Secret`, `access.UserID`, and
`access.Permission` together. The adapter uses the maintained
`github.com/dghubble/oauth1` package for RFC 5849 signing rather than the
unmaintained Flickr API kits.

## Upload behavior

`PhotoUploadWorkflow.Upload` streams the declared number of bytes without
buffering the entire photo or video. Every metadata form field participates in
the OAuth HMAC-SHA1 signature; the binary `photo` part is explicitly excluded,
as required by Flickr. The operation rejects short and long readers, does not
follow redirects, limits the XML response body, and returns the newly created
Flickr photo ID.

## Official documentation

- <https://www.flickr.com/services/api/>
- <https://www.flickr.com/services/api/auth.oauth.html>
- <https://www.flickr.com/services/api/upload.api.html>
- <https://www.flickr.com/services/developer/api/>
- <https://www.flickr.com/help/terms/api>
