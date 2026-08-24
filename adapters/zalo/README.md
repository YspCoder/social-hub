# Zalo Official Account adapter

The `zalo/official-account` adapter implements the documented Zalo OA
OpenAPI contracts that fit social-hub's common model:

- OAuth v4 authorization-code exchange with PKCE and rotating refresh tokens;
- v3 consultation text messages;
- linked OA information and OA-scoped user profiles;
- `X-ZEvent-Signature` verification and typed webhook decoding.

It deliberately does not treat ZBS Template Message, GMF group chat, voice, or
Zalo Social Login as OA capabilities. Article creation is also excluded from
the common `Publisher`: the official API is asynchronous and requires typed
title, cover, and body-block inputs that `socialhub.CreatePostRequest` cannot
represent without losing semantics.

Zalo OA OpenAPI is a business integration surface. Individual operations can
require a verified OA, an active OA package, a linked Zalo Cloud Account, and
the corresponding permission granted to the application.

## Configuration

```yaml
adapter: zalo/official-account
settings:
  base_url: https://openapi.zalo.me
  oauth_base_url: https://oauth.zaloapp.com
accounts:
  - id: support-vn
    app_id: "360846524940903967"
    secret_ref: env://ZALO_APP_SECRET
    access_token_ref: vault://social/zalo/support-vn/access-token
    webhook:
      # OA Secret Key, not the application secret.
      secret_ref: env://ZALO_OA_SECRET
    settings:
      oa_id: "388613280878808645"
```

`access_token_ref` is required. `app_id` plus `secret_ref` enables the OAuth
helper. `app_id` plus `webhook.secret_ref` enables webhook verification. The
optional `settings.oa_id` rejects callbacks and OA profiles routed to the wrong
configured account.

## OAuth v4

Zalo generates the authorization URL in the developer console. After its
callback returns the single-use authorization code, exchange it with the same
43-character PKCE verifier:

```go
oauth, err := adapter.OAuth(ctx, "support-vn")
if err != nil {
	return err
}

token, err := oauth.Exchange(ctx, authorizationCode, codeVerifier)
```

OA access tokens are valid for 25 hours. Refresh tokens are valid for three
months and are single-use. Persist the complete token returned by `Refresh`
atomically because it contains the next refresh token:

```go
next, err := oauth.Refresh(ctx, current.RefreshToken)
```

The adapter client uses an externally managed access token so applications can
coordinate this rotation through their existing encrypted `TokenStore`.

## Messages and profiles

The common `Messenger` sends consultation text to one Zalo user ID. The user
must satisfy Zalo's current interaction window, OA permission, package, and
quota rules.

```go
text := "Your order is ready."
message, err := client.SendMessage(ctx, socialhub.SendMessageRequest{
	ConversationID: "2512523625412515",
	Text:           &text,
})
```

Use the typed workflows when Zalo-specific response data is needed:

```go
sent, err := client.MessageWorkflow().SendConsultationText(
	ctx, "2512523625412515", "Your order is ready.",
)
profile, err := client.ProfileWorkflow().GetUserProfile(ctx, "2512523625412515")
oa, err := client.ProfileWorkflow().GetOA(ctx)
```

## Webhooks and limits

Pass the exact request bytes to `Verify` before calling `Decode`; JSON
re-encoding changes the signature input. The decoder preserves the raw event,
the `num_retry` count, typed message attachments, and a normalized
`socialhub.Message` for message events.

Zalo documents application-level limits of 4,000 OA API requests per minute
and 4,000 Article API requests per minute. Responses expose
`X-RateLimit-Limit` and `X-RateLimit-Remain`; platform error `-32` is mapped to
`socialhub.ErrRateLimited`. Message quotas are separate business limits.

Official references:

- [OA OpenAPI overview](https://developers.zalo.me/docs/official-account/bat-dau/kham-pha)
- [OAuth v4](https://developers.zalo.me/docs/official-account/bat-dau/xac-thuc-va-uy-quyen-cho-ung-dung-new)
- [Consultation text messages](https://developers.zalo.me/docs/official-account/tin-nhan/tin-tu-van/gui-tin-tu-van-dang-van-ban)
- [Webhook events](https://developers.zalo.me/docs/official-account/webhook/tong-quan)
- [Rate limits](https://developers.zalo.me/docs/official-account/phu-luc/gioi-han-toc-do-api)
- [OA error codes](https://developers.zalo.me/docs/official-account/phu-luc/ma-loi)
