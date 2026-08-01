# social-hub

`social-hub` is a Go SDK that exposes capability-oriented adapters for social
media APIs. Applications import only the platform adapters they use and work
against the common interfaces in `pkg/socialhub`.

The project is under active development. The public API is not stable before
the first tagged alpha release.

## Implemented adapters

| Adapter | Capabilities | Status |
|---|---|---|
| `x/v2` | OAuth2 PKCE, posts, users/timelines, replies, reactions, media upload | Local contract tests |
| `facebook/page` | OAuth2, Page posts/feed, comments/likes, photo upload, webhooks | Local contract tests |
| `wechat/official-account` | App token, follower profiles, customer-service messages, drafts, materials, XML/AES webhooks | Local contract tests |

No adapter has been validated against a real platform account yet. The initial
development phase intentionally uses deterministic local HTTP fixtures only.

## Development

```powershell
go test ./...
go test -race ./...
go vet ./...
```

See [the implementation blueprint](docs/social-hub-blueprint.md) for the
supported-platform plan, architecture, and delivery milestones.
