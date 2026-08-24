# Google Photos Library API v1 adapter

`adapters/googlephotos` is a bounded, read-only adapter for the current Google
Photos Library API v1. Its entire surface is restricted to albums and media
items created by the same OAuth client application.

## 2025 and later capability boundary

Google refocused the Library API on app-created content on March 31, 2025. The
following scopes were removed:

```text
https://www.googleapis.com/auth/photoslibrary
https://www.googleapis.com/auth/photoslibrary.readonly
https://www.googleapis.com/auth/photoslibrary.sharing
```

The current scopes that remain are:

```text
https://www.googleapis.com/auth/photoslibrary.appendonly
https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata
https://www.googleapis.com/auth/photoslibrary.edit.appcreateddata
```

This adapter requires only
`photoslibrary.readonly.appcreateddata`. It rejects removed Library scopes when
they are recorded in `approval.scopes`, and never treats the old broad read
scope as a fallback.

The official Discovery document retains some compatibility-era scope and
sharing entries even in its current revision. Google's 2025 Updates page is
explicit that `albums.share`, `albums.unshare`, and all `sharedAlbums` methods
are no longer available and return `403 PERMISSION_DENIED`. They are therefore
not exposed here.

This package does not provide access to a user's general photo library. An app
that needs the user to select existing content must use the separate Google
Photos Picker API and its user-mediated session model.

## Implemented surface

| Workflow | REST operation | Current authorization and visibility |
| --- | --- | --- |
| List albums | `GET /v1/albums` | `photoslibrary.readonly.appcreateddata`; app-created only |
| Get album | `GET /v1/albums/{albumId}` | Same scope; requested album must be app-created |
| List media | `GET /v1/mediaItems` | Same scope; app-created only |
| Get media | `GET /v1/mediaItems/{mediaItemId}` | Same scope; requested item must be app-created |
| Search media | `POST /v1/mediaItems:search` | Same scope; app-created albums and media only |

Search supports an app-created album ID or typed date, media type, content,
feature, and archived-media filters. Album and filters are mutually exclusive.
The legacy `excludeNonAppCreatedData` field is intentionally not exposed
because the current read scope already enforces that boundary.

Album creation and media upload are still current write capabilities under
`photoslibrary.appendonly`, but they are deliberately outside this adapter.
Media creation is a two-stage contract: upload bytes to obtain a temporary
upload token, then call `mediaItems.batchCreate` and interpret per-item results.
Album edits and media organization use `photoslibrary.edit.appcreateddata`.
Keeping those stateful write workflows out of this minimum read adapter avoids
presenting a partial upload implementation as complete.

`mediaItems.batchGet` is also not included in the minimum surface. Callers can
use the typed `GetMediaItem` operation for individual stable IDs.

## Authentication

The adapter accepts only an externally managed OAuth 2.0 Bearer access token.
It does not implement authorization-code exchange, refresh-token persistence,
or token refresh. `access_token_ref` is resolved at client creation time by the
configured `socialhub.SecretResolver`.

```yaml
version: 1
platforms:
  - adapter: google-photos/library-v1
    product: photos-library-api
    settings:
      user_agent: social-hub/googlephotos
    accounts:
      - id: primary
        access_token_ref: env://GOOGLE_PHOTOS_ACCESS_TOKEN
        approval:
          account_type: oauth2
          scopes:
            - https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata
```

Client IDs, client secrets, token stores, webhook credentials, and raw token
values are rejected from adapter configuration. Credential acquisition and
refresh belong to the application that supplies the secret reference.

## Use

Importing the package registers `google-photos/library-v1` with social-hub:

```go
import (
    "context"
    "fmt"

    "social-hub/adapters/googlephotos"
    "social-hub/pkg/socialhub"
)

func listAppMedia(ctx context.Context) error {
    config := socialhub.AdapterConfig{
        Adapter: "google-photos/library-v1",
        Product: "photos-library-api",
        Accounts: []socialhub.AccountConfig{{
            ID:             "primary",
            AccessTokenRef: "env://GOOGLE_PHOTOS_ACCESS_TOKEN",
            Approval: socialhub.ApprovalConfig{Scopes: []string{
                googlephotos.ScopeReadAppCreatedData,
            }},
        }},
    }

    adapter, err := socialhub.Open(ctx, "google-photos/library-v1", config)
    if err != nil {
        return err
    }
    defer adapter.Close()

    generic, err := adapter.Client(ctx, "primary")
    if err != nil {
        return err
    }
    api := generic.(*googlephotos.Client).GooglePhotos()

    page, err := api.ListMediaItems(ctx, googlephotos.ListMediaItemsRequest{
        Page: googlephotos.PageOptions{PageSize: 50},
    })
    if err != nil {
        return err
    }
    for _, item := range page.Items {
        fmt.Println(item.ID, item.Filename, item.MIMEType)
    }
    return nil
}
```

Search requests preserve the provider's typed filter structure:

```go
page, err := api.SearchMediaItems(ctx, googlephotos.SearchMediaItemsRequest{
    Filters: &googlephotos.SearchFilters{
        DateFilter: &googlephotos.DateFilter{
            Ranges: []googlephotos.DateRange{{
                StartDate: googlephotos.Date{Year: 2026, Month: 1, Day: 1},
                EndDate:   googlephotos.Date{Year: 2026, Month: 12, Day: 31},
            }},
        },
        MediaTypeFilter: &googlephotos.MediaTypeFilter{
            MediaTypes: []googlephotos.MediaType{googlephotos.MediaTypePhoto},
        },
    },
    Page: googlephotos.PageOptions{PageSize: 100},
})
```

Google only permits `orderBy` with a date filter and no content, feature, or
media-type filter. The adapter enforces this and the documented limits for
dates, date ranges, category counts, media types, and page sizes before making
a request.

## Pagination and data lifetime

`nextPageToken` is an opaque provider token. The adapter bounds it, rejects
whitespace and control characters, and sends it only as the documented query
or JSON body field. It never parses or predicts a subsequent token.

Albums use a maximum page size of 50. Media list and search operations use a
maximum of 100. Google can omit the repeated response field when a page is
empty; the adapter returns an empty `Items` slice in that case.

Album and media item IDs are stable and should be persisted instead of the
complete response. Google documents that media `baseUrl` values must be
parameterized before use and expire after 60 minutes. This adapter preserves
the URL but does not fetch media bytes or treat it as a durable download URL.

Every core entity and page retains bounded provider JSON in `Raw`. That data
can contain private media metadata and short-lived byte URLs and must be
handled according to the Google Photos API User Data and Developer Policy.

## Quotas, errors, and security

The current Library API quota is 10,000 requests per Google Cloud project per
day. Requests for media bytes through a base URL use a separate quota of
75,000 per project per day. A quota failure returns HTTP `429`; provider limits
and approved increases are managed in Google Cloud Console rather than inferred
by this adapter.

`ResponseMeta` preserves request IDs, `Retry-After`, cache/lifecycle headers,
and optional quota or rate-limit headers when Google sends them. Google does
not guarantee a remaining-quota header, so Cloud Console is authoritative.

`APIError` wraps a platform-neutral `socialhub.Error` while retaining Google's
standard `error.code`, `error.message`, `error.status`, typed `details`, and
legacy `errors` fields. Provider error JSON is recursively sanitized for the
configured access token and credential-like keys. Quota errors and `5xx`
failures are retryable; invalid credentials and missing scope are user-action
errors, with the current required scope attached to permission failures.

Successful responses must be bounded JSON objects with a JSON content type.
List values require valid provider IDs, and detail operations reject a response
whose ID differs from the requested ID.

The API origin is fixed to `https://photoslibrary.googleapis.com`. The adapter
clones the supplied HTTP client, removes its cookie jar, and disables redirects
so a Bearer token cannot be forwarded to a `Location` origin. Transport errors
discard request URLs.

## Official sources

Official material reviewed on 2026-08-25:

- <https://photoslibrary.googleapis.com/$discovery/rest?version=v1>
- <https://developers.google.com/photos/library/reference/rest>
- <https://developers.google.com/photos/library/reference/rest/v1/albums>
- <https://developers.google.com/photos/library/reference/rest/v1/mediaItems>
- <https://developers.google.com/photos/library/guides/authorization>
- <https://developers.google.com/photos/library/guides/access-media-items>
- <https://developers.google.com/photos/library/guides/api-limits-quotas>
- <https://developers.google.com/photos/support/updates>
- <https://developers.google.com/photos/support/api-policy>

The reviewed Discovery document identifies `photoslibrary:v1`, revision
`20260820`. This package adds no third-party dependency.
