# Telegram Bot API adapter

Package `social-hub/adapters/telegram` implements the stable messaging,
media-send, and webhook subset of Telegram Bot API 10.2.

Implemented contracts:

- Bot Token authentication through `access_token_ref`;
- common text `Messenger`, including replies;
- common text `Publisher` and message deletion when
  `account.settings.default_chat_id` is configured;
- typed `BotWorkflow` for `getMe`, photo/video/audio/document/animation sends,
  webhook registration, and webhook deletion;
- Telegram file IDs, HTTP URLs, and streaming multipart uploads;
- `X-Telegram-Bot-Api-Secret-Token` verification and Update decoding with a
  typed model plus raw JSON for forward-compatible Bot API fields;
- Telegram error mapping, including flood-control `parameters.retry_after`.

The adapter uses the MIT-licensed, zero-dependency
`github.com/go-telegram/bot` module for maintained Bot API models and multipart
encoding. Social-hub disables that library's startup `getMe` request and
background polling, supplies the configured HTTP client and server URL, bounds
responses, and maps errors into `socialhub.Error`.

Cloud Bot API uploads are limited to 10 MB for photos and 50 MB for other
supported files. Reader uploads require a safe base filename and declared size;
the stream is also bounded if the declaration is wrong. Telegram uploads media
as part of a send operation, so the independent common `MediaUploader`
lifecycle is intentionally unavailable.

Configure `webhook.secret_ref` with a 1-256 character Telegram secret token to
enable the common `WebhookHandler`. Telegram webhooks and `getUpdates` long
polling are mutually exclusive; this adapter exposes webhooks and does not run a
polling loop. Generic message history, arbitrary message lookup, and publication
status polling are not available in the Bot API.
