# AliExpress Affiliate API adapter

Registration name: `aliexpress/affiliate-api-v2`

This adapter implements the AliExpress Open Platform TOP `v=2.0` publisher
path for AliExpress Affiliate Portals: discover commissionable products,
inspect product details, generate tracked links, and reconcile attributed
orders. These commerce resources intentionally do not implement the organic
`Publisher`, `Fetcher`, `Reactor`, or `Messenger` interfaces.

## Implemented workflows

| social-hub method | AliExpress method | Contract boundary |
|---|---|---|
| `Products().SearchProducts` | `aliexpress.affiliate.product.query` | Typed category, price, product type, sort, target locale, delivery, fields, and 1-50 page controls |
| `Products().GetProductDetails` | `aliexpress.affiliate.productdetail.get` | Batch product IDs with target locale, country, fields, and tracking context |
| `Links().GenerateLinks` | `aliexpress.affiliate.link.generate` | Standard (`0`) or hot-product (`2`) links; 1-50 source values per call |
| `Orders().ListOrders` | `aliexpress.affiliate.order.list` | Required PST interval and status, optional time basis/locale/fields, and 1-50 page controls |
| `Orders().GetOrders` | `aliexpress.affiliate.order.get` | Batch lookup by order or sub-order IDs |

The initial surface excludes featured-promotion, hot-product download,
smart-match, category, and index-based order methods. Product and order
objects retain their complete successful provider JSON in `Raw`, while
`ExactValue` preserves identifiers, counts, money, and commissions without
`float64` coercion. Request prices remain integer cents and numeric request
fields are bounded to the provider's signed 64-bit `Long` contract.

## Configuration

`client_id` is the AliExpress Open Platform `app_key`; `secret_ref` resolves
to the `app_secret`. The implemented Affiliate methods are documented with
`authType=0`, and the official samples call them without an access token, so
`access_token_ref` is intentionally rejected.

```yaml
version: 1
platforms:
  - adapter: aliexpress/affiliate-api-v2
    product: affiliate-api
    accounts:
      - id: global-publisher
        client_id: "12345678"
        secret_ref: env://ALIEXPRESS_APP_SECRET
        approval:
          account_type: aliexpress-affiliate-publisher
        settings:
          default_tracking_id: "default"
          default_app_signature: ""
```

`default_tracking_id` is optional for product discovery but required by link
generation. `default_app_signature` maps only to the optional Affiliate
business parameter `app_signature`; it is not the Open Platform request
signature and must not contain the `app_secret`.

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/aliexpressaffiliate"
```

## Protocol and data guarantees

- Production uses `POST https://api-sg.aliexpress.com/sync` and rejects
  redirects so signed credentials cannot move to another origin. The endpoint
  cannot be overridden, and the cloned HTTP client does not retain a cookie
  jar.
- The five methods use the official `Protocol.TOP` path. Business parameters
  are form encoded; `app_key`, `format=json`, `method`, `sign_method=sha256`,
  `simplify=true`, the GMT+8 `yyyy-MM-dd HH:mm:ss` timestamp, `v=2.0`, and
  `sign` are TOP common query parameters. Each common parameter occurs exactly
  once in the signed query.
- Signing is uppercase hexadecimal
  `HMAC-SHA256(app_secret, sorted(non-empty key + value))`, including the TOP
  common and business parameters and excluding `sign` itself.
- Gateway `error_response`, method `resp_code`, request IDs, and retry hints are
  mapped to `socialhub.Error`. Provider free text is used only for internal
  classification and is not returned to callers; unknown provider codes are
  preserved without guessing undocumented numeric classifications.
- Order-list timestamps are formatted in the API's documented fixed PST
  timezone. The public contract does not state a universal maximum interval,
  so callers choose and checkpoint their own bounded windows.

## Qualification and limits

An AliExpress Open Platform application, the AE-Affiliate API permission
group, an approved Affiliate Portals publisher account, and a valid tracking
ID are external prerequisites. API access can depend on application category
and commercial review. The only accepted non-empty qualification marker is
`approval.account_type: aliexpress-affiliate-publisher`; it records that review
in capability metadata but does not bypass provider checks.

The public Affiliate method metadata specifies page sizes of 1-50 and a
maximum of 50 source values for link generation. It does not publish one
universal QPS or daily quota: application categories carry their own API flow
control policies and runtime provider errors are authoritative. The adapter
therefore classifies explicit limit/frequency/flow-control responses as
retryable rate limits without hard-coding an invented quota.

## Official contract reviewed

- [AliExpress Open Platform API reference](https://open.aliexpress.com/doc/api.htm)
- [Affiliate product query](https://open.aliexpress.com/doc/api.htm#/api?cid=21407&path=aliexpress.affiliate.product.query&methodType=GET/POST)
- [Affiliate product detail](https://open.aliexpress.com/doc/api.htm#/api?cid=21407&path=aliexpress.affiliate.productdetail.get&methodType=GET/POST)
- [Affiliate link generation](https://open.aliexpress.com/doc/api.htm#/api?cid=21407&path=aliexpress.affiliate.link.generate&methodType=GET/POST)
- [Affiliate order list](https://open.aliexpress.com/doc/api.htm#/api?cid=21407&path=aliexpress.affiliate.order.list&methodType=GET/POST)
- [Affiliate order detail](https://open.aliexpress.com/doc/api.htm#/api?cid=21407&path=aliexpress.affiliate.order.get&methodType=GET/POST)
- [Open Platform application and API permission guide](https://open.aliexpress.com/doc/doc.htm#/?docId=503)
- [Official SDK guide](https://open.aliexpress.com/doc/doc.htm#/?docId=638)

The current official method metadata and its generated
`com.global.iop:iop-api-sdk:1.3.5-ae` source archive were reviewed on
2026-08-25. The generated `TopExecutor`, `IopUtils`, request classes, response
DTOs, and examples are the protocol and schema authority used here. No runtime
SDK dependency is added.
