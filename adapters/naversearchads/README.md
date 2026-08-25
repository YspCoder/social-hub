# NAVER Search AD API v2 adapter

Adapter name: `naver/search-ad-api-v2`

This package implements advertiser-scoped NAVER paid-search workflows:

- Campaign list/get, paused create, budget/period updates, explicit pause or enable, and guarded delete;
- Ad Group list/get, paused create, bid/budget updates, explicit pause or enable, and guarded delete;
- Keyword list/get, paused batch create (up to 100), same-field batch update (up to 200), and guarded delete;
- synchronous entity Stats using KST ranges or documented date presets;
- asynchronous Stat Report jobs and bounded authenticated TSV/CSV downloads.

Organic NAVER content, assets, ads/creatives, business channels, estimates,
Master Reports, billing, and agency customer-link management are intentionally
outside this adapter version.

## Access and authentication

Create an API license in NAVER Search Advertiser Center under **Tools > API
Manager**. Every request uses the API license, secret key, and numeric customer
ID issued or authorized for that advertiser.

The adapter sends:

```text
X-Timestamp: Unix milliseconds
X-API-KEY: API license
X-Customer: advertiser customer ID
X-Signature: Base64(HMAC-SHA256(secret, timestamp.METHOD.path))
```

`METHOD` is uppercase and the signature includes only the path, never the
query string. Redirects are rejected so signed headers cannot cross origins.
The API origin is fixed to `https://api.searchad.naver.com`; adapter-level
settings cannot override it. NAVER does not use OAuth or refresh tokens for
this API.

## Configuration

Both credential values are resolved from secret references. For this adapter,
`access_token_ref` stores the API license reference and `secret_ref` stores the
HMAC secret-key reference.

```yaml
version: 1
platforms:
  - adapter: naver/search-ad-api-v2
    product: search-ad-api
    accounts:
      - id: naver-search-kr
        access_token_ref: env://NAVER_SEARCH_AD_API_LICENSE
        secret_ref: env://NAVER_SEARCH_AD_SECRET_KEY
        settings:
          customer_id: 1234567
```

`client_id`, `app_id`, `token_store`, webhook settings, and OAuth approval
fields are rejected because they are not part of NAVER's Search AD
authentication contract. One social-hub account maps to exactly one
`X-Customer` boundary, and every returned Campaign, Ad Group, and Keyword is
checked against that customer ID.

## Use

```go
package main

import (
	"context"

	"social-hub/adapters/naversearchads"
	"social-hub/pkg/socialhub"
)

func createPausedCampaign(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "naver/search-ad-api-v2", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "naver-search-kr")
	if err != nil {
		return err
	}
	client := base.(*naversearchads.Client)

	campaign, err := client.Campaigns().CreateCampaign(ctx, naversearchads.CreateCampaignRequest{
		Name: "social-hub search",
		Type: naversearchads.CampaignWebSite,
	})
	if err != nil {
		return err
	}

	// Creation is always paused. Enabling delivery is a separate explicit call.
	_, err = client.Campaigns().SetCampaignPaused(ctx, campaign.ID, false)
	return err
}
```

Synchronous Stats preserve every selected value as `JSONValue`, so integer
spend and future platform fields never pass through `float64`:

```go
stats, err := client.Statistics().Stats(ctx, naversearchads.StatQuery{
	IDs:       []string{campaign.ID},
	Fields:    []naversearchads.StatField{naversearchads.StatImpressions, naversearchads.StatClicks, naversearchads.StatSpend},
	DatePreset: naversearchads.DateYesterday,
})
```

For asynchronous reports, create a job with a `YYYYMMDD` date, poll
`GetStatReport` until `BUILT`, then call `DownloadStatReport`. Downloads accept
only same-origin `/report-download` URLs returned by NAVER, re-sign that path,
reject redirects and unexpected encodings, and write at most the configured
byte limit.

## Safety and operational behavior

- Campaigns, Ad Groups, and Keywords are always created with `userLock=true`.
- Enabling an Ad Group requires an enabled, eligible parent Campaign.
- Enabling a Keyword requires enabled, eligible parent Campaign and Ad Group.
- Delete methods first read the resource and require it to be paused.
- NAVER does not document idempotency keys. Ambiguous mutation failures return
  `ErrOutcomeUnknown`; mixed Keyword batch results return `ErrPartialMutation`.
- Keyword batch updates require every item to use the same field set because
  NAVER's `fields` query parameter applies to the whole request.
- The public Swagger does not publish one stable global QPS value. HTTP 429 and
  platform code `1016` map to retryable rate-limit errors; coordinate limiting
  across replicas and treat provider responses as authoritative.
- Only `socialhub.WithCallTimeout` is supported. Caller request IDs,
  idempotency keys, and generic field selection are rejected because NAVER
  does not document those generic request contracts.

No real NAVER advertiser credentials were used for local verification.

## Official sources

- [NAVER Search AD API specification](https://naver.github.io/searchad-apidoc/)
- [Official documentation and samples repository](https://github.com/naver/searchad-apidoc)
- [Official error-code table](https://github.com/naver/searchad-apidoc/blob/master/NaverSA_API_Error_Code_MAP.md)

The official v2 management Swagger, v1 Report Swagger, Python/PHP/Java signing
and Stat Report samples, and error table were reviewed on 2026-08-25.
