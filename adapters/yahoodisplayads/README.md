# LINE Yahoo Display Ads API v20 adapter

Adapter name: `line-yahoo/display-ads-api-v20`

This package implements advertiser-bound Auction Display Ads workflows:

- Campaign reads, paused-first CPC creation, bounded updates, explicit
  enablement, and guarded deletion;
- Ad Group reads and paused-first CPC batch management;
- image Banner Ad management using existing LINE Yahoo Media IDs;
- asynchronous standard AD Report jobs with uncompressed CSV, TSV, or XML
  downloads;
- OAuth 2.0 authorization-code exchange and refresh-token rotation.

Guaranteed Campaigns, Guaranteed Ad Groups and Ads, Media and Video upload,
Responsive, Carousel, Dynamic, and LINE Official Account friend ads, Feeds,
Audiences, Targeting, Stats, Conversions, and Account Administration are
intentionally outside this initial adapter surface. Organic LINE or Yahoo
content is also outside this advertising adapter.

## Access and OAuth

Register an application for LINE Yahoo Ads API access and obtain advertiser or
MCC authorization. The OAuth authorization request uses the `yahooads` scope:

```text
https://biz-oauth.yahoo.co.jp/oauth/v1/authorize
```

The adapter supports either a referenced static Bearer access token or managed
OAuth 2.0 refresh. For the initial grant, call `Adapter.OAuth`, generate an
authorization URL with `AuthorizationURL`, and exchange the returned code with
`Exchange`. Store the resulting refresh token in the configured secret backend.
Managed clients refresh through the official token endpoint and preserve token
rotation in the configured `socialhub.TokenStore`.

The API and OAuth origins are fixed to the official LINE Yahoo HTTPS endpoints.
Adapter-level endpoint overrides are rejected; inject a controlled `http.Client`
transport when request observation is required without changing credential
destinations.

Redirects are rejected so authorization and account headers cannot cross
origins. Access tokens, client secrets, refresh tokens, authorization codes,
and sensitive provider messages are bounded and redacted from returned errors.

## Configuration

Static access-token configuration:

```yaml
version: 1
platforms:
  - adapter: line-yahoo/display-ads-api-v20
    product: display-ads-api
    accounts:
      - id: yahoo-display-jp
        access_token_ref: env://LINE_YAHOO_ADS_ACCESS_TOKEN
        approval:
          scopes: [yahooads]
        settings:
          account_id: 123456789
```

Managed OAuth refresh configuration:

```yaml
version: 1
platforms:
  - adapter: line-yahoo/display-ads-api-v20
    product: display-ads-api
    accounts:
      - id: yahoo-display-mcc-client-a
        client_id: your-oauth-client-id
        secret_ref: env://LINE_YAHOO_ADS_CLIENT_SECRET
        approval:
          scopes: [yahooads]
        settings:
          account_id: 123456789
          base_account_id: 987654321
          refresh_token_ref: env://LINE_YAHOO_ADS_REFRESH_TOKEN
```

`account_id` is always the advertiser account placed in API request bodies.
`base_account_id` is independently sent as `x-z-base-account-id` and defaults
to `account_id`. Set it to an MCC account only when acting for an authorized
child advertiser. An MCC header never replaces the child advertiser's
body-level `accountId`. One social-hub account maps to exactly one such pair.

Do not configure both `access_token_ref` and `refresh_token_ref`. Managed OAuth
requires `client_id`, `secret_ref`, and `settings.refresh_token_ref`. `app_id`
and webhook configuration are rejected because they are not part of this API
contract.

## Paused-first management

```go
package main

import (
	"context"

	"social-hub/adapters/yahoodisplayads"
	"social-hub/pkg/socialhub"
)

func createPausedCampaign(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "line-yahoo/display-ads-api-v20", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "yahoo-display-jp")
	if err != nil {
		return err
	}
	client := base.(*yahoodisplayads.Client)

	campaign, _, err := client.Campaigns().CreateCampaign(ctx, yahoodisplayads.CampaignAdd{
		Name:         "social-hub display",
		Goal:         yahoodisplayads.CampaignGoalWebsiteTraffic,
		BudgetAmount: 10_000,
		CPC:          100,
		StartDate:    "20260825",
	})
	if err != nil {
		return err
	}

	// Creation is always PAUSED. Delivery requires a separate explicit call.
	_, err = client.Campaigns().SetCampaignsEnabled(ctx, []int64{campaign.CampaignID}, true)
	return err
}
```

Campaign goals are account-authority values returned by LINE Yahoo rather than
a closed global enum. `CampaignGoalWebsiteTraffic` is supplied for the common
`WEBSITE_TRAFFIC` goal; other uppercase underscore-delimited authority values
can be passed after confirming account eligibility.

Campaigns, Ad Groups, and Banner Ads are created as `PAUSED`; callers must
explicitly enable each level. Delete methods read the selected resources first
and reject deletion unless they are paused. Parent Campaign and Ad Group
boundaries are verified for child operations.

Banner creation is intentionally limited to `BANNER_AD` with
`mainMediaFormat=IMAGE`. It references an existing positive `mediaId`; the
adapter does not upload, replace, or inspect Media assets.

Mutations retain every provider result in `MutationResult.Items`. A mixed batch
returns the result with `ErrPartialMutation`. A transport failure, server error,
malformed success envelope, or missing result that leaves write state ambiguous
returns `ErrOutcomeUnknown`; reconcile by provider ID and advertiser state
before retrying. LINE Yahoo does not expose a generic idempotency-key contract.

Only `socialhub.WithCallTimeout` is accepted for individual calls. Caller
request IDs, generic field selection, and idempotency keys are rejected.

## Reports

`CreateReport` creates one asynchronous standard AD definition using `NONE`
compression, `UTF8` encoding, and `EN` language. Deleted entities are included
by default, matching LINE Yahoo; set `ExcludeDeleted` to emit
`reportIncludeDeleted=FALSE`. Poll `GetReport` until its status is `COMPLETED`,
then call `DownloadReport`. Failed, canceled, and unfinished jobs are never
downloaded. Standard AD reports deliberately omit `reportTypeCondition`, as
required by the v20 contract; that condition is reserved for the documented
special report types.

Downloads reject redirects and content encodings, recognize explicit JSON
error envelopes, require the documented `application/octet-stream` HTTP media
type, and stream directly to the supplied writer. The selected CSV, TSV, or XML
format describes the bytes inside that response. Passing `0` as the maximum
uses `DefaultMaxReportBytes`, currently 256 MiB. A positive value sets a
caller-owned bound. ZIP and Excel CSV output are not requested or supported.

## Limits and operations

- Campaign mutation batches accept at most 300 operands; Campaign selectors
  and pages accept at most 2,000 IDs and entries.
- Ad Group and Ad mutation batches accept at most 2,000 operands; their ID
  filters accept at most 1,000 IDs and pages at most 10,000 entries.
- Report mutations accept at most 30 operands and Report selectors/pages at
  most 500 IDs or entries.
- LINE Yahoo documents 5 QPS for Display Ads Account, Ad creation, Proposal,
  and Reporting service groups. Coordinate limiting across processes and treat
  provider responses as authoritative because service/account quotas can vary.
- Provider error `0003` maps to a retryable rate-limit error. If the response
  omits a retry delay, the adapter sets `RetryAfter` to 30 seconds, matching the
  documented minimum wait before retrying.

No real advertiser credentials were used for local verification.

## Official sources

- [Display Ads API v20 developer guide](https://ads-developers.yahoo.co.jp/en/ads-api/developers-guide/display-v20.html)
- [Display Ads API v20 reference](https://ads-developers.yahoo.co.jp/reference/ads-display-api/v20/)
- [Official QPS limits](https://ads-developers.yahoo.co.jp/en/ads-api/developers-guide/qps.html)
- [Official OpenAPI documents](https://github.com/yahoojp-marketing/ads-display-api-documents)
- [Official Java samples](https://github.com/yahoojp-marketing/ads-display-api-java-samples)

The official v20 OpenAPI contract and Java samples were reviewed on
2026-08-25. They are protocol references only; this package does not introduce
a generated-client runtime dependency.
