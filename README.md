# social-hub

`social-hub` is a Go SDK that exposes capability-oriented adapters for social
media APIs. Applications import only the platform adapters they use and work
against the common interfaces in `pkg/socialhub`.

The project is under active development. The public API is not stable before
the first tagged alpha release.

## Development

```powershell
go test ./...
go test -race ./...
go vet ./...
```

See [the implementation blueprint](docs/social-hub-blueprint.md) for the
supported-platform plan, architecture, and delivery milestones.
