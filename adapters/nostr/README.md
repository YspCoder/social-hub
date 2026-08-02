# Nostr NIP-01 adapter

Adapter name: `nostr/nip-01`

This adapter implements the decentralized Nostr relay protocol rather than a
single hosted social API. It covers signed NIP-01 events plus the common
thread, deletion, repost, reaction, identifier, and media-metadata conventions
from NIP-09, NIP-10, NIP-18, NIP-19, NIP-25, and NIP-92.

## Identity and configuration

A read-only account needs a public key in 64-character lowercase hex, `npub`,
or `nprofile` form. A writable account additionally puts an `nsec` or exact
64-character hex private key behind `access_token_ref`:

```yaml
version: 1
platforms:
  - adapter: nostr/nip-01
    product: relay-protocol
    accounts:
      - id: community-main
        access_token_ref: env://NOSTR_PRIVATE_KEY
        settings:
          public_key: npub1...
          relay_urls:
            - wss://relay-one.example
            - wss://relay-two.example/nostr
          write_quorum: 1
```

`public_key` may be omitted when a private key is configured; when both are
present they must match. Relay URLs must use `ws` or `wss`, cannot contain
credentials, query parameters, or fragments, and are de-duplicated after
normalization. Use `wss` outside local development.

Incoming user IDs accept hex, `npub`, and `nprofile`. Event IDs accept hex,
`note`, and `nevent`. Common model IDs are always returned as canonical
64-character lowercase hex. Relay hints embedded in NIP-19 identifiers do not
cause dynamic connections: the client connects only to explicitly configured
relays, which keeps its network boundary predictable.

## Capabilities

| Capability | Coverage |
|---|---|
| `Fetcher` | kind 0 profile metadata, kind 1 notes, author note pages, and NIP-10 comments |
| `Publisher` | signed kind 1 text notes, replies, quotes, publication lookup, and NIP-09 deletion requests; signer required |
| `Reactor` | NIP-25 likes/removal and NIP-10 comments/deletion; signer required |
| `MediaUploader` | Not exposed; Nostr core has no uniform binary upload protocol |
| `Messenger` | Not exposed; a correct NIP-17 implementation also needs gift wrapping and recipient relay discovery |
| `WebhookHandler` | Not exposed; Nostr uses persistent WebSocket subscriptions, not HTTP callbacks |

Fetched NIP-92 `imeta` tags map URL, MIME, dimensions, byte size, hash, alt
text, blurhash, and fallback URLs into common `Media` values. An `imeta` URL is
mapped only when the same URL occurs in note content, as required by NIP-92.

## Relay consistency

Reads query all configured relays concurrently, verify every event ID and
BIP-340 Schnorr signature, de-duplicate by event ID, and succeed when at least
one relay reaches `EOSE`. Event extensions retain the relays that returned the
event and bounded details for partial relay failures.

Writes publish the same signed event to all relays. `write_quorum` is the
minimum accepted `OK` count and defaults to `1`. The `nostr.publish` extension
contains the successful and failed relay sets. Standard NIP-01 error prefixes
map as follows:

| Relay prefix | Common error |
|---|---|
| `rate-limited` | `ErrRateLimited`, retryable |
| `blocked`, `restricted`, `mute` | `ErrPermissionDenied`, user action |
| `invalid`, `pow` | `ErrInvalidArgument`, permanent |
| `duplicate` | `ErrConflict`, permanent |
| `error` or transport failure | platform error, retryable |

Nostr timestamps have one-second precision. List cursors therefore preserve
both the last timestamp and the number of events already consumed at that
second. Merged pages follow NIP-01 ordering: `created_at` descending, then event
ID lexicographically ascending.

## Threads, reactions, and reposts

Direct replies contain one NIP-10 `root` tag. Deeper replies contain both
`root` and `reply` markers, with author and relay hints where known. Quotes use
`q` tags and remain distinct from replies in `Post.Relations`.

The common `ReactionLike` publishes `+`. Nostr-specific dislike and emoji
content plus NIP-18 reposts are available through `InteractionWorkflow`:

```go
common, err := adapter.Client(ctx, "community-main")
if err != nil {
    return err
}
client := common.(*nostr.Client)

reaction, err := client.ReactWithContent(ctx, socialhub.ReactionRequest{
    TargetID: eventID,
    Kind:     socialhub.ReactionLike,
}, "-")

repost, err := client.Repost(ctx, eventID)
```

`DeletePost`, `DeleteComment`, and reaction removal publish signed kind 5
deletion requests. Per NIP-09, they cannot guarantee that every relay or client
will erase a previously observed event. The adapter also rejects deletion of
events not authored by the configured key.

## Implementation and validation

The adapter pins `fiatjaf.com/nostr` at commit `a8080728893f` for relay
connections, NIP-01 envelopes, filters, NIP-10 parsing, and NIP-19 encoding.
The upstream optimized event serializer is not checkptr-safe under Go's race
mode, so this package uses a standard-library canonical JSON encoder and
`btcec/schnorr` for the event hash/signature boundary, then verifies every
event before mapping it. This keeps `go test -race` usable without disabling
signature verification.

All contract tests use deterministic local WebSocket relays and exercise the
actual `REQ`, `EVENT`, `EOSE`, `OK`, `CLOSED`, and `CLOSE` wire messages. No
public relay or private interface is used.

Official references:

- <https://github.com/nostr-protocol/nips/blob/master/01.md>
- <https://github.com/nostr-protocol/nips/blob/master/09.md>
- <https://github.com/nostr-protocol/nips/blob/master/10.md>
- <https://github.com/nostr-protocol/nips/blob/master/18.md>
- <https://github.com/nostr-protocol/nips/blob/master/19.md>
- <https://github.com/nostr-protocol/nips/blob/master/25.md>
- <https://github.com/nostr-protocol/nips/blob/master/92.md>
