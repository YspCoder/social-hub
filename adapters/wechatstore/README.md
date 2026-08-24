# WeChat Store read-only adapter

Adapter name: `wechat/store-shop`

This package implements the self-managed merchant credential flow and a
minimal catalog read surface for WeChat Store (微信小店):

- `POST /cgi-bin/stable_token` in ordinary and explicit force-refresh modes;
- `GET /channels/ec/basics/info/get`;
- `POST /channels/ec/product/list/get`;
- `POST /channels/ec/product/get`.

It does not implement orders, customer phone numbers, addresses, funds,
after-sales service, customer service, writes, media upload, webhooks, or ISV
third-party authorization. Category reads are also intentionally absent: the
current category contracts contain both old and new trees plus nested store,
brand, product, certificate-group, dynamic attribute, and publication-rule
schemas. A partial category model would conceal material qualification rules.

The package has no third-party Go dependency.

## Naming and product boundary

The official documentation says WeChat Store was introduced as an upgrade of
视频号小店. Current product and navigation names use 微信小店, while many API
paths retain the historical `/channels/ec/...` namespace. The adapter follows
the current product name and does not rewrite those official paths.

This package is for the **merchant self-managed mode** documented by WeChat.
The separate ISV mode uses merchant authorization and an
`authorizer_access_token`; it is not interchangeable with this adapter's
AppID/AppSecret flow.

## Configuration

Each account represents one self-managed WeChat Store AppID. AppID and
AppSecret are obtained from the WeChat Store console under the self-managed
developer tools described by the official guide.

```yaml
version: 1
platforms:
  - adapter: wechat/store-shop
    product: store-shop
    accounts:
      - id: primary
        app_id: wx0000000000000000
        secret_ref: env://WECHAT_STORE_SECRET
      - id: secondary
        app_id: wx1111111111111111
        secret_ref: env://WECHAT_SECONDARY_STORE_SECRET
```

The API origin is fixed to `https://api.weixin.qq.com`. The configured HTTP
client is copied with redirects disabled and its cookie jar removed. This
adapter retrieves stable tokens itself and rejects `access_token_ref`, token
stores, webhook configuration, and externally supplied API origins.

## Stable access token

Ordinary calls are coordinated per client and cached until five minutes
before provider expiry. The official contract reviewed on 2026-08-25 states:

- the returned lifetime is no more than 7,200 seconds;
- the stable-token endpoint is limited to 10,000 calls per minute and 500,000
  calls per day;
- force refresh is limited to 20 calls per day and consecutive calls must be
  at least 30 seconds apart;
- ordinary and force-refresh modes can invalidate or replace credentials as
  described by WeChat.

The adapter enforces the 30-second force-refresh interval within one client.
It cannot coordinate the provider-wide daily limit across application
replicas. No product endpoint quota is stated on the reviewed product pages,
so this package does not invent one; the provider response and store console
remain authoritative.

```go
token, err := client.Credentials().GetStableAccessToken(ctx)
```

`token.Value` is a credential. Explicit
`ForceRefreshStableAccessToken` invalidates the previous provider token and
should be reserved for credential recovery.

## Store information

The implemented endpoint returns the documented store name, image URL,
subject type, lifecycle status, original store ID, local-life flag, and open
timestamp. It does not return an order, customer phone number, or customer
address.

```go
base, err := adapter.Client(ctx, "primary")
if err != nil {
	return err
}
client := base.(*wechatstore.Client)

store, err := client.Store().GetInfo(ctx)
```

Documented subject types are `企业` and `个体工商户`. Documented store statuses
are `opening`, `open_finished`, `closing`, and `close_finished`.

## Product catalog

Product listing uses `next_key` cursor pagination. `PageSize` defaults to 10
and must not exceed the documented maximum of 30. Omitting `Status` requests
all products except the provider exclusions described by WeChat. Use a
pointer when filtering because status `0` is itself a documented value.

```go
listed := wechatstore.ProductListStatusListed
page, err := client.Catalog().ListProducts(ctx, wechatstore.ListProductsRequest{
	Status:   &listed,
	PageSize: 30,
})
if err != nil {
	return err
}

next, err := client.Catalog().ListProducts(ctx, wechatstore.ListProductsRequest{
	Status:   &listed,
	PageSize: 30,
	NextKey:  page.NextKey,
})
```

The list endpoint documents status values `0` (initial), `5` (listed), `6`
(recycle bin), and `11` (delisted). Product detail supports online data,
draft data, or both through `ProductDataOnline`, `ProductDataDraft`, and
`ProductDataBoth`. Detail status enums reproduce the current official values;
callers must still tolerate platform evolution because this is a continuous
API.

```go
detail, err := client.Catalog().GetProduct(ctx, wechatstore.GetProductRequest{
	ProductID: page.ProductIDs[0],
	DataType:  wechatstore.ProductDataOnline,
})
```

The typed product model exposes stable identifiers, titles, images,
description, current states, prices, categories, attributes, SKUs, stock, and
merchant codes. Unknown provider fields are discarded. Neither product
objects nor pages retain a `Raw` response.

## Eligibility, qualifications, and compliance

Successful authentication does not imply permission to operate every store
or product API. Store activation, account state, subject eligibility, IP
allowlisting, category permission, deposit, brand qualification, product
qualification, and platform enforcement are independent provider-side gates.
The official category guide says the current storefront uses the new
`cats_v2` tree, while APIs still expose both old and new trees, and that some
categories require reviewed certificates or brand/product materials.

This SDK does not grant those permissions, determine whether a subject or
product is legally eligible, or replace review in the WeChat Store console.
Errors such as `48001`, `10020050`, and `10020177` are surfaced as approval
required with a documentation link; restrictions and account closure states
remain user-action failures.

Although this adapter excludes customer order PII, store identifiers, catalog
text, prices, stock, product codes, qualification references, and images can
still be commercially sensitive or regulated. Callers are responsible for
authorization, purpose limitation, access controls, secure retention,
deletion, auditability, intellectual-property rights, advertising and product
claims, and applicable cross-border or industry rules.

## Error and sensitive-data behavior

WeChat commonly returns business errors as HTTP 200 JSON containing
`errcode`, `errmsg`, and sometimes `rid`. The adapter classifies documented
common and endpoint-specific codes while retaining only numeric `errcode` and
a validated request ID. Provider `errmsg` and complete response bodies are
discarded because they may echo AppID, AppSecret, access tokens, request
identifiers, or product content.

Transport failures are stripped of their URL wrapper so query access tokens
cannot enter returned errors. Sensitive tokens and catalog-containing public
models implement redacted `String` and `GoString`. There is no error `Raw`
field and no success-response `Raw` field.

## Official sources

Official material reviewed via the current WeChat documentation on
2026-08-25:

- <https://developers.weixin.qq.com/doc/store/shop/>
- <https://developers.weixin.qq.com/doc/store/shop/dev_before/guide.html>
- <https://developers.weixin.qq.com/doc/store/shop/dev_before/commErrCode.html>
- <https://developers.weixin.qq.com/doc/store/shop/API/apimgnt/common/api_getstableaccesstoken.html>
- <https://developers.weixin.qq.com/doc/store/shop/API/storemanage/api_mmecapi_basicinfo.html>
- <https://developers.weixin.qq.com/doc/store/shop/API/channels-shop-product/shop/api_getproductlist.html>
- <https://developers.weixin.qq.com/doc/store/shop/API/channels-shop-product/shop/api_getproduct.html>
- <https://developers.weixin.qq.com/doc/store/shop/guide/catalog/category.html>
- <https://developers.weixin.qq.com/doc/store/shop/guide/catalog/product.html>
