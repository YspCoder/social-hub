# Patreon API v2 adapter

Package `social-hub/adapters/patreon` targets the official Patreon API v2.
It models a creator account around one configured Campaign and does not use the
deprecated API v1 resource model.

## Implemented contracts

| Surface | Support |
|---|---|
| Common `Fetcher` | Authorized identity, individual Campaign Posts, and cursor-paginated Campaign Posts |
| Typed `CampaignWorkflow` | Configured Campaign lookup and owned Campaign pagination |
| Typed `MemberWorkflow` | Campaign Member lookup, pagination, payment state, and entitled tier IDs |
| Common `WebhookHandler` | Signed member, pledge, and Post event verification and decoding |
| Common `Publisher` | Not exposed because Patreon API v2 has no creator Post publishing endpoint |
| Comments and reactions | Not exposed by Patreon API v2 |
| Media uploads and messages | Not exposed by Patreon API v2 |

Post HTML is preserved in common `Post.Text`. Embedded media URLs are mapped as
remote document media without guessing the provider's MIME type. Raw JSON:API
resources remain available under `patreon.user` and `patreon.post` extensions,
and typed Campaign/Member models preserve their raw resource objects.

## Configuration and OAuth

```yaml
adapter: patreon/api-v2
product: api
accounts:
  - id: creator
    client_id: "patreon-v2-client-id"
    secret_ref: env://PATREON_CLIENT_SECRET
    access_token_ref: env://PATREON_ACCESS_TOKEN
    settings:
      campaign_id: "123456"
      user_id: "7890"
    webhook:
      secret_ref: env://PATREON_WEBHOOK_SECRET
    approval:
      scopes: [identity, campaigns, campaigns.posts, campaigns.members]
```

`settings.campaign_id` is required and selects the Campaign used by common Post
and typed Member workflows. `settings.user_id` is optional and pins identity
responses and common author filters to the expected creator.

`access_token_ref` is optional so a process can run as a webhook-only receiver.
Without it, API fetch capabilities are not exposed. `webhook.secret_ref` is
also optional and independently controls the inbound webhook capability.
`client_id` and `secret_ref` are required only when using `Adapter.OAuth`.

The OAuth helper implements the API v2 Authorization Code flow and rotating
Refresh Token grant. Patreon returns a new single-use refresh token on refresh;
callers must atomically replace both stored tokens. API and OAuth clients reject
redirects to avoid forwarding credentials to another origin.

Important scopes include:

- `identity` for the authorized user.
- `campaigns`, `campaigns.posts`, and `campaigns.members` for creator data.
- `campaigns.members[email]` for Member email. The adapter requests `email`
  only when this scope is explicitly recorded in account approval config.
- `campaigns.members.address` for postal data. Addresses are intentionally not
  requested or modeled by this adapter's default Member workflow.
- `w:campaigns.webhook` manages remote webhook registrations; it is not needed
  merely to verify an already configured webhook secret.

An empty approval scope list defers authorization decisions to Patreon. A
non-empty list is enforced locally before the request.

## JSON:API and pagination

API v2 requires explicit `fields[resource]` selection. The adapter requests only
fields used by its stable models and disables default included-resource trees.
Member identity fields can be `null` when a member has chosen to hide their
identity; this is treated as valid data rather than a mapping failure.

List operations use `page[count]` and replay Patreon's opaque `page[cursor]`
value without interpreting it. Common Post time filters return
`CodeUnsupported` because the Campaign Posts endpoint does not publish a time
filter contract. Patreon API v2 exposes Post reads but no Post comments API, so
`ListComments` also returns `CodeUnsupported`.

## Webhook security

Patreon signs the exact request body in `X-Patreon-Signature` using HMAC-MD5
with the webhook secret and identifies the trigger in `X-Patreon-Event`. The
adapter implements that mandated algorithm with constant-time comparison,
requires a bounded non-empty POST body, verifies before decoding, and returns a
deterministic SHA-256 Event ID derived from the trigger and body.

The Patreon webhook contract does not include a signed timestamp. Applications
must deduplicate Event IDs and maintain their own replay window or processed
event store. The raw JSON:API payload is preserved in `WebhookPayload.Raw`.

## Rate limits and API maturity

Patreon currently documents two standard limits: 100 requests per 2 seconds
per client and 100 requests per minute per access token. More than 2,000 HTTP
4xx responses in 10 minutes can trigger an edge block for 30 minutes. Limits
may change; the adapter classifies `429` as retryable and preserves
`Retry-After` or JSON `retry_after_seconds` for the shared retry layer.

API v1 is deprecated and new v1 client creation is restricted. Early-access
Live endpoints are deliberately not presented as stable capabilities here.

No external Go SDK is used. `mxpv/patreon-go` last released in 2017 and models
the deprecated v1 Campaign/Pledge API. Patreon's official `patreon-js` package
is also deprecated and directs integrations to use API v2 directly. Reusing
either would introduce stale scopes, routes, and data models, so this adapter
uses the repository's bounded authenticated transport.

## Official documentation

- <https://docs.patreon.com/>
- <https://docs.patreon.com/#api-v2-oauth>
- <https://docs.patreon.com/#apiv2-resource-endpoints>
- <https://docs.patreon.com/#api-v2-webhook-endpoints>
- <https://docs.patreon.com/#rate-limits>
- <https://docs.patreon.com/#migrating-from-api-v1-to-api-v2>
