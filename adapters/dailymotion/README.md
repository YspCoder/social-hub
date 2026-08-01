# Dailymotion API v2 adapter

Package `social-hub/adapters/dailymotion` targets the official Dailymotion API
v2 at `https://api.dailymotion.com/v2`. It uses only documented HTTP contracts.

## Access prerequisites

Dailymotion API v2 is intended for Dailymotion Pro organizations. An
organization Owner or Admin must create a Private API key in Dailymotion
Studio and grant the scopes required by the application. The key and secret
are used with the OAuth 2.0 Client Credentials grant.

Client Credentials tokens normally expire after 1,800 seconds. Dailymotion
does not return a refresh token for this grant, so the adapter obtains a new
token before expiry and can share it through `socialhub.TokenStore`. A static
Bearer token can instead be supplied through `access_token_ref`.

Supported scope bundles are `bundle.public`, `bundle.user`,
`bundle.publisher`, and `bundle.organization`. Individual `account.*`,
`profile.*`, `video.*`, `playlist.*`, `live.*`, `player.*`,
`organization.*`, and `analytics.manage` scopes are accepted as documented by
Dailymotion. The adapter checks the configured grants before network I/O.

## Implemented contracts

| Surface | Support |
|---|---|
| Common `Fetcher` | Managed profiles and owned videos; API v2 does not expose comments |
| Typed `ProfileWorkflow` | Current account, profile read/update, webhook configuration |
| Typed `VideoWorkflow` | Video read/list/create/update/delete with Dailymotion-specific publication fields |
| Typed `VideoUploadWorkflow` | Upload-session creation, streaming multipart transfer, and publication |
| Typed `PlaylistWorkflow` | Playlist CRUD and ordered membership operations |
| Common `Publisher` / `MediaUploader` | Not exposed; video publication requires typed category, visibility, audience, and source semantics |
| Reactions / messaging | Not exposed by Dailymotion API v2 |
| Common `WebhookHandler` | Not exposed because Dailymotion does not publicly document the `X-DM-Signature` verification algorithm |

The adapter does not claim livestream, Player, analytics, download-URL, or
comment support. Configuring profile webhook events is supported, but receiving
those events without a documented signature verifier is outside the secure
common webhook contract.

## Configuration

```yaml
adapter: dailymotion/api-v2
accounts:
  - id: publisher
    client_id: "dailymotion-private-api-key"
    secret_ref: env://DAILYMOTION_PRIVATE_API_SECRET
    approval:
      scopes:
        - bundle.organization
    settings:
      profile_id: "managed-profile-id"
```

`settings.profile_id` is the default managed profile used by list, create, and
common fetch operations. It can be overridden by typed request fields. For an
externally managed token, replace `client_id` and `secret_ref` with
`access_token_ref` while retaining the token's actual scopes in
`approval.scopes`.

## Upload lifecycle

```go
workflow := client.VideoUploadWorkflow()
session, err := workflow.Initialize(ctx, "launch.mp4", size)
if err != nil {
	return err
}

uploaded, err := workflow.Upload(ctx, session.ID, file)
if err != nil {
	_ = workflow.Abort(session.ID)
	return err
}

video, err := workflow.Publish(ctx, session.ID, dailymotion.CreateVideoRequest{
	Title:      "Launch",
	Category:   "tech",
	Visibility: "public",
	IsForKids:  false,
})
_ = uploaded
```

Upload sessions are process-local. `Upload` streams exactly the declared byte
count as the `file` multipart field and intentionally omits the API Bearer
token from the returned upload URL request. Production upload URLs must use
HTTPS on Dailymotion-owned hosts. Custom API gateways and local tests may use
the configured API origin. Redirects are not followed.

## Official documentation

- <https://developers.dailymotion.com/reference/introduction>
- <https://developers.dailymotion.com/docs/authenticate>
- <https://developers.dailymotion.com/reference/api-scopes>
- <https://developers.dailymotion.com/docs/upload-videos>
- <https://developers.dailymotion.com/reference/api-errors>
