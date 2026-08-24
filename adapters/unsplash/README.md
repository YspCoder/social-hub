# Unsplash API v1 adapter

Adapter name: `unsplash/api-v1`

This package implements a bounded public-read surface of the official
Unsplash JSON API v1:

- `GET /search/photos`;
- `GET /photos/{assetSlug}`;
- `GET /users/{username}`;
- `GET /users/{username}/photos`;
- `GET /users/{username}/collections`;
- `GET /collections`;
- `GET /collections/{collectionId}`;
- `GET /collections/{collectionId}/photos`;
- the event-only `GET /photos/{id}/download` endpoint.

User OAuth, uploads, likes, collection writes, private resources, random
photos, topics, statistics, and webhooks are deliberately excluded. Complete
provider objects remain available through `Raw` fields.

## Authentication and configuration

Every request explicitly selects API v1 and uses public application
authentication:

```text
Accept-Version: v1
Authorization: Client-ID <application-access-key>
```

Despite the scheme name, `Client-ID` carries the application's **Access Key**.
The adapter resolves it from `access_token_ref` so it is not stored as a plain
`client_id` or placed in a URL. Unsplash also accepts `Bearer <user-token>` for
OAuth user authentication, but Bearer tokens, authorization-code exchange,
refresh behavior, user scopes, and writes are outside this adapter.

```yaml
version: 1
platforms:
  - adapter: unsplash/api-v1
    product: api
    accounts:
      - id: public-read
        access_token_ref: env://UNSPLASH_ACCESS_KEY
```

The Access Key and Secret Key must remain confidential under the API
guidelines. The HTTP client uses only `https://api.unsplash.com`, does not
follow redirects or use a cookie jar, and never puts the Access Key in the
query string.

## Reading photos

```go
package main

import (
	"context"
	"fmt"

	"social-hub/adapters/unsplash"
	"social-hub/pkg/socialhub"
)

func search(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "unsplash/api-v1", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "public-read")
	if err != nil {
		return err
	}
	client := base.(*unsplash.Client)
	page, err := client.Photos().SearchPhotos(ctx, unsplash.SearchPhotosRequest{
		Query:         "Shanghai skyline",
		PerPage:       20,
		ContentFilter: unsplash.ContentFilterHigh,
		Orientation:   unsplash.OrientationLandscape,
		Language:      unsplash.SearchLanguageSimplifiedChinese,
	})
	if err != nil {
		return err
	}
	for _, photo := range page.Results {
		fmt.Println(photo.URLs.Regular, photo.User.Name)
	}
	return nil
}
```

Search supports the complete language enum currently published in the
official OpenAPI, including English, Simplified Chinese, and Traditional
Chinese. Page-number endpoints default to page 1 and 10 items and accept at
most 30 items per page. `ResponseMeta` retains `Link`, `X-Total`, `Warning`,
`X-Ratelimit-Limit`, `X-Ratelimit-Remaining`, HTTP status, and request ID;
next and previous page numbers are parsed from `Link` without following
provider-supplied URLs.
Search results and list resources are summary objects. Use the individual
photo, user, or collection endpoint when full details are needed.

## Hotlinking, attribution, and download events

Unsplash's technical guidelines are part of the integration contract:

1. Display images by directly embedding a URL from `Photo.URLs`. Do not fetch,
   proxy, replace, or persist image bytes in order to serve them from another
   origin. Image requests to `images.unsplash.com` do not consume JSON API
   quota.
2. Attribute both Unsplash and the photographer whenever a photo is shown,
   link to the photographer's Unsplash profile, and add
   `utm_source=<your_app_name>&utm_medium=referral` to links back to Unsplash.
3. When a user performs a download-like action, such as inserting a photo in
   a post or setting it as a background, call `TrackDownload` with that exact
   photo's `Links.DownloadLocation`, including its query parameters.

```go
photo, err := client.Photos().GetPhoto(ctx, "Dwu85P9SOIk")
if err != nil {
	return err
}

// Call only when the user actually performs a download-like action.
_, err = client.Photos().TrackDownload(ctx, photo.Links.DownloadLocation)
return err
```

`TrackDownload` records an analytics event. It validates but does not return
the endpoint's URL, fetch image bytes, expose an `io.Reader`, or follow a
redirect. It is not a file-download API and its location must not be used as
an image source. Use `Photo.URLs.*` for display; the official guideline names
`Photo.URLs.Full` when directing a user to the full image.

## Rate limits and approval

Current official documentation states 50 JSON requests per hour in demo mode
and 1,000 requests per hour after production approval. Higher limits require
contacting Unsplash. Response headers are authoritative for the configured
application; the adapter performs no automatic retry, throttling, or cache.
HTTP 429 and a rate-limit HTTP 403 map to retryable
`socialhub.CodeRateLimited` errors.

An Access Key authenticates the application but does not waive API terms or
grant production approval. The guidelines prohibit selling unaltered
Unsplash photos, replicating Unsplash's core experience, abusing automated
requests, using the Unsplash name directly as the application name, or using
the Unsplash logo as the application icon. Confirm the current terms and
production-review requirements before release.

## Official sources

Official material reviewed on 2026-08-25:

- <https://unsplash.com/documentation>
- <https://unsplash.com/spec/v1.json>
- <https://help.unsplash.com/en/articles/2511245-unsplash-api-guidelines>
- <https://help.unsplash.com/en/articles/2511258-guideline-triggering-a-download>
- <https://help.unsplash.com/en/articles/3887917-when-should-i-apply-for-a-higher-rate-limit>
- <https://unsplash.com/api-terms>

The adapter has no third-party dependency.
