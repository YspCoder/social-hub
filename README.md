<div align="center">

<img src="assets/brand/social-hub-logo.png" width="360" alt="social-hub logo">

<p><strong>One typed interface for the world's social platforms.</strong></p>
<p>A capability-oriented Go SDK and self-hosted MCP bridge for global and China-market social APIs.</p>

<p>
  <img alt="Go 1.26" src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&amp;logoColor=white">
  <img alt="MCP 2026-07-28" src="https://img.shields.io/badge/MCP-2026--07--28-7C3AED">
  <img alt="168 adapter packages" src="https://img.shields.io/badge/adapters-168-111827">
  <img alt="Alpha" src="https://img.shields.io/badge/status-alpha-F43F5E">
  <img alt="GitHub stars" src="https://img.shields.io/github/stars/YspCoder/social-hub?style=flat&amp;color=2563EB">
</p>

<p>
  <a href="README.md"><strong>English</strong></a>
  ·
  <a href="README.zh-CN.md">简体中文</a>
</p>

</div>

---

`social-hub` gives Go applications one stable set of contracts for publishing,
fetching, media, reactions, messaging, and webhooks while preserving typed
platform extensions where a lowest-common-denominator API would lose meaning.
Applications compile only the adapters they use, configure multiple accounts,
and keep credentials behind runtime secret references.

> [!IMPORTANT]
> `social-hub` is under active development. The public API is not stable before
> the first tagged alpha release, and credentialed platform validation is still
> required before production use.

## Why social-hub

| Principle | What it means in practice |
|---|---|
| Capability-oriented | Depend on `Publisher`, `Fetcher`, `Reactor`, `Messenger`, or another narrow interface instead of a monolithic client. |
| Global + China coverage | Use the same model for X, Facebook, Telegram, WeChat, Weibo, Douyin, and a broad catalog of creator, messaging, ads, analytics, and commerce APIs. |
| Type-safe normalization | Work with common `User`, `Post`, `Media`, `Comment`, and `Message` models while retaining platform extensions. |
| Dependency control | Adapter registration follows the `database/sql` pattern; blank-import only what a binary needs. |
| Multi-account by design | Address every client by `{adapter, account_id}` and keep multiple apps or brands in one strict YAML/JSON configuration. |
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

The stable core is deliberately small:

| Capability | Common operations |
|---|---|
| `Publisher` | Publish content, inspect asynchronous status, delete posts |
| `Fetcher` | Get users/posts, list posts and comments |
| `MediaUploader` | Begin resumable uploads, upload parts, complete and inspect media |
| `Reactor` | Add/remove reactions, create/delete comments |
| `Messenger` | Send and retrieve messages |
| `WebhookHandler` | Verify signatures and decode normalized events |

Platform-specific workflows remain typed. For example, WeChat material
management lives in `extensions/material`, while short-video publication lives
in `extensions/video` instead of being forced through an unsafe generic map.

## Quickstart

### 1. Import the adapters you need

Adapter packages register themselves from `init()`. The following binary can
open X and WeChat Official Account configurations; no other adapter is linked:

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

Capability support is account- and approval-aware. A capability group may be
available while an individual method remains unsupported by a platform; always
handle `socialhub.CodeUnsupported` and `socialhub.CodeApprovalRequired`.

## Agent and MCP support

`cmd/social-hub-mcp` exposes the common contracts as typed MCP tools for Codex
and other MCP clients. It supports local `stdio` and stateless Streamable HTTP.
The server is self-hosted: social-hub never receives your platform credentials.

```powershell
go build -o bin/social-hub-mcp.exe ./cmd/social-hub-mcp
$env:SOCIAL_HUB_CONFIG = "H:\path\to\social-hub.yaml"
bin\social-hub-mcp.exe --transport stdio
```

The default MCP binary includes X, Facebook Page, Telegram, WeChat Official
Account, Weibo, and Douyin. Read tools are enabled by default; publish,
reaction, comment, message, and delete tools appear only when the deployer
explicitly allowlists each operation.

See the [MCP self-hosting guide](docs/mcp.md), the
[`use-social-hub-mcp` Agent Skill](skills/use-social-hub-mcp/SKILL.md), and the
[MCP example configuration](examples/mcp/config.yaml).

## Adapter catalog

The repository currently contains **168 registered adapter packages**. The
table below records implemented surface area, provider approval requirements,
and the latest contract-review status. Browse `adapters/` for package-specific
README files and typed workflows.

<details>
<summary><strong>View the complete adapter catalog</strong></summary>

<br>

| Adapter | Capabilities | Status |
|---|---|---|
| `x/v2` | OAuth2 PKCE, posts, users/timelines, replies, reactions, media upload | Local contract tests |
| `x/ads-api-v12` | Ads Account, paused Campaign/Line Item management, safe existing-Tweet association, synchronous Stats, OAuth1 | Local contract tests; Ads API Standard Access and account role required |
| `facebook/page` | OAuth2, Page posts/feed, comments/likes, photo upload, webhooks | Local contract tests |
| `facebook/marketing-api-v25` | Ad Accounts, paused Campaign/Ad Set/Creative/Ad creation and management, synchronous Insights, appsecret_proof | Local contract tests; App Review, ad-account role, and ads_management/ads_read required |
| `facebook/conversions-api-v26` | Typed Web/App/offline/business-messaging conversion events, local PII normalization/SHA-256, exact decimals, test events, appsecret_proof | Local contract tests; official Meta Node Business SDK v26.0.0 and Graph API v26 contract, ads_management and Pixel/dataset authorization required |
| `oceanengine/marketing-api-v3` | Advertiser-scoped paused Project/Promotion creation and management, synchronous custom reports, OAuth token lifecycle | Local contract tests; Ocean Engine app capability groups and advertiser authorization required |
| `tencentads/marketing-api-v1.3` | Advertiser, paused Campaign/AdGroup creation and management, AdCreative templates, daily/hourly reports, OAuth token lifecycle | Local contract tests; Tencent Ads scopes and advertiser authorization required |
| `kuaishou/magnetic-engine-marketing-api` | Advertiser, paused Campaign/Unit/Creative creation and management, real-time reports, OAuth token lifecycle | Local contract tests; continuous per-endpoint revisions, corporate MAPI registration and advertiser authorization required |
| `tiktok/business-marketing-api-v1.3` | Advertiser, paused Campaign/Ad Group/batch Ad creation and management, synchronous integrated reports, long-term Marketing token exchange | Local contract tests; TikTok for Business app approval, scopes, and advertiser authorization required |
| `tiktok/events-api-v2` | Web/App/Offline/CRM conversion batches, local PII normalization/SHA-256, exact JSON numbers, test events, LDU, typed first-party attribution | Local contract tests; Business API v1.3 route, Events Manager token or Developer App permission required; App Events are allowlist-only |
| `googleads/api-v25` | Customer, Campaign Budget, paused Search Campaign/Ad Group/RSA management, bounded paginated GAQL Search, OAuth2 refresh | Local contract tests; approved developer token, adwords scope, and Customer authorization required |
| `google/search-ads-360-reporting-api-v0` | Directly accessible Customers, exact-value paginated Search Ads 360 Query Language reports, Custom Columns, and field compatibility metadata with manager-account context and OAuth2 refresh | Current complete REST v0 Discovery revision 20260820 reviewed 2026-08-24; Google OAuth verification, enabled Cloud API, doubleclicksearch scope, and Search Ads 360 account access required; gRPC-only SearchStream and mutations intentionally excluded |
| `google/merchant-api-v1` | Merchant Account/issues, Data Sources, Product/ProductInput lifecycle, exact-value Merchant Query Language reports, dynamic quota groups, and Shopping product limits | Stable Accounts/Products/Data Sources/Reports/Quota v1 Discovery revision 20260819 reviewed 2026-08-24; content scope, enabled Cloud API, and Merchant Center account role required; catalog changes affect Shopping Ads eligibility but do not manage campaign spend |
| `alimama/taobao-union-api-v2` | Publisher material discovery, item details, typed item/material affiliate-link conversion, Tao Password creation, and exact-value order attribution | Current TOP v2 official metadata and `bububa/opentaobao` v1.4.6 schemas reviewed 2026-08-24; TOP app/API package, Taobao Union publisher/media registration, promotion position, and method-specific commercial eligibility required; quotas are dynamic and order windows can tighten from 3 hours to about 20 minutes during major promotions |
| `jd/union-open-api-v1.0` | Jingfen goods discovery, website/app affiliate-link conversion, optional JD command generation, and exact-value order-row attribution | Current official v1.0 contracts reviewed 2026-08-24; JD Open Platform app, Union publisher, registered website/app/traffic media, and method-specific commercial eligibility required; quotas are dynamic, order windows are capped at one hour, and windows exceeding 10,000 rows must be subdivided |
| `pinduoduo/duoduo-jinbao-api-v1` | Channel goods recommendations, signed-goods detail, web/mobile/mini-program/social affiliate-link generation, and exact-value incremental order attribution | Current official Duoduo Jinbao directory and `mimicode/tksdk` commit `3a31074c` schemas reviewed 2026-08-24; Pinduoduo app, publisher qualification, approved media/PID and method grants required; quotas are dynamic, optional social surfaces and pull-new are filing/allowlist gated, and order sync is limited to the provider's recent-order boundary |
| `vipshop/union-open-api-v2` | Keyword goods discovery, batch and channel-aware marketing detail, web/deep-link/mini-program/quick-app affiliate-link generation, and attributed order reconciliation | Current official V2 method/struct metadata and `mimicode/tksdk` commit `3a31074c` schemas reviewed 2026-08-24; Vipshop app, bound Union publisher, OAuth authorization, method grants, and source-IP allowlisting required; limits are dynamic per grant/method and error `1008` is retryable |
| `aliexpress/affiliate-api-v2` | Commissionable product search/detail, tracked-link generation, and exact-value attributed order reconciliation | Current TOP v2 official method metadata and `com.global.iop:iop-api-sdk:1.3.5-ae` schemas reviewed 2026-08-24; Open Platform app, AE-Affiliate API grant, approved Affiliate Portals publisher, and tracking ID required; public quotas are dynamic and fixed order-window or batch-ID limits are not hard-coded |
| `amazon/creators-api-catalog-v1` | Associates catalog search, batch item detail, variations, Browse Nodes, OffersV2, exact-value commerce data, and Partner Tag-attributed URLs | Current Catalog v1 and official SDK 1.2.0 contracts reviewed 2026-08-24; PA-API 5 is retired, final Associates acceptance plus 10 qualifying sales/30 days and marketplace Partner Tag required; initial quota is at most 1 TPS/8,640 TPD and Product Advertising Content normally must refresh within 24 hours |
| `google/data-manager-api-v1` | Atomic event ingestion for Google Ads/Analytics/Floodlight destinations, local PII normalization/SHA-256, exact JSON decimals, consent, cart data, warnings, and GCP/AWS wrapped-key metadata | Local contract tests; stable v1 Discovery revision 20260729, datamanager scope and destination write access required; request acceptance is not downstream conversion completion |
| `google/display-video-360-api-v4` | Advertiser, paused Campaign, draft Insertion Order, and draft standard RTB Line Item management/duplication, OAuth2 refresh | Local contract tests; DV360 Partner/Advertiser access and display-video scope required; Bid Manager reporting is separate |
| `google/campaign-manager-360-api-v5` | Profile/Advertiser, archived-first Campaign management, advertiser-bound Placement/Ad reads, direct ReportData queries, existing Report execution, bounded file downloads, OAuth2 refresh | Local contract tests; CM360 account/API access, user-profile permissions, and dfatrafficking/dfareporting consent required |
| `google/ad-manager-api-v1` | Network-bound Company/AdUnit/Order/LineItem reads, hidden Interactive Report creation, asynchronous execution and paginated rows, OAuth2 refresh | Local contract tests; v1 Beta, Ad Manager network/API access, user role/team permissions, and admanager scope required for report creation |
| `google/adsense-management-api-v2` | Publisher-bound Account/AdClient/AdUnit/Channel/Site reads, alerts/payments/policy issues, bounded JSON and saved reports, OAuth2 refresh | Local contract tests; AdSense account and user OAuth required; service accounts unsupported and restricted AdSense for Platforms mutations intentionally omitted |
| `google/admob-api-v1` | Publisher-bound Account/App/Ad Unit reads, bounded AdMob Network and third-party Mediation reports, OAuth2 refresh | Local contract tests; complete stable v1 REST surface, user OAuth required, 100,000-row streaming report bound; v1beta mutations intentionally omitted |
| `unity/ads-publisher-manage-api-v2` | Organization-bound Application, Placement, test-mode, and Test Device management, Basic/long-lived Bearer service-account auth | Local contract tests; complete current 18-operation Publisher API surface, required Monetize roles, no Ad Unit or eCPM Target resources |
| `unity/advertising-management-api-v1` | Organization-bound App, Campaign, Bid, Targeting, Budget, Creative, and Creative Pack management, Basic/long-lived Bearer service-account auth | Local contract tests; all 42 current non-deprecated operations, separately granted Advertising Management API access and Growth roles required; Advertiser v2 migration pending |
| `unity/advertising-statistics-api-v2` | Organization-bound Acquisition and SKAN reports, 114/7 typed metrics, streaming CSV/JSON with gzip/deflate and CSV EOF verification | Local contract tests; complete current 2-operation Statistics v2 surface, Advertise Stats API Viewer or MMP Viewer role required |
| `ironsource/advertiser-reporting-api-v4` | MMP-attributed advertiser reports, billable Cost reports, SKAN delivery and conversion-value reports with exact JSON values and bounded CSV streaming | Current v4 contract reviewed 2026-08-17; advertiser Bearer access required, 100 requests/minute per endpoint, billing data intentionally separated from MMP installs |
| `huawei/petal-ads-marketing-api-v1` | Overseas advertiser-account and Campaign reads, five synchronous v2 report levels with exact dynamic values, Huawei OAuth2 exchange/refresh | Current official contract reviewed 2026-08-17; Petal Ads app capabilities and advertiser authorization required, 600 requests/minute and 360,000/day/account; mainland China contract intentionally separate |
| `huawei/jinghong-marketing-api-v1` | Mainland advertiser-account and Campaign reads, four synchronous v2 report levels with exact dynamic values, Huawei OAuth2 exchange/refresh | Current official contract reviewed 2026-08-17; enterprise verification, Marketing API review, and advertiser authorization required, 600 requests/minute and 360,000/day/authorized account; overseas Petal Ads contract intentionally separate |
| `pangle/publisher-reporting-api-v2` | Non-mainland publisher income, app/placement delivery, bidding, and exact revenue reports with signed single-day queries | Current official contract reviewed 2026-08-17; Pangle Data API role and Security Key required, 2 QPS and 100,000-row truncation boundary; domestic 穿山甲 contract intentionally separate |
| `pangle/app-placement-management-api-v1.1.13` | Non-mainland publisher App and typed Native/Banner/App Open/Rewarded/Interstitial placement management, blocking rules, CPM, and expected-CPM updates | Current complete 7-endpoint contract reviewed 2026-08-17; permission-scoped role and SHA-1 Security Key required, sandbox isolated, `50007` app verification is asynchronous; domestic 穿山甲 contract intentionally separate |
| `xiaohongshu/spotlight-reporting-api` | Advertiser balance plus account/campaign/unit/creative/keyword/note/SPU/series/search-word offline and account-through-target realtime reports with exact dynamic JSON values | Current unversioned contract reviewed 2026-08-17; approved Xiaohongshu app, Spotlight advertiser authorization, and account/report scopes required; reporting-only and token refresh intentionally external |
| `xiaohongshu/spotlight-marketing-api` | Advertiser-bound Campaign list/current cascade editing, Unit list, Creative search, and explicit resume/pause/delete actions with mutation reconciliation | Current unversioned contract reviewed 2026-08-17; approved Xiaohongshu app, Spotlight authorization, and ad_query/ad_manage scopes required; no paused-first create contract, OAuth, asset upload, or webhooks |
| `applovin/max-reporting-apis` | Aggregated MAX revenue, user-level revenue, and revenue/impression/session cohort reports with bounded JSON/CSV downloads | Local contract tests; complete current 5-endpoint MAX Reporting surface, Report Key required, rolling 45-day window; campaign and ad-unit management intentionally excluded |
| `applovin/growth-campaign-management-api-v1` | APP/WEB Campaign, Creative Set, Asset upload/library, and cross-Campaign association management with raw-header API-key auth | Local contract tests; complete current 18-endpoint management surface, whitelist-only, 1,000 requests/60 seconds and 100-error/5-minute lockout policy |
| `applovin/growth-reporting-apis` | APP/WEB advertiser, asset, and APP-only publisher/playable reports with exact JSON values and validated bounded CSV streaming | Local contract tests; complete current 5-endpoint Growth Reporting surface, query Report Key auth, APP 45-day and WEB 90-day Campaign windows, account-type verification |
| `applovin/growth-conversion-api-v1` | Standard, lead-gen, and restricted lead-gen server conversion batches with typed event data, exact JSON decimals, and raw-header auth | Local contract tests; complete current 14-event surface, atomic 1-100 event validation, deny-by-default restricted PII policy, HTTP 400 whole-batch drop semantics |
| `adjust/s2s-api` | Typed Event, Session, and publisher Ad Revenue measurement with exact decimals, flat custom parameters, device/consent metadata, and S2S Security Bearer auth | Local contract tests; unversioned contract verified 2026-08-10, Session measurement requires Adjust enablement and Ad Revenue requires the package |
| `appsflyer/mobile-s2s-events-api3` | Single-event mobile S2S measurement with stringified event values, typed nested custom data, device identifiers, hashed user data, sharing filters, DMA consent, and raw-header S2S auth | Local contract tests; current API v3 contract, strict 1 KB JSON body, iOS app-ID/OS safeguards; HTTP 200 is minimum acceptance and callers own deduplication |
| `branch/events-api-v2` | Real-time Standard and Custom mobile attribution events with exact decimals, content items, device identifiers, DMA consent, and SKAN response fields | Local contract tests; current 24-event v2 surface, body Branch Key auth, allowlisted public-IPv4 override, HTTP 200-only success; callers own deduplication |
| `airbridge/s2s-events-api-v2` | Mobile-app and Web attribution events with UUIDv4 deduplication, exact decimals, typed semantic/Product data, browser tracking, DMA consent, and Bearer auth | Local contract tests; current v2 two-endpoint surface, 25 Standard Event constants, 24-hour event window, 1,000 requests/minute per endpoint, HTTP 200-only success |
| `kochava/s2s-measurement` | Paid-account install notifications, post-install events, iOS/tvOS IDFA updates, ATT/DMA consent, Apple Search Ads attribution, and optional Strict Authentication | Current unversioned S2S contract reviewed 2026-08-17; single-event requests under 2 MiB, App GUID binding and paid provisioning required; callers own deduplication |
| `google/analytics-data-api-v1beta` | GA4 Property metadata, compatibility checks, aggregate Core/Realtime/Pivot reports and batches, user OAuth or Service Account JWT | Local contract tests; REST v1beta, GA4 Property access and analytics.readonly scope required; user-level Audience Exports intentionally omitted |
| `google/youtube-analytics-api-v2` | Channel/content-owner targeted reports, revenue scope gating, private Analytics groups and group items, OAuth2 refresh | Local contract tests; user OAuth only, Channel and CMS Content Owner bindings are isolated from YouTube Data API v3; bulk Reporting API remains separate |
| `google/youtube-reporting-api-v1` | Channel/content-owner asynchronous bulk-report jobs, system-managed reports, metadata filters, bounded CSV/gzip downloads, user OAuth or Content Owner Service Account JWT | Local contract tests; full v1 REST surface, account-bound `onBehalfOfContentOwner`, secure same-origin downloads; first report can take up to 48 hours |
| `naver/search-ad-api-v2` | Customer-bound paused Campaign/Ad Group/Keyword management, KST synchronous Stats, asynchronous Stat Reports, and bounded authenticated downloads with HMAC-SHA256 request signing | Current official v2 management/v1 report contracts reviewed 2026-08-17; API license, secret key, and advertiser customer authorization required; fixed global QPS is not publicly specified |
| `line/ads-api-v3` | Authorized Ad Account and Campaign discovery, asynchronous report metadata, and exact-value synchronous Campaign/Ad Group/Ad online reports with JWS HS256 signing | Current restricted v3.12.3 partner contracts reviewed 2026-08-24; existing API-enabled LINE Ads Group, LY invitation, Access/Secret keys, and Data Provider, Ad Tech, or Reporting entitlement required; 2 requests/second/API user and at most 30 concurrent Online Reports; new LINE Ads account applications stopped on 2026-06-30 during LINE Yahoo Ads integration |
| `line-yahoo/search-ads-api-v20` | Advertiser-bound paused-first STANDARD CPC Campaign/Ad Group/Keyword management, per-item mutation outcomes, asynchronous Reports, bounded CSV/TSV/XML downloads, and OAuth2 refresh | Current official v20 OpenAPI and Java samples reviewed 2026-08-24; `yahooads` authorization and advertiser/MCC rights required; `accountId` remains separate from `x-z-base-account-id`; regular services are typically 5 QPS and error `0003` requires at least 30 seconds before retry; Shopping Search Ads, Ads, Creatives, and Assets are intentionally excluded |
| `line-yahoo/display-ads-api-v20` | Advertiser-bound paused-first Auction CPC Campaign/Ad Group and image Banner Ad management, per-item mutation outcomes, asynchronous standard AD Reports, bounded CSV/TSV/XML downloads, and OAuth2 refresh | Current official v20 OpenAPI and Java samples reviewed 2026-08-24; `yahooads` authorization and advertiser/MCC rights required; existing image Media IDs only; `accountId` remains separate from `x-z-base-account-id`; Display Ads service groups are 5 QPS and error `0003` requires at least 30 seconds before retry; Guaranteed workflows, uploads, advanced creatives, targeting, and conversions are intentionally excluded |
| `kakao/moment-open-api-v4` | Ad Account, Campaign ON-first/create-then-OFF management, Ad Group budget/bid/status, Creative status, guarded deletion, synchronous exact-value reports, and Business Authentication exchange | Current official v4 contract reviewed 2026-08-24; Kakao Business account, Biz app, Moment permission, Business consent, and Ad Account rights required; no refresh token or generic idempotency contract; complex targeting, asset/Creative creation, Message delivery, and multi-account aggregate reports intentionally excluded |
| `shopee/ads-api-v2` | Shop Ads balance/toggles, product and keyword recommendations, product Campaign settings, exact-value daily/hourly performance, HMAC-signed authorization-code exchange and rotating refresh tokens | Current official v2 Ads/Public contracts reviewed 2026-08-24; Open Platform app permission, seller authorization, linked Shop, Partner Key and declared source IPs required; partner/shop/endpoint plus daily quotas are dynamic; campaign mutations intentionally excluded |
| `mercadolibre/product-ads-api-v2` | Product Ads advertiser discovery, Campaign/item search and detail, exact-value aggregate/summary/daily metrics, OAuth Authorization Code with optional PKCE and rotating refresh tokens | Current official Product Ads v2/read and OAuth contracts reviewed 2026-08-24; seller Product Ads enablement and main-account authorization required, metrics are limited to a 90-day window, quotas are dynamic per Client ID/endpoint; writes, Display Ads, and Brand Ads intentionally excluded |
| `mercadolibre/display-ads-api-v1` | Display Ads advertiser discovery, Campaign/Line Item/Creative reads, exact-value daily and summary metrics at all three hierarchy levels, OAuth Authorization Code with optional PKCE and rotating refresh tokens | Current official Display Ads v1 and OAuth contracts reviewed 2026-08-24; Commercial Advisor enablement required, metrics start at 2022-09-01 and are limited to a 90-day window, quotas are dynamic per Client ID/endpoint; writes, Product Ads, and Brand Ads intentionally excluded |
| `mercadolibre/brand-ads-api` | Brand Ads advertiser discovery, Campaign/Item/Keyword reads, advertiser/Campaign/Keyword exact-value daily and summary metrics, OAuth Authorization Code with optional PKCE and rotating refresh tokens | Current unversioned Brand Ads and OAuth contracts reviewed 2026-08-24; Official Store/My Page, green reputation, three-listing minimum, product enablement, and advertiser authorization required; metrics start at 2023-02-09 and quotas are dynamic per Client ID/endpoint; writes, Product Ads, and Display Ads intentionally excluded |
| `thetradedesk/platform-api-v3` | Advertiser settings, bounded Campaign query/read, single-flight Campaign creation and partial updates, static long-lived or managed short-lived `TTD-Auth` tokens | Current official REST v3 OpenAPI and guides reviewed 2026-08-24; signed onboarding contracts, Platform API credentials, service permissions, and advertiser access required; endpoint limits are dynamic, REST Campaign metrics are not in the current public contract, and My Reports/REDS/GraphQL remain separate |
| `xandr/digital-platform-api` | Advertiser and advertiser-scoped Campaign reads with bounded paging, proprietary session authentication, exact budget values, and dynamic rate diagnostics | Current continuously deployed Microsoft Advertising/Xandr contract reviewed 2026-08-24; Digital Platform API user access and member permissions required, sessions are activity-bound with a 24-hour hard maximum, authentication is limited to 10 successful calls/5 minutes, and service/user quotas are dynamic |
| `yandex/direct-api-v5` | Advertiser/agency-client-bound classic Text Campaign, Ad Group, and Keyword management with weekly-budget draft/suspend safety gates, per-item mutation outcomes, point metadata, and bounded Reports | Current official API v5 contract reviewed 2026-08-17; production JSON services use v501 while Sandbox stays v5; approved Direct API application, `direct:api` OAuth authorization, API agreement, account/IP rights, five concurrent requests per advertiser, and dynamic point quota apply; UPC management intentionally excluded |
| `baidu/marketing-api-2026-08` | Search Account, paused Campaign/AdGroup CRUD, guarded basic Creative CRUD, synchronous/asynchronous reports, OAuth2 exchange/refresh | Local contract tests; unversioned contract verified 2026-08; Baidu developer/app review, API permissions, and advertiser authorization required |
| `microsoftads/api-v13` | Account validation, paused Search Campaign/Ad Group/RSA/Keyword management, asynchronous Campaign Performance reports with bounded download, Microsoft identity OAuth2 refresh | Local contract tests; developer token, msads.manage consent, and Customer/Account authorization required |
| `microsoftads/conversions-api-v1` | Atomic Page Load/Custom conversion batches, Microsoft-specific email normalization/SHA-256, exact JSON decimals, ecommerce/hotel context, consent and attribution IDs | Local contract tests; complete current ingestion operation, UET tag and tag-specific CAPI token required; credentials are separate from Advertising API v13 |
| `taboola/backstage-api-v1` | Advertiser Account, paused-first Campaign/Campaign Item management, Campaign Summary and Realtime Campaign reports, OAuth2 Client Credentials | Local contract tests; Backstage API credentials and advertiser-account authorization require Taboola approval |
| `outbrain/amplify-api-v0.1` | Marketer, Budget, disabled-first Campaign/PromotedLink management, Campaign/Promoted Content and periodic reports, Basic login token lifecycle | Local contract tests; Amplify API access and Marketer authorization require Outbrain approval |
| `criteo/marketing-solutions-api-2026-01` | Advertiser portfolio, Campaign and off-first Ad Set management, advertiser-scoped JSON Statistics reports, OAuth2 Client Credentials | Local contract tests; Criteo API credentials, Campaign/Analytics scopes, and advertiser authorization required |
| `appleads/campaign-management-api-v5` | Organization ACL, paused Campaign/Ad Group/Keyword/Ad management, product-page Creatives, synchronous reports, ES256 OAuth2 Client Credentials | Local contract tests; Apple Ads Advanced account, API user, organization role, and searchadsorg authorization required |
| `amazonads/sponsored-products-v3` | Profiles, paused Sponsored Products Campaign/Ad Group/Product Ad/Keyword management, Reporting v3 create/status, Login with Amazon OAuth2 refresh | Local contract tests; Amazon Ads onboarding and profile authorization required; Unified API/reporting migration applies |
| `pinterest/ads-v5.28` | Ad Accounts, paused Campaign/Ad Group/Ad management, synchronous account Analytics, OAuth2 refresh/client credentials | Local contract tests; approved app, Business Access, advertiser role, billing, and ads:read/ads:write authorization required |
| `pinterest/conversions-api-v5.28` | Web/App/Offline conversion batches, local match-key normalization/SHA-256, exact decimals, test mode, typed v5.28 App/Device context | Local contract tests; complete current ingestion operation, Pinterest Business Ad Account and Conversion Token or ads:write OAuth token required |
| `linkedin/marketing-202607` | Ad Account, draft-first Campaign Group/Campaign/Creative management, synchronous Ad Analytics, OAuth2 partner refresh | Local contract tests; Marketing API tier, Ad Account role, and r_ads/rw_ads/r_ads_reporting authorization required |
| `linkedin/conversions-api-202608` | Single/atomic batch conversion events, local email normalization/SHA-256, all current match identifiers, exact decimal-string values, deduplication IDs | Local contract tests; complete 202608 ingestion operation, enabled Conversion Rule and Direct API token or rw_conversions/r_ads partner authorization required |
| `snapchat/marketing-api-v1` | Ad Account, paused Campaign/Ad Squad/Ad management, synchronous account Stats, OAuth2 refresh | Local contract tests; Business Manager OAuth app, Organization/Ad Account roles, and snapchat-marketing-api authorization required |
| `snapchat/conversions-api-v3` | Pixel Web/Offline and Snap App conversion batches, local PII normalization/SHA-256, exact decimals, test-event validation/logs/stats | Local contract tests; current v3 REST contract, organization-owned asset and static long-lived CAPI or OAuth token required |
| `facebook/messenger-platform` | Page-scoped text/media messages, PSID profiles, signed messaging webhooks | Local contract tests |
| `instagram/login-v26` | OAuth2, professional profiles/media/comments, container publishing, scoped messaging/profiles, webhooks | Local contract tests |
| `linkedin/rest-202607` | OAuth2/OIDC, versioned posts, comments/reactions, image upload | Local contract tests |
| `tiktok/v2` | Login Kit OAuth2, Display API, typed Direct Post workflow, webhooks | Local contract tests |
| `tiktok/research-api-v2` | Approved-research Video queries, public User metadata, and anonymized Comment/reply reads with typed field masks, query DSL, and opaque search continuation | Current official Research API v2 contracts reviewed 2026-08-25, including Query Videos updated 2026-08-19; `research.data.basic` Client Access Token and project-specific researcher approval required, commercial users are ineligible, general quota is 1,000 requests/100,000 records per UTC day, and archival freshness plus approved-purpose data governance apply |
| `youtube/data-v3` | Google OAuth2, channels/videos/comments/ratings, typed video upload | Local contract tests |
| `google-business-profile/v4` | Google OAuth2 refresh, business Locations, Local Posts, typed EVENT/OFFER publishing, Reviews and owner replies | Local contract tests; API access approval required |
| `vimeo/api-v3.4` | OAuth2, users/videos/feeds/comments/likes, typed TUS video upload | Local contract tests |
| `dailymotion/api-v2` | OAuth2 Client Credentials, managed profiles/videos/playlists, typed multipart video upload | Local contract tests |
| `flickr/services-api` | OAuth1 HMAC-SHA1, profiles/photos/comments/favorites, photosets, typed streaming upload | Local contract tests |
| `giphy/v1` | API-key GIF/Sticker discovery, Random ID, analytics pingbacks, typed streaming upload | Local contract tests |
| `openverse/api-v1` | Read-only image/audio search and detail metadata with anonymous or externally managed OAuth access | Current API v1 contract reviewed 2026-08-25; license and attribution metadata is preserved, media bytes are never fetched, and anonymous/authenticated quota tiers remain distinct |
| `pixabay/api` | Read-only image/video search metadata with typed filters, pagination, and quota diagnostics | Current unversioned API reviewed 2026-08-25; API key required, default quota is 100 requests/60 seconds, repeated searches require a 24-hour cache, and provider display/hotlinking terms apply |
| `google/books-api-v1` | Public Volume search/detail with typed metadata, API-key or OAuth2 Bearer authentication, and bounded offset paging | Current Books API v1 Discovery reviewed 2026-08-25; My Library, purchases, private shelves, download tokens, media downloads, and writes are intentionally excluded |
| `apple/itunes-search-api` | Credential-free Store search and lookup for music, podcasts, movies, audiobooks, software, and ebooks | Current public unversioned contract reviewed 2026-08-25; no writes, purchases, media downloads, Apple Music library access, JSONP, or affiliate-link construction |
| `imgur/v3` | Client-ID/OAuth2, profiles/images/comments/Gallery votes, albums, typed streaming upload | Local contract tests |
| `soundcloud/public-api-v1` | OAuth2.1 PKCE, users/tracks/comments/likes/reposts, typed activity feed and streaming track upload | Local contract tests |
| `strava/api-v3` | OAuth2 refresh, athletes/activities/comments, typed manual activity and streaming file upload, webhooks | Local contract tests |
| `mixcloud/api` | OAuth2 browser flow, users/Cloudcasts/comments/search, Favourite/Repost/Listen Later/Follow, typed streaming MP3 upload | Local contract tests |
| `spotify/web-api-v1` | OAuth2 PKCE, profiles/catalog/library/playlists, Premium-gated Spotify Connect playback | Local contract tests |
| `applemusic/api` | ES256 developer tokens, storefront/catalog/library/playlists, recently played history | Local contract tests |
| `lastfm/web-services-v2` | Signed browser auth, track/artist/album discovery, profiles/listening history, now playing, scrobbling, love state | Local contract tests |
| `musicbrainz/ws-v2` | Credential-free artist/release/recording/work metadata, typed lookup and browse, shared public-service request gate | Local contract tests; public WS/2 smoke |
| `listenbrainz/api-v1` | Public listening history, playing now, feedback and JSPF playlists; token-authorized scrobbling, feedback and deletion | Local contract tests; public API v1 smoke |
| `trakt/v2` | OAuth2 browser/device flows, movie/TV catalog, history/watchlist/ratings sync, scrobbling, comments | Local contract tests |
| `tmdb/v3` | Bearer/API-key app auth, movie/TV/person catalog, user sessions, favorites/watchlist/ratings | Local contract tests |
| `letterboxd/api-v0` | OAuth2/OIDC, film search/catalog, members/activity/watchlists, diary/review log entries, likes/ratings/watched state | Local contract tests; access-request beta |
| `myanimelist/v2` | OAuth2 plain PKCE, anime/manga catalog and rankings, profiles, personal list reads and mutations | Local contract tests; API v2 beta |
| `anilist/graphql-v2` | OAuth2, anime/manga discovery, profiles, media-list tracking, text/list activities, replies and likes | Local contract tests; public GraphQL smoke |
| `kitsu/edge` | Caller-managed OAuth2 tokens, JSON:API anime/manga discovery, profiles, library tracking, posts and comments | Local contract tests; public edge API smoke |
| `simkl/v1` | OAuth2 confidential/PKCE/PIN flows, movie/TV/anime catalog and trending, incremental library sync, batched history/ratings, scrobbling | Local contract tests; public CDN smoke |
| `tvmaze/public-api` | Credential-free show/episode/season catalog, broadcast and web schedules, people/credits, incremental updates | Local contract tests; public API smoke |
| `pinterest/v5` | OAuth2, owned Pins/account reads, typed board-aware Pin and video upload workflow | Local contract tests |
| `reddit/data-api` | OAuth2, profiles/submissions/comments, typed subreddit submission, human-initiated votes | Local contract tests |
| `reddit/ads-api-v3` | Ad Account, Funding Instruments, paused Campaign/Ad Group/existing-Post Ad management, synchronous paginated Reports, OAuth2 refresh | Local contract tests; adsread/adsedit authorization required |
| `reddit/conversions-api-v3` | Web/App/Offline conversion batches, local match-key normalization/SHA-256, exact decimals, test events, LDU | Local contract tests; complete current v3 ingestion operation, Reddit Pixel and conversion token or adsconversions OAuth scope required |
| `stackexchange/api-v2.3` | API key/OAuth2 PKCE, users/questions/answers/comments, typed Q&A workflow, human-initiated votes | Local contract tests |
| `hackernews/firebase-v0` | Credential-free users/items, six story feeds, direct comment trees, max item and incremental updates | Local contract tests; public API v0 smoke |
| `producthunt/graphql-v2` | Typed Post, Comment, Topic, Collection, User, and Viewer reads with Relay cursors, partial GraphQL data/errors, and complexity-budget metadata | Current live Product Hunt API v2 GraphQL reference reviewed 2026-08-24; externally managed Bearer token and public scope required, GraphQL allows 6,250 complexity points/15 minutes/application, and commercial use or third-party writes require Product Hunt approval |
| `github/rest-api` | Authenticated/public User reads, visible/public Repository discovery and detail, repository Issue/Pull Request representations, and issue-comment reads with Link-page and rate/version/scope metadata | Current REST version `2026-03-10` and official OpenAPI reviewed 2026-08-24; externally managed GitHub token and endpoint-specific permissions required, redirects and writes are intentionally excluded, and primary/secondary limits remain token- and endpoint-dependent |
| `gitlab/rest-api-v4` | Authenticated/visible User, Project, project Issue, and issue Note reads with exact decimal IDs, offset-page metadata, self-managed origins, and dynamic rate diagnostics | Current REST v4 OpenAPI and endpoint documentation reviewed 2026-08-24; externally managed Bearer-compatible token required, project paths and keyset pagination are excluded, and GitLab.com/Self-Managed quotas remain endpoint- and instance-dependent |
| `bitbucket/cloud-rest-api-v2` | Current User, accessible Workspace, Repository, Pull Request, and PR Comment reads with Bearer or Atlassian API-token Basic authentication and validated opaque continuations | Current Cloud REST 2.0 OpenAPI reviewed 2026-08-24; legacy and current `read:*:bitbucket` scope vocabularies are supported, deprecated workspace listing, removed Issues, writes, App passwords, and token lifecycle management are intentionally excluded |
| `unsplash/api-v1` | Public Photo search/detail, User profiles/photos/collections, Collection discovery/detail/photos, and required download-event tracking with Link-page and quota metadata | Current official API v1 contract and guidelines reviewed 2026-08-24; confidential application Access Key required, demo/production quotas are 50/1,000 requests per hour, hotlinking and attribution are mandatory, and OAuth, uploads, likes, and writes are intentionally excluded |
| `tenor/api-v2` | Search, Featured, Categories, and Posts discovery with stable client identity, typed media formats, and opaque continuation positions | Current official Google Tenor API v2 contract reviewed 2026-08-24; only previously enabled API keys are usable because new API clients have not been accepted since January 2026, default quota is 1 request/second, media URLs must refresh at least every 24 hours, and Tenor attribution is mandatory |
| `google-photos/library-v1` | App-created Album and Media Item list/detail reads plus typed app-created-media search with opaque pagination | Current Library API v1 Discovery revision `20260820` and 2025 capability changes reviewed 2026-08-24; externally managed OAuth Bearer with `photoslibrary.readonly.appcreateddata` required, broad read and sharing scopes were removed after 2025-03-31, general-library selection belongs to Picker API, and Library reads allow 10,000 requests/project/day |
| `yelp/places-api-v3` | Business search/detail, review excerpts, category taxonomy, plan-aware error/rate metadata, and bounded offset paging | Current official Places v3 OpenAPI reviewed 2026-08-24; private Bearer API key and applicable plan required, Reviews require Enhanced or Premium access, non-paying Business Search retrieval is capped at 240 results, quotas are plan/endpoint dependent, and Yelp data caching is limited to 24 hours |
| `discourse/rest-api` | Per-instance API keys, users/Posts/replies, typed Topics/private messages, uploads, likes, signed webhooks | Local contract tests |
| `forem/api-v1` | Per-instance API keys, users/Articles/threaded comments, typed Article publishing and reactions | Local contract tests |
| `lemmy/api-v3` | Per-instance JWTs, people/Posts/comments, typed community publishing/votes/private messages, Pictrs image upload | Local contract tests |
| `nostr/nip-01` | Multi-relay signed events, profiles/notes/threads, NIP-09 deletion, NIP-18 reposts, NIP-25 reactions, NIP-92 media metadata | Local WebSocket contract tests |
| `indieweb/micropub` | W3C Micropub bearer auth, h-entry publishing, source/config/syndication queries, typed editing and Media Endpoint upload | Local protocol contract tests |
| `dribbble/v2` | OAuth2 Publishing API, owned profiles/Shots, typed image publishing, Projects and Attachments | Local contract tests |
| `deviantart/api-v1-20240701` | OAuth2.1 PKCE, users/Deviations/galleries/comments, text Status publishing, favourites | Local contract tests |
| `snapchat/public-profile-v1` | OAuth2, typed read-only Public Profile discovery and Spotlight workflow | Local contract tests |
| `mastodon/rest` | Per-instance OAuth2, profiles/statuses/home timeline, media, favourites/boosts, instance discovery | Local contract tests |
| `matrix/client-server-v1.19` | Per-homeserver bearer tokens, profiles/room events, threads/reactions, raw media upload, incremental sync | Local contract tests |
| `misskey/api` | Per-instance tokens/MiAuth, users/Notes/home timeline, Drive media, emoji reactions, instance discovery | Local contract tests |
| `bluesky/atproto` | Per-PDS sessions, profiles/posts/feeds/threads, repo records, blobs, likes/reposts | Local contract tests |
| `threads/api` | OAuth2, text/reply/quote publishing, remote media containers, replies, insights, discovery, moderation, reposts | Local contract tests |
| `twitch/helix` | OAuth2 user/app tokens, users/VODs, streams, channels, schedules, clips, chat, EventSub webhooks | Local contract tests |
| `steam/web-api` | Batched Player Summary reads using an ordinary user Web API key plus keyless App News v2 reads through an isolated public transport | Current official Steam Web API v2 method and authentication documentation reviewed 2026-08-24; SteamID values preserve the full uint64 range, no stable quota is claimed, and publisher APIs plus schema-incomplete Friend/Owned/Recent reads are intentionally excluded |
| `kick/public-api-v2` | OAuth2.1 user/app tokens, typed channels/V2 livestream discovery/chat, event subscriptions, RSA webhooks | Local contract tests |
| `peertube/rest-v1` | Per-instance OAuth2 password/refresh grants, accounts/videos/comments, ratings, channels, typed streaming video upload | Local contract tests |
| `whatsapp/cloud-v25` | User/system-user tokens, text/media/template messages, media lifecycle, business profiles, signed webhooks | Local contract tests |
| `tumblr/v2` | API-key/OAuth2, NPF posts, inline media, blogs/dashboard/tagged feeds, notes, likes/follows | Local contract tests |
| `wordpress.com/rest-v1.1` | OAuth2, WordPress.com/Jetpack sites, Posts, Comments/Likes, typed publishing and streaming Media | Local contract tests |
| `google/blogger-api-v3` | Typed read-only Blog, Post, Comment, and static Page lookup/list/search workflows with ownership validation and opaque pagination | Current Blogger v3 Discovery revision `20260816` reviewed 2026-08-25; externally managed OAuth Bearer with `blogger.readonly` or full Blogger scope required, quotas remain Google Cloud project/policy dependent, provider HTML requires caller-side rendering sanitization, and all state-changing methods are intentionally excluded |
| `patreon/api-v2` | OAuth2 refresh, creator identity/Campaigns/Posts/Members, signed webhooks | Local contract tests |
| `mailchimp/marketing-api-v3.0` | Privacy-bounded Audience/List and Campaign metadata plus aggregate Campaign reports using account-specific API-key authentication | Current Swagger 3.0.91 contract reviewed 2026-08-25; contacts, member activity, campaign content, sending, and all mutations are intentionally excluded; provider concurrency is 10 processing requests per user |
| `line/messaging-api` | Channel tokens, typed push/reply/multicast messages, profiles, inbound content, quotas, signed webhooks | Local contract tests |
| `viber/bot-api` | Commercial Bot tokens, typed messages/broadcasts, subscriber profiles/presence, HMAC webhooks | Local contract tests |
| `kakao/login-talk-rest` | OAuth2, authorized-user profiles, approved friend discovery, self/friend Talk messages | Local contract tests |
| `zalo/official-account` | OAuth v4 PKCE, consultation messages, OA/user profiles, signed webhooks | Local contract tests |
| `vk/v5.199` | User/community/service tokens, walls/reposts, profiles, photos, comments/likes, messages, Callback API | Local contract tests |
| `telegram/bot-api` | Bot messages, channel text posts, typed media sends, webhook registration/verification | Local contract tests |
| `discord/v10` | Bot messages, users/channel history, replies/reactions, Gateway discovery | Local contract tests |
| `slack/web-api` | Bot/user tokens, channel messages/threads, reactions, external files, signed Events API | Local contract tests |
| `lark/openapi` | Feishu/Lark dual-region tokens, chats/threads, reactions, IM resources, encrypted events | Local contract tests |
| `microsoft-teams/graph-v1` | Global/national-cloud Graph v1.0, chat/channel threads, hosted content, reactions, basic change notifications | Local contract tests |
| `wechat/official-account` | App token, follower profiles, customer-service messages, drafts, materials, XML/AES webhooks | Local contract tests |
| `wechat/mini-program` | `code2Session`, stable access-token retrieval/cache, user-authorized subscription messages, and consented phone-number exchange with AppID watermark verification | Current continuous WeChat Mini Program server contracts reviewed 2026-08-25; AppID/AppSecret and eligible account capabilities required, `code2Session` is not OAuth, stable-token ordinary calls publish 10,000/minute and 500,000/day while force refresh is 20/day with 30-second spacing, and OpenID/session keys/phone data require strict China-region privacy handling |
| `wechat/store-shop` | Self-managed stable-token lifecycle, Store profile, bounded Product listing, and typed Product detail reads | Current continuous WeChat Store contract reviewed 2026-08-25; merchant activation, subject/category/brand/product qualifications and IP allowlisting remain provider gates; orders, customer PII, funds, writes, media, webhooks, and ISV authorization are intentionally excluded |
| `wecom/corp-api` | Corp tokens, members, typed application messages, temporary media, encrypted callbacks | Local contract tests |
| `dingtalk/openapi-v1.0` | Corp-scoped app tokens, UnionID contact reads, typed application-robot group and OTO batch messages | Local contract tests |
| `qq/bot-api` | App tokens, C2C/group/channel messages, scene-bound URL media, Ed25519 callbacks | Local contract tests |
| `weibo/v2` | OAuth2, posts/reposts, users/timelines, comments/likes, image upload | Local contract tests |
| `douyin/openapi` | OAuth2 user/client tokens, users/videos/comments, direct/chunked upload, webhooks | Local contract tests |
| `toutiao/openapi` | OAuth2 user/client tokens, authorized profiles, owned videos, direct/chunked upload | Local contract tests |
| `xigua/openapi` | OAuth2 user/client tokens, authorized profiles, owned videos, 16 GiB multipart workflow | Local contract tests |
| `kuaishou/openapi` | OAuth2, user profiles, direct/fragment video upload, mandatory-cover publication | Local contract tests |
| `bilibili/open-platform` | OAuth2, v2 request signing, creator profiles, video/cover upload, archive management | Local contract tests |
| `bilibili/live-open-platform-v2` | v1 HMAC signing, project lifecycle/heartbeats, binary WebSocket auth/events, cluster failover | Build verified; credentialed validation required |
| `ximalaya/open-api-v1` | Server-signed Category discovery, category Album lists, Album track browsing, and multi-condition Album/Track search | Current official Open Platform content, signing, onboarding, model, and quota contracts reviewed 2026-08-24; approved app/product, `app_secret`, `serverAuthenticateStaticKey`, source-IP allowlist, real device identity, and launch reporting obligations required; shared allowance is 5,000 requests/minute and 280,000/hour, while OAuth, paid purchase, uploads, writes, callbacks, and media caching are intentionally excluded |
| `xiaohongshu/share-js` | Approved-app token signing and media-only client Share JS handoff | Local contract tests |
| `zhihu/data-api` | Access Secret auth, site search, hot list, authorized-user content reads, OAuth2 | Local contract tests |

No credentialed adapter has been validated against a real platform account yet.
Deterministic local fixtures remain the required baseline; opt-in public read
smoke tests are identified in the table above.

</details>

## Repository map

```text
adapters/            Opt-in platform adapter packages
cmd/social-hub-mcp/  Self-hosted MCP server
config/              Configuration examples and schemas
docs/                Architecture and deployment guides
examples/            Runnable integration examples
extensions/          Typed platform-specific capabilities
internal/            Shared transport and implementation details
pkg/socialhub/       Stable public contracts and models
skills/              Agent operating guidance
```

## Documentation

- [Architecture, platform research, WBS, and delivery plan](docs/social-hub-blueprint.md)
- [MCP self-hosting and Codex configuration](docs/mcp.md)
- [Agent Skill for operating social-hub MCP](skills/use-social-hub-mcp/SKILL.md)
- Adapter-specific setup under `adapters/<name>/README.md`

## Development status

The repository uses deterministic local contracts as its baseline. Public
read-only smoke checks are opt-in, and credentialed adapters still require
validation against approved platform accounts before production rollout.

```powershell
go build ./...
go test ./...
go test -race ./...
go vet ./...
```

When adding an adapter, keep provider-specific APIs typed, document scopes and
commercial approval gates, and register the package without expanding the core
interface for a single platform.
