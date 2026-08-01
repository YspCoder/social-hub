# Snapchat Public Profile API v1 adapter

Package `social-hub/adapters/snapchat` targets the official Snapchat Public
Profile API v1 and Marketing API OAuth 2.0 endpoints.

Implemented contracts:

- OAuth authorization-code exchange and refresh-token rotation with the
  `snapchat-profile-api` scope;
- typed `PublicProfileWorkflow` for an authorized profile, `my_profile`, public
  creator search, and authorized Spotlight list/get reads;
- mapping Public Profiles and Spotlights to common `User` and `Post` models;
- top-level and per-subrequest status validation, including HTTP 200
  `ERROR`/`PARTIAL` responses and request ID preservation.

The Public Profile API is allowlist-only. Create the OAuth app in Snap Business
Manager, not the Snap Kit Developer Portal, then request allowlisting through a
Snap contact. Login Kit provides app-scoped identity and Creative Kit provides a
client-side share entry point; neither is a server-side feed or publisher API.

All common capabilities are intentionally unavailable. Snapchat's publishing
flow requires an AES-256-CBC encrypted media object, upload parts of at most 32
MB (up to 35 parts), multipart finalization, and a separate Story or Spotlight
creation request. That flow needs a dedicated typed encrypted-upload contract
and is deferred from this first read-only adapter.

Marketing API rate limits are currently described as an average of 20 requests
per second per app and 10 requests per second per access token. Treat HTTP 429
as retryable and do not hard-code these published averages as permanent quotas.

Example account settings:

```yaml
adapter: snapchat/public-profile-v1
settings:
  base_url: https://businessapi.snapchat.com
  auth_url: https://accounts.snapchat.com/login/oauth2/authorize
  token_url: https://accounts.snapchat.com/login/oauth2/access_token
accounts:
  - id: creator
    client_id: "snap-business-oauth-client-id"
    secret_ref: env://SNAPCHAT_CLIENT_SECRET
    access_token_ref: env://SNAPCHAT_ACCESS_TOKEN
    settings:
      profile_id: "76da494b-76bc-4bbb-bb27-c5a66fb0d1ab"
    approval:
      scopes:
        - snapchat-profile-api
```

Official documentation:

- <https://developers.snap.com/marketing-api/Public-Profile-API/GetStarted>
- <https://developers.snap.com/marketing-api/Public-Profile-API/Profiles>
- <https://developers.snap.com/marketing-api/Public-Profile-API/ProfileAssetManagement>
- <https://developers.snap.com/marketing-api/Public-Profile-API/CreatorDiscovery>
- <https://developers.snap.com/marketing-api/Ads-API/authentication>
- <https://developers.snap.com/marketing-api/Ads-API/rate-limits>
