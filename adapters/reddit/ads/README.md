# Reddit Ads API v3 adapter

`adapters/reddit/ads` implements account-scoped Reddit Ads API v3 workflows.
It is intentionally separate from `adapters/reddit`, which implements the
organic Reddit Data API.

Registration name: `reddit/ads-api-v3`

## Included workflows

- configured Ad Account read
- Funding Instrument list and exact account-scoped selection
- Campaign list, get, create, and update
- Ad Group list, get, create, and update
- Ad list, get, create from an existing Reddit Post ID, and update
- synchronous, paginated reports with dynamic metric values
- OAuth 2.0 authorization-code exchange and refresh
- endpoint-group `RateLimit-Policy` and `RateLimit` parsing

Post/creative creation, media upload, targeting taxonomy, custom audiences,
catalogs, Conversions API, and deletion are outside this initial contract.

## Configuration

```yaml
version: 1
platforms:
  - adapter: reddit/ads-api-v3
    product: ads-api
    settings:
      user_agent: "linux:com.example.socialhub:v0.1.0 (by /u/example)"
    accounts:
      - id: reddit-paid-main
        client_id: ${REDDIT_CLIENT_ID}
        secret_ref: env://REDDIT_CLIENT_SECRET
        access_token_ref: env://REDDIT_ADS_ACCESS_TOKEN
        approval:
          scopes: [adsread, adsedit]
        settings:
          ad_account_id: a2_123456
```

Credential fields are secret references, not literal secrets. A descriptive
User-Agent is mandatory for API and OAuth requests. Reddit recommends the
format `platform:app-id:version (by /u/username)`.

## Typed usage

```go
package main

import (
	"context"

	redditads "social-hub/adapters/reddit/ads"
	"social-hub/pkg/socialhub"
)

func createCampaign(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "reddit/ads-api-v3", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	common, err := adapter.Client(ctx, "reddit-paid-main")
	if err != nil {
		return err
	}
	client := common.(*redditads.Client)

	_, err = client.Campaigns().CreateCampaign(ctx, redditads.CreateCampaignRequest{
		FundingInstrumentID: "604212",
		Name:                "Launch campaign",
		Objective:           redditads.ObjectiveClicks,
	})
	return err
}
```

Every Campaign, Ad Group, and Ad create call sends
`configured_status: "PAUSED"` and rejects a response that does not preserve
that state. Mutations read and validate the account-scoped Funding Instrument
or parent chain before sending the write request.

Starting July 13, 2026, Reddit requires `conversion_pixel_id` for every Ad
Group and CBO Campaign. This adapter enforces that requirement. It also keeps
the current Campaign objective constants separate because Reddit has announced
an objective-enum migration for September 30, 2026.

## OAuth

Use `Adapter.OAuth` to create an authorization URL and exchange or refresh a
token. The helper uses HTTP Basic authentication at the token endpoint and
requests `duration=permanent` so Reddit can issue a refresh token.

Official Ads scopes:

| Scope | Purpose |
|---|---|
| `adsread` | Read advertising data and reports |
| `adsedit` | Create and update advertising resources |
| `adsconversions` | Submit conversion events; not exposed by this adapter version |
| `adsdatadeletion` | Submit deletion jobs; not exposed by this adapter version |

## Rate limits

The latest official endpoint-group quotas relevant to this adapter are:

| Endpoint group | Quota | Window |
|---|---:|---:|
| Campaign Management Read | 400 | 60 seconds |
| Campaign Management Write | 200 | 60 seconds |
| Funding Instruments | 30 | 60 seconds |
| Reporting | 60 | 60 seconds |

`Client.RateLimit()` returns the most restrictive state from the latest
response. A 429 error uses the longest positive reset in the `RateLimit` header
as `socialhub.Error.RetryAfter` when it exceeds `Retry-After`.

Reports using the `HOUR` breakdown for more than seven days remain accepted
before Reddit's announced October 30, 2026 cutoff and are rejected locally on
or after that date.

## References

- [Reddit Ads API v3](https://ads-api.reddit.com/docs/v3/)
- [Campaign setup](https://ads-api.reddit.com/docs/v3/guides/programs/campaign/campaign-setup)
- [Reports](https://ads-api.reddit.com/docs/v3/api/get-a-report)
- [Rate limiting](https://ads-api.reddit.com/docs/v3/guides/quick-start/rate-limiting)

The adapter is covered by deterministic local contract tests. It has not been
validated against a credentialed Reddit Ad Account.
