# Openverse API v1 adapter

Adapter name: `openverse/api-v1`

This package implements a bounded, read-only surface of the official Openverse
API v1 at the fixed origin `https://api.openverse.org/v1`:

- `GET /images/`
- `GET /images/{uuid}/`
- `GET /audio/`
- `GET /audio/{uuid}/`

It searches image and audio metadata only. It does not fetch media bytes,
follow media URLs, write to Openverse, register OAuth applications, obtain or
refresh tokens, or expose experimental `unstable__*` request parameters.

## Configuration

Openverse supports anonymous reads. A logical social-hub account therefore
needs only an ID:

```yaml
version: 1
platforms:
  - adapter: openverse/api-v1
    product: api
    accounts:
      - id: public-search
```

An externally managed Openverse OAuth access token can be supplied through a
secret reference. The adapter sends it as a Bearer credential and never logs
or returns it:

```yaml
version: 1
platforms:
  - adapter: openverse/api-v1
    product: api
    accounts:
      - id: authenticated-search
        access_token_ref: env://OPENVERSE_ACCESS_TOKEN
```

Openverse's token endpoint uses the OAuth client-credentials grant with
`client_id`, `client_secret`, and `grant_type=client_credentials`. Token
provisioning and renewal remain the caller's responsibility so application
secrets never enter this read adapter.

```go
package main

import (
	"context"

	"social-hub/adapters/openverse"
	"social-hub/pkg/socialhub"
)

func search(ctx context.Context, config socialhub.AdapterConfig) (openverse.ImageSearchResponse, error) {
	adapter, err := socialhub.Open(ctx, "openverse/api-v1", config)
	if err != nil {
		return openverse.ImageSearchResponse{}, err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "public-search")
	if err != nil {
		return openverse.ImageSearchResponse{}, err
	}
	client := base.(*openverse.Client)
	return client.Images().SearchImages(ctx, openverse.ImageSearchRequest{
		SearchRequest: openverse.SearchRequest{
			Query:     "Shanghai skyline",
			Licenses:  []openverse.License{openverse.LicenseCC0, openverse.LicensePDM},
			PageSize: 10,
		},
		Category: openverse.ImageCategoryPhotograph,
	})
}
```

## Search limits

The adapter exposes the stable query, creator, tags, title, source, excluded
source, license, license type, extension, mature-content, category, image size,
image aspect-ratio, and audio-length filters. It deliberately requires at
least one of query, creator, tags, or title and enforces documented mutual
exclusions before making a request.

Anonymous search pages are capped at 20 results. Bearer-authenticated pages are
capped at 50 results. Both modes are limited to a search depth of 240 using
`page * page_size`. Current Openverse default throttles documented in source
are 5 requests/hour plus 100/day for anonymous clients, 100/minute plus
10,000/day for standard OAuth clients, and 200/minute plus 20,000/day for
enhanced OAuth clients. Provider quota headers are exposed by scope through
`ResponseMeta.RateLimits`; HTTP 429 maps to retryable
`socialhub.CodeRateLimited`, with a bounded `Retry-After` when supplied.

## Licensing and attribution

Every result preserves Openverse's `license`, `license_version`, `license_url`,
`creator`, `creator_url`, `source`, `provider`, `foreign_landing_url`, and
`attribution` fields, plus the complete bounded provider object in `Raw`.
Openverse aggregates metadata and does not guarantee that license information
is accurate or that reuse is permitted. Before using an asset, callers must
inspect its source page, verify the current license and rights status, provide
the required attribution, and comply with all applicable license terms.

The official API origin cannot be overridden. Redirects are disabled, the
cookie jar is removed from the cloned HTTP client, successful bodies are
bounded by the shared 8 MiB transport limit, and error bodies are normalized
to at most 64 KiB of valid JSON with exact Bearer-token redaction.

## Official sources

Official Openverse material reviewed on 2026-08-25:

- <https://docs.openverse.org/api/reference/>
- <https://docs.openverse.org/api/reference/authentication_and_throttling.html>
- <https://github.com/WordPress/openverse/blob/main/documentation/api/terms_of_service.md>
- <https://github.com/WordPress/openverse/blob/main/api/conf/urls/__init__.py>
- <https://github.com/WordPress/openverse/blob/main/api/conf/urls/auth_tokens.py>
- <https://github.com/WordPress/openverse/blob/main/api/conf/settings/rest_framework.py>
- <https://github.com/WordPress/openverse/blob/main/api/api/constants/restricted_features.py>
- <https://github.com/WordPress/openverse/blob/main/api/api/docs/image_docs.py>
- <https://github.com/WordPress/openverse/blob/main/api/api/docs/audio_docs.py>
- <https://github.com/WordPress/openverse/blob/main/api/api/serializers/media_serializers.py>
- <https://github.com/WordPress/openverse/blob/main/api/api/serializers/image_serializers.py>
- <https://github.com/WordPress/openverse/blob/main/api/api/serializers/audio_serializers.py>
- <https://github.com/WordPress/openverse/blob/main/api/api/utils/pagination.py>

The adapter adds no dependency and performs no retries, caching, media
downloads, or writes.
