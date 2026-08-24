# Strava API v3 adapter

Package `social-hub/adapters/strava` targets the official Strava API v3. The
contract was verified against the public Swagger and developer documentation on
2026-08-03.

## Implemented contracts

| Surface | Support |
|---|---|
| Common `Fetcher` | Authenticated athlete, owned activity, page-number activity history, and cursor-paginated comments |
| Typed `ActivityWorkflow` | Manual activity creation and owned activity updates |
| Typed `ActivityUploadWorkflow` | Streaming FIT, TCX, GPX, compressed variants, and limited strength-training JSON uploads; upload status polling |
| Common `WebhookHandler` | Application challenge plus athlete/subscription-scoped activity and deauthorization event decoding |
| Common `Publisher` | Not exposed because a portable post lacks sport type, local start time, and elapsed time |
| Common `MediaUploader` | Not exposed because an activity file creates an activity asynchronously rather than an independent media object |
| Reactions and messages | Not exposed; API v3 lists comments and kudoers but does not publish comment/kudos mutations or direct messaging |

All API numeric IDs are decoded without `float64` conversion. Common activities
use the Strava activity ID as `Post.ID`, the athlete ID as `AuthorID`, the
description (falling back to the activity name) as `Text`, and preserve sport,
distance, timing, visibility, device, and source data in `strava.activity`.
Distance, moving time, elevation gain, kudos, and comment counts are also mapped
as explicitly defined common metrics. Callers displaying activity source data
must follow Strava's device attribution and brand requirements.

## Configuration

```yaml
adapter: strava/api-v3
product: api-v3
accounts:
  - id: primary-athlete
    client_id: "12345"
    secret_ref: env://STRAVA_CLIENT_SECRET
    access_token_ref: env://STRAVA_ACCESS_TOKEN
    settings:
      athlete_id: "123456789"
      subscription_id: 2468
    webhook:
      token_ref: env://STRAVA_WEBHOOK_VERIFY_TOKEN
    approval:
      scopes: [read, activity:read_all, activity:write]
```

`access_token_ref` and a positive decimal `settings.athlete_id` are required.
`settings.subscription_id` and `webhook.token_ref` are optional but must be
configured together. One Strava application can have only one webhook
subscription; the account subscription ID is used to reject deliveries routed
to the wrong application subscription.

`client_id` and `secret_ref` are only required when using `Adapter.OAuth`.
Strava access tokens expire after roughly six hours. Every successful refresh
can return a replacement refresh token, so callers must atomically persist the
new access and refresh tokens and always use the most recently returned refresh
token.

The OAuth helper accepts these current scopes:

- `read` and `read_all`
- `profile:read_all` and `profile:write`
- `activity:read`, `activity:read_all`, and `activity:write`

An empty configured approval scope list defers permission checks to Strava. A
non-empty list is enforced before requests; `activity:read_all` satisfies the
adapter's `activity:read` requirement.

## Activity workflows

Manual creation requires `name`, `sport_type`, an RFC 3339 local start time, and
a positive whole-second elapsed time. It also supports description, distance,
trainer, and commute fields. Updates support name, sport type, description,
trainer, commute, home-feed mute, and gear selection. API v3 does not expose
activity deletion.

Activity file uploads are streamed through multipart encoding and enforce the
declared byte count without buffering the full file. Supported `data_type`
values are `fit`, `fit.gz`, `tcx`, `tcx.gz`, `gpx`, `gpx.gz`, and `json`.
Strava's JSON format is currently limited to structured `WeightTraining`,
`HighIntensityIntervalTraining`, `Workout`, and `Crossfit` activities. Uploads
are asynchronous; poll `GetUpload` no more frequently than once per second and
wait for `ActivityID` or an `Error` value.

Activity history uses Strava's numeric `page` cursor and the API's `before` and
`after` epoch filters. Because Strava does not return total-page metadata, a
full page produces a next-page cursor; callers stop after receiving a short or
empty page. Comment pagination replays the opaque `after_cursor` returned on the
last comment.

## Webhook security boundary

Strava validates subscription ownership only during the GET challenge by
echoing the configured `hub.verify_token`; the adapter returns the documented
JSON object containing `hub.challenge`.

Strava does **not** document a signature or shared secret on subsequent POST
events. `WebhookHandler.Verify` therefore provides structural validation and
constant account filtering only: it requires a bounded non-empty POST body and
matches `owner_id` plus `subscription_id`. It is not cryptographic
authentication. Deploy the callback over HTTPS, restrict routing where
possible, acknowledge valid events within two seconds, process asynchronously,
and deduplicate the deterministic SDK event ID. Strava retries failed pushes up
to three total attempts.

## Rate limits and access tiers

Default application limits are 200 overall requests per 15 minutes and 2,000
per day, plus a separate non-upload limit of 100 per 15 minutes and 1,000 per
day. The natural 15-minute windows reset at minutes 0, 15, 30, and 45; the daily
window resets at midnight UTC. Inspect `X-RateLimit-*` and
`X-ReadRateLimit-*` on responses. The adapter classifies `429` as retryable and
preserves `Retry-After` when present.

New applications start with one-athlete Single Player capacity. The settings
dashboard can grant a self-service 10-athlete tier with higher published limits;
more than ten connected athletes requires application review. Creating an API
application currently requires a Strava subscription.

The current base URL is `https://www.strava.com/api/v3`. Strava has announced
`https://api-v3.strava.com` for availability starting 2027-01-04; `settings.base_url`
is intentionally configurable for that transition without changing this
adapter identity.

No external Go SDK is linked. The archived historical `strava/go.strava`
client is not used, and the adapter relies on the repository's bounded bearer
transport and protocol fixtures.

## Official documentation

- <https://developers.strava.com/docs/reference/>
- <https://developers.strava.com/docs/authentication/>
- <https://developers.strava.com/docs/uploads/>
- <https://developers.strava.com/docs/webhooks/>
- <https://developers.strava.com/docs/rate-limits/>
- <https://developers.strava.com/docs/changelog/>
- <https://developers.strava.com/swagger/swagger.json>
