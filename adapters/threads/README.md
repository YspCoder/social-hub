# Meta Threads API adapter

Package `social-hub/adapters/threads` implements the official Meta Threads
API. The default Graph endpoint is `https://graph.threads.com`, and OAuth
authorization uses `https://www.threads.com/oauth/authorize`. The adapter does
not use the former `.net` hosts, Instagram credentials, cookies, browser
automation, or private endpoints.

The default `base_url` is intentionally unversioned. Threads apps may select a
Graph API version in Meta's app settings, or explicitly include a supported
version in `base_url`. This avoids pinning the SDK to a version that differs
from the app's configured migration state.

Implemented contracts:

- authorization-code exchange, long-lived-token exchange, and refresh;
- configured profile, individual Thread, own Threads, and direct-reply reads;
- auto-published text posts, replies, quotes, deletion, and publish status;
- typed remote image, video, and 2-20 item carousel container lifecycle;
- post and account insights with scalar or structured metric values preserved;
- approved public-profile/posts lookup, keyword search, and mentions;
- reply hide/unhide, pending-reply review, reposts, and dynamic publishing
  quota reads.

The common `Publisher` is limited to text because `CreatePostRequest.MediaIDs`
represents already-uploaded media. Threads instead fetches public HTTPS media
URLs into asynchronous containers. Use `ContainerWorkflow` for media, poll,
link, topic, location, spoiler, ghost-post, reply-control, and reply-approval
features. Create carousel child containers with `CarouselItem: true`, then
create a carousel parent with 2-20 returned child IDs. Poll and link
attachments are mutually exclusive.

`MediaUploader`, `Messenger`, and `WebhookHandler` are disabled. Threads does
not accept byte uploads or expose direct messaging through this API. A webhook
surface is not advertised until an official, testable payload and signature
contract is available. The `Reactor` maps common comments to text replies, but
does not claim like mutation. Reposts use `RepostWorkflow`; retain its returned
repost ID because deletion targets that ID, not the original Thread ID.

## Authentication and permissions

Use the app ID and app secret from a Threads use-case app. They are not
interchangeable with unrelated Meta or Instagram app credentials. Persist the
app-scoped Threads `user_id` returned by the short-lived token exchange in
`account.settings.user_id`, then store the access token behind
`access_token_ref`. The adapter resolves secrets at runtime and never writes
rotated tokens back to configuration.

Relevant permissions are capability-specific:

| Capability | Permissions |
|---|---|
| Common profile and own Threads | `threads_basic` |
| Direct replies | `threads_read_replies` |
| Publish, reply, quote, containers, repost, quota | `threads_content_publish` |
| Delete posts and replies | `threads_delete` |
| Post and account insights | `threads_manage_insights` |
| Public profile and profile posts | `threads_profile_discovery` |
| Keyword search | `threads_keyword_search` |
| Mentions | `threads_manage_mentions` |
| Reply moderation and approval | `threads_manage_replies` |
| Location tagging | `threads_location_tagging` |

When `approval.scopes` is configured, the client rejects a typed operation
before network I/O if its permission is absent. An empty scope list means the
grant is unknown, so Meta remains the authority and may return an approval
error.

Example account settings:

```yaml
adapter: threads/api
accounts:
  - id: threads-main
    client_id: ${THREADS_APP_ID}
    secret_ref: env://THREADS_APP_SECRET
    access_token_ref: env://THREADS_ACCESS_TOKEN
    approval:
      scopes:
        - threads_basic
        - threads_content_publish
        - threads_read_replies
        - threads_delete
    settings:
      user_id: "1234567890"
```

For media publication, provide public HTTPS URLs that Meta can fetch. The API
does not accept local paths or media bytes through the container workflow.
Poll container status until it reaches a terminal state before publishing.

Publishing limits are returned by `PublishingQuotaWorkflow`. Treat those
server-provided totals and durations, HTTP 429, and `Retry-After` as the source
of truth rather than hard-coding a global quota.

All current tests use deterministic local HTTP fixtures. The adapter has not
yet been validated with a real Threads app or account.

Official resources:

- <https://developers.facebook.com/docs/threads/>
- <https://developers.facebook.com/docs/threads/get-started/get-access-tokens-and-permissions>
- <https://developers.facebook.com/docs/threads/posts/>
- <https://developers.facebook.com/docs/threads/retrieve-and-discover-posts/>
- <https://developers.facebook.com/docs/threads/insights/>
- <https://developers.facebook.com/docs/threads/reply-management/>
- <https://developers.facebook.com/docs/threads/publishing-limit/>
- <https://github.com/fbsamples/threads_api>
