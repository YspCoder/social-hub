# Matrix Client-Server API v1.19 adapter

Package `social-hub/adapters/matrix` implements a focused, unencrypted subset
of the official Matrix Client-Server API v1.19. Each configured account points
to its own homeserver, so self-hosted and hosted Matrix deployments use the
same adapter.

Implemented contracts:

- existing Matrix access-token authentication through `access_token_ref`;
- profile lookup, room event lookup, room history, and incremental `/sync`;
- common text `Publisher` when `default_room_id` is configured;
- common `Fetcher`, `Messenger`, comments as `m.thread` events, and likes as a
  `👍` `m.reaction` annotation;
- typed unencrypted text, notice, emote, media-message, reaction, redaction,
  and room-history operations through `EventWorkflow`;
- typed streaming raw-body media upload through `MediaWorkflow`, returning the
  server-issued `mxc://` URI;
- Matrix error codes, request IDs, `Retry-After`, and legacy
  `retry_after_ms` mapping.

Matrix event IDs need room context for event lookup and redaction. Normalized
post, comment, and message IDs therefore use a reversible opaque composite ID:
`mx:<base64url-room-id>.<base64url-event-id>`. A raw `$event_id` is accepted
only when the account has a `default_room_id`. Dynamic Matrix IDs are escaped
as individual URL path segments, including IDs that contain `/`.

The common `MediaUploader` is intentionally unavailable because Matrix v1.19
uses a single raw-body upload rather than social-hub's resumable lifecycle. Use
`client.MediaWorkflow().Upload` and then `client.EventWorkflow().SendMedia`.
The declared upload byte count is enforced without buffering the complete
file. Homeservers define their own upload limits and can reject MIME types or
accounts that have exhausted quota.

Matrix clients receive events by long-polling `/sync`; this is not a signed
webhook protocol, so the common `WebhookHandler` is unavailable. Persist each
successful `next_batch` token and pass it as `SyncRequest.Since` for the next
incremental request. The typed response intentionally exposes joined-room
timelines only and does not claim to be a full local Matrix state store.

This first slice does not implement E2EE or encrypted attachments. It also
does not implement login, UIAA, OAuth discovery, token refresh, VoIP,
federation, Application Services, or room administration. Supply an already
provisioned access token through a secret resolver. Encountering an
`m.room.encrypted` event through common mapping returns `ErrUnsupported`
instead of exposing ciphertext as message text.

The stable [`mautrix-go`](https://github.com/mautrix/go) SDK was evaluated, but
its encryption and bridge-oriented surface is unnecessary for this bounded
HTTP/JSON adapter. A future E2EE or bridge package should use a mature Matrix
SDK rather than extending this adapter with custom cryptography.

Example account settings:

```yaml
adapter: matrix/client-server-v1.19
accounts:
  - id: community-main
    access_token_ref: env://MATRIX_ACCESS_TOKEN
    settings:
      homeserver_url: https://matrix.example.org
      user_id: "@social-hub:example.org"
      device_id: "SOCIALHUB01"
      default_room_id: "!room-id:example.org"
```

Matrix rate limits are homeserver-defined and endpoint-specific. `/sync` is
not rate-limited by the v1.19 contract, while event sending, redaction,
relations, and media upload can be limited. Treat HTTP 429 and `Retry-After` as
authoritative; `retry_after_ms` is retained only as the specification's
deprecated compatibility fallback.

Official documentation:

- <https://spec.matrix.org/v1.19/client-server-api/>
- <https://spec.matrix.org/v1.19/client-server-api/#using-access-tokens>
- <https://spec.matrix.org/v1.19/client-server-api/#get_matrixclientv3sync>
- <https://spec.matrix.org/v1.19/client-server-api/#put_matrixclientv3roomsroomidsendeventtypetxnid>
- <https://spec.matrix.org/v1.19/client-server-api/#post_matrixmediav3upload>
- <https://spec.matrix.org/v1.19/client-server-api/#rate-limiting>
