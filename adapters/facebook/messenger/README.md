# Facebook Messenger Platform adapter

Adapter identity: `facebook/messenger-platform`

This package implements the official Messenger Platform contracts for
Facebook Pages on Graph API `v26.0`. It is intentionally separate from
`facebook/page`: Messenger conversations are not Page posts, feeds, comments,
or likes.

Implemented capabilities:

- common `Messenger` text sends and replies;
- typed `MessageWorkflow` text and image/audio/video/file attachment sends;
- common `Fetcher.GetUser` and typed `ProfileWorkflow` for PSID-scoped basic
  profiles;
- HMAC-SHA256 webhook verification, GET subscription challenges, typed
  messaging-event decoding, and normalized inbound/outbound messages.

`Publisher`, common `MediaUploader`, and `Reactor` are disabled. Messenger
Platform does not expose arbitrary message history, so `GetMessage` returns
`unsupported`. Remote attachments and reusable attachment IDs stay in the
typed workflow because the common model cannot safely infer Messenger's media
type.

## Configuration

```yaml
adapter: facebook/messenger-platform
accounts:
  - id: support-page
    access_token_ref: env://META_PAGE_ACCESS_TOKEN
    settings:
      page_id: "123456789"
    webhook:
      secret_ref: env://META_APP_SECRET
      token_ref: env://META_WEBHOOK_VERIFY_TOKEN
```

Use one account entry per Facebook Page. `access_token_ref` must resolve to a
Page access token and is sent as an `Authorization: Bearer` header. Token
issuance, extension, revocation, and durable rotation remain application
responsibilities.

`webhook.secret_ref` is the Meta App Secret used for POST signatures.
`webhook.token_ref` is the application-chosen verification token used only for
the GET subscription challenge; they are not interchangeable.

## Permissions and policy boundary

Sending normally requires `pages_messaging`. Subscribing the Page to webhook
fields also requires `pages_manage_metadata`. Production access can require
Advanced Access, App Review, Business Verification, the correct Page role or
business asset assignment, and a live app. PSID profile lookup additionally
requires Meta's **Business Asset User Profile Access** feature; an empty profile
response is mapped to `socialhub.ErrApprovalRequired` with actionable guidance.

A person must initiate the Messenger conversation. Free-form messages are
limited to Meta's standard 24-hour messaging window. This adapter supports only
the current `RESPONSE` and `UPDATE` messaging types and does not attempt to
bypass the window. Window-closed subcode `1545041` is surfaced as a user-action
conflict.

The historical `CONFIRMED_EVENT_UPDATE`, `ACCOUNT_UPDATE`, and
`POST_PURCHASE_UPDATE` message tags are not implemented; Meta returns error 100
for them from 2026-04-27. Marketing Messages and Sponsored Messages have
separate eligibility, consent, billing, and product contracts and are also
outside this adapter's first release.

Example text reply:

```go
message, err := client.MessageWorkflow().SendText(ctx, messenger.TextMessageRequest{
	RecipientID: psid,
	Text:        "Your order is ready.",
	Type:        messenger.MessagingResponse,
	ReplyToID:   inboundMessageID,
})
```

Attachment URLs must be public HTTPS URLs. Set `Reusable` only when sending a
URL that Meta should retain as a reusable attachment; otherwise pass a
previously returned attachment ID.

## Webhooks and limits

Pass the exact raw POST bytes to `Verify` before `Decode`; JSON re-encoding
changes the `X-Hub-Signature-256` input. The HTTP layer should return `200`
within five seconds, enqueue work asynchronously, and deduplicate
`socialhub.Event.ID` because callbacks can be delivered more than once.

The decoder accepts only `object=page` entries routed to the configured Page.
It types `message`, `delivery`, `read`, `postback`, and `reaction` events,
preserves unknown messaging payloads as generic raw events, and normalizes
message attachments, replies, direction, parties, and timestamps.

Meta does not publish one fixed numeric request limit for all Messenger Send
API integrations. Do not hard-code a global requests-per-second value. Treat
Graph error `613`, HTTP `429`, `Retry-After`, transient flags, Page/app usage
signals, and current Meta dashboard limits as runtime authority. Retry only
errors classified as retryable by `socialhub.Error`.

All tests use deterministic local HTTP and signed-webhook fixtures. No real
Page token or production Meta app was used. The implementation adds no
Messenger-specific third-party dependency; public GitHub examples were used
only to cross-check protocol shapes.

Official references:

- <https://developers.facebook.com/documentation/business-messaging/messenger-platform>
- <https://developers.facebook.com/documentation/business-messaging/messenger-platform/send-messages>
- <https://developers.facebook.com/documentation/business-messaging/messenger-platform/webhooks>
- <https://developers.facebook.com/documentation/business-messaging/messenger-platform/identity/user-profile>
- <https://developers.facebook.com/documentation/business-messaging/messenger-platform/overview>
- <https://github.com/fbsamples/original-coast-clothing>
