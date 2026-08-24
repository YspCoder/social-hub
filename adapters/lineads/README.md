# LINE Ads API v3 adapter

Adapter name: `line/ads-api-v3`

This package implements the current restricted LINE Ads API v3 read surface
verified against specification `3.12.3` (released 2026-08-05):

- ad accounts authorized by one API-enabled Group;
- Campaign list and filtering;
- asynchronous performance-report metadata list;
- synchronous JSON online reports at Campaign, Ad Group, or Ad level.

Campaign creation, status changes, Ad Group, Ad, and Media mutations, report
creation/deletion/download, link requests, and group administration are
deliberately excluded. The initial adapter is safe for inventory and reporting
discovery and never changes delivery state.

## Access status and account lifecycle

LINE Ads API is not a generally self-service public API. The current official
documentation publishes three partner contracts:

- `data-general-partner` (Data Provider (General));
- `certificated-ad-tech-general-partner` (Ad Tech (General));
- `reporting-general-partner` (Reporting (General)).

LY Corporation must invite the user into an API-enabled Group. Access and
Secret keys are then obtained from that Group's settings in Ad Manager. A Data
Provider (General) entitlement can use `ListAdAccounts`, but the adapter blocks
Campaign and report calls locally because those resources are absent from that
partner's published specification. Reporting (General) and Ad Tech (General)
entitlements can use all four workflows.

The official Japanese LINE Ads service page states that new LINE Ads account
applications stopped on 2026-06-30 as the product is integrated into LINE
Yahoo Ads. This adapter therefore targets existing authorized LINE Ads Groups;
it does not provide an account-opening path or imply that a new API entitlement
can be obtained.

The API response exposes each Ad Account's `country`, and country-specific
features are enforced by the provider. The current specification contains
explicit JP/TW restrictions, but does not publish one universal list of
countries in which a new API partnership can be requested. The adapter does
not invent a country allowlist: use the returned Ad Account country and the
entitlement granted by LY Corporation as authoritative. Availability outside
Japan must be confirmed through the relevant local LINE/LY business channel.

## Authentication

This API does not use OAuth and has no token refresh. Every request is signed
with the Group's static Access key and Secret key using LINE's JWS-based HS256
scheme:

```text
JOSE header = {"alg":"HS256","kid":"<Access key>","typ":"text/plain"}
payload = SHA256(body) + "\n" + content-type + "\n" + YYYYMMDD + "\n" + canonical-uri
token = base64url(header) + "." + base64url(payload) + "." +
        base64url(HMAC-SHA256(secret-key, encoded-header + "." + encoded-payload))
Authorization: Bearer <token>
Date: <RFC1123 GMT timestamp>
```

URL-safe Base64 retains padding, matching the official sample. These workflows
have no body, so the digest is SHA-256 of the empty byte sequence and the
content-type payload line is empty. The canonical URI contains the path
(`/api/v3/...`) but not the query string. `Date` must be within 15 minutes of
the provider clock, so production hosts need synchronized system time.

Rotate a Secret key in Ad Manager and update its secret reference when it is
regenerated. The API origin is fixed to `https://ads.line.me/api`; adapter-level
settings cannot override it. Redirect following and the configured HTTP
client's Cookie Jar are disabled because credentials and signatures must not
cross the official origin boundary.

## Configuration

`client_id` stores the non-secret Access key, while `secret_ref` resolves the
Secret key at runtime. `group_id` is the API-enabled Group, not an Ad Account
ID. An `AdAccountID` passed to Campaign or report workflows must come from
`ListAdAccounts` for that configured Group. LINE Ads remains the final
authorization boundary and can reject an account with HTTP 403.

```yaml
version: 1
platforms:
  - adapter: line/ads-api-v3
    product: ads-api
    accounts:
      - id: line-ads-existing-group
        client_id: your-line-ads-access-key
        secret_ref: env://LINE_ADS_SECRET_KEY
        approval:
          account_type: reporting-general-partner
        settings:
          group_id: your-api-enabled-group-id
```

`app_id`, `access_token_ref`, `token_store`, webhook fields, and OAuth scopes
are rejected because they are not part of this authentication contract.
Per-call configuration supports only `socialhub.WithCallTimeout`; caller
request IDs, idempotency keys, and generic field selection are rejected.

```go
package main

import (
	"context"

	"social-hub/adapters/lineads"
	"social-hub/pkg/socialhub"
)

func readCampaignReport(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "line/ads-api-v3", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "line-ads-existing-group")
	if err != nil {
		return err
	}
	client := base.(*lineads.Client)

	result, err := client.Management().GetOnlineReport(ctx, lineads.GetOnlineReportRequest{
		AdAccountID: "authorized-ad-account-id",
		Level:       lineads.ReportLevelCampaign,
		Since:       "2026-08-01",
		Until:       "2026-08-07",
		Size:        100,
	})
	if err != nil {
		return err
	}

	for _, row := range result.Rows {
		_ = row.Statistics["impression"].String()
	}
	return nil
}
```

Provider IDs and monetary micros use `ExactValue`; dynamic online-report
statistics use `RawValue`. Both preserve JSON without intermediate `float64`
coercion. Each resource and response also retains its complete provider JSON
in `Raw` so newly added fields remain available before the typed model catches
up.

## Limits and failure handling

- Official rate limit: 2 requests per second for each API user.
- Online Report Read request quota: at most 30 simultaneous requests for each
  API user and API; the provider recommends about 20 concurrent threads for
  calls taking three seconds.
- Successful and failed responses expose `X-Request-Quota-Limit` and
  `X-Request-Quota-Used` when the request-quota contract applies.
- HTTP 429 maps to a retryable `socialhub.CodeRateLimited` error. HTTP 403 maps
  to `socialhub.CodeApprovalRequired` because the official contract describes
  it as missing API or Ad Account authorization.
- Provider `errors[].reason`, `errors[].property`, and non-JSON response text
  are not exposed through public errors because those free-text fields may
  reflect account data or request values. `APIError` exposes the HTTP-derived
  platform code and sanitized quota headers only.

The 2 request/second limit must be coordinated across all processes sharing an
API user. `social-hub` does not treat the per-response quota headers as a
distributed limiter.

## Official sources

- [LINE Ads API documentation](https://ads.line.me/public-docs/)
- [Current version registry](https://ads.line.me/public-docs/meta.js)
- [Reporting (General) specification 3.12.3](https://ads.line.me/public-docs/pages/v3/3.12.3/reporting-general-partner/)
- [Ad Tech (General) specification 3.12.3](https://ads.line.me/public-docs/pages/v3/3.12.3/certificated-ad-tech-general-partner/)
- [Data Provider (General) specification 3.12.3](https://ads.line.me/public-docs/pages/v3/3.12.3/data-general-partner/)
- [Official Japanese LINE Ads service lifecycle page](https://www.lycbiz.com/jp/service/line-ads/)

The official sources above were reviewed on 2026-08-25. No third-party GitHub
client was adopted; the provider's live specification is the protocol
contract, and this package adds no dependency.
