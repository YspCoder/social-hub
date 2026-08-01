# TikTok API v2 adapter

Package `social-hub/adapters/tiktok` targets TikTok for Developers API v2. It
is independent of `social-hub/adapters/douyin`; credentials, scopes, endpoints,
models, and approval processes are not interchangeable.

Implemented contracts:

- Login Kit OAuth v2 authorization-code, PKCE, and refresh-token flows;
- Display API authorized-user profile, video lookup, and video pagination;
- typed `ContentWorkflow` for creator info, Direct Post video initialization,
  sequential chunk upload, and publication status;
- `TikTok-Signature` HMAC verification with a five-minute replay window;
- webhook normalization and TikTok envelope error mapping, including errors
  returned with HTTP 200.

The common `Publisher` and `MediaUploader` are deliberately unavailable.
Direct Post requires current creator privacy choices, explicit user consent,
brand/AIGC disclosures, a media-transfer source, and asynchronous status. Use:

```text
ContentWorkflow.CreatorInfo -> InitVideo -> UploadChunk -> Status
```

`PULL_FROM_URL` requires an HTTPS URL under a domain or prefix verified in the
TikTok developer portal. FILE_UPLOAD chunks are sequential and follow TikTok's
5-64 MB normal chunk and 128 MB final merged chunk rules. Signed upload URLs
never receive the user access token.

Apps need approval for `video.publish`. Unaudited clients are restricted to
private `SELF_ONLY` posts and small active-user limits. The caller must render
the latest `CreatorInfo` choices and comply with TikTok's sharing UX and music
usage requirements.

Example account settings:

```yaml
adapter: tiktok/v2
accounts:
  - id: creator
    client_id: "client-key"
    secret_ref: env://TIKTOK_CLIENT_SECRET
    access_token_ref: env://TIKTOK_ACCESS_TOKEN
    settings:
      open_id: "user-open-id"
    approval:
      scopes:
        - user.info.basic
        - video.list
        - video.publish
```
