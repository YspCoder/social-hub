# Duoduo Jinbao adapter

Registration name: `pinduoduo/duoduo-jinbao-api-v1`

This adapter implements the Pinduoduo Open Platform `V1` form gateway for a
bounded Duoduo Jinbao publisher path: discover recommended goods, inspect a
signed good, generate an affiliate link for one good, and reconcile orders by
last update time. These commerce resources intentionally do not implement the
organic `Publisher`, `Fetcher`, `Reactor`, or `Messenger` interfaces.

## Implemented workflows

| social-hub method | Pinduoduo method | Notes |
|---|---|---|
| `Goods().RecommendGoods` | `pdd.ddk.goods.recommend.get` | typed offset, 1-400 limit, and optional budget/hot/brand/mall channel; a zero limit is normalized to 20 |
| `Goods().GetGoods` | `pdd.ddk.goods.detail` | current `goods_sign` identifier required; optional PID, bounded JSON attribution parameters, and recommendation `search_id` |
| `Links().GeneratePromotionLinks` | `pdd.ddk.goods.promotion.url.generate` | one `goods_sign` per call, required PID, web/mobile/short-link controls, and optional approved WeChat/QQ/Weibo/schema outputs |
| `Orders().ListIncrementalOrders` | `pdd.ddk.order.list.increment.get` | explicit last-update range, page size 10-100 (default 50), page 1-10000, and exact-value attribution/order amounts |

The initial surface deliberately excludes personalized device identifiers,
cash gifts, agent/tool-provider APIs, live-room links, PID creation or media
binding, and information-flow filing uploads. `pull_new` remains available as
an explicit optional flag because the provider requires a separate allowlist;
the adapter does not imply that an account has that grant.

## Configuration

`client_id` is the Pinduoduo application `client_id`; `secret_ref` resolves to
its `client_secret`. `access_token_ref` is optional and resolves to the
`access_token` common form parameter. It is never sent as an HTTP bearer token.

```yaml
version: 1
platforms:
  - adapter: pinduoduo/duoduo-jinbao-api-v1
    product: duoduo-jinbao-api
    accounts:
      - id: main-publisher
        client_id: pdd-client-id
        secret_ref: env://PDD_CLIENT_SECRET
        access_token_ref: env://PDD_ACCESS_TOKEN
        approval:
          account_type: duoduo-jinbao-publisher
        settings:
          default_pid: "1234567_123456789"
```

`default_pid` is optional at configuration time. Promotion-link generation
must supply `PID` when no default is configured. Goods detail uses the request
PID when present and otherwise uses the default, allowing search/detail/link
attribution to stay on the same registered promotion position.

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/pddunion"
```

## Protocol and data guarantees

- Production defaults to `https://gw-api.pinduoduo.com/api/router`. Requests
  are POSTed as `application/x-www-form-urlencoded;charset=utf-8`. The gateway
  is fixed and cannot be overridden by configuration.
- Common parameters use `type=<method>`, `client_id`, Unix-seconds
  `timestamp`, `data_type=JSON`, `version=V1`, and an optional `access_token`.
- Signing follows uppercase
  `MD5(client_secret + sorted(key + value) + client_secret)`. `sign` itself is
  excluded from the digest, and an empty optional access token is omitted.
- Redirects are rejected and caller cookie jars are ignored so the signed form
  and credentials cannot move to a different origin.
- `goods_sign_list` is encoded as a JSON array before form encoding. Optional
  booleans are omitted unless the caller explicitly supplies a pointer; the
  required short-link choice is always transmitted.
- `ExactValue` preserves JSON strings and numbers for goods/order/group IDs,
  prices in fen, commissions, rates, counts, and timestamps. Typed goods,
  links, and order rows retain complete successful objects in `Raw`.
- `goods_gallery_urls` accepts both current arrays and legacy strings that
  contain a JSON array. Malformed provider shapes are surfaced rather than
  silently discarded.
- HTTP and `error_response` failures are mapped to `socialhub.Error`, retaining
  bounded provider codes and request IDs. Free-text messages are used only for
  classification and are not exposed because they can echo request data. QPS
  failures and documented service faults are retryable; credentials,
  qualification, filing, and allowlist failures require account action.

`custom_parameters` must be a JSON object of at most 64 bytes. It is returned
with attributed orders and may contain a caller-defined user/channel key;
callers are responsible for minimization, consent, retention, and avoiding raw
personal data.

## Qualification, quota, and order windows

A Pinduoduo Open Platform application, Duoduo Jinbao publisher qualification,
an approved media relationship, a registered PID, and the relevant API package
are platform-side prerequisites. WeChat web-view/mini-program, QQ mini-program,
Weibo, pull-new, and information-flow placements can require additional filing,
application configuration, or allowlisting. Recording
`approval.account_type` marks external review in capability metadata but does
not bypass provider checks.

Pinduoduo assigns call limits dynamically by application, method, account,
permission package, and traffic tier; there is no safe universal QPS value to
hard-code. Incremental order queries cover recently updated Duoduo Jinbao
orders (the provider contract currently describes a latest-90-day boundary).
Callers should use small contiguous windows, persist the last successful end
timestamp, and paginate until the reported total is exhausted.

Official contracts:

- <https://open.pinduoduo.com/application/document/api?id=pdd.ddk.goods.recommend.get>
- <https://open.pinduoduo.com/application/document/api?id=pdd.ddk.goods.detail>
- <https://open.pinduoduo.com/application/document/api?id=pdd.ddk.goods.promotion.url.generate>
- <https://open.pinduoduo.com/application/document/api?id=pdd.ddk.order.list.increment.get>

These four contracts and the production gateway were rechecked on 2026-08-25.

Reference implementation reviewed for generated V1 common parameters and
current response schemas: `mimicode/tksdk` commit
`3a31074c5dfce17f7b77bf73e73a41bb38cc4220` (Apache-2.0). It is not added as a
runtime dependency.
