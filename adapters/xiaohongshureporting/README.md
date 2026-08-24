# Xiaohongshu Spotlight reporting adapter

Adapter name: `xiaohongshu/spotlight-reporting-api`

This package implements the read-only, advertiser-scoped Xiaohongshu Spotlight
(小红书聚光) balance and reporting surface reviewed on 2026-08-25. It uses the
unversioned `https://adapi.xiaohongshu.com` API and sends the resolved token only
in the `Access-Token` header.

## Covered workflows

- Account budget and balance: total, cash, rebate, credit, frozen, available,
  compensation, account budget, daily budget limit, and current-day spend.
- Offline reports: account, campaign, unit, creative, keyword, note, SPU,
  and search word.
- Realtime reports: account, campaign, unit, creative, keyword, and target.
- Exact dynamic response values: JSON numbers are not converted through
  `float64`; nested realtime DTOs remain available through `ReportValue.Decode`.

Campaign management, creative upload, OAuth exchange/refresh, conversion
tracking, and webhooks are deliberately outside this reporting-only adapter.
Xiaohongshu does not publish a stable version segment for these routes, so the
adapter metadata records the contract review date rather than inventing one.

## Configuration

```go
import (
	_ "social-hub/adapters/xiaohongshureporting"

	"social-hub/pkg/socialhub"
)

config := socialhub.AdapterConfig{
	Adapter: "xiaohongshu/spotlight-reporting-api",
	Product: "spotlight-reporting-api",
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
request. Report request types intentionally do not expose an advertiser field,
which prevents one account client from being used to query another advertiser.
Tokens remain secret references and are resolved only at runtime. Redirects are
rejected, cookie jars are disabled, and the API origin is fixed so credentials
cannot cross origins.

## Access requirements

Production access requires an approved Xiaohongshu developer application, a
Spotlight advertising account, advertiser authorization, and the relevant
account/report scopes. Account type and scope eligibility are controlled by
Xiaohongshu; a valid token alone does not grant report or finance access.

The package does not guess token refresh behavior. Refresh and authorization
code exchange should be implemented against the exact application type and
current OAuth grant approved in the Xiaohongshu console, then the resulting
access token should be supplied through `access_token_ref`.

## Operational boundaries

- Xiaohongshu documents an application-level ceiling of 3,000 calls per
  minute. The realtime creative report's 300 QPS pool is shared across all
  customers, so callers should pace below the nominal ceiling and honor 429
  responses and `Retry-After`.
- Offline `page_size` is locally limited to 500; realtime `page_size` is
  limited to 100.
- `need_hourly_data` is accepted only for today's Asia/Shanghai date.
- Request JSON is limited to 1 MiB and the shared transport limits responses to
  8 MiB.
- Redirects are rejected. `CallOption` timeouts and request IDs are supported;
  idempotency keys and field selection are rejected for read-only reports.
- Platform `msg`, `message`, and response bodies are never copied into returned
  errors. Errors preserve only the bounded platform code, request ID, HTTP
  status, and `Retry-After`.
- Unknown business codes are permanent `platform_error` values. HTTP 429 and
  5xx responses, and equivalent numeric business codes, are retryable.

Official entry point:
<https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=4889>

The implementations in `bububa/spotlight-mapi`, `jundaychan/spotlight-mapi`,
and `ArtisanCloud/MediaX` were audited only as contract references. None is a
runtime dependency: their transport, response-bound, logging, numeric, or
dependency-weight tradeoffs do not match social-hub's safety requirements.
