# Microsoft Teams Microsoft Graph v1.0 adapter

`microsoft-teams/graph-v1` integrates Microsoft Teams chat and channel messages
through documented Microsoft Graph `v1.0` endpoints. It preserves targets,
thread replies, HTML bodies, inline hosted content, reactions, soft deletion,
and basic change subscriptions through typed workflows.

## Product contract

| Item | Contract |
|---|---|
| Adapter name | `microsoft-teams/graph-v1` |
| API product/version | Microsoft Graph `v1.0` |
| Global endpoint | `https://graph.microsoft.com/v1.0` |
| US Government L4 endpoint | `https://graph.microsoft.us/v1.0` |
| US Government L5 (DoD) endpoint | `https://dod-graph.microsoft.us/v1.0` |
| China endpoint | `https://microsoftgraph.chinacloudapi.cn/v1.0` |
| Authentication | Bearer delegated or application access token resolved from `access_token_ref` |
| Verification status | Deterministic local contract tests only; no real Microsoft 365 tenant has been exercised |

Import the package to register the adapter, following the same opt-in model as
`database/sql` drivers:

```go
import _ "social-hub/adapters/microsoftteams"
```

Each account selects its national cloud explicitly. Tokens are cloud-specific
and must be issued by the corresponding Microsoft identity platform authority;
a token issued for one cloud cannot be reused against another cloud.

```yaml
version: 1
platforms:
  - adapter: microsoft-teams/graph-v1
    accounts:
      - id: contoso-delegated
        access_token_ref: secret://microsoft/contoso/delegated-token
        webhook:
          secret_ref: secret://microsoft/contoso/client-state
        approval:
          scopes:
            - ChannelMessage.Send
            - ChannelMessage.Read.All
            - ChannelMessage.ReadWrite
            - ChatMessage.Send
            - Chat.Read
            - Chat.ReadWrite
        settings:
          cloud: global
          token_kind: delegated
          tenant_id: 11111111-2222-3333-4444-555555555555
          actor_id: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
          default_team_id: 99999999-8888-7777-6666-555555555555
          default_channel_id: 19:channel-id@thread.tacv2
      - id: china-reader
        access_token_ref: secret://microsoft/china/application-token
        approval:
          scopes:
            - ChannelMessage.Read.All
            - Chat.Read.All
        settings:
          cloud: china
          token_kind: application
          tenant_id: 00000000-1111-2222-3333-444444444444
```

`default_chat_id` is mutually exclusive with `default_team_id` plus
`default_channel_id`. Omit all default-target settings when every call should
name its destination. Adapter-level `base_url` is reserved for deterministic
tests and approved gateways.

## Permission and token boundaries

The adapter consumes issued access tokens. OAuth 2.0 Authorization Code,
`offline_access`, refresh-token rotation, confidential-client credentials, and
tenant authority selection remain in the application's credential layer.
Creating a new account client resolves the current `access_token_ref` value.

| Operation | Delegated work/school token | Application token |
|---|---|---|
| Send channel message or reply | `ChannelMessage.Send` | Not supported for ordinary sends |
| Send chat message or reply | `ChatMessage.Send` or `Chat.ReadWrite` | Not supported for ordinary sends |
| Read channel messages/replies | `ChannelMessage.Read.All` | `ChannelMessage.Read.Group` or `ChannelMessage.Read.All` |
| Read chat messages/replies | `Chat.Read` or `Chat.ReadWrite` | `ChatMessage.Read.Chat`, `Chat.Read.All`, or `Chat.ReadWrite.All` |
| Edit/soft-delete channel message | `ChannelMessage.ReadWrite` | Not supported for ordinary messages |
| Edit/soft-delete chat message | `Chat.ReadWrite` | Not supported for ordinary messages |
| Channel reaction | `ChannelMessage.Send` | Not supported |
| Chat reaction | `Chat.ReadWrite` or `ChatMessage.Send` | Not supported |

`Teamwork.Migrate.All` is intentionally not treated as an ordinary send
permission. Application-only `policyViolation` updates are a DLP-specific
contract and are not exposed as normal editing. When `approval.scopes` is
configured, calls fail locally with `CodeApprovalRequired` before network I/O
if no applicable scope is recorded; when scopes are omitted, Graph remains the
authority.

## Targets, messages, and pagination

Graph message IDs do not identify their chat or channel on their own. The
typed API therefore uses `Target` and `MessageRef`. Common IDs use reversible
base64url references:

```text
chat~<chat-id>
channel~<team-id>~<channel-id>
<conversation-ref>~<root-message-id>
<conversation-ref>~<root-message-id>~<reply-message-id>
```

Use `ConversationRef`, `ParseConversationRef`, `EncodeMessageRef`, and
`ParseMessageRef` instead of assembling these values manually. Common
`ListPostsRequest.UserID` carries a conversation ref, not an Entra user ID.

`Client.MessageWorkflow()` supports root/reply send, get, list, update, and
soft-delete across chats and channels. Message bodies preserve Graph `text` or
`html` content. Typed lists cap `$top` at 50 and return Graph's opaque
`@odata.nextLink`; follow-up calls verify that the cursor remains in the
configured national cloud and API root before issuing a request.

The common `Messenger`, `Publisher`, `Fetcher`, and comment APIs expose only
plain text where the mapping is unambiguous. Common publishing uses the
configured default target. HTML, importance, attachments, and inline content
remain available through the typed workflow.

Ordinary message editing is currently exposed only for `cloud: global`, which
matches the documented deployment availability. Other clouds return
`CodeUnsupported` before network I/O. Send, read, reactions, and soft-delete
use their documented national-cloud endpoints.

## Hosted content and files

Typed sends accept inline `HostedContent` using Graph's temporary-ID contract:

```go
request := microsoftteams.SendRequest{
    Target: microsoftteams.Target{Kind: microsoftteams.TargetChat, ChatID: chatID},
    Body: microsoftteams.MessageBody{
        ContentType: "html",
        Content: `<p>Status</p><img src="../hostedContents/1/$value">`,
    },
    HostedContents: []microsoftteams.HostedContent{{
        TemporaryID: "1",
        ContentType: "image/png",
        ContentBytes: pngBytes,
    }},
}
```

Hosted content is limited to 4 MB per message. It is not a file-upload API.
Teams file attachments live in SharePoint or OneDrive and require those
products' upload and sharing workflow before their Graph-backed attachment is
referenced. For that reason this adapter does not advertise the common
`MediaUploader` capability.

## Reactions

`Client.ReactionWorkflow()` calls `setReaction` and `unsetReaction` for root
messages and replies. It accepts Graph compatibility values such as `like`,
`angry`, `sad`, `laugh`, `heart`, and `surprised`, as well as supported Unicode
reaction values. The common `Reactor` maps only `ReactionLike` to `like`.
Reactions are always applied as the delegated caller; an explicit common
`ActorID` must match configured `actor_id`.

## Basic change notifications

`webhook.secret_ref` stores the secret `clientState` used when creating a
subscription and validating every delivered notification. The adapter:

- returns the URL-decoded `validationToken` from `HandleChallenge` for the HTTP
  layer to echo as `text/plain` within Graph's validation deadline;
- compares every `clientState` in constant time and, when configured, checks
  `tenant_id`;
- supports basic notifications without embedded resource content;
- rejects `encryptedContent` and `validationTokens` with `CodeUnsupported`
  rather than treating an unverified rich notification as trusted data.

Teams basic notification payloads do not always include a top-level `id`. In
that case the adapter derives a stable SHA-256 event ID from the subscription,
change type, resource, and subscription expiration. Consumers should durably
deduplicate on `socialhub.Event.ID` and acknowledge valid batches promptly.

`Client.SubscriptionWorkflow()` creates, renews, and deletes `/subscriptions`.
It always sends `includeResourceData: false` and the configured `clientState`.
Teams subscriptions whose expiration is more than one hour away must include
an HTTPS `lifecycleNotificationUrl`.

## Rate limits and errors

Teams messaging throttles are resource-sensitive. Microsoft documents limits
around one request per second per app/tenant and channel or chat, one message
send per second by a user to a given channel or chat, and roughly four requests
per second per app against one team. Exact limits can vary by API and tenant;
Graph responses remain authoritative.

HTTP 429 maps to retryable `socialhub.CodeRateLimited` and preserves a bounded
numeric `Retry-After`. Authentication, authorization, not-found, conflict, and
5xx responses map to the common error taxonomy while retaining the sanitized
Graph error code, message, and request ID. Rate limiters should key hot message
operations by account plus target rather than only by tenant.

## References and provenance

- [Send chatMessage](https://learn.microsoft.com/en-us/graph/api/chatmessage-post?view=graph-rest-1.0)
- [Get chatMessage](https://learn.microsoft.com/en-us/graph/api/chatmessage-get?view=graph-rest-1.0)
- [List channel messages](https://learn.microsoft.com/en-us/graph/api/channel-list-messages?view=graph-rest-1.0)
- [List chat messages](https://learn.microsoft.com/en-us/graph/api/chat-list-messages?view=graph-rest-1.0)
- [List replies](https://learn.microsoft.com/en-us/graph/api/chatmessage-list-replies?view=graph-rest-1.0)
- [Update chatMessage](https://learn.microsoft.com/en-us/graph/api/chatmessage-update?view=graph-rest-1.0)
- [Soft-delete chatMessage](https://learn.microsoft.com/en-us/graph/api/chatmessage-softdelete?view=graph-rest-1.0)
- [Set reaction](https://learn.microsoft.com/en-us/graph/api/chatmessage-setreaction?view=graph-rest-1.0)
- [Hosted content](https://learn.microsoft.com/en-us/graph/api/resources/chatmessagehostedcontent?view=graph-rest-1.0)
- [Teams message change notifications](https://learn.microsoft.com/en-us/graph/teams-changenotifications-chatmessage)
- [Webhook delivery and validation](https://learn.microsoft.com/en-us/graph/change-notifications-delivery-webhooks)
- [National cloud deployments](https://learn.microsoft.com/en-us/graph/deployments)
- [Service-specific throttling](https://learn.microsoft.com/en-us/graph/throttling-limits)
- [Delegated authorization code flow](https://learn.microsoft.com/en-us/graph/auth-v2-user)
- [Official Microsoft Graph Go SDK](https://github.com/microsoftgraph/msgraph-sdk-go)

`github.com/microsoftgraph/msgraph-sdk-go` v1.100.0 was reviewed as the official
reference for generated Graph v1.0 paths and models. It is not imported and
adds no module dependency.
