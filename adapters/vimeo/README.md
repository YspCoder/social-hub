# Vimeo API v3.4 adapter

This package implements Vimeo API v3.4 through public, documented HTTP
contracts. It does not use cookies, scraping, browser automation, private
endpoints, or reverse-engineered behavior.

## Capabilities

| Capability | Support | Required OAuth scopes |
|---|---|---|
| Common `Fetcher` | Users, videos, account/user videos, comments | `public` |
| Common `Reactor` | Like/unlike, comments, replies, comment deletion | `interact`; `delete` for deletion |
| Typed `FeedWorkflow` | `/me/feed` and user feeds | `public` |
| Typed `VideoUploadWorkflow` | TUS initialize/upload/complete, status, update, delete | `upload`, `edit`, `public`, and `delete` as used |
| Common `Publisher` / `MediaUploader` | Not exposed | A Vimeo video resource is created by the upload workflow |
| Messaging / webhooks | Not exposed | Not part of this adapter contract |

Vimeo upload access requires platform-side application approval. A granted
OAuth scope does not by itself guarantee that Vimeo has enabled uploads for the
application or account.

## Configuration

```yaml
adapter: vimeo/api-v3.4
product: vimeo-api
accounts:
  - id: creator
    client_id: ${VIMEO_CLIENT_ID}
    secret_ref: env://VIMEO_CLIENT_SECRET
    access_token_ref: env://VIMEO_ACCESS_TOKEN
    approval:
      scopes: [public, interact, upload, edit, delete]
    settings:
      user_id: "12345678"
```

`client_id` and `secret_ref` are needed only when using `Adapter.OAuth`.
`access_token_ref` is required for an API client. `settings.user_id` is
optional, but when present it is used to verify `/me` and reaction actor
identity.

The OAuth helper supports authorization-code exchange and client credentials.
It intentionally has no refresh method because Vimeo's current official SDK
contract does not expose a generally supported refresh flow.

## TUS upload lifecycle

```go
workflow := client.VideoUploadWorkflow()
session, err := workflow.Initialize(ctx, vimeo.VideoUploadRequest{
	Name: "Launch video",
	Size: size,
})
if err != nil {
	return err
}

var parts []socialhub.UploadedPart
for partNumber := 0; uploaded < size; partNumber++ {
	part, err := workflow.UploadPart(ctx, session.ID, partNumber, nextChunk)
	if err != nil {
		return err
	}
	parts = append(parts, *part)
	uploaded += part.Size
}
post, err := workflow.Complete(ctx, session.ID, parts)
```

Upload sessions are process-local. `UploadPart` streams at most the advertised
part size, never sends the API Bearer token to the signed TUS URL, and accepts
production upload URLs only on HTTPS Vimeo hosts. Every successful PATCH must
return an `Upload-Offset` that exactly matches the bytes sent.

## API references

- [API reference](https://developer.vimeo.com/api/reference)
- [Authentication](https://developer.vimeo.com/api/authentication)
- [Video uploads](https://developer.vimeo.com/api/upload/videos)
- [Rate limiting](https://developer.vimeo.com/guidelines/rate-limiting)
