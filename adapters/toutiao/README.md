# Toutiao OpenAPI adapter

Registry name: `toutiao/openapi`

Package `social-hub/adapters/toutiao` implements the public Toutiao OAuth and
small-video APIs exposed through the Douyin Open Platform documentation.

Implemented contracts:

- OAuth 2.0 Authorization Code exchange, user-token refresh, and application
  `client_token` helpers;
- authorized-user profile from the Toutiao OAuth origin;
- owned-video list and per-video data;
- direct and multipart video upload;
- asynchronous small-video publication and local pending-state tracking;
- typed `video.Workflow` built on the common upload and publish interfaces.

Toutiao uses two distinct origins. Business video APIs use
`https://open.douyin.com`, while authorization, token, and user-information
requests use `https://open.snssdk.com`. The adapter keeps separate transports
for these origins and refuses redirects. User APIs require a user
`access_token` and its app-scoped `open_id`; an application `client_token` is
returned only by the OAuth helper and cannot create an account client.

```yaml
adapter: toutiao/openapi
product: openapi
accounts:
  - id: primary
    client_id: awxxxxxxxx
    secret_ref: env://TOUTIAO_CLIENT_SECRET
    access_token_ref: env://TOUTIAO_USER_ACCESS_TOKEN
    settings:
      open_id: app-scoped-open-id
    approval:
      scopes: [user_info, toutiao.video.create, toutiao.video.data]
```

The app and requested scopes must be approved in the platform console. Missing
approval is returned as `socialhub.CodeApprovalRequired` with the required
scope and approval URL; the adapter does not emulate restricted or
enterprise-only APIs.

The publication API supports small videos only, with a documented maximum
duration of one minute and maximum file size of 300 MiB. Files over 50 MiB use
multipart upload in this adapter, so the platform's mandatory multipart rule
for files over 128 MiB is always satisfied. The part size is 20 MiB; non-final
parts must be at least 5 MiB. Upload completion does not imply publication
approval.

No public deletion, comment, reaction, message, or signed webhook contract is
included in this adapter version. `Abort` discards local upload state because
no remote multipart-abort endpoint is documented. Validation is currently
limited to deterministic local contract tests; no credentialed live smoke test
has been run.

Official documentation:

- <https://open.douyin.com/platform/resource/docs/develop/permission/toutiao-or-xigua/OAuth2.0/>
- <https://open.douyin.com/platform/resource/docs/ability/content-management/toutiao-publish-solution>
- <https://open.douyin.com/platform/resource/docs/openapi/video-management/toutiao/create-video/upload-video>
- <https://open.douyin.com/platform/resource/docs/openapi/video-management/toutiao/create-video/publish-video/>
- <https://open.douyin.com/platform/resource/docs/openapi/video-management/toutiao/search-video/account-video-list/>
- <https://open.douyin.com/platform/resource/docs/openapi/video-management/toutiao/search-video/video-data/>

Last verified: 2026-08-02.
