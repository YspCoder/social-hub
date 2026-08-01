# Douyin OpenAPI adapter

Registry name: `douyin/openapi`

Implemented capabilities:

- OAuth 2.0 Authorization Code, user-token refresh, refresh-token renewal, and
  application-level `client_token` helpers
- current authorized user profile, video list, video data, and comment list
- direct video upload and `/video/part/*` multipart upload
- asynchronous video creation, status lookup, and deletion
- typed `video.Workflow` built on the common upload and publish contracts
- comment replies for the authorized user's videos
- SHA-1 webhook verification, event decoding, and challenge response generation

User APIs require a user `access_token` and the app-scoped `open_id`. A
`client_token` is intentionally returned through a separate OAuth helper and is
never accepted by the account client.

```yaml
adapter: douyin/openapi
product: openapi
accounts:
  - id: primary
    client_id: awxxxxxxxx
    secret_ref: env://DOUYIN_CLIENT_SECRET
    access_token_ref: env://DOUYIN_USER_ACCESS_TOKEN
    settings:
      open_id: app-scoped-open-id
    approval:
      scopes: [user_info, video.list, video.data, video.create, video.delete]
```

Video creation requires `video.create`, explicit user awareness for every
publication, and platform review. Upload completion does not mean publication
approval. Multipart sessions have no documented remote abort endpoint; the
typed `Abort` operation only discards local workflow state and the platform
upload expires according to platform policy.

Direct messages and like mutation are not exposed by the general public user
OpenAPI. Enterprise-only products are not emulated.

Official documentation:

- <https://open.douyin.com/platform/resource/docs/develop/permission/overall-permission/>
- <https://open.douyin.com/platform/resource/docs/openapi/video-management/douyin/create/upload/>
- <https://open.douyin.com/platform/resource/docs/openapi/video-management/douyin/create/create-video>
- <https://open.douyin.com/platform/resource/docs/develop/webhooks/summarize/>

Last verified: 2026-08-01.
