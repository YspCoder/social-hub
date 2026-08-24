# Huawei Jinghong Momentum Marketing API adapter

`adapters/jinghongads` implements the Huawei Jinghong Momentum (鲸鸿动能)
Marketing API for advertiser accounts in the Chinese mainland. It is
intentionally isolated from the overseas Petal Ads adapter and uses only the
mainland business origin `https://ads.cloud.huawei.com`.

Official references:

- Overview and limits: <https://developer.huawei.com/consumer/cn/doc/promotion/ads_api02-0000001058566534>
- OAuth authorization-code exchange: <https://developer.huawei.com/consumer/cn/doc/promotion/ads_api09-0000001058204667>
- OAuth token refresh: <https://developer.huawei.com/consumer/cn/doc/promotion/ads-shuaxinacces-0000001873268749>
- Linked advertiser accounts: <https://developer.huawei.com/consumer/cn/doc/promotion/ads-glzhlb-0000002525138850>
- Campaign query: <https://developer.huawei.com/consumer/cn/doc/promotion/ads_new_api04-0000001405200520>
- Advertiser report: <https://developer.huawei.com/consumer/cn/doc/promotion/ads_api66-0000001059044298>
- Campaign report: <https://developer.huawei.com/consumer/cn/doc/promotion/ads_api67-0000001057938581>
- Ad Group report: <https://developer.huawei.com/consumer/cn/doc/promotion/ads_api68-0000001059204284>
- Creative report: <https://developer.huawei.com/consumer/cn/doc/promotion/ads_api69-0000001058204677>
- Error codes: <https://developer.huawei.com/consumer/cn/doc/promotion/ads_api80-0000001436816510>

The official mainland contract was reviewed on 2026-08-25. GitHub searches for
Jinghong Momentum, Huawei Marketing API, and Huawei Ads Marketing API Go
clients did not find a maintained, broadly adopted SDK for this REST contract,
so this adapter implements the official wire format directly.

## Implemented surface

| Workflow | Endpoint | Notes |
|---|---|---|
| `ListAccounts` | `GET /openapi/v1/account/profile/query` | Advertiser accounts linked to the authorized identity |
| `ListCampaigns` | `GET /ads/v1/promotion/campaign/query` | Mainland new-delivery contract; Huawei requires a JSON body on this GET |
| `AdvertiserReport` | `POST /openapi/v2/reports/advertiser/query` | Metric filter types 1-3 |
| `CampaignReport` | `POST /openapi/v2/reports/campaign/query` | Metric filter types 4-6 |
| `AdGroupReport` | `POST /openapi/v2/reports/adgroup/query` | Metric filter types 7-9 |
| `CreativeReport` | `POST /openapi/v2/reports/creative/query` | Metric filter types 10-12 |

`MetricFilter` exposes semantic greater-than-or-equal, less-than-or-equal, and
between modes. The adapter maps those modes to Huawei's report-level wire
values. Report columns vary by level and dimensions, so `ReportRow` uses
bounded dynamic keys and `ReportValue` preserves exact JSON numbers, strings,
and null without converting money or ratios through `float64`.

Huawei's current Creative report response table names the total field
`total_number`, while its example uses `total_num`. The response decoder accepts
either shape and rejects responses that contain conflicting values.

## Prerequisites and configuration

Huawei requires enterprise real-name verification, a Marketing API application
and review, and advertiser authorization before API access. These are external
prerequisites; this adapter reports missing scope or authorization as a
user-action error and does not fall back to the overseas product.

An access token is always resolved from `access_token_ref`. Configure
`client_id` and `secret_ref` together only when the OAuth helper is needed.
Reporting requires `account.settings.advertiser_id`; account discovery can be
used to obtain the authorized advertiser IDs.

```yaml
version: 1
platforms:
  - adapter: huawei/jinghong-marketing-api-v1
    product: marketing-api
    accounts:
      - id: jinghong-mainland-advertiser
        client_id: "12345678"
        secret_ref: env://JINGHONG_CLIENT_SECRET
        access_token_ref: env://JINGHONG_ACCESS_TOKEN
        settings:
          advertiser_id: "662000616"
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
Refresh Token when a refresh response omits it. Callback URLs must be HTTPS,
must not include query or fragment components, and may not exceed 255 bytes.

## Contract and security boundaries

- The API and OAuth origins are fixed to Huawei's official mainland HTTPS
  endpoints. Adapter-level endpoint overrides are rejected.
- Mainland report requests do not send the Petal Ads `is_abroad` field.
- API calls are limited to 600 requests per minute and 360,000 requests per day
  per authorized account. `DefaultQuotaPolicy` exposes these values to a shared
  limiter.
- HTTP 2xx is not sufficient for success. The adapter checks Huawei business
  codes and preserves bounded platform and request metadata in common errors.
- OAuth redirects and API redirects are rejected so credentials and Bearer
  tokens cannot be forwarded to another origin. OAuth error text is redacted.
- Request JSON is limited to 1 MiB. Dynamic report pages are limited to 10,000
  rows, 512 scalar fields per row, 128-byte keys, and 4 KiB scalar values.

The initial adapter is read-only. Campaign mutation, creative asset upload,
finance, and tools endpoints should be added as typed workflows rather than
being presented through the common organic-social interfaces.
