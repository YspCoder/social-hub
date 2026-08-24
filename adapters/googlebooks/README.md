# Google Books API v1 adapter

Adapter name: `google/books-api-v1`

This package implements a bounded, read-only surface of the official Google
Books API v1 at the fixed production origin `https://books.googleapis.com`:

| Workflow | REST operation | Authorization |
| --- | --- | --- |
| Search public volumes | `GET /books/v1/volumes` | API key or OAuth 2.0 Bearer token |
| Get one public volume | `GET /books/v1/volumes/{volumeId}` | API key or OAuth 2.0 Bearer token |

The package registers itself with social-hub when imported. It does not expose
My Library, bookshelves, purchases, write operations, download tokens, media
downloads, `saleInfo`, or `userInfo`.

## Authentication and configuration

An API key identifies the Google Cloud project for public-data access, quota,
and reporting. Store the key behind `account.secret_ref`; the adapter sends it
only as the documented `key` query parameter:

```yaml
version: 1
platforms:
  - adapter: google/books-api-v1
    product: books-api
    accounts:
      - id: public-catalog
        secret_ref: env://GOOGLE_BOOKS_API_KEY
```

An OAuth access token identifies an authorized Google user and project. Store
it behind `account.access_token_ref` and record the exact Books scope:

```yaml
version: 1
platforms:
  - adapter: google/books-api-v1
    product: books-api
    accounts:
      - id: oauth-catalog
        access_token_ref: env://GOOGLE_BOOKS_ACCESS_TOKEN
        approval:
          scopes:
            - https://www.googleapis.com/auth/books
```

Both credential references may be configured on one account. In that case the
request carries the API key and Bearer token using their respective documented
locations. At least one credential is required.

This adapter accepts static credentials only. It does not implement an OAuth
authorization-code flow, exchange authorization codes, persist refresh tokens,
or refresh access tokens. Credential acquisition and renewal are the caller's
responsibility through the configured `socialhub.SecretResolver`.

## Use

```go
package main

import (
	"context"
	"fmt"

	"social-hub/adapters/googlebooks"
	"social-hub/pkg/socialhub"
)

func searchBooks(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "google/books-api-v1", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "public-catalog")
	if err != nil {
		return err
	}
	books := base.(*googlebooks.Client).Volumes()

	page, err := books.Search(ctx, googlebooks.SearchVolumesRequest{
		Query:      "subject:distributed-systems inauthor:Kleppmann",
		MaxResults: 10,
		OrderBy:    googlebooks.VolumeOrderRelevance,
		PrintType:  googlebooks.SearchPrintBooks,
	})
	if err != nil {
		return err
	}
	for _, volume := range page.Items {
		fmt.Println(volume.ID, volume.Info.Title, volume.Info.Authors)
	}
	return nil
}
```

## Search and pagination

`SearchVolumesRequest.Query` is required and supports Google's documented
full-text operators: `intitle:`, `inauthor:`, `inpublisher:`, `subject:`,
`isbn:`, `lccn:`, and `oclc:`. Public category browsing is available through
the `subject:` operator.

Pagination is offset based. `StartIndex` begins at `0`; `MaxResults` defaults
to `10` and cannot exceed `40`. The adapter also exposes the documented
`filter`, `orderBy`, `printType`, `projection`, and two-letter `langRestrict`
controls. `paid-ebooks` is a public catalog filter only: it does not expose a
purchase operation or any sale fields.

## Data boundary

Typed successful responses preserve public Volume metadata including IDs,
ETags, titles, subtitles, authors, publisher and publication date,
descriptions, ISBN/ISSN identifiers, page counts, print type, categories,
ratings, dimensions, reading modes, language, maturity rating, image links,
preview/info links, viewability, embeddability, public-domain state, text-to-
speech permission, and search snippets.

Successful values intentionally have no `Raw` field. Even when a request uses
OAuth, Google may include user-specific purchase or library information in the
upstream Volume representation; the typed decoder drops `saleInfo`, `userInfo`,
download tokens, and all other fields outside this public catalog contract.

Volume descriptions and search snippets may contain provider-supplied HTML.
Callers must sanitize them for their rendering context. Image, preview,
information, reader, canonical, and self links are untrusted external metadata;
the adapter returns the strings but never follows them or downloads their
content.

## Quotas, errors, and transport security

The official Books API documentation does not publish a stable numeric quota
for these methods. The active Google Cloud project quota page and provider
quota errors are authoritative. `ResponseMeta` preserves bounded request IDs,
`Retry-After`, and quota or rate-limit response headers when Google supplies
them; the adapter does not infer unavailable quota values.

`APIError` wraps a platform-neutral `socialhub.Error` and retains Google's
standard error envelope. Its `Raw` value is always bounded, sanitized, valid
JSON, including when the provider returns non-JSON or oversized content. API
keys and access tokens are redacted from retained error fields, response
metadata, and transport errors. HTTP `429` and transient `5xx` responses map to
retryable errors; invalid credentials and insufficient authorization require
user action.

The origin cannot be overridden. The adapter clones the supplied HTTP client,
removes its cookie jar, and disables redirects so credentials cannot be
forwarded to another origin. Successful and error response bodies are bounded
before decoding. The package performs no retries, caching, writes, or media
requests.

## Official sources

Official material reviewed on 2026-08-25:

- <https://books.googleapis.com/$discovery/rest?version=v1>
- <https://developers.google.com/books/docs/v1/using>
- <https://developers.google.com/books/docs/v1/reference/volumes/list>
- <https://developers.google.com/books/docs/v1/reference/volumes/get>
- <https://cloud.google.com/apis/design/errors>

The reviewed Discovery document identifies Books API `v1`, revision
`20260818`, with production root URL `https://books.googleapis.com/`. This
package adds no dependency.
