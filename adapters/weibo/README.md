# Weibo Open API v2 adapter

Registry name: `weibo/v2`

Implemented capabilities:

- OAuth 2.0 Authorization Code URL and token exchange
- publish text and image posts, delete posts, and inspect publish status
- create reposts as independent `Post` entities with `RelationRepost`
- retrieve users, posts, authorized user timelines, and visible comments
- upload JPEG, PNG, and GIF images through `statuses/upload_pic`
- like/unlike, create/reply/delete comments

Weibo write APIs require the caller's real IP (`rip`). Configure it per account
as `settings.source_ip`; read-only operations remain usable without it.

```yaml
adapter: weibo/v2
product: open-api
accounts:
  - id: primary
    client_id: "123456"
    secret_ref: env://WEIBO_CLIENT_SECRET
    access_token_ref: env://WEIBO_ACCESS_TOKEN
    settings:
      source_ip: 203.0.113.10
```

The public OAuth flow does not provide a generally available refresh token.
Applications must reauthorize when the access token expires unless their Weibo
commercial agreement explicitly supplies a different lifecycle.

Direct messages and realtime callbacks are not exposed because those products
require separate approval. The adapter never falls back to cookies, private
endpoints, or browser automation.

Official documentation:

- <https://open.weibo.com/wiki/API>
- <https://open.weibo.com/wiki/Oauth2>
- <https://open.weibo.com/wiki/2/statuses/update>
- <https://open.weibo.com/wiki/2/statuses/upload_pic>

Last verified: 2026-08-01.
