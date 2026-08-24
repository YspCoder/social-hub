# Apple iTunes Search API adapter

Adapter name: `apple/itunes-search-api`

This package implements the public, unversioned Apple iTunes Search API at the
fixed production origin `https://itunes.apple.com`:

| Workflow | HTTP operation | Authentication |
| --- | --- | --- |
| Search Store metadata | `GET /search` | None |
| Lookup Store metadata | `GET /lookup` | None |

The typed result model covers common music, podcast, movie, audiobook,
software, and ebook metadata. The adapter does not expose purchases, writes,
Apple Music API library operations, StoreKit, Enterprise Partner Feed access,
media downloads, JSONP, or affiliate-link construction.

## Configuration

Apple does not require credentials for these endpoints. A social-hub logical
account ID is still required so callers can select a client consistently:

```yaml
version: 1
platforms:
  - adapter: apple/itunes-search-api
    product: itunes-search-api
    accounts:
      - id: public-us-store
```

Adapter and account settings, client IDs, app IDs, secret references, access
token references, account token stores, OAuth scopes, and webhook
configuration are rejected. A process-wide Token Store option is harmlessly
ignored so this public adapter can share a Hub with authenticated adapters. The
production origin cannot be overridden.

## Use

```go
package main

import (
	"context"
	"fmt"

	"social-hub/adapters/itunessearch"
	"social-hub/pkg/socialhub"
)

func searchMusic(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "apple/itunes-search-api", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "public-us-store")
	if err != nil {
		return err
	}
	catalog := base.(*itunessearch.Client).Catalog()

	response, err := catalog.Search(ctx, itunessearch.SearchRequest{
		Term:      "Miles Davis",
		Country:   "US",
		Media:     itunessearch.MediaMusic,
		Entity:    itunessearch.EntityAlbum,
		Attribute: itunessearch.AttributeArtistTerm,
		Limit:     10,
	})
	if err != nil {
		return err
	}
	for _, result := range response.Results {
		fmt.Println(result.CollectionName, result.ArtistName, result.CollectionViewURL)
	}
	return nil
}
```

An identifier lookup selects exactly one family:

```go
response, err := catalog.Lookup(ctx, itunessearch.LookupRequest{
	IDs:    []int64{909253},
	Entity: itunessearch.EntityAlbum,
	Limit:  5,
})
```

Numeric Apple and AMG families accept up to 200 positive IDs and are encoded
as one comma-separated parameter. UPC/EAN accepts one 8-to-14 digit value;
ISBN accepts the documented 13-digit form. `LookupRequest` also exposes the
documented `entity`, `limit`, and `sort=recent` controls. There is no pagination
or page token in this API.

## Search contract

`SearchRequest.Term` is required. Country defaults to `US`; media defaults to
`all`; limit defaults to `50`; language defaults to `en_us`; result-key version
defaults to `2`; explicit content defaults to `Yes`. The package validates the
official media/entity and media/attribute combinations before sending a
request. Limit values are restricted to Apple's documented `1..200` range.

The supported media values are `movie`, `podcast`, `music`, `musicVideo`,
`audiobook`, `shortFilm`, `tvShow`, `software`, `ebook`, and `all`. Languages
are `en_us` and `ja_jp`; result-key versions are `1` and `2`. The JSONP
`callback` parameter is deliberately unavailable.

## Data and promotional-content boundary

The successful response preserves Apple's `resultCount` and `results`
envelope. Results expose identifiers, names, Store URLs, artwork and preview
URLs, prices, explicitness, track metadata, genres, dates, podcast feeds,
descriptions, software compatibility and rating fields, and common ebook
metadata. Provider-supplied descriptions may contain markup and must be
sanitized for the caller's rendering context.

Preview, feed, artwork, screenshot, artist, collection, track, and seller URLs
are untrusted external metadata. This package returns those strings but never
follows, streams, downloads, saves, caches, or rewrites them. It never adds
affiliate parameters.

Apple's promotional-content terms restrict previews and artwork to promotion
of the corresponding Store content. Store badges and direct Store links must
be proximate; song and music-video previews require attribution, must be
streamed only, and must not be downloaded, saved, cached, synchronized with
video, or used for independent entertainment. Callers are responsible for the
current terms at the official documentation URL.

## Rate guidance, errors, and transport

Apple currently describes the Search API limit as approximately 20 calls per
minute and explicitly marks it subject to change. The package does not turn
that guidance into a fixed limiter. Callers should cache catalog requests where
their content rights permit and apply a shared, configurable rate policy.

HTTP `429` and transient `5xx` responses map to retryable social-hub errors.
`ResponseMeta` preserves bounded Apple request IDs and `Retry-After` when
present. `APIError.Raw` is retained only when the provider body is valid,
bounded JSON and after sensitive keys are structurally redacted; non-JSON and
oversized errors do not receive a fabricated raw payload.

The supplied HTTP client is cloned, its cookie jar is removed, and redirects
are disabled. Successful bodies must be bounded JSON objects with Apple's
observed JSON media types. The production service currently returns plain JSON
as `text/javascript; charset=utf-8`; the adapter also accepts standard JSON
media types but never requests or accepts JSONP callbacks. `resultCount` must
equal the decoded result array length. Per-call options support timeout only;
caller request IDs, idempotency keys, and generic field masks are rejected.

The adapter performs no retries, writes, authentication, media requests,
caching, or fixed rate limiting.

## Official sources

Official material and production response headers reviewed on 2026-08-25:

- <https://performance-partners.apple.com/search-api>
- <https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/>
- <https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/Searching.html>
- <https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/LookupExamples.html>
- <https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/UnderstandingSearchResults.html>

The Performance Partners page is the current official entry point. The Apple
Developer archive identifies documentation version `1.0.3`, last updated
2017-09-19; that is a documentation revision, not a versioned HTTP API. The
package adds no dependency.
