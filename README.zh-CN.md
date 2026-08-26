<div align="center">

<img src="assets/brand/social-hub-logo.png" width="360" alt="social-hub Logo">

<p><strong>用一套类型安全的接口，连接全球社交平台。</strong></p>
<p>面向全球及中国大陆社媒 API 的能力型 Go SDK 与自部署 MCP 桥接服务。</p>

<p>
  <img alt="Go 1.26" src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&amp;logoColor=white">
  <img alt="MCP 2026-07-28" src="https://img.shields.io/badge/MCP-2026--07--28-7C3AED">
  <img alt="169 个适配器产品" src="https://img.shields.io/badge/adapters-169-111827">
  <img alt="Alpha" src="https://img.shields.io/badge/status-alpha-F43F5E">
  <img alt="GitHub stars" src="https://img.shields.io/github/stars/YspCoder/social-hub?style=flat&amp;color=2563EB">
</p>

<p>
  <a href="README.md">English</a>
  ·
  <a href="README.zh-CN.md"><strong>简体中文</strong></a>
</p>

<p><sub>文档基线：2026-08-26（Asia/Shanghai）</sub></p>

</div>

---

`social-hub` 为发布、内容读取、媒体上传、互动、消息和 Webhook 提供统一契约；当最小公共能力会损失平台语义时，则通过 typed extension 保留平台专属能力。应用只编译实际使用的适配器，可以在同一配置中管理多个平台和账号，凭据始终通过运行时 Secret 引用解析。

> [!IMPORTANT]
> 项目仍处于活跃开发阶段。首个正式 alpha tag 发布前，公共 API 可能调整。仓库中存在某个适配器，不代表任意账号已经获得该平台的全部能力。

## 项目状态

| 项目 | 状态 | 当前基线 |
|---|:---:|---|
| 公共模型与能力接口 | ✅ | 位于 `pkg/socialhub` |
| 编译期适配器注册 | ✅ | 169 个已注册 API 产品 |
| YAML/JSON 严格配置 | ✅ | 配置版本 `1`，未知字段会报错 |
| 运行时 Secret 引用 | ✅ | 内置 `env://`，支持自定义 resolver |
| 每个适配器都有 README | ❌ | 168/169 个适配器包 |
| 每个适配器都有本地测试文件 | ❌ | 104/169 个适配器包 |
| 每个适配器都完成真实账号验证 | ❌ | 当前不作此承诺 |
| 稳定公开版本 | ❌ | Alpha 开发阶段 |

`✅` 表示该项已具备，`❌` 表示覆盖尚不完整。包级状态见[适配器目录](docs/adapter-catalog.md)。

## 为什么选择 social-hub

| 设计原则 | 落地方式 |
|---|---|
| 按能力拆分 | 业务只依赖 `Publisher`、`Fetcher`、`Reactor`、`Messenger` 等细粒度接口，不依赖臃肿的万能 Client。 |
| 海外与中国平台统一 | 全球平台和中国市场 API 使用一致的接入模型，但不假设它们的审核规则相同。 |
| 类型安全的数据模型 | 公共层统一 `User`、`Post`、`Media`、`Comment`、`Message`，平台差异保留在扩展中。 |
| 依赖按需引入 | 注册机制参考 `database/sql`；通过 blank import 只把需要的平台编译进最终二进制。 |
| 原生支持多账号 | 使用 `{adapter, account_id}` 精确选择账号，同一应用可管理多个品牌或平台应用。 |
| 凭据不落配置明文 | YAML/JSON 只保存 `env://` 等引用，可通过 `SecretResolver` 接入 Vault 或 KMS。 |
| Agent 可安全使用 | MCP 服务由用户自行部署，默认只读，所有写操作必须由部署者逐项 allowlist。 |

## 架构

```text
 业务应用 / Agent
         |
         +------------------------+
         |                        |
  pkg/socialhub 公共契约    cmd/social-hub-mcp
         |                    （可选、自部署）
         |
     适配器注册表
         |
  +------+------+------+-------------------+
  |             |             |             |
 X / Meta   Telegram        微信         自定义适配器
  |             |             |             |
  +-------------+-------------+-------------+
                  平台 API
```

公共 Client 暴露六类可选能力：

| Capability | 公共操作 |
|---|---|
| `Publisher` | 发布内容、查询异步发布状态、删除动态 |
| `Fetcher` | 获取用户和动态、分页读取动态和评论 |
| `MediaUploader` | 创建分片会话、上传分片、完成上传、查询媒体状态 |
| `Reactor` | 添加或移除互动、创建或删除评论 |
| `Messenger` | 发送和获取消息 |
| `WebhookHandler` | 验签并解码统一事件 |

平台特有流程继续保持类型安全。例如，微信临时/永久素材放在 `extensions/material`，短视频分片上传与发布工作流放在 `extensions/video`，不会退化成不受约束的 `map[string]any`。

## 快速开始

### 1. 按需引入适配器

适配器通过 `init()` 注册：

```go
import (
	_ "social-hub/adapters/wechat/officialaccount"
	_ "social-hub/adapters/x"
)
```

### 2. 使用 Secret 引用配置多平台账号

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

内置 resolver 支持 `env://NAME`。需要 Vault、KMS 或企业密钥系统时，通过 `socialhub.WithSecretResolver` 注入自定义实现。

### 3. 打开适配器和账号 Client

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

Capability 会结合账号类型、scope 和平台审批状态动态声明。即使某个 capability group 可用，组内个别方法仍可能返回 `socialhub.CodeUnsupported` 或 `socialhub.CodeApprovalRequired`。

## 中国平台适配策略

- 微信生态：区分公众号、小程序和微信小店；素材、草稿、客服消息和 `code2Session` 不合并成一个 OAuth Client。
- 微博：公共 `PostRelation` 保留 reply、quote、repost 关系，转发语义不会被压平成普通文本。
- 抖音、快手和哔哩哔哩：视频初始化、分片上传、封面和发布状态使用 typed workflow。
- 企业资质与商业授权：capability 保留 approval、scope、原因和官方文档地址。
- 国内数据边界：OpenID、UnionID、手机号和 session key 等敏感标识不进入日志或 MCP 工具参数。

## Agent 与 MCP

`cmd/social-hub-mcp` 将公共契约暴露为 typed MCP tools，可供 Codex 等 Agent 使用。服务支持本地 `stdio` 和无状态 Streamable HTTP，由用户自行部署，social-hub 不接收或托管平台凭据。

```powershell
go build -o bin/social-hub-mcp.exe ./cmd/social-hub-mcp
$env:SOCIAL_HUB_CONFIG = "H:\path\to\social-hub.yaml"
bin\social-hub-mcp.exe --transport stdio
```

默认 MCP 二进制包含 X、Facebook Page、Telegram、微信公众号、微博和抖音。读取工具默认开启；发布、互动、评论、消息和删除工具只有在部署者显式配置 allowlist 后才会注册。

完整配置见 [MCP 自部署指南](docs/mcp.md)、[`use-social-hub-mcp` Agent Skill](skills/use-social-hub-mcp/SKILL.md) 和 [MCP 配置示例](examples/mcp/config.yaml)。

## 适配器目录

适配器目录直接依据 `adapters/**/adapter.go` 中的注册名整理，分别记录 Go package、独立 README 和本地测试文件是否存在，不把这些信号等同于真实账号授权。

[查看全部 169 个已注册适配器产品 →](docs/adapter-catalog.md)

## 仓库结构

```text
adapters/            按需引入的平台适配器
assets/brand/        品牌与 Logo 资源
cmd/social-hub-mcp/  自部署 MCP Server
config/              配置示例
docs/                架构、目录和部署文档
examples/            接入示例
extensions/          平台特有的 typed capabilities
internal/            共享 transport 和内部实现
pkg/socialhub/       公共接口与数据模型
skills/              Agent 操作规范
```

## 文档

- [架构与维护蓝图](docs/social-hub-blueprint.md)
- [完整适配器目录](docs/adapter-catalog.md)
- [MCP 自部署与 Codex 配置](docs/mcp.md)
- [Agent 使用 social-hub MCP 的 Skill](skills/use-social-hub-mcp/SKILL.md)
- 各适配器目录下的 `README.md`

## 开发

```powershell
go build ./...
go test ./...
go test -race ./...
go vet ./...
```

真实平台验证不属于默认 CI。需要凭据的验证应显式启用，使用已获授权的沙箱或测试账号，并确保日志和产物不包含 Token、用户标识或原始 Webhook 数据。
