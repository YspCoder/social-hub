# Dribbble API v2 adapter

Package `social-hub/adapters/dribbble` targets the official Dribbble API v2.
This is a Publishing API for the authorized user's own profile and content; it
is not a discovery, designer-search, hiring, or aggregate-feed API.

## Implemented contracts

| Surface | Support |
|---|---|
| Common `Fetcher` | Authorized user, owned Shot lookup, and owned Shot pagination |
| Typed `ShotWorkflow` | Asynchronous image Shot creation, metadata update, and deletion |
| Typed `ProjectWorkflow` | Authorized-user Project listing and CRUD |
| Typed `AttachmentWorkflow` | Asynchronous attachment upload and deletion |
| Common `Publisher` / `MediaUploader` | Not exposed because publishing requires Dribbble-specific multipart fields and eligibility checks |
| Reactions, comments, feeds, messages, webhooks | Not exposed by API v2 |

API v2 removed the public v1 discovery/feed, comment, like, follower, and
rebound interaction endpoints. The adapter therefore returns
`CodeUnsupported` for common comment listing and does not claim the related
capabilities. Video fields returned by Shot reads are mapped to common media,
but the API does not support creating video Shots.

## Configuration and OAuth

```yaml
adapter: dribbble/v2
product: publishing-api
accounts:
  - id: designer
    client_id: "dribbble-client-id"
    secret_ref: env://DRIBBBLE_CLIENT_SECRET
    access_token_ref: env://DRIBBBLE_ACCESS_TOKEN
    settings:
      user_id: "123456"
    approval:
      scopes: [public, upload]
```

`access_token_ref` and the positive authorized-user `settings.user_id` are
required for an API client. `client_id` and `secret_ref` are needed when using
`Adapter.OAuth`. The helper implements Dribbble's OAuth 2.0 web authorization
code flow with `public` and `upload` scopes. Dribbble does not document refresh
tokens, so expired or revoked access requires authorization again. API and
token clients reject redirects to prevent credentials from being forwarded to
another origin.

## Publishing constraints

- Shot creation requires `upload` and a player or team account. Images must be
  GIF, JPEG, or PNG; exactly 400x300 or 800x600; and no larger than 8 MiB.
- The SDK streams uploads, verifies the caller-declared byte length, and leaves
  pixel-dimension validation to Dribbble so it does not buffer the image.
- Shot creation is asynchronous (`202 Accepted` plus `Location`). The resource
  can return `404 Not Found` until processing completes.
- Scheduled publishing additionally requires a Pro user, team, or team member.
  A Shot supports no more than 12 tags.
- Attachment creation requires ownership plus Pro/team eligibility. Files are
  limited to 10 MiB and are also accepted asynchronously without a resource ID.

Platform eligibility is not inferable from OAuth scopes alone. A server-side
permission or account-tier rejection is preserved as a typed `socialhub.Error`
so callers can distinguish it from a retryable transport or quota failure.

## Rate limits and policy

OAuth requests are limited to 60 per minute and 1,440 per day per authenticated
user. `Client.RateLimit()` records the latest `X-RateLimit-Limit`,
`X-RateLimit-Remaining`, and `X-RateLimit-Reset` snapshot. A `429` response is
retryable, using `Retry-After` or the reset timestamp when available.

Dribbble's API Terms prohibit products that compete with or replace Dribbble,
designer social networks, designer or hiring search tools, job services, and
scraping. User actions may be automated only when specifically requested by the
authenticated user. Applications remain responsible for content rights,
branding, removal requests, and current platform terms.

No external Go SDK is used. The available `github.com/pims/assist` package is
an incomplete client last released in 2016 for the retired v1 API; the adapter
instead uses the repository's bounded authenticated transport and the current
public v2 HTTP contracts.

## Official documentation

- <https://developer.dribbble.com/v2/>
- <https://developer.dribbble.com/v2/oauth/>
- <https://developer.dribbble.com/v2/shots/>
- <https://developer.dribbble.com/v2/projects/>
- <https://developer.dribbble.com/v2/attachments/>
- <https://developer.dribbble.com/terms/>
