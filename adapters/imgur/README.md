# Imgur API v3 adapter

Package `social-hub/adapters/imgur` targets Imgur's official API v3 at
`https://api.imgur.com/3`. It does not use cookies, scraping, browser
automation, private endpoints, or deprecated OAuth code/PIN flows.

## Access and commercial-use boundary

Every application must register for an Imgur `client_id`. Public reads,
anonymous uploads, and anonymous albums use `Authorization: Client-ID`; account
and Gallery mutations require an OAuth2 Bearer token.

Imgur classifies an application as commercial when it earns or plans to earn
money, includes advertising, or belongs to a commercial organization. Such
applications must register with Imgur and use Imgur's official RapidAPI plan,
whose endpoint and mandatory `X-Mashape-Key` differ from the public API. This
adapter currently does not model that commercial header, so its default
transport must not be treated as commercial-use support.

Imgur must not be used as a generic CDN, and applications remain responsible
for Imgur's API Terms, content policy, attribution, caching, deletion, and
user-consent requirements.

## Implemented contracts

| Surface | Support |
|---|---|
| Common `Fetcher` | Public account/image reads, Bearer account-image pages, and flattened Gallery comments |
| Common `Publisher` | Shares exactly one existing uploaded image to the public Gallery; deletion only removes it from Gallery |
| Common `Reactor` | Gallery up-vote/veto, comment/reply creation, and comment deletion |
| Typed `ImageWorkflow` | Public reads, anonymous or account streaming upload, metadata update, deletion, and favorite toggle |
| Typed `AlbumWorkflow` | Public reads plus anonymous or account create/update/delete |
| Typed `GalleryWorkflow` | Image sharing, Gallery removal, and explicit up/down/veto votes |
| Typed `CreditWorkflow` | Current application and user credit counters from `/credits` |
| Common `MediaUploader` | Not exposed; Imgur uses one multipart request instead of a detached/resumable media lifecycle |
| Messaging / webhooks | Not exposed by the documented API v3 contracts |

Imgur's favorite endpoint is toggle-only, so it is intentionally not mapped to
common `ReactionLike`. Common likes use the directionally explicit Gallery
`up` and `veto` vote operations.

## Configuration

```yaml
adapter: imgur/v3
accounts:
  - id: publisher
    client_id: "imgur-client-id"
    secret_ref: env://IMGUR_CLIENT_SECRET
    access_token_ref: env://IMGUR_ACCESS_TOKEN
    settings:
      username: "imgur-username"
```

`client_id` is always required. `access_token_ref` is optional; omitting it
creates a public/anonymous client. `secret_ref` is needed only when using
`Adapter.OAuth` to refresh user tokens. `settings.username` selects the account
used by common `GetUser` and `ListPosts`; authenticated calls fall back to
Imgur's `me` alias when it is omitted.

Anonymous uploads and albums return deletehashes. A deletehash is the only
credential that can update or delete that anonymous resource: persist it
securely at creation time, never expose it as a public object ID, and do not
assume it can be recovered later.

## OAuth behavior

`OAuthClient.AuthorizationURL` generates Imgur's currently documented
`response_type=token` flow. The browser returns access and refresh values in
the redirect URI fragment; server-side callback handlers do not receive URL
fragments, so the application must extract them in a trusted client and pass
them to its backend securely. The adapter deliberately does not implement the
deprecated authorization-code or PIN flows.

`OAuthClient.Refresh` sends the refresh-token grant to
`https://api.imgur.com/oauth2/token`. Imgur access tokens normally expire after
one month; refresh tokens do not publish an expiry. Store both as secrets and
replace the persisted refresh token if Imgur returns a rotated value.

## Upload and Gallery behavior

`ImageWorkflow.Upload` streams exactly the declared byte count as the `image`
multipart field without buffering the full image or video. Short and long
readers are rejected. Anonymous uploads use `Client-ID`; configured user
uploads use Bearer auth.

Sharing to Gallery sends `terms=1`, which bypasses Imgur's not-yet-accepted
terms error. Applications must obtain and record the user's acceptance before
calling `ShareImage` or common `Publish`; the SDK cannot establish legal
consent on the application's behalf. Removing a Gallery post preserves the
underlying image. Deleting the source image is a separate `DeleteImage` call.

## Rate credits

Imgur uses credits rather than a single request window. The published baseline
is approximately 12,500 ordinary requests or 1,250 uploads per application per
day; ordinary calls usually cost one credit and uploads cost ten. A separate
IP-wide limit permits 1,250 POST requests per hour for non-commercial access.
Each response exposes remaining user, client, and POST credits in rate-limit
headers, while `CreditWorkflow.Credits` provides the current user/client
counters. Limits are policy inputs and must remain configurable rather than
hard-coded into a generic limiter.

## Official documentation

- <https://apidocs.imgur.com/>
- <https://api.imgur.com/oauth2/addclient>
- <https://api.imgur.com/endpoints/gallery>
