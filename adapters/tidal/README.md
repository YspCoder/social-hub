# TIDAL Developer Platform API v2 adapter

Adapter name: `tidal/api-v2`

This package implements a deliberately small, read-only catalog surface of
the official TIDAL API v2:

- `GET /searchResults`;
- `GET /artists` and `GET /artists/{id}`;
- `GET /albums` and `GET /albums/{id}`;
- `GET /tracks` and `GET /tracks/{id}`.

Playback, streaming, downloads, DRM, `sourceFile`, `usageRules`, private user
collections, shares and `shareCode`, writes, media replacement, and OAuth
token exchange or refresh are excluded. The adapter always calls the fixed
production origin `https://openapi.tidal.com/v2` and has no third-party Go
dependency.

The contract was verified on 2026-08-26 against the current OpenAPI 3.0.1
document, TIDAL API specification version `1.10.111`.

## Authentication

Every request uses an externally managed OAuth 2.0 bearer token:

```text
Accept: application/vnd.api+json
Authorization: Bearer <access-token>
```

TIDAL documents both client credentials and authorization code with PKCE.
All seven implemented endpoints accept client credentials without a scope,
so that is the recommended flow for third-party server-side catalog reads.
For PKCE tokens, collection search requires `r_usr` and `search.read`, while
the artist, album, and track collection endpoints require `r_usr`; the
single-resource endpoints require no scope. `r_usr` is marked internal by the
current authorization material and should not be requested by third-party
applications.

The adapter does not acquire, persist, refresh, or introspect tokens. Supply a
valid token through a secret reference and renew it outside this package.
Capabilities use `ApprovalUnknown` because possessing a configured token does
not prove that TIDAL has approved the caller or that the token is still valid.

```yaml
version: 1
platforms:
  - adapter: tidal/api-v2
    product: tidal-api
    accounts:
      - id: catalog
        access_token_ref: env://TIDAL_ACCESS_TOKEN
```

The configured HTTP client is copied with redirects disabled and its cookie
jar removed, preventing a bearer token from being forwarded to another
origin.

## Catalog reads

```go
package main

import (
	"context"
	"fmt"

	"social-hub/adapters/tidal"
	"social-hub/pkg/socialhub"
)

func search(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "tidal/api-v2", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "catalog")
	if err != nil {
		return err
	}
	client := base.(*tidal.Client)
	page, err := client.Catalog().Search(ctx, tidal.SearchRequest{
		Query:       "Nina Simone",
		CountryCode: "US",
		Include:     []string{"artists", "albums"},
	})
	if err != nil {
		return err
	}
	fmt.Println(len(page.Items), len(page.Included))
	return nil
}
```

TIDAL IDs and cursors are opaque strings. To retrieve the next album or track
page, copy `Page.NextCursor` into a new request and resend the original
filters, country, sort, and include values. The adapter validates
`links.next`, extracts only `page[cursor]`, and never follows a provider link.

Relationships appear only when requested through `Include`. An absent entry
in a resource's `Relationships` map therefore means "not requested", not an
empty relationship. Generic included resources, provider attributes, and
complete successful documents remain available through bounded `Raw` fields.
Provider enums are plain strings so future values remain decodable.

The official `/searchResults` schema defines `data` as an array without a
single-item cardinality guarantee. `Search` therefore returns
`Page[SearchResult]` and does not invent a singleton contract.

## Errors and rate limits

TIDAL's current OpenAPI specification documents HTTP 429 but publishes no
fixed numeric quota and no mandatory rate-limit response headers. The adapter
does not invent a quota or retry automatically. `ResponseMeta` retains any
observed `Retry-After`, `X-RateLimit-*`, request, cache, and lifecycle headers
only after bounding them, rejecting control characters, and filtering the
configured token. Only `WithCallTimeout` is supported; request IDs,
idempotency keys, and generic field selection are rejected before transport.
HTTP 429 and 5xx responses map to retryable socialhub errors; 401, 403, and
404 map to their platform-neutral categories. Both the documented
`application/vnd.api+json` error envelope and the production service's
observed `application/json` error envelope are accepted and recursively
sanitized before `APIError.Provider` and `APIError.Raw` are exposed. The
default error string uses a fixed message rather than logging provider detail.

## Official sources

Official material reviewed on 2026-08-26:

- <https://developer.tidal.com/>
- <https://tidal-music.github.io/tidal-api-reference/>
- <https://tidal-music.github.io/tidal-api-reference/tidal-api-oas.json>
- <https://developer.tidal.com/documentation/api-sdk/api-sdk-authorization>

The reviewed OpenAPI document is version `3.0.1`; its TIDAL API specification
version is `1.10.111` and its sole production server is
`https://openapi.tidal.com/v2`. The official responses had these SHA-256
digests:

| Reference | HTTP | SHA-256 |
|---|---:|---|
| Developer portal | 200 | `2FEB0CAFF7A2B42FC33584C1AB44449A6DB66262A31A0CE949738D42CEE06886` |
| API reference entry | 200 | `684EC15FA79CB69A1B220CAB12401236F77CE0AEA073FF0870D576617F7AAB5F` |
| OpenAPI document | 200 | `5D8E43848405CB8C6C2DFF6739E3ED2FEE5AC81C163A16BF3F517F90B097FFBF` |
| Authorization documentation entry | 200 | `2FEB0CAFF7A2B42FC33584C1AB44449A6DB66262A31A0CE949738D42CEE06886` |
| Unauthenticated production 401 body | 401 | `265C64BC1D79AB32C62375C851DAD47AC118C9B46C40A5EDF783AAE9EFC27380` |

One unauthenticated catalog request confirmed that the production service
currently returns its missing-authorization error as `application/json`. No
bearer token was supplied, and no credentialed production request was sent.
