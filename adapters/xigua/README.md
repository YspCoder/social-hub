# Xigua OpenAPI adapter

Registry name: `xigua/openapi`

Package `social-hub/adapters/xigua` implements the public Xigua OAuth and
small-video APIs exposed through the Douyin Open Platform documentation.

Implemented contracts:

- OAuth 2.0 Authorization Code exchange, user-token refresh, and application
  `client_token` helpers;
- authorized-user profile from the Xigua OAuth origin;
- owned-video list and per-video data;
- direct and multipart video upload;
- asynchronous small-video publication and local pending-state tracking;
- typed Xigua publication metadata for title, summary, cover timestamp,
  original declaration, and reward eligibility;
- typed `video.Workflow` built on the common upload and publish interfaces.

Xigua uses two distinct origins. Business video APIs use
`https://open.douyin.com`, while authorization, token, and user-information
requests use `https://open-api.ixigua.com`; interactive authorization uses
`/oauth/connect`. The adapter keeps separate transports
for these origins and refuses redirects. User APIs require a user
`access_token` and its app-scoped `open_id`; an application `client_token` is
returned only by the OAuth helper and cannot create an account client.

```yaml
adapter: xigua/openapi
product: openapi
accounts:
  - id: primary
    client_id: awxxxxxxxx
    secret_ref: env://XIGUA_CLIENT_SECRET
    access_token_ref: env://XIGUA_USER_ACCESS_TOKEN
    settings:
      open_id: app-scoped-open-id
    approval:
      scopes: [user_info, xigua.video.create, xigua.video.data]
```

The app and requested scopes must be approved in the platform console. Missing
approval is returned as `socialhub.CodeApprovalRequired` with the required
scope and approval URL; the adapter does not emulate restricted or
enterprise-only APIs.

The publication API supports small videos only, with a documented maximum
duration of one minute and maximum multipart file size of 16 GiB. Files up to
128 MiB use direct upload; larger files use multipart upload as required by the
platform. The part size is 20 MiB and non-final parts must be at least 5 MiB.
Upload completion does not imply publication approval. Titles contain 5-30
Unicode characters and summaries contain at most 400 characters. Original and
reward flags additionally depend on account identity and creator eligibility.

No public deletion, comment, reaction, message, or signed webhook contract is
included in this adapter version. `Abort` discards local upload state because
no remote multipart-abort endpoint is documented. Validation is currently
limited to deterministic local contract tests; no credentialed live smoke test
has been run.

Official documentation:

- <https://open.douyin.com/platform/resource/docs/develop/permission/toutiao-or-xigua/OAuth2.0/>
- <https://open.douyin.com/platform/resource/docs/openapi/account-permission/xigua-get-permission-code/>
- <https://open.douyin.com/platform/resource/docs/ability/content-management/xigua-publish-solution>
- <https://open.douyin.com/platform/resource/docs/openapi/video-management/xigua/create-video/upload-video>
- <https://open.douyin.com/platform/resource/docs/openapi/video-management/xigua/create-video/publish-video/>
- <https://open.douyin.com/platform/resource/docs/openapi/video-management/xigua/search-video/account-video-list/>
- <https://open.douyin.com/platform/resource/docs/openapi/video-management/xigua/search-video/video-data/>

GitHub reference evaluation: <https://github.com/xopenapi/douyin-open-api-go>
was checked for
historical Xigua endpoint and model names, but is not imported. It has no
license or release, was last pushed in 2020, and its generated requests use an
obsolete query-token contract instead of the current `access-token` header.

Last verified: 2026-08-02.
