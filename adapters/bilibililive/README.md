# Bilibili Live Open Platform adapter

Package `social-hub/adapters/bilibililive` implements Bilibili's public Live
Open Platform contracts for approved interactive projects.

Registration name:

```text
bilibili/live-open-platform-v2
```

Implemented contracts:

- request signing with the documented `x-bili-*` headers, body MD5,
  HMAC-SHA256, and signature version `1.0`;
- `POST /v2/app/start`, `/v2/app/end`, `/v2/app/heartbeat`, and
  `/v2/app/batchHeartbeat`;
- typed project, anchor, WebSocket, and batch-heartbeat models;
- 16-byte big-endian WebSocket protocol framing, bounded recursive zlib
  decoding, AUTH and 20-second heartbeat packets;
- ordered cluster failover and reconnect without exposing `auth_body` in logs;
- typed payloads for danmaku, mirror danmaku, gifts, Super Chat, guard, likes,
  room entry, live start/end, and interaction-end commands;
- raw JSON preservation for documented fields added after this adapter build and
  for unknown future commands.

## Configuration

`client_id` is the Bilibili `AccessKeyId`, `secret_ref` resolves the
`AccessKey Secret`, and `app_id` is the numeric Live Open Platform project ID.
The broadcaster identity code is intentionally passed only to `StartProject`;
it is not persistent account configuration.

```yaml
version: 1
platforms:
  - adapter: bilibili/live-open-platform-v2
    accounts:
      - id: live-project
        client_id: your-access-key-id
        secret_ref: env://BILIBILI_LIVE_ACCESS_KEY_SECRET
        app_id: "1234567890123"
```

The API origin is fixed to Bilibili's HTTPS endpoint,
`https://live-open.biliapi.com`. Adapter-level settings are rejected so signed
credentials cannot be redirected to another origin.

## Lifecycle requirements

Call `StartProject`, connect with the unchanged `WebSocketInfo` through the same
client, and call the REST project heartbeat about every 20 seconds. The
WebSocket stream maintains its separate protocol heartbeat automatically.
These are two different heartbeats and neither replaces the other.

Always call `ProjectSession.End` when the project finishes. Bilibili states
that omitting `/v2/app/end` can keep interactive items online, block the next
session, and affect revenue. Interactive projects can expire after 60 seconds
without REST heartbeats; H5 plugins and tools can expire after 180 seconds.
Batch heartbeat requests are conservatively limited to 199 unique game IDs,
matching the official “less than 200” request contract.

The inbound live stream is a WebSocket capability, not an HTTP Webhook, and is
therefore exposed through `LiveMessages()` instead of
`socialhub.WebhookHandler`.

## Approval and identity boundaries

Access requires Live Open Platform developer registration, an approved
application/project, a valid broadcaster identity code, and any applicable
room/IP allowlists. `open_id` is the event identity; legacy `uid` fields are
deprecated and normally zero. `union_id` is empty unless Bilibili separately
grants that capability.

This adapter is separate from `bilibili/open-platform`. The creator platform
uses different origins, OAuth user tokens, and signature version `2.0`; those
credentials and request signers are not interchangeable.

Official documentation:

- [Authentication and error codes](https://open-live.bilibili.com/document/74eec767-e594-7ddd-6aba-257e8317c05d)
- [Application API](https://open-live.bilibili.com/document/eba8e2e1-847d-e908-2e5c-7a1ec7d9266f)
- [Binary protocol](https://open-live.bilibili.com/document/657d8e34-f926-a133-16c0-300c1afc6e6b)
- [Commands](https://open-live.bilibili.com/document/f9ce25be-312e-1f4a-85fd-fef21f1637f8)

Bilibili also lists `VTB-LINK/bianka` as a community Go SDK. It is a useful
contract reference, but Bilibili explicitly makes users responsible for its
risk. This package instead reuses the repository's existing
`github.com/coder/websocket` dependency and shared transport, avoiding Bianka's
additional Resty, Gorilla WebSocket, and `pkg/errors` dependency graph.
