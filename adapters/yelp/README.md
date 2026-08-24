# Yelp Places API v3 adapter

Adapter name: `yelp/places-api-v3`

This package implements a bounded, read-only surface of Yelp Places API v3:

- business search by location or coordinates;
- the seven non-Premium business attribute filters documented for Search;
- business details by ID or alias;
- review excerpts for a business;
- the Yelp category taxonomy.

It deliberately excludes Premium-only fields and filters, Partner APIs,
Events, GraphQL, AI APIs, phone search, business match, autocomplete, OAuth
token endpoints, writes, and webhooks. The complete JSON for each successful
implemented resource and response remains available through its `Raw` field.

## Authentication

Yelp Fusion stopped using OAuth in 2017. Requests use a private API key:

```text
GET https://api.yelp.com/v3/...
Authorization: Bearer <private-api-key>
```

The adapter resolves that key from `access_token_ref`; it does not create,
refresh, or persist keys. It rejects OAuth client fields, token stores,
approval scopes, account settings, and webhook configuration because they do
not belong to this API-key contract. Yelp currently requires the private key
to contain exactly 128 ASCII letters, digits, hyphens, or underscores. The API
origin is fixed, redirect following is disabled, and cookie jars are ignored.

```yaml
version: 1
platforms:
  - adapter: yelp/places-api-v3
    product: places-api
    accounts:
      - id: places-read
        access_token_ref: env://YELP_API_KEY
```

Import the package to register its factory, then use the typed workflow:

```go
package main

import (
	"context"
	"fmt"

	"social-hub/adapters/yelp"
	"social-hub/pkg/socialhub"
)

func nearby(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "yelp/places-api-v3", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "places-read")
	if err != nil {
		return err
	}
	client := base.(*yelp.Client)
	response, err := client.Places().SearchBusinesses(ctx, yelp.SearchBusinessesRequest{
		Location: "San Francisco, CA",
		Term:     "coffee",
		Limit:    20,
	})
	if err != nil {
		return err
	}
	for _, business := range response.Businesses {
		fmt.Println(business.Name, business.Rating)
	}
	return nil
}
```

## Request and response contract

Business Search requires exactly one of `Location` or the complete
`Latitude`/`Longitude` pair. The adapter enforces Yelp's documented radius,
locale, price, sort, page-size, and offset bounds. `OpenNow` and `OpenAt` are
mutually exclusive. Categories and prices are encoded as comma-separated
query values. `Attributes` accepts only Yelp's seven documented non-Premium
values and rejects duplicates; Premium-only filters remain outside this
adapter because an API key does not prove plan entitlement.

The current Business Details schema names the hours discriminator
`hour_type`, while Yelp's official examples still use `hours_type`. `Hours`
accepts both names and exposes one `Type`. It also accepts either the
documented hours array or the singleton object in the current official
example. `Review.TimeCreated` remains a string because Yelp returns a local
timestamp such as `2016-08-29 00:41:13` without a timezone.

The common `socialhub.Fetcher` is intentionally unavailable: mapping a Yelp
business, its location and opening hours, review excerpts, and taxonomy to
generic social posts would discard provider semantics.

## Plans, licensing, and display

An API key proves authentication only. It does not grant a commercial data
license, a Premium or Partner plan, or permission to ignore Yelp's data
display and attribution requirements. Applications must confirm their Yelp
Places plan and contract before production use. Plan-gated responses are
returned as typed `APIError` values with the original Yelp error code.

Yelp's current Business Search changelog limits non-paying clients to at most
240 retrievable matches, even though the request page size is at most 50.
This adapter validates one page; callers remain responsible for stopping at
the plan's result ceiling.

Yelp documents that cached API data may be retained for at most 24 hours.
Business IDs may be retained for backend matching as described in the rate
limiting guidance. Applications must not treat the `Raw` field as permission
to retain provider data beyond those terms.

## Rate limits

Yelp applies both QPS controls and daily, plan- and endpoint-dependent quotas.
It does not publish one universal QPS number. The current Starter trial is
documented as 300 calls per 24 hours with reset at midnight UTC, but callers
must use their own plan and response headers as authoritative rather than
hard-coding that value.

Every response exposes these header values unchanged in `ResponseMeta`:

```text
RateLimit-DailyLimit
RateLimit-Remaining
RateLimit-ResourceDailyLimit
RateLimit-ResourceRemaining
RateLimit-ResetTime
```

`TOO_MANY_REQUESTS_PER_SECOND`, `ACCESS_LIMIT_REACHED`, and HTTP 429 map to a
retryable `socialhub.CodeRateLimited`. `RetryAfter` uses the standard
`Retry-After` header first and otherwise derives a bounded delay from Yelp's
ISO 8601 `RateLimit-ResetTime`.

## Official sources

Official material reviewed on 2026-08-25:

- <https://docs.developer.yelp.com/docs/places-intro>
- <https://docs.developer.yelp.com/reference/v3_business_search>
- <https://docs.developer.yelp.com/reference/v3_business_info>
- <https://docs.developer.yelp.com/reference/v3_business_reviews>
- <https://docs.developer.yelp.com/reference/v3_all_categories>
- <https://docs.developer.yelp.com/docs/resources-supported-locales>
- <https://docs.developer.yelp.com/docs/places-rate-limiting>
- <https://docs.developer.yelp.com/changelog/new-limits-for-business-search-endpoint>
- <https://github.com/Yelp/yelp-fusion>

The adapter has no third-party dependency.
