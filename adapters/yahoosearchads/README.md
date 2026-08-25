# LINE Yahoo Search Ads API v20 adapter

Adapter name: `line-yahoo/search-ads-api-v20`

This package implements advertiser-bound LINE Yahoo Search Ads workflows:

- STANDARD Campaign reads, paused-first manual CPC creation, updates, explicit
  enablement, and guarded deletion;
- Ad Group reads and paused-first CPC batch management;
- biddable Keyword reads and paused-first batch management;
- asynchronous Report jobs with uncompressed CSV, TSV, or XML downloads;
- OAuth 2.0 authorization-code exchange and refresh-token rotation.

Shopping Search Ads (`Ssa*` services), Ads, Creatives, Assets, organic LINE or
Yahoo content, billing, and account administration are intentionally outside
this initial adapter surface.

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
Managed clients refresh through the official token endpoint and preserve a
previous refresh token when a refresh response does not rotate it.

Redirects are rejected so authorization and account headers cannot cross
origins. Access tokens, client secrets, refresh tokens, authorization codes,
account identifiers, and provider free text are not retained in returned
errors. OAuth callbacks require HTTPS, except for HTTP loopback callbacks used
by installed applications.

## Configuration

Static access-token configuration:

```yaml
version: 1
platforms:
  - adapter: line-yahoo/search-ads-api-v20
    product: search-ads-api
    accounts:
      - id: yahoo-search-jp
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
  - adapter: line-yahoo/search-ads-api-v20
    product: search-ads-api
    accounts:
      - id: yahoo-search-mcc-client-a
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
`base_account_id` is the account sent as `x-z-base-account-id`; it defaults to
`account_id`. Set it to the MCC account only when acting for an authorized child
advertiser. This split is deliberate: an MCC header never replaces the child
advertiser's body-level `accountId`. One social-hub account maps to exactly one
such pair.

Do not configure both `access_token_ref` and `refresh_token_ref`. Managed OAuth
requires `client_id`, `secret_ref`, and `settings.refresh_token_ref`. `app_id`
`token_store`, `approval.account_type`, adapter-level endpoint settings, and
webhook configuration are rejected because they are not consumed by this API
contract. API and OAuth requests always use the official HTTPS origins.

## Paused-first management

```go
package main

import (
	"context"

	"social-hub/adapters/yahoosearchads"
	"social-hub/pkg/socialhub"
)

func createPausedCampaign(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "line-yahoo/search-ads-api-v20", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "yahoo-search-jp")
	if err != nil {
		return err
	}
	client := base.(*yahoosearchads.Client)

	campaign, _, err := client.Campaigns().CreateCampaign(ctx, yahoosearchads.CampaignAdd{
		Name:         "social-hub search",
		BudgetAmount: 10_000,
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

Campaign creation is intentionally limited to `STANDARD` Campaigns using the
manual `CPC` strategy. Campaigns, Ad Groups, and Keywords are created as
`PAUSED`; callers must explicitly enable each level. Delete methods read the
selected resources first and reject deletion unless they are paused. Parent
Campaign and Ad Group boundaries are verified for child operations.

Mutations retain every provider result in `MutationResult.Items`. A mixed batch
returns the result with `ErrPartialMutation`. A transport failure, server error,
malformed success envelope, or missing result that leaves write state ambiguous
returns `ErrOutcomeUnknown`; reconcile by provider ID and advertiser state
before retrying. LINE Yahoo does not expose a generic idempotency-key contract.

Only `socialhub.WithCallTimeout` is accepted for individual calls. Caller
request IDs, generic field selection, and idempotency keys are rejected.

## Reports

`CreateReport` creates one asynchronous definition using `NONE` compression,
`UTF8` encoding, and `EN` language. The public surface supports ACCOUNT,
CAMPAIGN, ADGROUP, AD, KEYWORDS, and SEARCH_QUERY reports in CSV, TSV, or XML.
Poll `GetReport` until its status is `COMPLETED`, then call `DownloadReport`.
Failed and unfinished jobs are never downloaded.

Downloads reject redirects and content encodings, recognize explicit JSON
error envelopes, require the official `application/octet-stream` success media
type, and stream directly to the supplied writer. Passing `0` as the maximum
uses `DefaultMaxReportBytes`, currently 256 MiB. A positive value sets a smaller
caller-owned bound. ZIP output is not requested or supported.

## Limits and operations

- Management mutations accept at most 2,000 operands; selector ID filters at
  most 1,000 IDs; ordinary pages at most 10,000 entries.
- Report mutations accept at most 30 operands and Report pages at most 500
  entries.
- LINE Yahoo documents a typical limit of 5 QPS for regular Search Ads
  services. Coordinate limiting across processes and treat provider responses
  as authoritative because service/account quotas may differ.
- Provider error `0003` maps to a retryable rate-limit error. If the response
  omits a retry delay, the adapter sets `RetryAfter` to 30 seconds, matching the
  documented minimum wait before retrying.

No real advertiser credentials were used for local verification.

## Official sources

- [Search Ads API v20 developer guide](https://ads-developers.yahoo.co.jp/en/ads-api/developers-guide/search-v20.html)
- [Search Ads API v20 reference](https://ads-developers.yahoo.co.jp/reference/ads-search-api/v20/)
- [Official QPS limits](https://ads-developers.yahoo.co.jp/en/ads-api/developers-guide/qps.html)
- [Official OpenAPI documents](https://github.com/yahoojp-marketing/ads-search-api-documents)
- [Official Java samples](https://github.com/yahoojp-marketing/ads-search-api-java-samples)

The official v20 OpenAPI contract and Java samples were reviewed on
2026-08-25. They are protocol references only; this package does not introduce
a generated-client runtime dependency.
