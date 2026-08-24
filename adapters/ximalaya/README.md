# Ximalaya Open Platform API adapter

Adapter name: `ximalaya/open-api-v1`

This package implements a bounded, read-only content subset of the current
Ximalaya Open Platform server API. Every request uses the documented
`server_api_version=1.0.0` contract and the HTTPS primary origin
`https://api.ximalaya.com`.

| Typed workflow | Official endpoint | Pagination |
| --- | --- | --- |
| `ListCategories` | `GET /categories/list` | none |
| `ListAlbums` | `GET /v2/albums/list` | page/count, count 1..50 |
| `BrowseAlbum` | `GET /albums/browse` | page/count, count 1..200 |
| `SearchAlbums` | `GET /v2/search/albums` | page/count, count 1..50, first 5,000 results |
| `SearchTracks` | `GET /v2/search/tracks` | page/count, count 1..50, first 5,000 results |

The selected routes were independently confirmed live on 2026-08-25: each
returned the documented JSON error envelope for a request missing `app_key`.
Paid purchasing, user OAuth data, recommendations, radio, distribution,
uploads, writes, incremental synchronization, push callbacks, automatic
pagination, and data-reporting endpoints are outside this adapter.

## Access approval

An API key alone is not sufficient for production use. Ximalaya's current
official onboarding material requires all of the following:

- agree the permitted integration scope with Ximalaya's business team;
- create an application whose technical access method is `API` and submit it
  for review;
- obtain `serverAuthenticateStaticKey` and configure the calling server's IP
  allowlist in the application review settings;
- report the real device identity and `client_os_type` for the integration;
- integrate and validate playback, album-browse, and album-exposure reporting
  before submitting a pure API integration for launch approval.

This read-only package does not implement those three mandatory reporting
flows. Using it does not by itself satisfy Ximalaya's launch requirements.
Error `102` or `111` is surfaced as `CodeApprovalRequired`; signature, app,
static-key, time, nonce, or IP-allowlist failures remain user-action errors
with the official onboarding-requirements URL attached.

## Configuration

`client_id` contains the public `app_key`. `secret_ref` resolves the private
`app_secret`. `access_token_ref` resolves `serverAuthenticateStaticKey`; it is
used as a second secret reference here, not as an OAuth access token.

```yaml
version: 1
platforms:
  - adapter: ximalaya/open-api-v1
    product: open-api
    accounts:
      - id: approved-content
        client_id: your-app-key
        secret_ref: env://XIMALAYA_APP_SECRET
        access_token_ref: env://XIMALAYA_SERVER_AUTH_STATIC_KEY
        settings:
          client_os_type: 4
          device_id: 19AAB430-9CB8-4325-ACC5-D7D386B68960
          device_id_type: UUID
```

The example uses pure server API type `4`. For a client-originated request
relayed through a server, configure the actual client type documented by
Ximalaya instead. Device identity must be stable and must follow the official
OAID, Android ID, IDFA, or UUID rules; the adapter does not invent or persist a
device ID.

## Request signing

For each call the adapter:

1. adds `app_key`, `client_os_type`, `device_id`, `device_id_type`, a fresh
   cryptographically random `nonce`, the current Unix millisecond `timestamp`,
   and `server_api_version=1.0.0`;
2. sorts every parameter except `sig` by parameter name and joins the original,
   unescaped values as `key=value&...`;
3. Base64-encodes the UTF-8 canonical string;
4. computes HMAC-SHA1 using `app_secret + serverAuthenticateStaticKey` and
   keeps the raw 20-byte result;
5. computes lowercase MD5 over those raw bytes to produce `sig`;
6. URL-encodes the query once for transport.

Neither signing secret is transmitted. Redirects and cookies are disabled,
transport errors discard request URLs, and error bodies recursively redact
credential, device, nonce, and signature fields. Successful bodies are scanned
for both private signing values before their bounded JSON is exposed as `Raw`.

## Usage

```go
base, err := adapter.Client(ctx, "approved-content")
if err != nil {
	return err
}
client := base.(*ximalaya.Client)

albums, err := client.Ximalaya().ListAlbums(ctx, ximalaya.ListAlbumsRequest{
	CategoryID: 6,
	Dimension:  ximalaya.AlbumsHot,
	Page:       1,
	Count:      20,
})
if err != nil {
	return err
}
```

Search enforces the official first-5,000 result boundary locally.
`BrowseAlbum` deliberately omits the optional
`access_token` and `third_uid` inputs because user purchase state and OAuth
privacy data are outside this adapter.

IDs use signed 64-bit integers matching the provider's `Int`/`Long` JSON
contract. Core categories, albums, tracks, pages, and complete success bodies
retain bounded provider JSON. Audio URLs and `can_download=true` do not grant
permission to cache media on a server; the official Album and Track model
documentation permits terminal download only and explicitly forbids server
download or caching.

## Quotas and errors

The official common-error document, updated 2026-03-30, currently states one
shared per-application allowance across all endpoints: 5,000 requests per
minute and 280,000 requests per hour. Provider error `104` maps to retryable
`CodeRateLimited`. Error `110` is a user-action risk-control limit and is not
automatically retryable. The platform does not document remaining-quota or
reset headers, so the adapter does not invent them or hard-code a limiter.

Ximalaya business failures can use HTTP 200 with `error_no`, `error_code`,
`error_desc`, and optional `service`. The adapter checks this envelope before
decoding every successful HTTP response. Provider errors and retained error
JSON are bounded to 64 KiB; success JSON is bounded to 8 MiB.

The official guide also lists `https://apihera.ximalaya.com` as a backup data
center. This package does not switch origins automatically because replaying a
signed nonce/timestamp pair is forbidden and failover policy belongs to the
calling application.

## Official sources

Official material reviewed on 2026-08-25:

- <https://open.ximalaya.com/api-docs>
- <https://open.ximalaya.com/api-docs/document?id=67> (common parameters)
- <https://open.ximalaya.com/api-docs/document?id=68> (device identity)
- <https://open.ximalaya.com/api-docs/document?id=69> (signature algorithm)
- <https://open.ximalaya.com/api-docs/document?id=70> (API access guide)
- <https://open.ximalaya.com/api-docs/document?id=107> (pre-access requirements)
- <https://open.ximalaya.com/api-docs/document?id=6> (free on-demand content)
- <https://open.ximalaya.com/api-docs/document?id=26> (content search)
- <https://open.ximalaya.com/api-docs/document?id=37> (data models)
- <https://open.ximalaya.com/api-docs/document?id=38> (errors and quotas)

This package uses only the Go standard library in addition to social-hub's
existing internal transport.
