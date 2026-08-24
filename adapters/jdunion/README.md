# JD Union adapter

Registration name: `jd/union-open-api-v1.0`

This adapter implements the JD Union Open API `v=1.0` gateway protocol for a
small, complete publisher path: discover Jingfen channel goods, convert a
website or app material into an affiliate link (and optionally a JD command),
and reconcile attributed order rows. These commerce resources intentionally do
not implement the organic `Publisher`, `Fetcher`, `Reactor`, or `Messenger`
interfaces.

## Implemented workflows

| social-hub method | JD method | Notes |
|---|---|---|
| `Goods().QueryJingfen` | `jd.union.open.goods.jingfen.query` | channel ID required; page size defaults to 20 and is capped at 50; typed pagination, ordering, PID, and current optional response fields |
| `Promotions().CreatePromotion` | `jd.union.open.promotion.common.get` | `materialId`, registered site/app/media ID, and scene 1 or 2 required; typed attribution identifiers, coupon combination, and optional JD command |
| `Orders().QueryOrderRows` | `jd.union.open.order.row.query` | explicit page/type/time inputs, at most one hour per request, child-publisher/key authorization selectors, optional order ID and current response fields |

The initial surface deliberately excludes device identifiers used for
personalized recommendations, invite-only agent/sub-publisher methods, and
deprecated request parameters. The goods-detail method
`jd.union.open.goods.promotiongoodsinfo.query` is also excluded because its
official contract page was access-restricted during the 2026-08-24 review;
Jingfen goods retain their complete provider JSON in `Raw` instead.

## Configuration

`client_id` is the JD `app_key`; `secret_ref` resolves to the `app_secret`.
`access_token_ref` is optional and, when configured, resolves to the JOS
`access_token` common parameter. It is never sent as an HTTP bearer token.

```yaml
version: 1
platforms:
  - adapter: jd/union-open-api-v1.0
    product: union-open-api
    accounts:
      - id: main-publisher
        client_id: jd-app-key
        secret_ref: env://JD_UNION_APP_SECRET
        access_token_ref: env://JD_UNION_ACCESS_TOKEN
        approval:
          account_type: jd-union-publisher
        settings:
          default_site_id: "435676"
```

`default_site_id` is optional at configuration time. A promotion call must
supply `SiteID` when no default is configured. The ID must identify the
registered website, app, or traffic media actually used for placement; using a
different filing can invalidate attributed orders.

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/jdunion"
```

## Protocol and data guarantees

- Production defaults to `https://router.jd.com/api`. Requests are POSTed as
  `application/x-www-form-urlencoded;charset=utf-8`; method input is a JSON
  `param_json` object under `goodsReq`, `promotionCodeReq`, or `orderReq`. The
  gateway is fixed and cannot be overridden by configuration.
- Common parameters use `format=json`, `v=1.0`, `sign_method=md5`, and a GMT+8
  `yyyy-MM-dd HH:mm:ss` timestamp.
- Signing follows uppercase
  `MD5(app_secret + sorted(key + value) + app_secret)`. `sign` is excluded and
  an empty optional `access_token` is omitted.
- Redirects are rejected and caller cookie jars are ignored so the signed form
  and credentials cannot move to a different origin.
- Current JOS `*_responce` roots and corrected `*_response` roots are both
  accepted. `queryResult`, `getResult`, and `result` may be JSON objects or
  JSON-encoded strings. Goods and order collections accept array and generated
  single-object wrapper forms without hiding malformed data.
- `ExactValue` preserves JSON strings and numbers for product/order IDs,
  prices, commissions, rates, counts, and timestamps without `float64`
  coercion. Typed goods and order rows retain their full JSON in `Raw`.
- HTTP, top-level JOS, and business-result codes are mapped to
  `socialhub.Error`; free-text messages are used only for classification and
  are not exposed because they can echo request data. Rate limits and
  documented service degradation are retryable; invalid tokens, commercial
  eligibility, and permission failures require caller or account action.

## Qualification, quota, and order windows

A JD Open Platform application, JD Union publisher registration, an approved
website/app/traffic-media record, and method-specific commercial eligibility
are platform-side prerequisites. Scene 2 main-site conversion, `subUnionId`,
`ext1`, PID/child-publisher order access, gift coupons, and some optional data
fields require additional approval. Recording `approval.account_type` marks
that external review in capability metadata but does not bypass JD checks.

JD assigns quotas dynamically by application, Union ID, method, and commercial
tier. There is no safe universal QPS value to hard-code; provider code `429` is
classified as a retryable rate limit. Order-row queries cover orders created in
the latest 90 days and enforce a maximum one-hour request window. Provider code
`2002452` means the selected window still contains more than 10,000 rows and
must be subdivided; `2002453` means the requested time range is invalid.

Official contracts:

- <https://union.jd.com/openplatform/api/v2?apiName=jd.union.open.goods.jingfen.query>
- <https://union.jd.com/openplatform/api/v2?apiName=jd.union.open.promotion.common.get>
- <https://union.jd.com/openplatform/api/v2?apiName=jd.union.open.order.row.query>

These three contracts and the production gateway were rechecked on 2026-08-25.

Reference implementations reviewed for route/result compatibility only:
`houseme/union-jd-go` commit `2ef2991b03a3735e9d4672024daec39ed73038a4`
(MIT) and `echoesink/jd-go` commit
`3f792b30773e66020885b4e0381552e17f10988d` (Apache-2.0). Neither is added as
a runtime dependency.
