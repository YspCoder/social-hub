# Feishu/Lark Open Platform adapter

`lark/openapi` integrates Feishu and Lark Open Platform IM v1 and Contact v3.
It uses only documented Open Platform endpoints and keeps platform-specific
message, resource, reaction, and event concepts available through typed
workflows.

## Product contract

| Item | Contract |
|---|---|
| Adapter name | `lark/openapi` |
| API product/version | IM v1 / Contact v3 |
| Feishu endpoint | `https://open.feishu.cn` |
| Lark endpoint | `https://open.larksuite.com` |
| Authentication | Bearer `tenant_access_token` or `user_access_token`, resolved from `access_token_ref` |
| Account identity | `region` and `token_kind` are required; `actor_id`, `tenant_key`, and `default_chat_id` are optional |
| Verification status | Deterministic local contract tests only; no real Feishu tenant or Lark workspace has been exercised |

Import the package to register the adapter, following the same opt-in model as
`database/sql` drivers:

```go
import _ "social-hub/adapters/lark"
```

One adapter can hold Feishu and Lark accounts at the same time. Credential
fields contain secret references, never literal token, verification-token, or
Encrypt Key values:

```yaml
version: 1
platforms:
  - adapter: lark/openapi
    accounts:
      - id: feishu-production
        app_id: cli_a1b2c3
        access_token_ref: secret://lark/feishu/tenant-access-token
        webhook:
          token_ref: secret://lark/feishu/verification-token
          aes_key_ref: secret://lark/feishu/encrypt-key
        approval:
          scopes:
            - im:message
            - im:message:send_as_bot
            - im:message:readonly
            - im:message.group_msg
            - im:resource
            - im:message.reactions:write_only
            - im:message.reactions:read
            - contact:contact.base:readonly
            - contact:user.base:readonly
        settings:
          region: feishu
          token_kind: tenant
          user_id_type: open_id
          actor_id: cli_a1b2c3
          tenant_key: tenant-key
          default_chat_id: oc_a1b2c3
      - id: lark-user
        app_id: cli_d4e5f6
        access_token_ref: secret://lark/global/user-access-token
        settings:
          region: lark
          token_kind: user
          user_id_type: open_id
          actor_id: ou_d4e5f6
```

`base_url` is an adapter-level override for deterministic tests or an approved
gateway. Production accounts should select the official origin through
`region: feishu` or `region: lark`.

## Capabilities and scopes

When `approval.scopes` is present, the adapter performs a local preflight and
returns `socialhub.CodeApprovalRequired` before network I/O when a method's
scope contract is not satisfied. When it is omitted, Open Platform remains the
authority. Scope availability depends on app type, app publication, tenant
administrator approval, and the installed application's data permissions.

| SDK capability | Open Platform operation | Scope notes |
|---|---|---|
| Tenant-token message send/reply/update/delete | IM v1 messages | One documented tenant message-send scope, normally `im:message` or `im:message:send_as_bot`; read operations accept an applicable message-read scope |
| User-token message send/reply/update/delete | IM v1 messages | Both `im:message` and `im:message.send_as_user` are required by the adapter contract |
| User fetch | Contact v3 user get | One applicable Contact read scope, normally `contact:contact.base:readonly`; `contact:user.base:readonly` is a separate field scope for basic profile fields such as name and avatar |
| Chat/thread history | IM v1 message list | Applicable message-read scope; common `ListPostsRequest.UserID` carries the `chat_id` |
| Image/file upload | IM v1 image/file create | Tenant token and `im:resource` or `im:resource:upload` |
| Reactions | IM v1 message reactions | `im:message.reactions:write_only`; common removal also requires reaction read access and a configured `actor_id` |
| Thread comments | IM v1 message reply/delete | Message write scopes; Lark replies are mapped to common comments |
| Events API | HTTP callback | Event subscriptions, app release, and event-specific permissions are configured in Open Platform |

Enterprise-only, unavailable, or unapproved capabilities are not silently
emulated. Configuration or scope failures return actionable
`CodeApprovalRequired`, `CodePermissionDenied`, or `CodeUnsupported` errors,
including the official scope-list URL where applicable. Applications can use
`Capabilities` at startup to disable unavailable UI and workflows.

## Message and reaction workflows

`Client.MessageWorkflow()` preserves Lark's `receive_id_type`, `msg_type`, and
JSON-string `content` contract. It supports text, rich post, interactive card,
image, file, audio, and video resource messages, plus reply, update, and delete.
The common `Messenger` supports text or one already uploaded resource. The
common `Publisher` uses `account.settings.default_chat_id`; omit that setting
when every destination must be explicit.

`socialhub.WithIdempotencyKey` maps to the documented message-body `uuid` field
for send and reply. It is deliberately removed from the HTTP
`Idempotency-Key` header. Operations without a documented UUID contract reject
the option.

`Client.ReactionWorkflow()` can create arbitrary emoji reactions and delete a
known reaction ID. The common `Reactor` maps `ReactionLike` to `THUMBSUP`.
Because Open Platform deletes by reaction ID rather than by emoji, common
removal lists reactions and selects the matching `actor_id`; configure that ID
to the authenticated user or app identity.

## IM resources

The common media interface models Open Platform's one-request image/file
upload as a local single-part session:

1. `BeginUpload` validates metadata and allocates a local session.
2. `UploadPart` accepts part `0`, verifies the declared byte count, and sends
   documented multipart fields to IM v1.
3. `CompleteUpload` validates the returned resource key and exposes a ready
   common `Media` object.

Images and animations are limited to 10 MB. Files, audio, and video are limited
to 30 MB. File categories preserve the documented `opus`, `mp4`, `pdf`, `doc`,
`xls`, `ppt`, and `stream` values. Resource keys are then sent through the
message content object; this API does not expose resumable or chunked upload.

## OAuth and token lifecycle

The adapter consumes an issued access token and does not own authorization
redirects, code exchange, refresh-token persistence, or tenant-token caching.

- `tenant_access_token` represents an app installed in one tenant and is used
  for bot messaging, reactions, Contact reads, and resource upload.
- `user_access_token` represents an authorized user. OAuth authorization and
  refresh use the app's user-token flow; user message calls require both user
  message scopes recorded above.
- Internal/self-built apps and store apps obtain tenant credentials through
  different documented token endpoints. Keep acquisition and renewal in the
  application's credential layer and update the value returned by
  `SecretResolver`.

Creating a new account client resolves the current secret value, so deployments
can integrate their existing secret manager and token refresh service without
placing refresh credentials inside adapter configuration.

## Rate limits

Open Platform rate limits are endpoint-, app-, tenant-, and resource-specific.
Common IM send, history, and upload endpoints document ceilings around 1000
requests per minute and 50 requests per second for eligible apps, while sending
to the same user or chat is additionally constrained to roughly 5 QPS. Actual
quotas can vary by application availability and tenant policy; callers must
treat the platform response as authoritative.

HTTP 429 and documented frequency-control codes such as `99991400`,
`99991401`, `230020`, `11232`, and `11233` map to
`socialhub.CodeRateLimited`. A valid `Retry-After` header is preserved. Apply
retry and rate limiting per account and operation, and serialize hot-chat sends
more aggressively than global traffic.

## Events API security

`webhook.token_ref` resolves the Verification Token used to bind a callback to
the configured app. Optional `webhook.aes_key_ref` resolves the Encrypt Key and
enables the complete encrypted callback contract:

- `Verify` hashes `timestamp + nonce + encrypt_key + raw_body` with SHA-256 and
  compares `X-Lark-Signature` in constant time.
- The encrypted body uses AES-256-CBC with `SHA-256(encrypt_key)` as the key,
  the first 16 decoded bytes as IV, and strict PKCS#7 padding validation.
- Verification Token, `app_id`, and optional `tenant_key` are checked after
  decryption.

The decoder supports URL verification, inbound IM messages, reaction-created
and reaction-deleted events, and unknown event retention. `HandleChallenge`
verifies and returns the required JSON challenge response. For normal events,
verify the exact raw bytes, durably deduplicate on event ID, and acknowledge
promptly before asynchronous processing.

## References and provenance

- [Send messages](https://open.feishu.cn/document/server-docs/im-v1/message/create)
- [Reply to messages](https://open.feishu.cn/document/server-docs/im-v1/message/reply)
- [Get a message](https://open.feishu.cn/document/server-docs/im-v1/message/get)
- [List messages](https://open.feishu.cn/document/server-docs/im-v1/message/list)
- [Get a user](https://open.feishu.cn/document/server-docs/contact-v3/user/get)
- [Upload an image](https://open.feishu.cn/document/server-docs/im-v1/image/create)
- [Upload a file](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/file/create)
- [Message reactions](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message-reaction/create)
- [Event Encrypt Key](https://open.feishu.cn/document/server-docs/event-subscription-guide/event-subscription-configure-/encrypt-key-encryption-configuration-case)
- [Permission scopes](https://open.feishu.cn/document/server-docs/application-scope/scope-list)
- [Official Go SDK](https://github.com/larksuite/oapi-sdk-go)

`github.com/larksuite/oapi-sdk-go/v3` v3.9.10 was reviewed as the official
reference for wire and event-security contracts. It is not imported and adds
no module dependency. The SDK's Go major version is not an Open Platform API
version; this adapter reports the actual products it calls as IM v1 and Contact
v3.
