# Google Places API (New) v1 adapter

Adapter name: `google-places/places-api-new-v1`

This package implements a bounded, read-only surface of the current Places API
(New) REST v1:

- Text Search (New), including `pageSize`, `pageToken`, and an optional
  `nextPageToken` response field;
- Nearby Search (New) within the documented circle restriction;
- Place Details (New);
- Place photo metadata embedded in Place responses;
- Place Photo (New) media lookup with redirect suppression and a validated,
  short-lived HTTPS media URI.

It deliberately excludes the legacy Places API, Autocomplete, routing and EV
extensions, generative or review content, writes, OAuth, and endpoints or
fields outside this package's typed contract. It never mixes legacy request
parameters such as `place_id`, `location`, or `photo_reference` into the New
API resource model.

## Authentication and project setup

Requests use the current official origin `https://places.googleapis.com/v1`
and send the API key only in `X-Goog-Api-Key`. The origin is not
configurable. Redirects are disabled and cookie jars are removed from the
cloned HTTP client.

The adapter resolves the key from `access_token_ref`; it does not create,
refresh, persist, or log keys.

```yaml
version: 1
platforms:
  - adapter: google-places/places-api-new-v1
    product: places-api-new
    accounts:
      - id: places-read
        access_token_ref: env://GOOGLE_PLACES_API_KEY
```

The Google Cloud project must have billing enabled and **Places API (New)**
enabled. Enabling a legacy Places service is not sufficient. Restrict the key
by both application type (for example, server IP addresses) and API to Places
API (New), following current Google Maps Platform key-security guidance.

## Typed field masks and billing SKUs

Text Search, Nearby Search, and Place Details always send
`X-Goog-FieldMask`. Wildcards, raw strings, spaces, and unsupported fields are
rejected. The adapter automatically includes `id` for resource identity and
then adds only the supplied typed `PlaceField` values. Text Search adds
`nextPageToken` only when `IncludeNextPageToken` is true. Photo media is the
documented exception that does not require a field mask.

Field selection is also a billing decision. Google currently assigns Place
data fields to different Places API (New) SKUs, including IDs Only,
Essentials, Pro, Enterprise, and Enterprise + Atmosphere families; Search and
Place Photo have operation-specific SKUs. The highest applicable requested
field group can determine the billed SKU. This SDK does not guess a budget,
request a wildcard, or hide a field expansion. Confirm the current SKU field
table and prices for the billing account before production use.

```go
package main

import (
	"context"
	"fmt"

	"social-hub/adapters/googleplaces"
	"social-hub/pkg/socialhub"
)

func search(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "google-places/places-api-new-v1", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "places-read")
	if err != nil {
		return err
	}
	client := base.(*googleplaces.Client)
	page, err := client.Places().TextSearch(ctx, googleplaces.TextSearchRequest{
		TextQuery: "coffee in Shanghai",
		PageSize:  10,
		Fields: []googleplaces.PlaceField{
			googleplaces.FieldDisplayName,
			googleplaces.FieldFormattedAddress,
			googleplaces.FieldLocation,
		},
		IncludeNextPageToken: true,
	})
	if err != nil {
		return err
	}
	for _, place := range page.Places {
		name := ""
		if place.DisplayName != nil {
			name = place.DisplayName.Text
		}
		fmt.Println(place.ID, name)
	}
	return nil
}
```

Callers must retain the initial Text Search parameters when using a returned
`NextPageToken`: Google requires every parameter other than `pageToken` and
`pageSize` to match the request that produced the token. The adapter validates
one request but does not retain search state or automatically fetch another
page.

Nearby Search accepts up to 50 values in each documented type list, a circle
radius from 0 through 50,000 meters, and at most 20 results. Text Search page
size is at most 20. Text Search accepts `minRating` from 0 through 5; Google
documents that values outside the 0.5 cadence are rounded up to the next 0.5.
A Place Details `SessionToken` is accepted only in the
documented URL- and filename-safe form of at most 36 ASCII characters; it must
belong to the same Cloud project and Autocomplete session when used.

## Photo contract

Request photo metadata with `FieldPhotos`. A photo `Name` has the resource
form `places/{place_id}/photos/{photo_reference}` and carries dimensions and
author attributions. `GetPhotoMedia` accepts that complete name, requires at
least one width or height from 1 through 4,800 pixels, appends `/media`, and
always sends `skipHttpRedirect=true`.

The package returns the JSON `PhotoMedia` resource only after its name matches
the request and `photoUri` is an absolute HTTPS URL without credentials or a
fragment. It does not follow the URI, download a photo, or expose the default
redirect response. The URI is documented as short-lived; callers must not
treat it or a photo resource name as a durable asset URL.

## Quotas and errors

Places API (New) quotas are dynamic, method- and project-specific, and can be
changed in Google Cloud. Pricing, monthly usage thresholds, and quota values
also vary over time and by billing account. This adapter intentionally states
no fixed call allowance and does not hard-code a limiter.

`ResponseMeta` preserves bounded `Retry-After`, request/trace identifiers, and
any returned `X-Goog-Quota-*`, `X-RateLimit-*`, or `RateLimit-*` headers without
assigning them universal semantics. Google `google.rpc.Status` errors are
returned as `APIError`. `RESOURCE_EXHAUSTED` and HTTP 429 are retryable rate
limits. `RetryAfter` uses the HTTP header first and otherwise accepts a bounded
`google.rpc.RetryInfo.retryDelay`. `UNAVAILABLE`, `INTERNAL`, deadline, and
gateway failures are retryable; authentication, permission, argument,
not-found, and precondition errors are not blindly retried.

Successful and error bodies are limited to 8 MiB. Successful responses must
be JSON objects with a JSON content type. Before typed decoding, the package
recursively redacts API-key and authorization fields plus exact occurrences of
the configured key. Typed resources retain this sanitized response in `Raw`.

## Regions, storage, and attribution

`languageCode` controls preferred localization. `regionCode` is the caller's
two-letter CLDR region and can affect returned details based on applicable
law; it is not merely formatting. Available functionality and permitted use
can differ by region. In particular, projects serving or billed in the EEA
must review the current Google Maps Platform EEA Terms and regional Places API
behavior rather than assuming another region's contract.

This package provides no cache. Google permits Place IDs to be retained, while
other Places content, photo resources, and media remain subject to the current
Google Maps Platform Terms and Places API policies. A `Raw` field is not
permission to cache, index, resell, train models on, or otherwise retain
provider content.

When Places data is shown without a Google map, applications must provide the
required Google Maps attribution according to the current display policies.
They must also render every returned `Place.Attributions` provider attribution
and every photo `AuthorAttributions` entry required by the photo policy. Do
not remove, obscure, or replace Google or third-party attribution, and do not
present Places content in a misleading combination with non-Google data.

## Official sources

Official contracts reviewed on 2026-08-26:

- <https://developers.google.com/maps/documentation/places/web-service/op-overview>
- <https://developers.google.com/maps/documentation/places/web-service/text-search>
- <https://developers.google.com/maps/documentation/places/web-service/nearby-search>
- <https://developers.google.com/maps/documentation/places/web-service/place-details>
- <https://developers.google.com/maps/documentation/places/web-service/place-photos>
- <https://developers.google.com/maps/documentation/places/web-service/choose-fields>
- <https://developers.google.com/maps/documentation/places/web-service/usage-and-billing>
- <https://developers.google.com/maps/documentation/places/web-service/policies>
- <https://developers.google.com/maps/documentation/places/web-service/get-api-key>
- <https://developers.google.com/maps/api-security-best-practices>
- <https://developers.google.com/maps/billing-and-pricing/sku-details#places>
- <https://github.com/googleapis/google-api-go-client/blob/main/places/v1/places-api.json>
- <https://github.com/googleapis/googleapis/blob/master/google/maps/places/v1/places_service.proto>
- <https://github.com/googleapis/googleapis/blob/master/google/maps/places/v1/place.proto>
- <https://github.com/googleapis/googleapis/blob/master/google/maps/places/v1/photo.proto>

The reviewed Discovery document identifies `places:v1`, revision `20260824`,
with root URL `https://places.googleapis.com/`. This adapter has no
third-party dependency.
