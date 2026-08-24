# LinkedIn Marketing API 202607 adapter

`linkedin/marketing-202607` provides account-scoped, typed paid-media
workflows for LinkedIn Marketing API version `202607`. It intentionally stays
separate from the organic `linkedin/rest-202607` adapter: an ad Creative is not
a common `socialhub.Post`.

## Supported workflows

- Get the configured Ad Account.
- Search, get, create, update, activate, pause, and archive Campaign Groups.
- Search, get, create, update, activate, pause, and archive Sponsored Updates
  Campaigns.
- Search, get, batch-create one, activate, pause, and archive Creatives that
  reference an existing `share` or `ugcPost` URN.
- Run synchronous Ad Analytics for the configured Ad Account with one of the
  `ACCOUNT`, `CAMPAIGN_GROUP`, `CAMPAIGN`, or `CREATIVE` pivots.
- Build authorization-code URLs, exchange authorization codes, and use the
  partner-restricted programmatic refresh-token grant.

Every create operation forces `DRAFT`. Call the corresponding status method
only after reviewing the returned resource. The initial adapter does not
accept inline Creative JSON, upload media, create Lead Gen forms, or implement
Lead Sync/webhooks.

## Configuration

```go
import (
	"context"

	_ "social-hub/adapters/linkedin/marketing"
	"social-hub/pkg/socialhub"
)

config := socialhub.AdapterConfig{
	Adapter: "linkedin/marketing-202607",
	Product: "marketing-api",
	Accounts: []socialhub.AccountConfig{{
		ID:             "north-america-demand",
		ClientID:       "${LINKEDIN_CLIENT_ID}",
		SecretRef:      "env://LINKEDIN_CLIENT_SECRET",
		AccessTokenRef: "env://LINKEDIN_ACCESS_TOKEN",
		Approval: socialhub.ApprovalConfig{Scopes: []string{
			"r_ads", "rw_ads", "r_ads_reporting",
		}},
		Settings: map[string]any{"ad_account_id": "511183580"},
	}},
}

adapter, err := socialhub.Open(context.Background(), config)
if err != nil {
	panic(err)
}
defer adapter.Close()
```

The application must provide a `socialhub.SecretResolver`; secret references
are resolved at client creation and are never sent in URL query parameters.
Configure each LinkedIn Ad Account as a separate social-hub account to keep
ownership checks and analytics queries bounded.

## Permissions and access tiers

Reads require `r_ads` or `rw_ads`; mutations require `rw_ads`; reporting
requires `r_ads_reporting`. LinkedIn also checks the authenticated member's Ad
Account role. A `VIEWER` remains read-only even when the token contains
`rw_ads`.

Marketing API access is tiered. Development tier limits mutation access to a
small number of Ad Accounts and supports one API-created test account;
Standard tier removes the account-management limit. Programmatic refresh
tokens are available only to approved Marketing Developer Platform partners.
Other applications must repeat the authorization-code flow when the access
token expires.

LinkedIn applies daily endpoint/member/application quotas that reset at
midnight UTC; concrete quota values are visible in the application's Developer
Portal. Ad Analytics additionally limits request complexity. This adapter caps
analytics requests at 20 fields and a 366-day range and validates the documented
15,000-row response ceiling.

## Protocol contract

All API requests include:

```text
Authorization: Bearer <access token>
Linkedin-Version: 202607
X-Restli-Protocol-Version: 2.0.0
```

Search uses cursor pagination (`pageSize`, `pageToken`, and
`metadata.nextPageToken`). Updates use `X-RestLi-Method: PARTIAL_UPDATE`.
Creative creation uses a one-element `BATCH_CREATE`; create IDs are read from
`X-RestLi-Id` or the batch result and then fetched back for ownership
validation.

Official references:

- [Marketing API versioning](https://learn.microsoft.com/en-us/linkedin/marketing/versioning)
- [Ads overview](https://learn.microsoft.com/en-us/linkedin/marketing/integrations/ads/ads-overview)
- [Campaign Groups](https://learn.microsoft.com/en-us/linkedin/marketing/integrations/ads/account-structure/create-and-manage-campaign-groups?view=li-lms-2026-07)
- [Campaigns](https://learn.microsoft.com/en-us/linkedin/marketing/integrations/ads/account-structure/create-and-manage-campaigns?view=li-lms-2026-07)
- [Creatives](https://learn.microsoft.com/en-us/linkedin/marketing/integrations/ads/account-structure/create-and-manage-creatives?view=li-lms-2026-07)
- [Ad Analytics](https://learn.microsoft.com/en-us/linkedin/marketing/integrations/ads-reporting/ads-reporting?view=li-lms-2026-07)
- [Rate limits](https://learn.microsoft.com/en-us/linkedin/shared/api-guide/concepts/rate-limits)
