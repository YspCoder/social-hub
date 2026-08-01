# Pinterest REST API v5 adapter

Package `social-hub/adapters/pinterest` targets the official Pinterest REST API
v5.28.0 and OAuth 2.0 endpoints.

Implemented contracts:

- authorized user-account lookup and owned-Pin list/get pagination;
- typed `PinWorkflow` for board-aware image URL and registered-video Pins;
- video registration, upload to the returned signed multipart endpoint, media
  status polling, and Pin deletion;
- authorization-code, continuous refresh-token, and client-credentials grants;
- Pinterest error, request ID, rate-limit reset, and HTTP status mapping.

The common `Publisher` and `MediaUploader` are unavailable because a Pin needs
Pinterest-specific `board_id` and discriminated `media_source` fields. The
signed upload URL and form parameters remain internal and are erased after one
successful upload. Pinterest API v5 does not expose organic Pin comments,
general likes, direct messages, or organic-content webhooks through these
public endpoints.

Apps require Pinterest API approval. Trial and Standard access tiers have
different quotas; Standard currently has a universal 100 requests/second per
user per app limit plus endpoint-category limits. Consume the returned
`x-ratelimit-*` headers rather than hard-coding those values.

Example account settings:

```yaml
adapter: pinterest/v5
accounts:
  - id: pinner
    client_id: "pinterest-app-id"
    secret_ref: env://PINTEREST_APP_SECRET
    access_token_ref: env://PINTEREST_ACCESS_TOKEN
    settings:
      user_id: "2783136121146311751"
    approval:
      scopes:
        - user_accounts:read
        - boards:read
        - boards:write
        - pins:read
        - pins:write
```
