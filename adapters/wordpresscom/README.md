# WordPress.com REST API v1.1 adapter

Package `social-hub/adapters/wordpresscom` targets the official WordPress.com
REST API v1.1. It supports WordPress.com sites and self-hosted WordPress sites
connected through Jetpack. It does not target the separate WordPress Core
`/wp-json/wp/v2` contract.

## Implemented contracts

| Surface | Support |
|---|---|
| Common `Fetcher` | Current OAuth user, site Posts, individual Posts, and Post comments |
| Common `Publisher` | Text Posts with optional featured media and public/private/draft/pending status |
| Typed `PostWorkflow` | Title, content, excerpt, slug, scheduling, categories, tags, featured image, Publicize, discussion controls, update, and restore |
| Common `MediaUploader` | Single-part streaming upload to the configured site's media library plus status lookup |
| Typed `MediaLibraryWorkflow` | Permanent media deletion |
| Common `Reactor` | Post like/unlike and comment/reply create/delete |
| Typed `SiteWorkflow` | Metadata for the configured WordPress.com or Jetpack-connected site |
| Messages and webhooks | Not exposed by this REST API adapter |

The raw platform objects remain available under `wordpress.user`,
`wordpress.post`, `wordpress.comment`, and `wordpress.media` extensions. Post
HTML is preserved in the common `Text` field; the adapter does not strip or
rewrite markup. Attachment maps are sorted before common-model mapping so
results remain deterministic.

## Configuration and OAuth

```yaml
adapter: wordpress.com/rest-v1.1
product: rest-api
accounts:
  - id: primary-blog
    client_id: "wordpress-com-client-id"
    secret_ref: env://WORDPRESS_COM_CLIENT_SECRET
    access_token_ref: env://WORDPRESS_COM_ACCESS_TOKEN
    settings:
      site: "example.wordpress.com"
      user_id: "123456"
    approval:
      scopes: [users, sites, posts, comments, media]
```

`settings.site` is required and accepts a positive site ID or hostname.
`settings.user_id` is optional and, when present, pins current-user responses
and reaction actors to a positive WordPress.com user ID.

`access_token_ref` is optional. Without it, the adapter exposes public site,
Post, and comment reads. OAuth is required for the current user, private
content, publishing, media, likes, and comments. When configured scopes are
present, the adapter checks granular permissions locally; `global` satisfies
all capabilities and `auth` satisfies the current-user read. An empty scope
list defers authorization decisions to WordPress.com.

`Adapter.OAuth` implements the OAuth2 Authorization Code flow and includes the
configured site as the `blog` authorization parameter. The token result
preserves the returned `blog_id` and `blog_url` alongside the common token.
WordPress.com does not document refresh-token issuance for this flow, so the
adapter does not expose a speculative refresh method. API and OAuth clients
reject redirects to avoid forwarding credentials to another origin.

## Posts, pagination, and media

- Common publishing maps at most one media ID to `featured_image`. Use
  `PostWorkflow` when title, taxonomy, scheduling, Publicize, or discussion
  settings are required.
- WordPress Posts are not reply or quote entities. Use `Reactor.Comment` for
  discussion replies.
- Post pagination preserves WordPress.com's opaque `page_handle`; callers must
  store and replay it without interpreting it. Comment pagination uses a
  positive page number.
- Media uploads stream one multipart field named `media[]`, enforce the
  caller-declared byte length, and use common upload part number `0`.
- Allowed file types and maximum sizes can vary by site plan, site settings,
  and current WordPress.com policy. The adapter validates the MIME/media family
  but leaves site-specific acceptance to the API and preserves its error.

## Rate limits and responsible use

WordPress.com does not publish a fixed numeric REST API quota. Automattic asks
clients to avoid excessive calls and may throttle abusive traffic. The adapter
classifies `429` as retryable and preserves a numeric `Retry-After` value when
present; applications should still cache stable site metadata and apply their
own per-account request budget.

Applications must request only data needed for their stated purpose, keep
cached data current, respect privacy and deletion changes, avoid spam or
automated unwanted interactions, and comply with Automattic's current API
terms and responsible-use guidance.

No external Go SDK is used. Available packages primarily target self-hosted
WordPress Core REST v2 routes and do not match WordPress.com OAuth, v1.1 site
routing, response wrappers, or error contracts. This adapter therefore reuses
the repository's bounded authenticated transport without adding a dependency.

## Official documentation

- <https://developer.wordpress.com/docs/api/>
- <https://developer.wordpress.com/docs/api/getting-started/>
- <https://developer.wordpress.com/docs/api/oauth2/>
- <https://developer.wordpress.com/docs/api/rest-api-reference/>
- <https://developer.wordpress.com/docs/api/1.1/get/sites/%24site/posts/>
- <https://developer.wordpress.com/docs/api/1.1/post/sites/%24site/posts/new/>
- <https://developer.wordpress.com/docs/api/1.1/post/sites/%24site/media/new/>
- <https://developer.wordpress.com/docs/api/guidelines-for-responsible-use-of-automattics-apis/>
