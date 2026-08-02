# DeviantArt adapter

`deviantart/api-v1-20240701` implements DeviantArt's official OAuth API v1
with API minor version `20240701`.

The adapter follows the current OAuth 2.1 contract for newly registered apps:
Authorization Code uses PKCE S256, redirect URIs must match exactly, and bearer
tokens are sent in the `Authorization` header. Legacy OAuth 2.0 applications
can still use the same token helper with PKCE.

## Supported capabilities

| Capability | API operations | Required scopes |
|---|---|---|
| Identity | authorized user and public profiles | `basic user` for `WhoAmI`; `browse` for profiles |
| Fetch | Deviation, Gallery All, profile Posts, Deviation comments | `browse` |
| Publish | text Status | `user.manage` |
| React | post/reply comment | `browse comment.post` |
| React | favourite/unfavourite | `browse collection` |

Typed workflows preserve DeviantArt-specific fields:

- `UserWorkflow`
- `DeviationWorkflow`
- `GalleryWorkflow`
- `StatusWorkflow`
- `CommentWorkflow`
- `CollectionWorkflow`

The common `Publisher` intentionally maps only text Status publishing. Artwork,
literature, and media publication use DeviantArt's separate Sta.sh
`submit -> publish` workflow and are not exposed as `MediaUploader` in this
initial adapter. The API does not provide signed webhooks or a direct-message
contract suitable for the common interfaces.

## Configuration

```yaml
version: 1
platforms:
  - adapter: deviantart/api-v1-20240701
    accounts:
      - id: artist
        client_id: "12345"
        secret_ref: env://DEVIANTART_CLIENT_SECRET # omit for a public PKCE client
        access_token_ref: env://DEVIANTART_ACCESS_TOKEN
        approval:
          scopes: [basic, user, browse, user.manage, comment.post, collection]
        settings:
          username: sample-artist
          user_id: 11111111-2222-3333-4444-555555555555 # optional but recommended
```

`username` selects the default Gallery/profile account. `user_id` is optional;
when provided it is used to validate authorized identity and normalized actor
IDs. Credential fields are secret references, never raw secrets.

## OAuth 2.1

```go
pkce, err := deviantart.NewPKCE()
if err != nil {
	return err
}

oauth, err := adapter.OAuth(ctx, "artist")
if err != nil {
	return err
}

authorizationURL, err := oauth.AuthorizationURL(
	"https://app.example/oauth/callback",
	state,
	pkce,
	[]string{"basic", "user", "browse"},
)

token, err := oauth.Exchange(ctx, code, "https://app.example/oauth/callback", pkce.Verifier)
```

`OAuthClient` also exposes `Refresh`, `ClientCredentials`, and `Revoke`.
Client Credentials requires a confidential client secret and is limited to
public endpoints. Access tokens expire after one hour; DeviantArt documents a
three-month refresh-token lifetime.

## Operational notes

- Every API request sends `dA-minor-version: 20240701` and an explicit
  `User-Agent`.
- DeviantArt uses adaptive rate limiting rather than a fixed quota. HTTP `429`
  is classified as retryable and `Retry-After`, when present, is preserved.
- This package is verified with deterministic local HTTP contract tests only;
  it has not been exercised against a real DeviantArt account.

Official documentation: <https://deviantart.readme.io/>
