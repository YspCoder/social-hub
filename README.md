<div align="center">

<img src="assets/brand/social-hub-logo.png" width="360" alt="social-hub logo">

<p><strong>One typed interface for the world's social platforms.</strong></p>
<p>A capability-oriented Go SDK and self-hosted MCP bridge for global and China-market social APIs.</p>

<p>
  <img alt="Go 1.26" src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&amp;logoColor=white">
  <img alt="MCP 2026-07-28" src="https://img.shields.io/badge/MCP-2026--07--28-7C3AED">
  <img alt="191 adapter products" src="https://img.shields.io/badge/adapters-191-111827">
  <img alt="Alpha" src="https://img.shields.io/badge/status-alpha-F43F5E">
  <img alt="GitHub stars" src="https://img.shields.io/github/stars/YspCoder/social-hub?style=flat&amp;color=2563EB">
</p>

<p>
  <a href="README.md"><strong>English</strong></a>
  ·
  <a href="README.zh-CN.md">简体中文</a>
</p>

<p><sub>Documentation baseline: 2026-08-26 (Asia/Shanghai)</sub></p>

</div>

---

`social-hub` gives Go applications one set of contracts for publishing,
fetching, media, reactions, messaging, and webhooks while preserving typed
platform extensions where a lowest-common-denominator API would lose meaning.
Applications compile only the adapters they use, configure multiple accounts,
and keep credentials behind runtime secret references.

> [!IMPORTANT]
> `social-hub` is under active development. The public API is not stable before
> the first tagged alpha release. An adapter existing in the repository does
> not mean every platform account is approved for every capability.

## Project status

| Area | Status | Current baseline |
|---|:---:|---|
| Public models and capability interfaces | ✅ | Available in `pkg/socialhub` |
| Compile-time adapter registry | ✅ | 191 registered API products |
| Strict YAML/JSON configuration | ✅ | Version `1`; unknown fields fail validation |
| Runtime secret references | ✅ | Built-in `env://` plus custom resolvers |
| README coverage for every adapter | ❌ | 190/191 adapter packages |
| Local test-file coverage for every adapter | ❌ | 104/191 adapter packages |
| Credentialed platform validation for every adapter | ❌ | Not currently claimed |
| Stable public release | ❌ | Alpha development |

`✅` means the item is present. `❌` means coverage is incomplete. See the
[adapter catalog](docs/adapter-catalog.md) for package-level status.

## Why social-hub

| Principle | What it means in practice |
|---|---|
| Capability-oriented | Depend on `Publisher`, `Fetcher`, `Reactor`, `Messenger`, or another narrow interface instead of a monolithic client. |
| Global + China coverage | Use the same integration model for global platforms and China-market APIs without pretending their approval rules are identical. |
| Type-safe normalization | Work with common `User`, `Post`, `Media`, `Comment`, and `Message` models while retaining typed platform extensions. |
| Dependency control | Adapter registration follows the `database/sql` pattern; blank-import only what a binary needs. |
| Multi-account by design | Address every client by `{adapter, account_id}` and keep multiple apps or brands in one strict configuration. |
| Credentials stay server-side | Configuration stores `env://` or custom secret references, not plaintext access tokens. |
| Agent-ready | Run the optional self-hosted MCP server in read-only mode by default and explicitly allow each mutation. |

## Architecture

```text
 Application / Agent
         |
         +------------------------+
         |                        |
  pkg/socialhub contracts   cmd/social-hub-mcp
         |                  (optional, self-hosted)
         |
   adapter registry
         |
  +------+------+------+-------------------+
  |             |             |             |
 X / Meta   Telegram      WeChat       Custom adapter
  |             |             |             |
  +-------------+-------------+-------------+
                Platform APIs
```

The common client exposes six optional capability groups:

| Capability | Common operations |
|---|---|
| `Publisher` | Publish content, inspect asynchronous status, delete posts |
| `Fetcher` | Get users and posts, list posts and comments |
| `MediaUploader` | Begin resumable uploads, upload parts, complete and inspect media |
| `Reactor` | Add or remove reactions, create or delete comments |
| `Messenger` | Send and retrieve messages |
| `WebhookHandler` | Verify signatures and decode normalized events |

Platform-specific workflows remain typed. WeChat material management lives in
`extensions/material`, while short-video publication lives in
`extensions/video` instead of being forced through an unsafe generic map.

## Quickstart

### 1. Import only the adapters you need

Adapter packages register themselves from `init()`:

```go
import (
	_ "social-hub/adapters/wechat/officialaccount"
	_ "social-hub/adapters/x"
)
```

### 2. Configure accounts with secret references

```yaml
version: 1
defaults:
  timeout: 15s
platforms:
  - adapter: x/v2
    accounts:
      - id: brand-global
        access_token_ref: env://SOCIAL_HUB_X_TOKEN

  - adapter: wechat/official-account
    accounts:
      - id: brand-cn
        app_id: wx_your_app_id
        secret_ref: env://SOCIAL_HUB_WECHAT_SECRET
```

The built-in resolver accepts `env://NAME`. Vault or KMS integrations can be
provided through `socialhub.WithSecretResolver`.

### 3. Open an adapter and account client

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	_ "social-hub/adapters/wechat/officialaccount"
	_ "social-hub/adapters/x"
	"social-hub/pkg/socialhub"
)

func main() {
	ctx := context.Background()
	file, err := os.Open("social-hub.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	config, err := socialhub.LoadConfig(file)
	if err != nil {
		log.Fatal(err)
	}

	adapterConfig := config.Platforms[0]
	adapter, err := socialhub.Open(ctx, adapterConfig.Adapter, adapterConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer adapter.Close()

	client, err := adapter.Client(ctx, "brand-global")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	capabilities, err := client.Capabilities(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("platform=%s fetch=%t publish=%t\n",
		client.Platform(),
		capabilities.Has(socialhub.CapFetch),
		capabilities.Has(socialhub.CapPublish),
	)
}
```

Capability support is account- and approval-aware. Always handle
`socialhub.CodeUnsupported` and `socialhub.CodeApprovalRequired`; a capability
group being present does not guarantee every method is available.

## Agent and MCP support

`cmd/social-hub-mcp` exposes the common contracts as typed MCP tools for Codex
and other MCP clients. It supports local `stdio` and stateless Streamable HTTP.
The server is self-hosted: social-hub never receives your platform credentials.

```powershell
go build -o bin/social-hub-mcp.exe ./cmd/social-hub-mcp
$env:SOCIAL_HUB_CONFIG = "H:\path\to\social-hub.yaml"
bin\social-hub-mcp.exe --transport stdio
```

The default binary includes X, Facebook Page, Telegram, WeChat Official
Account, Weibo, and Douyin. Read tools are enabled by default. Publish,
reaction, comment, message, and delete tools appear only when the deployer
explicitly allowlists each operation.

See the [MCP self-hosting guide](docs/mcp.md), the
[`use-social-hub-mcp` Agent Skill](skills/use-social-hub-mcp/SKILL.md), and the
[MCP example configuration](examples/mcp/config.yaml).

## Adapter catalog

The catalog is generated from the adapter names registered in
`adapters/**/adapter.go`. It records the Go package, package README, and local
test-file presence without conflating those signals with live-account approval.

[Browse all 191 registered adapter products →](docs/adapter-catalog.md)

## Repository map

```text
adapters/            Opt-in platform adapter packages
assets/brand/        Project branding and logo assets
cmd/social-hub-mcp/  Self-hosted MCP server
config/              Configuration examples
docs/                Architecture, catalog, and deployment guides
examples/            Integration examples
extensions/          Typed platform-specific capabilities
internal/            Shared transport and implementation details
pkg/socialhub/       Public contracts and models
skills/              Agent operating guidance
```

## Documentation

- [Architecture and maintenance blueprint](docs/social-hub-blueprint.md)
- [Complete adapter catalog](docs/adapter-catalog.md)
- [MCP self-hosting and Codex configuration](docs/mcp.md)
- [Agent Skill for operating social-hub MCP](skills/use-social-hub-mcp/SKILL.md)
- Adapter-specific setup under `adapters/<name>/README.md`

## Development

```powershell
go build ./...
go test ./...
go test -race ./...
go vet ./...
```

Credentialed platform checks are intentionally separate from default CI. Use
approved sandbox or test accounts, opt in explicitly, and keep tokens, user
identifiers, and raw webhook data out of logs and artifacts.
