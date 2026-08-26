# Tripadvisor Content API v1 adapter

Adapter name: `tripadvisor/content-api-v1`

This package implements the five documented, read-only Tripadvisor Content
API v1 operations:

- location search;
- nearby location search;
- location details;
- location photos;
- location reviews.

It deliberately excludes writes, Hotel Pricing, Tripadvisor Terra, automatic
pagination, and any endpoint whose public contract is unclear. Content API v1
is still publicly documented and subscribable, but Tripadvisor marks it for a
future migration to Terra. Terra has a different contract and is not silently
mixed into this adapter.

## Authentication and access

Every request sends the private API key in the documented `key` query
parameter. The adapter resolves the key from `access_token_ref`; it does not
create, refresh, persist, or log API keys. Redirects are disabled and cookie
jars are removed from the cloned HTTP client so a key cannot be forwarded to
another origin.

```yaml
version: 1
platforms:
  - adapter: tripadvisor/content-api-v1
    product: content-api
    accounts:
      - id: places-read
        access_token_ref: env://TRIPADVISOR_API_KEY
```

Tripadvisor currently permits one Content API key per account. A key must have
an IP or domain restriction configured before it can be used. Subscription
requires a credit card. The published offer includes the first 5,000 calls per
month without charge and bills overage, but the portal and the account order
remain authoritative.

Import the package to register its factory, then use the typed workflow:

```go
package main

import (
	"context"
	"fmt"

	"social-hub/adapters/tripadvisor"
	"social-hub/pkg/socialhub"
)

func nearby(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "tripadvisor/content-api-v1", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "places-read")
	if err != nil {
		return err
	}
	client := base.(*tripadvisor.Client)
	response, err := client.Places().SearchNearby(ctx, tripadvisor.SearchNearbyRequest{
		Coordinate: tripadvisor.Coordinate{Latitude: 42.3455, Longitude: -71.10767},
		Category:   tripadvisor.CategoryRestaurants,
	})
	if err != nil {
		return err
	}
	for _, location := range response.Data {
		fmt.Println(location.LocationID, location.Name)
	}
	return nil
}
```

## Request and response contract

Search returns at most 10 locations and has no pagination. Nearby Search
requires a coordinate; regular Search requires `SearchQuery` and optionally
accepts a coordinate. A radius is accepted only with a coordinate. Supported
categories, radius units, languages, photo sources, ISO 4217 currency shape,
coordinates, IDs, page size, and offset are validated before transport.

Photo and review page size is capped at 5, matching the public documentation.
Some beta subscriptions may have different limits; this adapter intentionally
uses the public portable limit. Provider `paging.next` and `paging.previous`
are retained for diagnostics but are never followed automatically.

The OpenAPI schema declares IDs as `int32`, while deployed integrations have
observed numeric and quoted decimal representations. `ID` accepts either form,
stores a canonical decimal string, and avoids integer precision or type drift.
Location Details and Review responses are checked against the requested
location ID. Photo and review entity IDs must be present.

Successful responses are limited to 8 MiB, must have a JSON content type, and
must contain a top-level object. A top-level `error` in an HTTP 200 response is
returned as `APIError`, not as successful data. Typed resources retain a
sanitized `Raw` value. Before decoding, the adapter structurally redacts
sensitive JSON fields and exact occurrences of the configured key, including
keys embedded in provider pagination URLs.

## Quotas and retry behavior

Tripadvisor documents a 50 QPS limit and 10,000 Search API calls per day.
Details, Photos, and Reviews consume the account's daily budget. The dedicated
rate-limit page describes the daily window as rolling 24 hours, while the FAQ
describes reset at midnight UTC. Because these official descriptions conflict
and commercial orders can differ, this adapter does not hard-code a limiter;
the portal, account order, and response headers are authoritative.

`ResponseMeta` preserves common rate-limit headers, `Retry-After`,
`x-amzn-requestid`, and `x-amz-apigw-id`. HTTP 429 maps to retryable
`socialhub.CodeRateLimited`. HTTP 408, 5xx, and gateway availability failures
map to retryable `socialhub.CodeTemporarilyUnavailable`. `Retry-After` accepts
seconds or an HTTP date. Authentication, permission, validation, conflict, and
not-found errors remain non-retryable.

## Caching and display requirements

Only `location_id` may be cached indefinitely. Tripadvisor prohibits caching,
storing, or indexing every other Content API field. This package provides no
cache. `Raw` exists only to process the current response and is not permission
to retain provider content.

Applications displaying Content API data must follow the current Display
Requirements and Review Implementation Policy, including all of these rules:

- place the Tripadvisor Mark close to all Tripadvisor content;
- load Tripadvisor logos and rating bars directly from returned Tripadvisor
  URLs, without downloading or locally storing them;
- render ratings with the returned Tripadvisor bubble image, never a
  reconstructed rating icon;
- keep review text out of page HTML and JavaScript source, load it externally,
  and block the review-loading path in `robots.txt` so search engines cannot
  index it;
- retain required review quote, Tripadvisor traveler, photo caption, source,
  user, ranking month/year, and other attribution data;
- do not filter, reorder, or combine Tripadvisor and third-party content in a
  way that misleads users;
- use returned photo URLs and do not cache or locally save photos.

Tripadvisor API content must not be used for AI model training or fine-tuning.
The Master Terms describe only limited internal, non-commercial RAG testing;
production, commercial, or broader AI use requires a separate agreement. An
API key proves authentication, not permission to ignore contractual display,
retention, attribution, restricted-site, or content-use terms.

## Official sources

Official material reviewed on 2026-08-26:

- <https://tripadvisor-content-api.readme.io/reference/overview>
- <https://tripadvisor-content-api.readme.io/reference/authentication>
- <https://tripadvisor-content-api.readme.io/reference/searchforlocations>
- <https://tripadvisor-content-api.readme.io/reference/searchfornearbylocations>
- <https://tripadvisor-content-api.readme.io/reference/getlocationdetails>
- <https://tripadvisor-content-api.readme.io/reference/getlocationphotos>
- <https://tripadvisor-content-api.readme.io/reference/getlocationreviews>
- <https://tripadvisor-content-api.readme.io/reference/rate-limits>
- <https://tripadvisor-content-api.readme.io/reference/caching-policy>
- <https://tripadvisor-content-api.readme.io/reference/review-implementation-policy>
- <https://tripadvisor-content-api.readme.io/reference/display-requirements>
- <https://tripadvisor-content-api.readme.io/reference/api-security>
- <https://tripadvisor-content-api.readme.io/reference/prohibited-sites-and-content>
- <https://tripadvisor-content-api.readme.io/reference/api-master-terms-new>

The adapter has no third-party dependency.
