# Shopee Ads API v2 adapter

Package `social-hub/adapters/shopeeads` implements shop-scoped, read-only
Shopee Ads API v2 workflows. Importing the package registers:

```text
shopee/ads-api-v2
```

The initial surface covers:

- real-time Ads credit balance and shop toggle state;
- recommended products and product keywords;
- paginated product-level Campaign IDs and selected Campaign settings; and
- shop CPC plus product Campaign daily/hourly performance.

Campaign creation and editing are intentionally excluded. The official Ads
module includes several generations of Product Ads and GMS mutation contracts,
while the reviewed Go SDKs expose some of those requests as untyped `any`.
Adding writes without a stable typed contract would make retry and outcome
reconciliation unsafe.

## Configuration

`client_id` contains the numeric Shopee `partner_id`; `secret_ref` resolves the
Partner Key; and `account.settings.shop_id` binds the client to one shop.

Use either a referenced access token:

```yaml
adapter: shopee/ads-api-v2
product: ads-api
accounts:
  - id: sg-shop
    client_id: "1000001"
    secret_ref: SHOPEE_PARTNER_KEY
    access_token_ref: SHOPEE_ACCESS_TOKEN
    approval:
      account_type: seller-in-house-system
    settings:
      shop_id: 600000
```

or a referenced refresh token for managed rotation:

```yaml
accounts:
  - id: sg-shop
    client_id: "1000001"
    secret_ref: SHOPEE_PARTNER_KEY
    settings:
      shop_id: 600000
      refresh_token_ref: SHOPEE_REFRESH_TOKEN
```

Managed rotation requires `socialhub.WithTokenStore(...)` to persist each new
access and refresh token before it is reused. Production stores must encrypt
credentials at rest. Shopee places the
access token and signature in the query string, so HTTP logs and telemetry must
redact `access_token`, `sign`, and all credential references.

The default global origin is `https://partner.shopeemobile.com`. Select another
official deployment with `settings.base_url`; the authorization URL is always
derived as `<base_url>/api/v2/shop/auth_partner`. Arbitrary origins and
authorization URLs are rejected so credentials cannot be redirected outside
Shopee. The current official alternatives are:

```text
China:   https://openplatform.shopee.cn
Brazil:  https://openplatform.shopee.com.br
Sandbox: https://openplatform.sandbox.test-stable.shopee.sg
CN test: https://openplatform.sandbox.test-stable.shopee.cn
```

## Authentication and signing

Shop requests use lowercase hexadecimal HMAC-SHA256 over:

```text
partner_id + api_path + timestamp + access_token + shop_id
```

Token exchange and refresh use the public signature:

```text
partner_id + api_path + timestamp
```

Shopee documents a five-minute request timestamp lifetime and a four-hour
access-token lifetime. Refresh tokens rotate; the managed token source always
stores and subsequently uses the newest returned refresh token. The token
decoder accepts both duration-style and Unix-timestamp `expire_in` values
because the official exchange and refresh examples currently use both forms.
Refresh responses must contain a newly rotated refresh token; a missing or
unchanged token is rejected before the old single-use token can be discarded.

Shopee's authorization endpoint does not define an OAuth `state` parameter.
Applications must correlate and protect the callback in their redirect URL or
session and must validate the returned shop identity before storing tokens.

## Exact report values

`ExactValue` retains balance, bid, spend, GMV, CTR, ROI, ROAS, CPC, conversion,
and count values as their original JSON tokens. Use `String`, `Bytes`, or
`Decode` with a decimal type; converting monetary values directly to `float64`
can lose precision.

## Quotas and authorization

Ads access depends on the Open Platform app type, Ads API permission, seller
authorization, shop linkage, KYC state where applicable, and declared source
IPs. The current Ads reference does not publish one fixed numeric QPS. It does
document partner-, shop-, and endpoint-level rate-limit errors plus a daily app
limit reset at 00:00 UTC+08:00. Callers should use those platform codes and
`Retry-After` when present rather than assume a universal rate.

The adapter maps HTTP and HTTP-200 error envelopes into `socialhub.Error`,
including provider error code, request ID, retry class, and bounded provider
message. A successful response may also contain a non-fatal warning, exposed
through `ResponseMeta`.

## Sources

- [Shopee Open Platform documentation](https://open.shopee.com/documents)
- [easycb/easycb-go](https://github.com/easycb/easycb-go), MIT, used for protocol cross-checking only
- [QuoVadis86/shopee-sdk](https://github.com/QuoVadis86/shopee-sdk), used for protocol cross-checking only

No third-party Shopee SDK is linked into this package. The official v2 Ads,
Public token, and host configuration contracts were reviewed on 2026-08-25.
