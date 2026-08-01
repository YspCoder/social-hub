# Kakao Login and Talk REST adapter

Package `social-hub/adapters/kakao` implements the public Kakao Login, Kakao
Talk Social, and Kakao Talk Message REST APIs. It uses documented HTTPS
endpoints only and does not use cookies, scraping, browser automation, private
mobile endpoints, or reverse-engineered protocols.

Implemented contracts:

- OAuth 2.0 authorization-code exchange and refresh, including optional client
  secret authentication;
- common `Fetcher` access to the currently authorized Kakao Login user;
- typed authorized-user and approved-friend discovery;
- typed default and custom template sends to My Chatroom or up to five selected
  friends;
- common text `Messenger` sends when a default Product Link URL is configured;
- documented Kakao REST error, scope-consent, approval, and quota mappings.

The common `Publisher`, `MediaUploader`, `Reactor`, and `WebhookHandler`
capabilities are intentionally unavailable. Kakao Login and Talk APIs do not
expose generic social posts, reusable media uploads, reactions, message lookup,
or an inbound Talk webhook contract for this product combination.

## Configuration

Each account represents one Kakao Login service user. `client_id` is the Kakao
REST API key. `access_token_ref` contains that user's access token, and
`account.settings.user_id` contains the positive decimal service user ID
returned by Kakao Login.

```yaml
adapter: kakao/login-talk-rest
accounts:
  - id: kakao-main
    client_id: ${KAKAO_REST_API_KEY}
    access_token_ref: env://KAKAO_USER_ACCESS_TOKEN
    settings:
      user_id: "123456789"
      default_link_url: https://example.com
      friend_message_approved: false
```

Set `secret_ref` only when Client secret is enabled in the Kakao Developers
console. The OAuth helper omits `client_secret` when it is disabled. Access and
refresh tokens are returned to the caller and are never persisted by this
package.

## Consent, approval, and message semantics

User profile fields depend on the consent items configured for the app. Friend
discovery and friend messages additionally require Kakao's review and the
appropriate consent scopes; set `friend_message_approved: true` only after the
app has approval. Otherwise the typed friend workflows return an explicit
approval-required error without making a request.

Default text templates require at least one Product Link. Common `SendMessage`
uses `settings.default_link_url` for both web and mobile web links. The friend
message endpoint accepts at most five unique UUIDs and may return partial
delivery in `MessageResult.Failures`. Kakao does not return a message ID, so the
common outbound `Message.ID` remains empty.

Kakao documents daily and per-user quotas. Talk Message has a 30,000-call daily
quota, with additional limits of 100 calls per sender, 100 per recipient, and
20 per sender-recipient pair. Token issuance is also limited per user. The
adapter maps quota responses to the common retryable rate-limit error; callers
should use the shared limiter and retry policy rather than assuming one global
window.

All current tests use deterministic local HTTP fixtures. The adapter has not
been validated with a real Kakao Developers app or user.

Official documentation:

- <https://developers.kakao.com/docs/en/kakaologin/rest-api>
- <https://developers.kakao.com/docs/en/kakaologin/common>
- <https://developers.kakao.com/docs/en/kakaotalk-social/rest-api>
- <https://developers.kakao.com/docs/en/kakaotalk-message/rest-api>
- <https://developers.kakao.com/docs/en/message-template/default>
- <https://developers.kakao.com/docs/en/rest-api/error-code>
- <https://developers.kakao.com/docs/en/getting-started/quota>

Last verified: 2026-08-02.
