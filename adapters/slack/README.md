# Slack Web API adapter

`slack/web-api` integrates Slack's unversioned Web API and Events API. It uses
only documented Slack endpoints and keeps Slack-specific workflows available
without widening the common `socialhub` interfaces.

## Product contract

| Item | Contract |
|---|---|
| Adapter name | `slack/web-api` |
| API product/version | Web API, unversioned |
| Default endpoint | `https://slack.com/api` |
| Authentication | Bearer bot token (`xoxb-...`) or user token (`xoxp-...`) resolved from `access_token_ref` |
| Account identity | `workspace_id` is required; `actor_id` and `default_channel_id` are optional |
| Message/post ID | `channel_id:ts`, for example `C012ABC:1785571200.000001` |
| Verification status | Deterministic local contract tests only; no real Slack installation has been exercised |

Import the package to register the adapter, following the same opt-in model as
`database/sql` drivers:

```go
import _ "social-hub/adapters/slack"
```

An account can be configured in YAML as follows. Credential fields contain
secret references, never literal token or signing-secret values.

```yaml
version: 1
platforms:
  - adapter: slack/web-api
    accounts:
      - id: acme-workspace
        access_token_ref: secret://slack/acme/access-token
        webhook:
          secret_ref: secret://slack/acme/signing-secret
        approval:
          scopes:
            - chat:write
            - users:read
            - channels:history
            - groups:history
            - im:history
            - mpim:history
            - reactions:write
            - files:write
            - files:read
        settings:
          workspace_id: T012ABC
          token_kind: bot
          actor_id: U012ABC
          default_channel_id: C012ABC
```

Use `token_kind: user` for a user token. `actor_id` lets inbound Events API
messages be classified as inbound or outbound. `default_channel_id` enables
the common `Publisher`; omit it when every destination must be supplied through
`ChatWorkflow`. Multiple Slack installations are represented as separate
accounts with distinct IDs and secret references.

## Capabilities and scopes

The adapter treats `approval.scopes` as a local preflight contract. When it is
present, a call with a missing scope fails before network I/O. When it is
omitted, Slack remains the authority and a `missing_scope` response is mapped
to `socialhub.CodeApprovalRequired` with Slack's `needed` scopes preserved.

| SDK capability | Slack methods | Required scopes and notes |
|---|---|---|
| Common publish/message | `chat.postMessage`, `chat.delete` | `chat:write`; posting to a public channel without joining may additionally require `chat:write.public` |
| Typed chat update | `chat.update` | `chat:write`; only messages owned by the authenticated actor can normally be updated |
| User fetch | `users.info` | `users:read` |
| Public channel history | `conversations.history`, `conversations.replies` | `channels:history` |
| Private channel history | `conversations.history`, `conversations.replies` | `groups:history` |
| Direct-message history | `conversations.history`, `conversations.replies` | `im:history` for DMs; `mpim:history` or `groups:history` for group DMs, depending on the installation |
| Reactions | `reactions.add`, `reactions.remove` | `reactions:write`; the common like operation maps to Slack's `thumbsup` emoji |
| Thread comments | `chat.postMessage`, `chat.delete` | `chat:write`; Slack threads are flat, so nested `ParentID` values are rejected |
| Files | `files.getUploadURLExternal`, upload URL, `files.completeUploadExternal`, `files.info` | `files:write` for upload; `files:read` for uncached remote status/info |
| Events API | signed HTTP callback | Slack app signing secret in `webhook.secret_ref`; event subscriptions and their scopes are configured in Slack |

The common `ListPostsRequest.UserID` carries a Slack conversation ID because
the common model has no channel selector. `Messenger.GetMessage` and common
post lookup target top-level messages through `conversations.history`; use
thread listing for replies.

## Typed workflows

`Client.ChatWorkflow()` exposes explicit-channel posting and updates. The
common `Publisher` uses the configured default channel and supports thread
replies, status lookup, and deletion. Slack visibility comes from the target
conversation and cannot be selected per message.

`Client.FileWorkflow()` implements Slack's required external-upload sequence:

1. `files.getUploadURLExternal` allocates a file and upload URL.
2. `UploadFilePart` posts the bytes to that URL without the Slack Bearer token.
3. `files.completeUploadExternal` completes the file, optionally sharing it to
   a channel or thread with a title, alt text, snippet type, and initial comment.

The common media interface completes uploads privately. Use `FileWorkflow`
when the file must be shared to a channel or thread. The legacy `files.upload`
method was retired on 2025-11-12 and is intentionally not implemented. Upload
URLs must be HTTPS and must not contain credentials; HTTP is accepted only for
an explicitly HTTP test/gateway origin.

## OAuth and token lifecycle

Slack app installation uses OAuth v2. The adapter consumes an already issued
bot or user access token and does not implement authorization redirects, code
exchange, token storage, or refresh. Slack token rotation is optional; when it
is enabled, access tokens expire after 12 hours and must be refreshed using the
installation refresh token. Keep that lifecycle in the application's
credential layer and update the value returned by `SecretResolver`.

This separation prevents one adapter instance from owning durable installation
state and allows deployments to use their existing secret manager. A refreshed
credential is picked up when a client is created again.

## Rate limits

Slack applies Web API limits per method, workspace, and app, with method tiers
and special limits for selected endpoints. A 429 response is mapped to
`socialhub.CodeRateLimited`; `Retry-After` is preserved as a duration and must
be honored before retrying that method for that workspace.

`conversations.history` and `conversations.replies` have an important
distribution-specific rule. Commercially distributed apps outside the Slack
Marketplace are limited to one request per minute and 15 objects per request
(for new apps since 2025-05-29, and existing affected installations since
2026-03-03). Marketplace apps and internal customer-built apps retain Tier 3
behavior and can request larger pages, up to Slack's documented maximum of
1000. The SDK therefore defaults these reads to 15, while allowing callers to
explicitly request up to 1000 when their installation class permits it.

Events API delivery has a separate event-volume limit. `app_rate_limited`
callbacks are decoded as typed Slack events so applications can alert or shed
load without treating them as malformed requests.

## Events API security

`Verify` computes Slack's `v0:{timestamp}:{raw_body}` HMAC-SHA256 signature,
uses constant-time comparison, and rejects timestamps outside a five-minute
window. It also rejects callbacks whose `team_id` conflicts with the configured
workspace. Always pass the exact raw request body to `Verify` before calling
`Decode`; re-encoding JSON changes the signature input.

The decoder supports URL verification, `message`, `app_mention`,
`message_changed`, message and non-message reactions, `file_shared`, unknown
event callbacks, and `app_rate_limited`. Unknown event payloads are retained as
raw JSON. Use `event_id` for idempotent processing and inspect Slack retry
headers included in `EventsPayload`.

The adapter verifies and decodes callbacks but does not write HTTP responses.
The hosting application remains responsible for returning Slack's URL challenge
or a prompt HTTP 200 response after durable acceptance.

## References and provenance

- [Web API](https://docs.slack.dev/apis/web-api/)
- [Events API](https://docs.slack.dev/apis/events-api/)
- [Verifying requests](https://docs.slack.dev/authentication/verifying-requests-from-slack/)
- [Installing with OAuth](https://docs.slack.dev/authentication/installing-with-oauth/)
- [Token rotation](https://docs.slack.dev/authentication/using-token-rotation/)
- [Rate limits](https://docs.slack.dev/apis/web-api/rate-limits/)
- [External upload allocation](https://docs.slack.dev/reference/methods/files.getUploadURLExternal/)
- [External upload completion](https://docs.slack.dev/reference/methods/files.completeUploadExternal/)

`github.com/slack-go/slack` v0.27.0 was reviewed as a mature reference for
wire and security contracts. It is not imported and adds no module dependency.
