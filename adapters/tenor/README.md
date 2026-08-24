# Google Tenor API v2 adapter

Adapter name: `tenor/api-v2`

This package implements a bounded, read-only surface of the official Google
Tenor API v2 at `https://tenor.googleapis.com/v2`:

- `GET /search`
- `GET /featured`
- `GET /categories`
- `GET /posts`

Google's current Quickstart states that, as of January 2026, it no longer
accepts new Tenor API clients. This adapter is therefore usable only by an
integration that already has an enabled Tenor API key. It cannot provision a
key or bypass that admission boundary.

## Configuration

Tenor requires the API key in the `key` query parameter. Google strongly
recommends a stable `client_key` on every request so it can distinguish the
integration. The adapter resolves the API key from `access_token_ref` and
requires `client_key` in account settings:

```yaml
version: 1
platforms:
  - adapter: tenor/api-v2
    product: gif-api
    accounts:
      - id: existing-tenor-client
        access_token_ref: env://TENOR_API_KEY
        settings:
          client_key: social_hub_app
```

```go
package main

import (
	"context"

	"social-hub/adapters/tenor"
	"social-hub/pkg/socialhub"
)

func search(ctx context.Context, config socialhub.AdapterConfig) (tenor.Page, error) {
	adapter, err := socialhub.Open(ctx, "tenor/api-v2", config)
	if err != nil {
		return tenor.Page{}, err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "existing-tenor-client")
	if err != nil {
		return tenor.Page{}, err
	}
	client := base.(*tenor.Client)
	return client.Discovery().Search(ctx, tenor.SearchRequest{
		Query: "excited",
		DiscoveryOptions: tenor.DiscoveryOptions{
			Country:      "US",
			Locale:       "en_US",
			Safety:       tenor.SafetyHigh,
			MediaFormats: []tenor.MediaFormatName{tenor.FormatTinyGIF, tenor.FormatGIF},
			Limit:        10,
		},
	})
}
```

For another page, repeat the same request and set
`DiscoveryOptions.NextPosition` to the previous page's non-empty
`NextPosition`. Tenor explicitly documents `pos` as an opaque value that might
be an integer, float, or string; it is not an array index. Search and Featured
limits are at most 50, and Posts accepts at most 50 IDs.

## Filters and response model

`SafetyOff`, `SafetyLow`, `SafetyMedium`, and `SafetyHigh` map exactly to
Tenor's documented `contentfilter` values. Omitting the value uses Tenor's
documented `off` default. `ContentSticker`, `ContentAnimatedSticker`, and
`ContentStaticSticker` map to the documented `searchfilter` combinations.

`MediaObject` contains only Tenor's documented `url`, `dims`, `duration`, and
`size` fields. `Post.MediaFormats` is a map keyed by content-format name; the
request validator accepts the format names in the current official table and
does not invent additional GIF variants. Supplying `media_filter` is strongly
recommended by Google and can reduce response size by more than 70%.

The Categories response's `path` inherits the original request parameters,
which can include the API key. Before exposing `Category.Path`, this adapter
requires the official Tenor Search origin and strips `key` and `client_key`.
It never follows provider-returned URLs. All API redirects are disabled, the
cookie jar is removed from the cloned HTTP client, successful responses are
bounded by the shared 8 MiB transport limit, and error bodies redact the exact
configured API key. Error `Raw` values are independently bounded to 64 KiB and
remain valid JSON even when the provider returns text or an oversized body.

## Rate, caching, and attribution

- Tenor documents a default limit of 1 request per second for API keys. Calls
  above that threshold fail. The adapter does not guess a higher account
  quota or silently sleep; HTTP 429 and Google `RESOURCE_EXHAUSTED`/quota
  errors map to retryable `socialhub.CodeRateLimited`, with a bounded
  `Retry-After` when supplied.
- Cached content URLs must be refreshed at least every 24 hours. Cached API
  responses should be refreshed frequently because rankings change, and
  callers must respect `ResponseMeta.CacheControl`.
- Every retrieved Tenor item must be attributed using one of the official
  forms: `Powered By Tenor`, `Search Tenor`, or `Via Tenor`.
- Commercial use is subject to Google API Terms, Tenor Additional Terms, and
  Tenor Developer Policies. Ad-enabled clients are permitted under those
  terms, but selling advertising directly on or through the content requires
  Google's prior written approval. Search results must not be reordered or
  modified contrary to the terms.

## Official sources

Official material reviewed on 2026-08-24:

- <https://developers.google.com/tenor/guides/quickstart>
- <https://developers.google.com/tenor/guides/endpoints>
- <https://developers.google.com/tenor/guides/response-objects-and-errors>
- <https://developers.google.com/tenor/guides/content-filtering>
- <https://developers.google.com/tenor/guides/rate-limits-and-caching>
- <https://developers.google.com/tenor/guides/attribution>
- <https://developers.google.com/tenor/guides/api-terms>
- <https://cloud.google.com/apis/design/errors>

The adapter has no third-party dependency. It performs no automatic retries,
caching, media fetches, or writes.
