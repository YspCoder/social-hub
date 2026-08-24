# Viber Bot API adapter

Adapter identity: `viber/bot-api`

Official contract: REST Bot API `7.3.0`

Documentation: <https://developers.viber.com/docs/api/rest-bot-api/>

## Access and authentication

Viber requires an active Bot and its account authentication token. Since
2024-02-05, new Viber Bots are available only on commercial terms through
Rakuten Viber or an official partner. The token is sent only in
`X-Viber-Auth-Token`; it is also the HMAC-SHA256 key used to verify webhook
bodies.

Configure the token as a secret reference rather than a literal value:

```yaml
adapter: viber/bot-api
accounts:
  - id: support
    access_token_ref: env://VIBER_BOT_TOKEN
    settings:
      sender_name: Social Hub
      sender_avatar: https://cdn.example.com/viber-avatar.jpg
```

`sender_name` is required because Viber includes a sender object in every
outbound message. It may contain at most 28 characters.

## Implemented capabilities

- Common `Messenger`: sends text to one subscribed Viber user.
- `MessageWorkflow`: typed text, picture, video, file, contact, location, URL,
  and sticker sends, plus broadcasts to as many as 300 subscribers.
- Common `Fetcher`: `GetUser("me")` reads the Bot account; other IDs read
  subscribed-user details. Posts, comments, timelines, and message history
  return `unsupported`.
- `AccountWorkflow`: Bot account details, subscriber profiles, and online state
  for as many as 100 user IDs.
- `WebhookWorkflow`: register, filter, and remove an HTTPS callback endpoint.
- Common `WebhookHandler`: verify `X-Viber-Content-Signature` over the exact raw
  body and decode message, subscription, conversation, delivery, seen, and
  failure events.

```go
common, err := adapter.Client(ctx, "support")
if err != nil {
    return err
}
client := common.(*viber.Client)

result, err := client.MessageWorkflow().Send(ctx, viber.SendRequest{
    Receiver: "subscriber-id-from-webhook",
    Message:  viber.TextMessage{Text: "Your request is ready."},
})
```

## Platform limits and operational notes

- A Bot can send to a user only after the user subscribes or within the single
  documented welcome-message window after `conversation_started`.
- Viber provides no endpoint for listing all subscribers. Persist sender/user
  IDs from verified callbacks.
- Request JSON is limited to 30 kB. Broadcast accepts at most 300 receivers and
  is documented at 500 requests per 10-second window.
- `get_user_details` may be called only twice per 12 hours for each user ID;
  cache the result outside the adapter.
- Picture, video, and file messages reference caller-hosted URLs. The adapter
  does not claim a media upload capability.
- Video and file declarations are bounded at 26 MB and 50 MB respectively.
- Viber retries failed webhook deliveries ten times with increasing intervals,
  so consumers must deduplicate `socialhub.Event.ID` and make handlers
  idempotent.
- Rich Media carousels and custom keyboards are not yet modeled. Use no private
  or browser-scraped API as a fallback.

All tests are deterministic local protocol tests. No credentialed live Bot was
used for validation.
