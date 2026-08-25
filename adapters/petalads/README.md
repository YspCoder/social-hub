# Huawei Petal Ads Marketing API adapter

`adapters/petalads` implements the overseas Huawei Petal Ads Marketing API.
It is intentionally separate from the China mainland Jinghong Momentum
(鲸鸿动能) Marketing API contract and always sends `is_abroad: true` for
reports.

Official references:

- Overview and limits: <https://developer.huawei.com/consumer/cn/doc/promotion/marketing-api-0000001174557681>
- OAuth authorization and exchange: <https://developer.huawei.com/consumer/cn/doc/promotion/marketing-api-process-4-0000001128438066>
- Campaign query: <https://developer.huawei.com/consumer/cn/doc/promotion/marketing-api-advertising-campaign2-0000001286335642>
- Advertiser report: <https://developer.huawei.com/consumer/cn/doc/promotion/marketing-api-data-advertisers-0000001174597587>
- Error codes: <https://developer.huawei.com/consumer/cn/doc/promotion/marketing-api-appendix3-0000001128438076>

The listed official routes and live regional/OAuth origins were re-verified on
2026-08-25. The documentation frontend was current but blocked direct document
body retrieval from the validation environment, so unverified field-level
assumptions and quota figures were not changed. This adapter implements the
documented REST wire format directly.

## Implemented surface

| Workflow | Endpoint | Notes |
|---|---|---|
| `ListAccounts` | `GET /ads/v1/account/profile/query` | Token-linked advertiser accounts; accepts both the documented array shape and Huawei's object example |
| `ListCampaigns` | `GET /ads/v1/promotion/campaign/query` | Page size 10-50; Huawei requires a JSON body on this GET |
| `AdvertiserReport` | `POST /openapi/v2/reports/advertiser/query` | Advertiser-level delivery data |
| `CampaignReport` | `POST /openapi/v2/reports/campaign/query` | Campaign filters and ordering |
| `AdGroupReport` | `POST /openapi/v2/reports/adgroup/query` | Campaign, Ad Group, app, placement, and pricing filters |
| `CreativeReport` | `POST /openapi/v2/reports/creative/query` | Campaign, Ad Group, Creative, placement, and pricing filters |
| `CountryReport` | `POST /openapi/v2/reports/country/query` | Country rows with optional Campaign, Ad Group, or Creative filter mode |

Report columns vary with level, dimensions, and the evolving conversion metric
set. `ReportRow` therefore maps bounded column names to `ReportValue`.
`ReportValue` preserves strings, exact JSON numbers, and null without converting
money or ratios through `float64`.

## Regions and configuration

| `region` | Official API origin |
|---|---|
| `asia-africa-latin-america` | `https://ads-dra.cloud.huawei.com` |
| `europe` | `https://ads-dre.cloud.huawei.com` |
| `russia` | `https://ads-drru.cloud.huawei.ru` |

An API client always resolves its token from `access_token_ref`. For first-time
authorization, an OAuth-only account may omit `access_token_ref`, exchange the
authorization code, persist the resulting credentials, and then open a new
adapter with an access-token reference. Configure `client_id` and `secret_ref`
together only when the OAuth helper is needed.
`advertiser_id` is optional for a directly authorized single advertiser, but
Huawei requires it for manager/service-provider identities and identities tied
to multiple advertisers.

```yaml
version: 1
platforms:
  - adapter: huawei/petal-ads-marketing-api-v1
    product: marketing-api
    accounts:
      - id: petal-eu-advertiser
        client_id: "12345678"
        secret_ref: env://PETAL_ADS_CLIENT_SECRET
        access_token_ref: env://PETAL_ADS_ACCESS_TOKEN
        settings:
          region: europe
          advertiser_id: "533350928594526848"
        approval:
          scopes:
            - https://www.huawei.com/auth/account/base.profile
            - https://ads.cloud.huawei.com/report
            - https://ads.cloud.huawei.com/promotion
            - https://ads.cloud.huawei.com/tools
            - https://ads.cloud.huawei.com/account
            - https://ads.cloud.huawei.com/finance
```

Huawei documents all six scopes above as the fixed authorization set. The
OAuth helper includes them, sets `access_type=offline`, requires caller state,
supports authorization-code exchange and refresh, and retains the existing
Refresh Token when a refresh response omits it. Authorization codes are
single-use and expire after approximately five minutes; the current Huawei
documentation describes Refresh Tokens as valid for approximately six months,
subject to change.

## Contract and security boundaries

- Mainland China is not a selectable region. This package uses only the three
  overseas origins and hard-codes report `is_abroad` to `true`.
- API and OAuth origins are fixed. Adapter-level endpoint settings are rejected
  so access tokens, authorization codes, Refresh Tokens, and client secrets
  cannot be redirected to caller-selected hosts.
- Huawei app access, the relevant Marketing API capabilities, and advertiser
  authorization are external prerequisites. Missing approval is returned as a
  user-action error rather than silently degrading to another product.
- API calls are limited to 600 requests per minute and 360,000 requests per day
  per account. `DefaultQuotaPolicy` exposes these values to a shared limiter.
- HTTP 2xx is not sufficient for success. The adapter checks Huawei business
  codes, classifies expired tokens, permissions, parameter errors, system
  failures, and rate limits, and preserves bounded platform/request metadata.
- OAuth redirects and API redirects are rejected and cloned HTTP clients discard
  Cookie Jars. OAuth callback URLs require HTTPS except for `localhost` or an
  explicit loopback IP. Provider free-form error text is not exposed; errors retain only
  numeric Huawei codes, HTTP status, bounded `DGW.id`/`oauth_trace_id` request
  identifiers that do not contain configured credentials/account identifiers,
  and deterministic `Retry-After` values.
- Per-call request IDs, idempotency keys, and field selection are rejected
  because this surface has no documented wire contract for them. Only
  `socialhub.WithCallTimeout` is forwarded after CallOption normalization.
- Request JSON is limited to 1 MiB. Dynamic report pages are limited to 10,000
  rows, 512 scalar fields per row, 128-byte keys, and 4 KiB scalar values.

The initial adapter is read-only. Campaign mutation, creative asset upload,
finance, and tools endpoints should be added as typed workflows rather than
being presented through the common organic-social interfaces.
