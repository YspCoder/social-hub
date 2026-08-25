# social-hub MCP 自部署指南

> 基线日期：2026-08-25（Asia/Shanghai）

`social-hub-mcp` 将 SDK 公共能力暴露为类型化 MCP tools，供 Codex 等 Agent 使用。项目不提供托管实例；配置、凭据、网络边界、日志和平台商业授权均由部署者负责。

## 1. 支持边界

| 能力 | 默认状态 | 说明 |
|---|:---:|---|
| Target 与 capability discovery | ✅ | 每次调用显式传入 adapter 和 account ID |
| 读取工具 | ✅ | 仍受适配器、账号、scope 和平台审批约束 |
| 新增型写操作 | ❌ | 必须按 operation 加入 `--allow-write` |
| 删除型写操作 | ❌ | 必须按 operation 加入 `--allow-destructive` |
| 通过 tool 参数提交凭据 | ❌ | 凭据只从服务端配置的 Secret 引用解析 |
| 媒体文件上传 | ❌ | 避免任意文件读取、URL SSRF 和大体积 base64 请求 |
| Webhook tool | ❌ | 验签需要真实 headers 与 raw body，应由独立 ingress 处理 |
| 平台 typed extension | ❌ | 广告、素材和视频工作流仍通过 Go SDK 调用 |

能力组存在不代表组内每个方法都可用。例如某些消息 adapter 只支持发送，不支持按 ID 获取；调用仍可能返回 `unsupported`。

## 2. 构建

项目要求 Go `1.26`：

```powershell
go build -o bin/social-hub-mcp.exe ./cmd/social-hub-mcp
```

默认二进制包含六个 adapter：

| 区域 | Adapter |
|---|---|
| 全球 | `x/v2`、`facebook/page`、`telegram/bot-api` |
| 中国 | `wechat/official-account`、`weibo/v2`、`douyin/openapi` |

social-hub 使用与 `database/sql` 类似的 `init()` 注册模式。运行时配置不能加载未编译的 Go package。部署其他平台时，在 `cmd/social-hub-mcp/builtin_adapters.go` 增加 blank import 后重新构建；也可以用 `pkg/mcpserver` 创建自定义入口。

## 3. 服务端配置

MCP 服务复用 SDK 的严格 YAML/JSON 配置：

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

示例文件位于 `examples/mcp/config.yaml`。同一个 adapter 只能出现一个配置块，一个配置块可以包含多个账号。MCP target 的唯一键是 `{adapter, account_id}`。

不要把 Token 写入 YAML。运行前由部署环境提供配置路径和被引用的 Secret：

```powershell
$env:SOCIAL_HUB_CONFIG = "H:\pro\social-hub\examples\mcp\config.yaml"
$env:SOCIAL_HUB_X_ACCESS_TOKEN = "..."
```

## 4. 本地 stdio

`stdio` 是默认 transport。stdout 只用于 MCP 协议，结构化日志写入 stderr：

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

平台 Token 不建议通过 `codex mcp add --env` 写入配置。优先使用 `env_vars` 转发现有环境变量，或由进程管理器注入。

## 5. Streamable HTTP

HTTP 模式使用无状态 MCP `2026-07-28` transport，默认只监听 loopback：

```powershell
bin\social-hub-mcp.exe --transport http --listen 127.0.0.1:8080
```

MCP endpoint 为 `http://127.0.0.1:8080/mcp`，健康检查为 `/healthz`。非 loopback 监听必须配置 Bearer Token：

```powershell
$env:SOCIAL_HUB_MCP_TOKEN = "a-long-random-value"
bin\social-hub-mcp.exe `
  --transport http `
  --listen 0.0.0.0:8080 `
  --bearer-token-ref env://SOCIAL_HUB_MCP_TOKEN
```

公网部署必须置于 TLS reverse proxy 后，并保持每请求 Bearer authentication。服务端同时启用 1 MiB request-body 上限、MCP localhost protection、Go Cross-Origin Protection、5 秒 header timeout 和 1 MiB header 上限。

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

## 6. Mutation policy

服务默认为只读。所有 mutation tool 都必须由进程级 allowlist 显式注册：

| 参数 / 环境变量 | Operation | Tool | 默认启用 |
|---|---|---|:---:|
| `--allow-write` / `SOCIAL_HUB_MCP_ALLOW_WRITE` | `publish_post` | `socialhub_publish_post` | ❌ |
| 同上 | `add_reaction` | `socialhub_add_reaction` | ❌ |
| 同上 | `create_comment` | `socialhub_create_comment` | ❌ |
| 同上 | `send_message` | `socialhub_send_message` | ❌ |
| `--allow-destructive` / `SOCIAL_HUB_MCP_ALLOW_DESTRUCTIVE` | `delete_post` | `socialhub_delete_post` | ❌ |
| 同上 | `remove_reaction` | `socialhub_remove_reaction` | ❌ |
| 同上 | `delete_comment` | `socialhub_delete_comment` | ❌ |

例如只允许发布和发送消息：

```powershell
$env:SOCIAL_HUB_MCP_ALLOW_WRITE = "publish_post,send_message"
bin\social-hub-mcp.exe --transport stdio
```

allowlist 是整个进程的权限上限，适用于该进程配置中的全部 target。不同账号需要不同权限时，应拆分配置和进程，并注册为不同 MCP server。Tool annotation 和 Agent 确认流程只是补充防线，不能替代服务端 allowlist。

## 7. Tool 清单

| 类型 | 默认注册 | Tool |
|---|:---:|---|
| Discovery | ✅ | `socialhub_list_targets`、`socialhub_get_capabilities` |
| Read | ✅ | `socialhub_get_user`、`socialhub_get_post`、`socialhub_list_posts`、`socialhub_list_comments`、`socialhub_get_message`、`socialhub_get_publish_status` |
| Additive mutation | ❌ | `socialhub_publish_post`、`socialhub_add_reaction`、`socialhub_create_comment`、`socialhub_send_message` |
| Destructive mutation | ❌ | `socialhub_delete_post`、`socialhub_remove_reaction`、`socialhub_delete_comment` |

所有输出使用 `{data, error}` envelope。错误只包含统一 code/class、HTTP status、request ID、retry-after、所需 scopes 和 approval URL；不返回 provider raw message、response body、account hash、Secret 引用或 Token。

## 8. 推荐调用顺序

```text
socialhub_list_targets
  -> socialhub_get_capabilities
  -> 读取当前资源
  -> 向用户确认 mutation 的 target 与内容
  -> 调用已由服务端 allowlist 的 mutation tool
  -> 依据 data/error envelope 核对结果
```

遇到 `rate_limited` 时遵守 `retry_after_ms`。遇到 `approval_required` 时停止重试并返回所需 scope 或审批入口。不要因为 capability group 存在就循环尝试不支持的方法。

## 9. 运行参数

| 参数 | 环境变量 | 默认值 |
|---|---|---|
| `--config` | `SOCIAL_HUB_CONFIG` | 必填 |
| `--transport` | `SOCIAL_HUB_MCP_TRANSPORT` | `stdio` |
| `--listen` | `SOCIAL_HUB_MCP_LISTEN` | `127.0.0.1:8080` |
| `--bearer-token-ref` | `SOCIAL_HUB_MCP_BEARER_TOKEN_REF` | 空 |
| `--allow-write` | `SOCIAL_HUB_MCP_ALLOW_WRITE` | 空，只读 |
| `--allow-destructive` | `SOCIAL_HUB_MCP_ALLOW_DESTRUCTIVE` | 空，只读 |

`defaults.timeout` 同时作为平台 HTTP client timeout，必须大于 0 且不超过 5 分钟。每个 tool 可通过 `call.timeout_ms` 指定不超过 120 秒的更短上下文。

## 10. 部署检查

| 检查项 | 要求 |
|---|:---:|
| 配置只保存 Secret 引用 | ✅ |
| stdout 不输出业务日志 | ✅ |
| 默认 mutation allowlist 为空 | ✅ |
| 非 loopback HTTP 无 Bearer Token | ❌ |
| 公网直接暴露明文 HTTP | ❌ |
| 多权限账号共用同一高权限进程 | ❌ |
| Agent 参数包含平台 Token | ❌ |

部署完成后，再根据实际 adapter README 核对账号类型、scope、配额和平台审批。MCP 层不会扩大底层平台账号的能力。
