# Xiaomi Global Reporting API adapter

Adapter name: `xiaomi/global-reporting-api`

This package implements the official Xiaomi Global Mi Ads Reporting API:

- `POST /foreign/data/queryData` for daily Effect or Brand delivery reports;
- `POST /foreign/data/queryDataName` for account, campaign, ad group, and ad
  creative names;
- `POST /foreign/token/createToken` and `POST /foreign/token/refreshToken`
  through `Adapter.Tokens`.

It is deliberately read-only apart from credential creation and rotation. The
separate Marketing API, campaign creation, delivery changes, creative upload,
and every other advertising mutation are outside this adapter. All requests
use the fixed production origin `https://global.e.mi.com`.

The contract was verified on 2026-08-26 against the current Reporting API
Access Guide, whose server copy was last modified on 2026-06-17.

## Access and configuration

Contact the Xiaomi Global Account Manager (AM) with the required account IDs
and names. After approval, Xiaomi supplies an `appId` and `appKey`. The token
and every account ID used by a query must belong to that approved external
application.

Report requests authenticate with three Cookie values:

- `access_token`: the current Reporting API token;
- `timestamp`: the current call time, different on every request;
- `uid`: a unique, cryptographically random string for every request.

Xiaomi documents `timestamp` only as a `Long`; the official guide still does
not identify its epoch unit. `settings.timestamp_unit` is therefore mandatory.
Confirm the unit with the account's AM and choose `unix_seconds` or
`unix_milliseconds`. The adapter makes values monotonically unique across all
clients created by one initialized adapter.

```yaml
version: 1
platforms:
  - adapter: xiaomi/global-reporting-api
    product: global-reporting-api
    settings:
      timestamp_unit: unix_milliseconds # Confirm with the Xiaomi AM.
    accounts:
      - id: xiaomi-global-agency
        app_id: "issued-app-id"          # Optional with secret_ref; needed for Tokens.
        secret_ref: env://XIAOMI_APP_KEY
        access_token_ref: env://XIAOMI_ACCESS_TOKEN
        settings:
          account_ids: [65, 307]
```

`access_token_ref` and at least one `account.settings.account_ids` entry are
required. `app_id` and `secret_ref` are optional but must be configured
together. `client_id`, `token_store`, webhooks, and custom API origins are
rejected because they are not part of this product contract.

## Report reads

```go
func readReport(ctx context.Context, adapter socialhub.Adapter, accountID socialhub.AccountID) error {
	base, err := adapter.Client(ctx, accountID)
	if err != nil {
		return err
	}
	client := base.(*xiaomiglobalreporting.Client)

	page, err := client.Reports().Query(ctx, xiaomiglobalreporting.ReportQuery{
		AdType: xiaomiglobalreporting.AdTypeEffect,
		Dimensions: []xiaomiglobalreporting.Dimension{
			xiaomiglobalreporting.DimensionCampaign,
			xiaomiglobalreporting.DimensionDate,
		},
		Begin: "2026-08-10", End: "2026-08-16",
		Language: xiaomiglobalreporting.LanguageEnglish,
		Page: 1, PageSize: 1000,
	})
	if err != nil {
		return err
	}
	fmt.Println(page.Total, page.RequestUID)
	return nil
}
```

An empty query `AccountIDs` list expands to the configured whitelist; it never
means every account visible to the token. Explicit IDs must be a subset of the
whitelist, and response rows and name records are checked against the same
boundary. Report dimensions must contain `DimensionDate` and at least one
reporting dimension.

Dates are encoded as complete UTC days (`00:00:00.000Z` through
`23:59:59.999Z`). A request may span at most seven calendar days. Hourly
reports are not supported. Dynamic report values preserve exact JSON and never
pass through `float64`.

## Token lifecycle

```go
typed := adapter.(*xiaomiglobalreporting.Adapter)
tokens, err := typed.Tokens(ctx, "xiaomi-global-agency")
if err != nil {
	return err
}
bundle, err := tokens.Create(ctx)
// Persist bundle.Token and bundle.RefreshExpiresAt in an encrypted store.
```

The helper returns credentials but never persists them. `Client`,
`TokenClient`, and `TokenBundle` string formatting is redacted. Redirects are
disabled and the cloned HTTP client's Cookie jar is removed, preventing Cookie
credentials or token bodies from crossing origins. Credential resolver and
transport errors are filtered before they are exposed.

Only `socialhub.WithCallTimeout` is accepted per call. Xiaomi does not document
caller request-ID, idempotency, or field-selection controls, so those options
are rejected. Successful responses must be HTTP 200 JSON envelopes with
business code `0`. Observed `traceId`, request, and correlation identifiers
are bounded, control-character checked, and credential-filtered before use in
errors. Codes `20002` and `100500`, HTTP 429, and HTTP 5xx are retryable; the
adapter never retries automatically.

## Limits

- `queryData` calls must be spaced strictly more than 2 seconds apart;
- `queryDataName` calls must be spaced strictly more than 0.3 seconds apart;
- one report request covers no more than seven UTC days and returns at most
  1000 rows per page;
- one name request supports at most 8000 IDs at each campaign, ad group, and
  ad creative level;
- daily data uses UTC+00:00 and hourly granularity is unavailable.

`DefaultQuotaPolicy` exposes the two minimum intervals for a shared limiter.
The adapter does not sleep or keep process-local quota state; applications
should coordinate limits across replicas that share one Xiaomi external app.

## Official sources

Official material reviewed on 2026-08-26:

- <https://global.e.mi.com/doc/reporting_api_guide.html>
- <https://global.e.mi.com/>

The official responses had these SHA-256 digests:

| Reference | HTTP | SHA-256 |
|---|---:|---|
| Reporting API Access Guide | 200 | `84F4D18D3872891BA44568A75D3E2838472F30633596FD4527E157D9A4652743` |
| Xiaomi Global portal | 200 | `5E2D94A2B64465137FDA99C82C5C05BBC7C7FF50CB5D368686E8E10A7BFE0AD7` |
| Empty `queryData` response | 200 | `73EA44CB635268523C037E767CBD3206621C56F91F7386A3D99A8E493EB46ADB` |
| Empty `createToken` response | 200 | `4548A46057A9773EC3313A892552E7EF730C8B4D22B2FB4504796DE1C0FD1C6F` |
| Empty `refreshToken` response | 200 | `F90E3D806B1637E984EAD6FCA3A4163107D7DC6E2E4D640376A418CD252262A0` |

The three empty production requests returned `application/json`, business code
`20001`, and a per-request `traceId`; their body digests are therefore
point-in-time evidence. No app ID, app key, access token, refresh token, or
account ID was supplied. No credentialed production request was sent, no token
was issued, and no advertising data was returned.
