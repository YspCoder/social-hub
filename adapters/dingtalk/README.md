# DingTalk OpenAPI adapter

`dingtalk/openapi-v1.0` integrates current DingTalk internal-application APIs
through the shared `social-hub` transport. The first version deliberately keeps
contact reads and application-bot sends as typed workflows.

## Supported surface

| Workflow | Operation | Platform requirement |
|---|---|---|
| Common `Fetcher` | `GetUser` by UnionID | `Contact.User.Read` |
| `ContactWorkflow` | Full `GetUserByUnionID` response | `Contact.User.Read` |
| `RobotWorkflow` | Group application-bot message | Enabled application robot and `robot_code` |
| `RobotWorkflow` | OTO batch message, at most 100 staff IDs | Enabled application robot and `robot_code` |
| `AuthWorkflow` | Force-refresh a managed application token | `client_id`, `secret_ref`, and `corp_id` |

The common `Messenger` capability is not advertised. DingTalk's robot endpoints
send messages but do not implement the arbitrary `GetMessage` operation required
by that interface. Stream event reception is also deferred to a later version.

## Configuration

```yaml
version: 1
platforms:
  - adapter: dingtalk/openapi-v1.0
    accounts:
      - id: mainland-operations
        client_id: dingxxxxxxxx
        secret_ref: env://DINGTALK_CLIENT_SECRET
        approval:
          scopes: [Contact.User.Read]
        settings:
          corp_id: ding-corp-id
          robot_code: dingxxxxxxxx
```

`client_id` is DingTalk's ClientId/AppKey. `secret_ref` resolves the
ClientSecret. `settings.corp_id` scopes both token acquisition and the shared
token-cache key. `settings.robot_code` is optional unless robot sends are used.
An externally managed token can instead be configured through
`access_token_ref`; explicit refresh is then unavailable.

The adapter uses the current multi-organization credential contract:

```text
POST /v1.0/oauth2/{corpId}/token
{"client_id":"...","client_secret":"...","grant_type":"client_credentials"}
```

This is intentionally different from the older generated-SDK contract
`POST /v1.0/oauth2/accessToken` with `appKey` and `appSecret`. The adapter does
not silently fall back to that endpoint. Applications migrating from the old
contract must provide `settings.corp_id` and verify that the application is
enabled for the current credential model.

Tokens refresh five minutes before expiry. Optional `socialhub.TokenStore`
entries are isolated by platform, product, account, and CorpId; 401 and
recognized invalid-token responses invalidate both memory and persistent cache.

## Robot messages

DingTalk requires `msgParam` to be a JSON-encoded string. Callers provide a JSON
object once and the adapter performs the second encoding:

```go
result, err := client.RobotWorkflow().SendGroupMessage(ctx, dingtalk.GroupMessageRequest{
    OpenConversationID: "cid...",
    Message: dingtalk.RobotMessage{
        Key:   "sampleText",
        Param: json.RawMessage(`{"content":"hello"}`),
    },
})
```

Parameters are bounded at 64 KiB and must be JSON objects. OTO recipient lists
must contain 1 to 100 unique staff IDs. The result preserves filtered,
flow-controlled, and invalid-recipient lists.

## Errors and quotas

Permission failures preserve DingTalk's `requestid` and `requiredScopes` and map
to `socialhub.ErrApprovalRequired`. HTTP 401 invalidates managed tokens. HTTP
429, `tooFast` codes, and legacy `90002`, `90006`, and `90018` map to retryable
rate-limit errors; HTTP 504 and other 5xx failures are retryable availability
errors.

DingTalk quotas depend on endpoint and purchased edition. General internal-app
documentation describes endpoint limits up to 40 QPS / 1,500 requests per
minute, while some standard entitlements enforce 20 QPS and monthly API quotas.
Treat platform response headers and console entitlements as authoritative. The
20-messages-per-minute custom-webhook robot is not implemented: application
robots are the supported product direction.

The adapter refuses redirects so credentials cannot be forwarded to a different
origin. Credential values are never logged.

## Upstream assessment

| Project | Use in this adapter |
|---|---|
| `alibabacloud-go/dingtalk` | Official generated schemas and paths were used as contract references. No dependency was added for three API calls. |
| `open-dingtalk/dingtalk-stream-sdk-go` | Preferred reference for a future Stream event workflow; not needed by this version. |
| `DingTalk-Real-AI/dingtalk-workspace-cli` | Token behavior reference only; it is an application rather than a reusable SDK layer. |
| `zhaoyunxing92/dingtalk` | Community and legacy API reference only. |
| `blinkbean/dingtalk` | Custom-webhook focus does not match the application-robot product used here. |

Deterministic local contract tests are the verification baseline. No live smoke
test is provided because every implemented operation requires real organization
credentials and platform approval.
