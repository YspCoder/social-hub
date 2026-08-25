# social-hub MCP 自部署指南

`social-hub-mcp` 将 SDK 的公共 `Client` capability 暴露为类型化 MCP tools，供 Codex 等 Agent 使用。项目不提供托管实例；配置、凭据、网络边界和平台商业授权均由部署者负责。

## 支持边界

- 默认只注册 discovery/read tools。
- mutation tools 必须通过启动策略逐项启用；未启用的工具不会出现在 MCP tool list 中。
- 每次调用必须显式传入 `target.adapter` 和 `target.account_id`，不使用隐式默认账号。
- 凭据只从 social-hub 配置中的 `secret_ref` / `access_token_ref` 解析，不能通过 MCP 参数提交。
- MCP 首版只覆盖公共 `Publisher`、`Fetcher`、`Reactor` 和 `Messenger` 接口。广告投放、微信素材、短视频分片上传等 typed extensions 仍由 Go SDK 直接调用。
- 不提供媒体文件上传 tool，避免 Agent 侧任意文件读取、URL SSRF 和大体积 base64 请求。
- 不提供 Webhook tool；Webhook 验签和解码必须接收真实 HTTP headers 与 raw body，应由独立 ingress 处理。

能力组表示该 adapter 暴露一组接口，不代表组内每个方法都可用。例如某些消息 adapter 只支持发送、不支持按 ID 获取；调用仍可能返回 `unsupported`。

## 构建

需要 Go 1.26：

```powershell
go build -o bin/social-hub-mcp.exe ./cmd/social-hub-mcp
```

默认二进制编译以下 adapter：

| 区域 | Adapter |
|---|---|
| 海外 | `x/v2`、`facebook/page`、`telegram/bot-api` |
| 中国 | `wechat/official-account`、`weibo/v2`、`douyin/openapi` |

social-hub 使用与 `database/sql` 类似的 `init()` 注册模式。运行时配置无法加载未编译的 Go package。部署其他平台时，在 `cmd/social-hub-mcp/builtin_adapters.go` 增加对应 blank import 后重新构建；`pkg/mcpserver` 本身不导入任何 adapter，因此也可用于自定义入口。

## social-hub 配置

配置格式与 SDK 完全一致。示例见 `examples/mcp/config.yaml`：

```yaml
version: 1
defaults:
  timeout: 15s
platforms:
  - adapter: x/v2
    accounts:
      - id: brand-x
        access_token_ref: env://SOCIAL_HUB_X_ACCESS_TOKEN
```

不要把 token 写入 YAML。运行前由部署环境设置引用的变量：

```powershell
$env:SOCIAL_HUB_CONFIG = "H:\pro\social-hub\examples\mcp\config.yaml"
$env:SOCIAL_HUB_X_ACCESS_TOKEN = "..."
```

同一个 adapter 只允许出现一个配置块；一个配置块可以包含多个账号。MCP target 的唯一键是 `{adapter, account_id}`。

## 本地 stdio

stdio 是默认 transport。stdout 仅用于 MCP 协议，结构化日志写入 stderr：

```powershell
bin\social-hub-mcp.exe --transport stdio
```

Codex 的 `~/.codex/config.toml` 示例：

```toml
[mcp_servers.social_hub]
command = "H:\\pro\\social-hub\\bin\\social-hub-mcp.exe"
args = ["--transport", "stdio"]
cwd = "H:\\pro\\social-hub"
env_vars = ["SOCIAL_HUB_CONFIG", "SOCIAL_HUB_X_ACCESS_TOKEN"]
startup_timeout_sec = 20
tool_timeout_sec = 120
enabled = true
required = false
default_tools_approval_mode = "writes"

[mcp_servers.social_hub.tools.socialhub_publish_post]
approval_mode = "prompt"
```

也可以通过 CLI 注册：

```powershell
codex mcp add social_hub --env SOCIAL_HUB_CONFIG=$env:SOCIAL_HUB_CONFIG -- H:\pro\social-hub\bin\social-hub-mcp.exe --transport stdio
codex mcp list
```

平台 token 不建议通过 `codex mcp add --env` 写入配置；优先让 `env_vars` 转发现有环境变量，或在进程管理器中注入。

## Streamable HTTP

HTTP 模式使用无状态 MCP `2026-07-28` transport，默认监听 loopback：

```powershell
bin\social-hub-mcp.exe --transport http --listen 127.0.0.1:8080
```

MCP endpoint 为 `http://127.0.0.1:8080/mcp`，健康检查为 `/healthz`。非 loopback 监听必须配置 Bearer token：

```powershell
$env:SOCIAL_HUB_MCP_TOKEN = "a-long-random-value"
bin\social-hub-mcp.exe `
  --transport http `
  --listen 0.0.0.0:8080 `
  --bearer-token-ref env://SOCIAL_HUB_MCP_TOKEN
```

公网部署必须在 TLS reverse proxy 后运行，并保持每请求 Bearer authentication。服务端同时启用 1 MiB request-body 上限、MCP localhost protection 和 Go Cross-Origin Protection。不要关闭这些边界。

远程 Codex 配置：

```toml
[mcp_servers.social_hub]
url = "https://social-hub.example.com/mcp"
bearer_token_env_var = "SOCIAL_HUB_MCP_TOKEN"
startup_timeout_sec = 20
tool_timeout_sec = 120
enabled = true
required = false
default_tools_approval_mode = "writes"
```

## Mutation policy

服务默认为只读。以下 operation 可由逗号分隔的 allowlist 启用：

| 参数 / 环境变量 | Operation | 注册的 tool |
|---|---|---|
| `--allow-write` / `SOCIAL_HUB_MCP_ALLOW_WRITE` | `publish_post` | `socialhub_publish_post` |
| 同上 | `add_reaction` | `socialhub_add_reaction` |
| 同上 | `create_comment` | `socialhub_create_comment` |
| 同上 | `send_message` | `socialhub_send_message` |
| `--allow-destructive` / `SOCIAL_HUB_MCP_ALLOW_DESTRUCTIVE` | `delete_post` | `socialhub_delete_post` |
| 同上 | `remove_reaction` | `socialhub_remove_reaction` |
| 同上 | `delete_comment` | `socialhub_delete_comment` |

例如只允许发布和发送消息：

```powershell
$env:SOCIAL_HUB_MCP_ALLOW_WRITE = "publish_post,send_message"
bin\social-hub-mcp.exe --transport stdio
```

allowlist 是整个进程的上限，适用于该进程配置中的全部 target。需要不同账号权限时，应拆分配置和进程，并给 Agent 注册不同 MCP server 名称。MCP tool annotations 和 Agent 确认流程只是补充防线，不能替代服务端 allowlist。

## Tool 清单

| 类型 | Tool |
|---|---|
| Discovery | `socialhub_list_targets`、`socialhub_get_capabilities` |
| Read | `socialhub_get_user`、`socialhub_get_post`、`socialhub_list_posts`、`socialhub_list_comments`、`socialhub_get_message`、`socialhub_get_publish_status` |
| Additive mutation | `socialhub_publish_post`、`socialhub_add_reaction`、`socialhub_create_comment`、`socialhub_send_message` |
| Destructive mutation | `socialhub_delete_post`、`socialhub_remove_reaction`、`socialhub_delete_comment` |

所有输出均使用 `{data, error}` envelope。错误只包含统一 `code` / `class`、HTTP status、request ID、retry-after、所需 scopes 和 approval URL；不会返回 provider raw message、response body、account hash、secret reference 或 token。

## 运行参数

| 参数 | 环境变量 | 默认值 |
|---|---|---|
| `--config` | `SOCIAL_HUB_CONFIG` | 必填 |
| `--transport` | `SOCIAL_HUB_MCP_TRANSPORT` | `stdio` |
| `--listen` | `SOCIAL_HUB_MCP_LISTEN` | `127.0.0.1:8080` |
| `--bearer-token-ref` | `SOCIAL_HUB_MCP_BEARER_TOKEN_REF` | 空 |
| `--allow-write` | `SOCIAL_HUB_MCP_ALLOW_WRITE` | 空，只读 |
| `--allow-destructive` | `SOCIAL_HUB_MCP_ALLOW_DESTRUCTIVE` | 空，只读 |

`defaults.timeout` 同时作为平台 HTTP client timeout，必须大于 0 且不超过 5 分钟。每个 tool 还可在 `call.timeout_ms` 中指定不超过 120 秒的更短调用上下文。
