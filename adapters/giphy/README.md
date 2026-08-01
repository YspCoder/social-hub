# GIPHY API v1 adapter

Package `social-hub/adapters/giphy` targets GIPHY's official API v1 at
`https://api.giphy.com/v1`, the official upload endpoint at
`https://upload.giphy.com/v1`, and response-provided analytics pingbacks. It
does not use cookies, scraping, browser automation, private endpoints, or media
URL rewriting.

## Integration policy boundary

GIPHY requires Search and Trending requests to be made directly from the end
user's client. Standard integrations must not proxy API calls or media loads,
cache GIPHY media or media URLs, rewrite returned URLs, reorder/filter search
or trending results, or mix GIPHY results with another provider in one grid.
Caching requires GIPHY's prior written approval and its revalidation contract.

This is therefore a client-side integration package, despite being written in
Go. Use it from an eligible Go desktop, mobile, or other direct client. A Go
backend must not expose it as a Search/Trending proxy unless GIPHY has
explicitly approved that architecture. The SDK does not make a prohibited
deployment compliant.

Every UI using the API must conspicuously display an approved "Powered by
GIPHY" attribution mark. Applications must also send the exact user query,
preserve result order, use the returned media URLs unchanged, and follow
GIPHY's API Terms and content-safety requirements.

## Access and quotas

GIPHY uses an API key in the `api_key` query parameter and does not use OAuth.
Create a separate key for each platform and each distinct integration section.
All new keys are beta keys limited to 100 API calls per hour. Production access
requires an application review and may involve custom pricing.

Beta keys are limited to 10 uploads per day and cannot assign an approved GIPHY
channel username. Production-approved keys remove those upload restrictions
according to the application's agreement. The adapter treats quotas as policy
inputs rather than hard-coded limiter constants and maps HTTP `429` plus
`Retry-After` into the common error model.

## Implemented contracts

| Surface | Support |
|---|---|
| Typed `DiscoveryWorkflow` | GIF/Sticker Search, Trending, Translate, Random, single/batch GIF lookup, Random ID, categories, autocomplete, related terms, and trending searches |
| Typed `UploadWorkflow` | Exact-length streaming multipart upload for animated GIF or video files up to 100MB |
| Typed `AnalyticsWorkflow` | View/click/send registration using validated response-provided `/v2/pingback_simple` URLs |
| Common `Fetcher` | Not exposed; GIPHY discovery is not a user timeline or post/comment contract |
| Common `Publisher` / `MediaUploader` | Not exposed; upload creates GIPHY media in one request rather than a platform-neutral post or resumable media resource |
| Reactions / messaging / webhooks | Not exposed by the documented GIPHY API contracts |

The adapter checks both HTTP status and GIPHY's `meta.status`. Error envelopes
may return `data: []` even when an endpoint normally returns an object, so meta
is classified before typed data decoding.

## Configuration

```yaml
adapter: giphy/v1
accounts:
  - id: gif-picker-web
    client_id: "giphy-api-key-for-this-platform-and-section"
```

`client_id` contains the GIPHY API key because GIPHY keys identify a direct
client integration. Keep it out of logs and error artifacts, apply provider
restrictions where available, and do not reuse it across platforms or product
sections. Custom endpoint settings exist for deterministic tests and approved
gateways:

```yaml
settings:
  base_url: https://api.giphy.com/v1
  upload_url: https://upload.giphy.com/v1
  analytics_origin: https://giphy-analytics.giphy.com
```

The client refuses HTTP redirects so an API key or analytics payload cannot be
forwarded to a different origin by the HTTP stack.

## Discovery and analytics

Pass a stable, non-personal `customer_id` on compatible discovery requests.
When no suitable identifier exists, call `DiscoveryWorkflow.RandomID` and
persist the returned random identifier for that user. When proxying has been
separately approved, supply `country_code` and `region` so GIPHY can enforce
regional content policy.

GIF/Sticker responses contain analytics URLs for `onload`, `onclick`, and
`onsent`. Call `GIF.TrackingURL` for the relevant event, then pass that URL and
the same `customer_id` to `AnalyticsWorkflow.Register`. Registration preserves
the signed analytics payload, adds a Unix-millisecond timestamp, and rejects
any origin or path outside the configured GIPHY analytics endpoint.

## Upload behavior

`UploadWorkflow.Upload` streams the declared number of bytes as the `file`
multipart field without buffering the complete asset. The operation accepts an
animated GIF (`image/gif`) or video MIME, rejects short and long readers, caps
the declared size at the documented 100MB maximum, and returns the new GIPHY
media ID. `username` should only be sent for a production-approved channel.

The adapter evaluated the community `github.com/peterhellberg/giphy` client but
does not depend on it: its core implementation predates current customer ID,
analytics, policy, upload, response-meta, and expanded rendition contracts.
Using the repository's shared transport keeps request bounds, errors, call
options, and redirect handling consistent with other adapters.

## Official documentation

- <https://developers.giphy.com/docs/api/>
- <https://developers.giphy.com/docs/api/endpoint/>
- <https://developers.giphy.com/docs/api/schema/>
- <https://developers.giphy.com/docs/>
