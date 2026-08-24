# Kakao Moment Open API v4 adapter

Adapter name: `kakao/moment-open-api-v4`

This package implements account-bound Kakao Moment management and reporting:

- ad-account detail and real-time balance;
- Campaign list/detail, ON-first creation followed immediately by OFF, common
  editing, Display daily budget, status, and guarded deletion;
- Ad Group list/detail, Display daily budget, manual bid, status, and guarded
  deletion;
- Creative list/detail, supported Display Creative status changes, and guarded
  deletion;
- synchronous Ad Account, Campaign, Ad Group, and Creative reports with exact
  dynamic JSON values;
- account-bound Kakao Business Authentication authorization-code exchange.

Ad Account creation, Ad Group creation and complex targeting, Creative
creation, asset upload, Message delivery, Audience, Customer File, Pixel,
BizForm, Ad View, and token-information/revocation APIs are intentionally
outside this initial surface. Kakao Login and Kakao Talk are implemented by the
separate `kakao/login-talk-rest` adapter.

## Access and Business Authentication

Kakao Moment requires a Kakao Business account, a Biz app, a registered
Business redirect URI, Moment API permission, user authorization, and rights
to each selected ad account. Requests use:

```text
Authorization: Bearer ${BUSINESS_ACCESS_TOKEN}
adAccountId: ${AD_ACCOUNT_ID}
```

The adapter supports a referenced static Business access token. Kakao Business
Authentication does not issue a refresh token. A token that expires after
long-term inactivity or is revoked must be authorized again and replaced in
the configured secret backend.

This adapter implements the current Business Authentication Bearer contract.
A Kakao REST API key or legacy `KakaoAK` value is not a Business access token
and must not be supplied as `access_token_ref`.

The supported consent items are:

- `moment_management` for reads and management;
- `moment_delete` in addition to `moment_management` for guarded deletes.

The OAuth helper always limits `resource_ids` to the configured
`moment:${AD_ACCOUNT_ID}`. It does not request `moment_create`, because Ad
Account creation is not implemented.

## Configuration

```yaml
version: 1
platforms:
  - adapter: kakao/moment-open-api-v4
    product: moment-open-api
    accounts:
      - id: kakao-moment-kr
        client_id: your-kakao-rest-api-key
        secret_ref: env://KAKAO_BUSINESS_CLIENT_SECRET
        access_token_ref: env://KAKAO_BUSINESS_ACCESS_TOKEN
        approval:
          scopes: [moment_management, moment_delete]
        settings:
          ad_account_id: 123456
```

`client_id` and `secret_ref` are needed only when using `Adapter.OAuth`.
Kakao enables the REST API Client secret by default, but an app may disable it;
omit `secret_ref` only when that setting is intentionally disabled.
For first-time authorization, an OAuth-only account may omit
`access_token_ref`; after exchange, persist the returned token reference and
open a new adapter before calling `Adapter.Client`. `access_token_ref` remains
required for API clients. `token_store`, `app_id`, `approval.account_type`,
adapter-level settings, and webhook configuration are rejected because they
do not belong to this contract.

The API and Business Authentication origins are fixed to:

```text
https://apis.moment.kakao.com/openapi/v4
https://kauth.kakao.com/oauth/business/authorize
https://kauth.kakao.com/oauth/business/token
```

```go
package main

import (
	"context"

	"social-hub/adapters/kakaomoment"
	"social-hub/pkg/socialhub"
)

func createCampaign(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "kakao/moment-open-api-v4", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "kakao-moment-kr")
	if err != nil {
		return err
	}
	client := base.(*kakaomoment.Client)

	budget := int64(100_000)
	campaign, err := client.Campaigns().CreateCampaignThenPause(ctx, kakaomoment.CampaignCreate{
		Name: "social-hub display",
		CampaignTypeGoal: kakaomoment.CampaignTypeGoal{
			CampaignType: "DISPLAY",
			Goal:         "VISITING",
		},
		DailyBudgetAmount: &budget,
	})
	if err != nil {
		return err
	}

	// Delivery remains disabled until an explicit ON request.
	return client.Campaigns().SetCampaignConfig(ctx, campaign.ID, kakaomoment.ConfigOn)
}
```

## ON-first creation and mutation safety

Kakao's Campaign create contract has no OFF request field and returns a new
Campaign as `ON`. `CreateCampaignThenPause` therefore creates an empty Campaign
and immediately calls `campaigns/onOff` with `OFF`. This is not atomic. If the
create result is ambiguous, the adapter returns `ErrOutcomeUnknown`. If the
Campaign ID is known but OFF cannot be confirmed, it returns the Campaign
together with `ErrReconciliationRequired`; reconcile that ID and disable it
before creating any child resources.

Kakao does not document a generic idempotency-key contract, so the adapter
rejects `socialhub.WithIdempotencyKey`. Caller request IDs and field selection
are also rejected; only `socialhub.WithCallTimeout` is accepted.

Delete methods first retrieve the resource and reject it unless `config` is
`OFF`. This is an SDK safety guard, not a provider-side atomic precondition.
Campaign deletion cascades to all child Ad Groups and Creatives, and Ad Group
deletion cascades to all child Creatives. Concurrent external changes can race
with the preflight, so callers must still reconcile destructive workflows.

Some Kakao Moment failures, notably Creative mutations, are returned as HTTP
200 with a JSON error envelope. The adapter inspects every non-empty success
body before accepting a mutation. Transport failures, 5xx responses, or
malformed success bodies that leave write state ambiguous return
`ErrOutcomeUnknown`.

Provider-supplied free-form error messages and OAuth `error_description` values
are not exposed. Errors retain bounded numeric Kakao codes, HTTP status, safe
request IDs, and `Retry-After` while returning a fixed local message.

## Reports

Reports accept one predefined `DatePreset`, an explicit `start`/`end` range of
at most 31 days, or neither to use Kakao's `TODAY` default. Campaign reports
accept up to 5 IDs, Ad Group reports up to 40, and Creative reports up to 100.
One client remains bound to one configured Ad Account; cross-account aggregate
reports are intentionally excluded even though the provider endpoint can
accept up to five accounts.

`ReportValue` preserves every dynamic dimension and metric as raw JSON. Use
`Bytes`, `String`, or `Decode` to consume it without an intermediate `float64`
conversion.

## Limits and operations

- Ad Account detail: one call per second per user account and app ID.
- Campaign daily-budget and status changes: one call every five seconds per
  user account and Ad Account.
- Ad Group daily-budget, manual-bid, and status changes: one call per second;
  status is additionally keyed by Ad Group.
- Creative status changes: one call per second per user account and Ad Account.
- Ad Account, Campaign, and Creative reports: one call every five seconds per
  documented account/app key; Ad Group reports: one call per second.

Coordinate rate limiting across processes and treat provider responses as
authoritative because account and app approval can impose narrower limits.
No real advertiser credentials were used for local verification.

## Official sources

- [Kakao Moment overview](https://developers.kakao.com/docs/en/kakaomoment/common)
- [Ad Account API](https://developers.kakao.com/docs/en/kakaomoment/ad-account)
- [Campaign API](https://developers.kakao.com/docs/en/kakaomoment/campaign)
- [Ad Group API](https://developers.kakao.com/docs/en/kakaomoment/ad-group)
- [Creative API](https://developers.kakao.com/docs/en/kakaomoment/creatives)
- [Report API](https://developers.kakao.com/docs/en/kakaomoment/report)
- [Error codes](https://developers.kakao.com/docs/en/kakaomoment/error-code)
- [Business Authentication REST API](https://developers.kakao.com/docs/en/business-auth/rest-api)

The official contract was last directly reviewed on 2026-08-24 against the
current v4 references. On 2026-08-25, the official documentation host was
unavailable from the validation environment, so that date is not recorded as
a new verification. The official REST documentation remains the protocol
authority. No mature third-party Go client was adopted.
