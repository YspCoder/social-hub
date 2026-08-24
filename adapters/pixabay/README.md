# Pixabay API adapter

Adapter name: `pixabay/api`

This package implements a bounded, public, read-only surface of the current
unversioned Pixabay API at the fixed official HTTPS origin:

- `GET https://pixabay.com/api/` for image search
- `GET https://pixabay.com/api/videos/` for video search

It does not download media, follow or request provider-returned page, preview,
image, thumbnail, or video URLs, upload content, access private user data, or
perform writes.

## Configuration

Pixabay requires an API key in the `key` query parameter. Keep the value in a
secret resolver; configuration stores only its reference:

```yaml
version: 1
platforms:
  - adapter: pixabay/api
    product: api
    accounts:
      - id: public-catalog
        access_token_ref: env://PIXABAY_API_KEY
```

```go
package main

import (
	"context"

	"social-hub/adapters/pixabay"
	"social-hub/pkg/socialhub"
)

func search(ctx context.Context, config socialhub.AdapterConfig) (pixabay.ImageSearchResponse, error) {
	adapter, err := socialhub.Open(ctx, "pixabay/api", config)
	if err != nil {
		return pixabay.ImageSearchResponse{}, err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "public-catalog")
	if err != nil {
		return pixabay.ImageSearchResponse{}, err
	}
	client := base.(*pixabay.Client)
	return client.Catalog().SearchImages(ctx, pixabay.ImageSearchRequest{
		SearchRequest: pixabay.SearchRequest{
			Query: "Shanghai skyline", Language: pixabay.LanguageEN, Category: pixabay.CategoryPlaces,
			SafeSearch: true, PerPage: 20,
		},
		ImageType: pixabay.ImageTypePhoto,
		Orientation: pixabay.OrientationHorizontal,
		Colors: []pixabay.Color{pixabay.ColorBlue, pixabay.ColorGray},
	})
}
```

The adapter exposes the documented `q`, `lang`, `category`, `min_width`,
`min_height`, `image_type`, `video_type`, `colors`, `orientation`,
`editors_choice`, `safesearch`, `order`, `page`, and `per_page` parameters.
Queries may be omitted to retrieve the public catalog. Query text is limited to
100 characters and `per_page` to 3-200; the default page size is 20.

## Quota and caching

The current default quota is 100 requests per 60 seconds and is associated
with the API key rather than the caller IP. `ResponseMeta` preserves
`X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset`; reset is
documented as the remaining number of seconds in the current window. HTTP 429
maps to retryable `socialhub.CodeRateLimited`.

Pixabay requires API requests to be cached for 24 hours and prohibits large
volumes of automated queries and systematic mass downloads. The adapter
exposes `RequiredCacheTTL` in every response but does not silently choose a
cache implementation. Production callers must place a 24-hour cache in front
of repeated searches.

## Display, hotlinking, and license terms

When displaying API search results, show users that the images and videos come
from Pixabay. Results preserve `pageURL`, contributor `user_id`, `user`, and
`userImageURL` for source and author presentation.

Returned image URLs may be used only for temporary search-result display.
Permanent image hotlinking is prohibited. Pixabay permits direct video
embedding but recommends storing videos on the caller's server. This adapter
does neither: it returns URL metadata without requesting, embedding,
downloading, rewriting, or following any provider URL. `webformatURL` is
documented as valid for 24 hours. `fullHDURL`, `imageURL`, and `vectorURL` are
preserved when present but are available only to accounts approved for full
API access.

The Content License does not require author attribution for content use,
although credit is appreciated; this is separate from the API documentation's
request to show Pixabay as the source of displayed search results. The current
Terms of Service, last updated November 18, 2024, prohibit selling or
distributing content on a standalone basis, bulk or systematic copying, and
specified trademark, misleading, illegal, and rights-infringing uses. Pixabay
does not warrant that third-party permissions have been obtained. Callers must
determine whether additional copyright, trademark, model, property, privacy,
or other permissions are required before using content.

## Security and response handling

The API key is added only after typed query construction. The origin cannot be
overridden, redirects are disabled, and the cloned HTTP client has no cookie
jar. The shared transport bounds successful responses at 8 MiB. Provider
plain-text errors are converted to at most 64 KiB of valid JSON with exact API
key and `key=` query-value redaction. The adapter never performs retries,
caching, downloads, or writes and adds no dependency.

## Official sources

Official material reviewed on 2026-08-25:

- <https://pixabay.com/api/docs/>
- <https://pixabay.com/service/terms/>
- <https://pixabay.com/service/license-summary/>
