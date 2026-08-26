# Foursquare Places API adapter

This package implements the current Foursquare Places API contract verified on
2026-08-26:

- adapter name: `foursquare/places-api-2025-06-17`
- origin: `https://places-api.foursquare.com`
- required version header: `X-Places-Api-Version: 2025-06-17`
- OpenAPI version: `20250617`
- authentication: `Authorization: Bearer <SERVICE_KEY>`

The adapter intentionally exposes only the Pro-field read surface:

- `GET /places/search`
- `GET /places/{fsq_place_id}`

`/photos` and `/tips` are Premium endpoints and are not implemented. Premium
response fields are rejected locally, because adding one to Search or Details
changes the request to Premium pricing. The legacy `api.foursquare.com/v3`
host, API-key header format, `fsq_id`, URL version parameter, OAuth, Managed
Users, and the retired standalone Categories endpoint are not supported.

## Configuration

```yaml
version: 1
platforms:
  - adapter: foursquare/places-api-2025-06-17
    product: places-api
    accounts:
      - id: primary
        access_token_ref: env://FOURSQUARE_SERVICE_KEY
```

```go
package main

import (
	"context"

	"social-hub/adapters/foursquare"
	"social-hub/pkg/socialhub"
)

func search(ctx context.Context, config socialhub.AdapterConfig) (*foursquare.PlacePage, error) {
	adapter, err := socialhub.Open(ctx, "foursquare/places-api-2025-06-17", config)
	if err != nil {
		return nil, err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "primary")
	if err != nil {
		return nil, err
	}
	client := base.(*foursquare.Client)
	return client.Places().SearchPlaces(ctx, foursquare.SearchRequest{
		Query: "coffee",
		LL:    &foursquare.Coordinate{Latitude: 31.2304, Longitude: 121.4737},
		Limit: 10,
		Fields: []foursquare.PlaceField{
			foursquare.FieldName,
			foursquare.FieldCategories,
			foursquare.FieldLocation,
		},
	})
}
```

When fields are requested, `fsq_place_id` is added automatically. Pagination
returns the raw `Link` header and a validated `NextCursor`; the adapter never
follows provider-supplied links. It performs no automatic retries and provides
no cache layer. `Retry-After` and `X-Fsq-Request-ID` are preserved in errors.
Search `radius` can be combined with an explicit `ll` or with the provider's
IP-biased geolocation, but not with `near` or a rectangular `ne`/`sw` bound.

## Commercial and data-use boundaries

- Enterprise plans document a combined 100 QPS limit. Pay-as-you-go and
  Sandbox plans document 50 QPS. Ask is limited to 1 QPS and is not exposed.
- Current list pricing is $30 CPM for Pro and $36 CPM for Premium, with up to
  10,000 free Pro calls. Confirm current pricing in the Foursquare console.
- The API does not provide a dependable credit or quota response header. Check
  usage in the Developer Console.
- Pay-as-you-go and Sandbox users may cache only `fsq_place_id`, Photo ID, and
  `fsq_addr_id`. Enterprise customers may cache other attributes only on a
  local device for up to 24 hours; server-side caching is not permitted.
- Displays must follow the official Foursquare Visual Crediting Policy.

See the [official Places API documentation](https://docs.foursquare.com/fsq-developers-places/)
and current [pricing](https://foursquare.com/pricing/#places_api) before release.
