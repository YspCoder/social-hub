# WhatsApp Cloud API adapter

Package `social-hub/adapters/whatsapp` implements Meta's official WhatsApp
Business Platform Cloud API. It uses Graph API `v25.0` at
`https://graph.facebook.com/v25.0` and does not use WhatsApp Web, cookies,
browser automation, scraping, or private endpoints.

Implemented contracts:

- common `Messenger` text messages and replies;
- typed media, approved-template, reaction, and read-receipt messages;
- streaming multipart media upload, metadata retrieval, and deletion;
- business profile read and update;
- HMAC-SHA256 webhook verification and subscription challenges;
- batched inbound-message and delivery-status webhook decoding with raw
  message/status objects preserved.

WhatsApp conversations are not generic social posts. `Publisher`, `Fetcher`,
common `MediaUploader`, and `Reactor` are therefore disabled. The Cloud API
does not provide arbitrary message-history or contact-profile lookup, so
`Messenger.GetMessage` returns an unsupported error. Use `MessageWorkflow`,
`MediaWorkflow`, and `BusinessProfileWorkflow` for the platform-specific
operations.

## Business setup and authentication

A production integration needs a Meta business portfolio, a WhatsApp Business
Account (WABA), and a registered business phone number. Template creation and
approval, business verification, display-name review, customer opt-in, billing,
and messaging/quality tiers are managed by Meta outside this SDK.

The Cloud API accepts user and system-user access tokens. Dashboard user tokens
expire after 24 hours. System-user tokens can be issued for up to 60 days or
without expiry, subject to Meta's current business and security controls. The
adapter consumes a pre-issued token from `access_token_ref`; token creation,
rotation, revocation, and durable storage remain the application's
responsibility.

The two principal permissions are capability-specific:

| Capability | Permission |
|---|---|
| Messages and media | `whatsapp_business_messaging` |
| Business profile | `whatsapp_business_management` |

When `approval.scopes` is configured, a missing permission is rejected before
network I/O. An empty scope list means the grant is unknown, so Meta remains the
authority.

Example configuration:

```yaml
adapter: whatsapp/cloud-v25
accounts:
  - id: whatsapp-main
    access_token_ref: env://WHATSAPP_SYSTEM_USER_ACCESS_TOKEN
    approval:
      scopes:
        - whatsapp_business_messaging
        - whatsapp_business_management
    settings:
      phone_number_id: "123456789012345"
      business_account_id: "987654321098765"
      app_secret_ref: env://META_APP_SECRET
      verify_token_ref: env://WHATSAPP_WEBHOOK_VERIFY_TOKEN
```

`phone_number_id` routes API calls and `messages` webhooks to one configured
account. `business_account_id` adds a WABA-level routing check. Use one account
entry per business phone number, including when several numbers belong to the
same WABA.

## Messaging policy boundary

Free-form text and media are generally limited to the 24-hour customer service
window opened by the customer's latest message. Outside that window, send a
Meta-approved template through `SendTemplate`. The adapter does not maintain a
local conversation-window clock because webhook delivery, Click-to-WhatsApp
rules, and Meta policy remain authoritative. Error `131047` is mapped to a
user-action permission failure; callers should select an approved template
instead of blindly retrying.

An HTTP success and returned `wamid` mean Meta accepted the request, not that the
message was delivered. Consume `whatsapp.status.*` webhook events and treat
`failed` as a terminal delivery result.

Passing an empty emoji to `SendReaction` removes the existing reaction. Template
parameters are accepted as JSON objects so new parameter variants can be used
without changing the common model.

## Media lifecycle

`UploadMedia` streams the declared bytes through multipart encoding and enforces
Meta's documented MIME-specific limits before network I/O: 5 MiB for JPEG/PNG,
16 MiB for supported audio and MP4/3GPP video, 100 KiB for WebP stickers, and
100 MiB for supported documents. Codec restrictions still require application-
level validation.

`GetMedia` returns Meta's authenticated download URL. The URL is valid for about
five minutes and downloading it also requires a valid access token. Uploaded
media should be treated as approximately 30-day storage; retain the source and
be prepared to upload it again. This adapter returns the signed URL but does not
download it, keeping credential-bearing cross-host downloads under application
control.

## Webhooks

Meta's subscription `verify_token` is an application-chosen handshake value. It
is not the Meta App Secret. `HandleChallenge` compares the former, while
`Verify` validates `X-Hub-Signature-256` over the exact raw POST body with the
App Secret.

The current decoder intentionally supports only the WABA `messages` field. It
expands batched inbound messages and message statuses into separate events and
preserves each raw object for forward compatibility. Other WABA fields, such as
template, phone-number, and Flow management updates, are ignored rather than
being advertised as correctly routed typed events.

Webhook delivery is at least once. Deduplicate `socialhub.Event.ID` in durable
application storage. For status events, the ID combines the `wamid`, status, and
timestamp so multiple delivery transitions remain distinct.

## Rate limits and verification status

WhatsApp applies dynamic throughput, WABA/app quotas, recipient pair limits,
quality limits, and policy limits. Do not hard-code a global requests-per-second
value. Treat HTTP 429, `Retry-After`, webhook delivery failures, quality state,
and platform codes such as `130429`, `131048`, and `131056` as the source of
truth. Retry only errors classified as retryable by `socialhub.Error`.

All current tests use deterministic local HTTP fixtures, including multipart
streaming and signed webhook payloads. The adapter has not been validated with a
real Meta app, WABA, or business phone number.

Official resources:

- <https://developers.facebook.com/docs/whatsapp/cloud-api/>
- <https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages/>
- <https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media/>
- <https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/>
- <https://www.postman.com/meta/whatsapp-business-platform/collection/wlk6lh4/whatsapp-cloud-api>
- <https://github.com/fbsamples/whatsapp-api-examples>
