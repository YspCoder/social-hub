# Vipshop Union adapter

Registration name: `vipshop/union-open-api-v2`

This adapter implements a bounded publisher surface of Vipshop Union Open API
V2: keyword goods discovery, batch and channel-aware goods detail, affiliate
link generation, and attributed-order reconciliation. Commerce resources do
not implement the organic `Publisher`, `Fetcher`, `Reactor`, or `Messenger`
interfaces.

## Implemented workflows

| social-hub method | Vipshop method | Notes |
|---|---|---|
| `Goods().SearchGoods` | `UnionGoodsV2Service.queryWithOauth` | required keyword, generated or caller-supplied request ID, page size 1-50, typed sort/price scenes, channel identity, and optional inventory/promotion/CPS fields |
| `Goods().GetGoods` | `UnionGoodsV2Service.getByGoodsIdsV2WithOauth` | 1-10 goods IDs, optional detail/stock/SKU/activity/coupon fields, and exact openId/chanTag propagation |
| `Goods().GetMarketingGoods` | `UnionGoodsV2Service.getGoodsDetailMarketingWithOauth` | one good with channel-user-aware price calculation and optional size/SKU selection |
| `Links().GeneratePromotionLinks` | `UnionUrlV2Service.genByGoodsIdWithOauth` | 1-50 goods IDs; CPS URL, deep link, WeChat/Alipay mini-program, quick-app, and Vipshop command outputs |
| `Orders().ListOrders` | `UnionOrderV2Service.orderListWithOauth` | order-time, update-time, or up to 50 order numbers; time windows are at most one hour and page size is at most 100 |

The initial surface excludes brand and personalized recommendation feeds, PID
creation, promotion-material catalogs, refund/virtual-order endpoints, link
parsing, channel landing pages, user verification, gift creation, and OAuth
authorization UI. Those contracts have distinct permissions and should be
added as separate workflows rather than hidden behind the five methods above.

## Configuration

`client_id` is the Vipshop `appKey`; `secret_ref` resolves to `appSecret`, and
`access_token_ref` resolves to the OAuth `accessToken`. The access token is a
signed query parameter required by these `WithOauth` methods, not an HTTP
Bearer token.

```yaml
version: 1
platforms:
  - adapter: vipshop/union-open-api-v2
    product: union-open-api
    accounts:
      - id: main-publisher
        client_id: vipshop-app-key
        secret_ref: env://VIPSHOP_APP_SECRET
        access_token_ref: env://VIPSHOP_ACCESS_TOKEN
        approval:
          account_type: vipshop-union-publisher
        settings:
          default_chan_tag: default_pid
          default_open_id: default_open_id
          default_ad_code: unionapi
```

When an operation omits `ChanTag` or `OpenID`, the adapter uses the configured
default and then the provider-defined `default_pid` or `default_open_id`.
`GeneratePromotionLinks` similarly defaults `AdCode` to `unionapi`; callers
should pass the exact `Goods.AdCode` returned by a material endpoint whenever
one is available. This fallback represents a channel publisher; an approved
tool-provider application must configure `default_ad_code: vendoapi` as
required by the official contract. An order query does not apply
`default_chan_tag` automatically because doing so would silently narrow
reconciliation results.
`approval.account_type`, when present, must be
`vipshop-union-publisher`; it records the external publisher approval in
capability metadata but does not grant platform permissions.

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/vipunion"
```

## Protocol and data guarantees

- The gateway is fixed to `https://vop.vipapis.com`; configuration cannot
  redirect credentials to another origin. Requests are POSTed as
  `application/json; charset=UTF-8` with common query parameters `appKey`,
  `accessToken`, Unix-seconds `timestamp`, `format=JSON`, `language=zh`,
  `service`, `method`, and `version=2.0.0`.
- Signing is uppercase HMAC-MD5 over sorted `key + value` common parameters
  followed by the exact JSON request bytes. The bytes signed are the bytes
  sent. Redirects are rejected so query credentials cannot move to another
  origin.
- Each request includes the provider-required `requestId`. A caller may set it
  with `socialhub.WithRequestID`; otherwise the adapter generates 128 random
  bits and sends the same value in the JSON body and `X-Request-ID` header.
- `RealCall` is always serialized. It must reflect whether the operation was
  triggered for live user display (`true`) or by a background synchronization
  job (`false`). The adapter does not infer this compliance signal.
- Typed goods, promotion links, and orders retain the complete successful
  provider object in `Raw`. Prices, commissions, rates, IDs, and attribution
  strings remain strings where the official contract defines them as strings.
- HTTP and `returnCode != 0` failures map to `socialhub.Error` without exposing
  provider free-form messages. Official code
  `1008` is retryable rate limiting; service/IO/database codes `1002`-`1006`
  are retryable; account registration, authorization, and package failures
  require user action.

## Authorization, quota, and privacy

A Vipshop Open Platform application, a bound Vipshop Union publisher account,
OAuth authorization, relevant Union API grants, and allowed source IPs are
platform-side prerequisites. The official console notes that configured IP
allowlists govern both gateway calls and `/oauth2/token` exchange. This
adapter consumes a resolved access token; authorization-code exchange,
refresh, revocation, and secure persistence remain application-owned. The
official OAuth guide documents a natural `access_token` lifetime of 90 days
and a `refresh_token` lifetime of one year; applications should refresh before
expiry and re-authorize after revocation or refresh-token expiry.
Applications and permission packages are managed in the official
[permission console](https://vop.vip.com/home#/console/app/permission); the
[OAuth guide](https://vop.vip.com/doccenter/viewdoc/33) describes the
authorization flow.

Vipshop publishes method-level traffic controls through account/API grants
rather than one safe global QPS value. Do not hard-code a universal rate.
Throttle per app, authorized Union account, service, and method; honor error
`1008` and `Retry-After` when present. Order reconciliation should use small,
contiguous update-time windows and persist the last completed boundary.

`openId`, optional mobile/user context, `statParam`, and order attribution can
be personal or linkable data. This surface deliberately excludes mobile and
common device/IP parameters. Callers remain responsible for consent,
minimization, retention, and the business approval required before `statParam`
is returned on orders.

Official contracts:

- <https://vop.vip.com/home#/api/method/detail/com.vip.adp.api.open.service.UnionGoodsV2Service-2.0.0/queryWithOauth>
- <https://vop.vip.com/home#/api/method/detail/com.vip.adp.api.open.service.UnionGoodsV2Service-2.0.0/getByGoodsIdsV2WithOauth>
- <https://vop.vip.com/home#/api/method/detail/com.vip.adp.api.open.service.UnionGoodsV2Service-2.0.0/getGoodsDetailMarketingWithOauth>
- <https://vop.vip.com/home#/api/method/detail/com.vip.adp.api.open.service.UnionUrlV2Service-2.0.0/genByGoodsIdWithOauth>
- <https://vop.vip.com/home#/api/method/detail/com.vip.adp.api.open.service.UnionOrderV2Service-2.0.0/orderListWithOauth>

The contracts, current V2 method metadata, production gateway, permission
console, and OAuth guide were last verified on 2026-08-25.

Reference implementation reviewed for generated V2 method names, common
parameters, signing, and response schemas: `mimicode/tksdk` commit
`3a31074c5dfce17f7b77bf73e73a41bb38cc4220` (Apache-2.0). It is not added as a
runtime dependency.
