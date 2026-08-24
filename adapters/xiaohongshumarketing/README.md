# Xiaohongshu Spotlight marketing adapter

Adapter name: `xiaohongshu/spotlight-marketing-api`

This package implements advertiser-scoped Xiaohongshu Spotlight (XHS Juguang)
campaign, unit, and creative management workflows reviewed on 2026-08-25. It
uses the unversioned `https://adapi.xiaohongshu.com` API and sends the resolved
token only in the `Access-Token` header.

## Covered workflows

- Campaign list, current cascade campaign editing, and explicit resume, pause,
  and delete actions.
- Unit list and explicit resume, pause, and delete actions.
- Creative search and explicit resume, pause, and delete actions.
- Typed current response fields plus bounded `Raw` JSON for provider additions.

Campaign creation is deliberately excluded. The current three-level
`/api/open/jg/cascade/create` contract does not document a paused-first field,
so the adapter does not risk creating immediately delivering ads. Unit and
creative editing, asset upload, organic note publishing, OAuth exchange and
refresh, reporting, and webhooks are also outside this package.

## Configuration

```go
import (
	_ "social-hub/adapters/xiaohongshumarketing"

	"social-hub/pkg/socialhub"
)

config := socialhub.AdapterConfig{
	Adapter: "xiaohongshu/spotlight-marketing-api",
	Product: "spotlight-marketing-api",
	Accounts: []socialhub.AccountConfig{{
		ID:             "brand-cn",
		AccessTokenRef: "env:XHS_SPOTLIGHT_ACCESS_TOKEN",
		Settings: map[string]any{
			"advertiser_id": uint64(1234567890),
		},
	}},
}
```

`advertiser_id` is bound when the client is created and injected into every
request. Request types do not expose it, preventing one account client from
operating on another advertiser. Tokens remain secret references and are
resolved only at runtime. Redirects are rejected so the `Access-Token` header
cannot cross origins. The API origin is fixed to Xiaohongshu's official HTTPS
host, and adapter-level settings are rejected.

## Access requirements

Production access requires an approved Xiaohongshu developer application, a
Spotlight advertiser authorization, and the relevant scopes:

- `ad_query` for campaign, unit, and creative reads.
- `ad_manage` for edits and status changes.

The package does not guess token exchange or refresh behavior. Supply the
approved advertiser token through `access_token_ref` and handle its lifecycle
outside this adapter.

## Write-safety boundaries

- `UpdateCampaign` uses the current `/api/open/jg/cascade/modify` route and
  requires callers to declare either product seeding (`4`) or lead generation
  (`9`). The provider's legacy editing routes stop supporting these objectives
  on 2026-09-01.
- Campaign patches generate the provider's `update_fields` Field Mask from
  non-nil request fields, so explicit zero and default values are not mistaken
  for omitted fields.
- The API does not document an idempotency-key contract. An ambiguous network
  failure, HTTP 408/5xx response, or invalid 2xx response returns
  `ErrOutcomeUnknown`; reconcile provider state before retrying.
- Batch status methods return the acknowledged IDs. A subset is returned with
  `ErrPartialMutation` so callers can reconcile only the missing resources.
- Status changes are always explicit methods. Reads never mutate delivery
  state, and there is no implicit activation after another write.

## Operational boundaries

- Xiaohongshu documents an application-level ceiling of 3,000 calls per
  minute; several reporting and note endpoints outside this package have
  additional shared QPS limits.
- Campaign and creative queries accept at most 20 IDs; unit queries accept at
  most 10. Page size is locally limited to 100.
- Campaign schedule dates use Asia/Shanghai day boundaries. All-day bitmaps use
  24 `1` characters per weekday; `0` means no delivery for that hour.
- Request JSON is limited to 1 MiB and the shared transport limits responses to
  8 MiB.
- `CallOption` timeouts and request IDs are supported. Idempotency keys and
  field selection are rejected.
- Platform `msg`, response bodies, and credentials are never copied into
  returned errors. Errors retain only bounded platform codes, request IDs, HTTP
  status, and `Retry-After`.

Official documentation:

- [Query campaigns](https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3150)
- [Current cascade editing](https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=4752)
- [List units](https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3044)
- [Search creatives](https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3158)
- [Update creative status](https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3157)
- [Scope requirements](https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3195)
- [Spotlight API FAQ and quotas](https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=4436)

The implementations in `bububa/spotlight-mapi`,
`jundaychan/spotlight-mapi`, and `ArtisanCloud/MediaX` were reviewed only as
contract references. None is a runtime dependency.
