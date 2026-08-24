# Taobao Union adapter

Registration name: `alimama/taobao-union-api-v2`

This adapter implements the Alibaba TOP `v=2.0` form protocol for the common
Taobao Union publisher path: discover commissionable materials, inspect item
details, convert item or material links, create a Tao Password, and reconcile
attributed orders. These commerce resources intentionally do not implement the
organic `Publisher`, `Fetcher`, `Reactor`, or `Messenger` interfaces.

## Implemented workflows

| social-hub method | TOP method | Notes |
|---|---|---|
| `Materials().SearchMaterials` | `taobao.tbk.dg.material.optional` | `adzone_id` required; page size is 1-100; typed common search, price, commission, coupon, Tmall, overseas, scene, and cursor filters |
| `Items().GetItems` | `taobao.tbk.item.info.get` | 1-40 item IDs; PC/wireless, IP, relation, promotion, and item-library scene controls |
| `Links().ConvertLinks` | `taobao.tbk.dg.general.link.convert` | typed `item_dto` and `material_dto`; per-entry platform codes are returned without hiding partial outcomes |
| `Links().CreateTaoPassword` | `taobao.tbk.tpwd.create` | only the effective `url` field is exposed; deprecated compatibility fields are omitted |
| `Orders().ListOrders` | `taobao.tbk.order.details.get` | created/paid/settled/updated time bases, status/role/scene filters, position cursor, and page metadata |

The initial link-conversion surface deliberately excludes shop/page DTOs,
legacy comma-separated lists, service-provider (`taobao.tbk.sc.*`) methods, and
invite-only targeting fields. Successful provider objects retain their complete
JSON in `Raw` where the typed model is intentionally smaller than the evolving
TOP response.

## Configuration

`client_id` is the TOP `app_key`; `secret_ref` resolves to the `app_secret`.
`access_token_ref` is optional and, when configured, resolves to the TOP
`session` common parameter. It is not sent as an HTTP bearer token.

```yaml
version: 1
platforms:
  - adapter: alimama/taobao-union-api-v2
    product: taobao-union-api
    accounts:
      - id: main-publisher
        client_id: "12345678"
        secret_ref: env://TAOBAO_TOP_APP_SECRET
        access_token_ref: env://TAOBAO_TOP_SESSION
        approval:
          account_type: taobao-union-publisher
        settings:
          default_adzone_id: "87654321"
          partner_id: ""
```

`default_adzone_id` is optional at configuration time. Calls to material search
and link conversion must supply an `AdzoneID` when no default is configured.
Use the official sandbox by setting the adapter-level `base_url` to
`https://gw.api.tbsandbox.com/router/rest`; production defaults to
`https://eco.taobao.com/router/rest`. These are the only accepted gateway
values so signed credentials cannot be redirected to an arbitrary host.

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/taobaounion"
```

## Protocol and data guarantees

- Requests use `application/x-www-form-urlencoded;charset=utf-8`, JSON output,
  `simplify=true`, and `v=2.0`.
- The timestamp is generated in GMT+8. TOP normally accepts at most about ten
  minutes of client/server clock skew.
- MD5 signing follows TOP's uppercase
  `MD5(app_secret + sorted(key + value) + app_secret)` contract. `sign` itself
  is excluded from the digest.
- Redirects are rejected and caller cookie jars are ignored so signed forms
  and credentials cannot move to a different origin.
- `ExactValue` preserves JSON strings and numbers for prices, commissions,
  identifiers, counts, and order amounts without `float64` coercion.
- TOP codes, sub-codes, request IDs, and retry hints are mapped to
  `socialhub.Error`; free-text error messages are discarded because they can
  echo request values. Link-conversion item-level codes remain in the
  successful result for caller reconciliation.

## Qualification, quota, and order windows

A TOP application, the relevant Taobao Union API package, publisher/media
registration, an authorized promotion position, and method-specific commercial
eligibility are platform-side prerequisites. Some relation, member-operation,
item-library, coupon, and order fields are permission-gated. Recording
`approval.account_type` marks that external review in capability metadata but
does not bypass TOP checks.

TOP quotas are assigned dynamically by application, API package, method, and
publisher tier; do not hard-code a universal QPS value. The adapter classifies
TOP call-limit errors as retryable rate limits, while daily or commercial
entitlement exhaustion may still require user action according to the returned
sub-code.

Order queries enforce the normal maximum three-hour interval. During major
promotions such as 618 or Double 11, TOP may reduce the accepted interval to
about 20 minutes; callers must split and resume from `PositionIndex` when the
platform reports that tighter boundary. Order IDs, money, and commission values
must be treated as strings through `ExactValue.String()` unless the caller
explicitly decodes them into an exact numeric type.

Official API category:
<https://open.taobao.com/API.htm?docType=2&docId=48340>

The official API category plus production and sandbox gateway availability
were rechecked on 2026-08-25.

Reference implementation reviewed for generated TOP schemas and signing
behavior: `bububa/opentaobao` v1.4.6, commit `3a362f9b140f` (no runtime
dependency is added by this adapter).
