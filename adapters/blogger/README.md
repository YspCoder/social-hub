# Google Blogger API v3 adapter

`adapters/blogger` is a bounded, read-only adapter for the current Google
Blogger API v3. It exposes provider-native Blog, Post, Comment, and Page
resources instead of mapping Blogger HTML and publication state into the
smaller cross-platform social models.

## Implemented surface

| Workflow | REST operation | Notes |
| --- | --- | --- |
| Get blog | `GET /v3/blogs/{blogId}` | Supports `maxPosts` and typed `view` |
| Resolve blog URL | `GET /v3/blogs/byurl` | Accepts an absolute HTTP(S) blog URL |
| List user blogs | `GET /v3/users/{userId}/blogs` | Supports `self`, status, role, view, and `fetchUserInfo` |
| Get post | `GET /v3/blogs/{blogId}/posts/{postId}` | Optional body, images, comments, and view |
| Resolve post path | `GET /v3/blogs/{blogId}/posts/bypath` | Path remains a query parameter, never a request route |
| List posts | `GET /v3/blogs/{blogId}/posts` | Typed status, dates, order, sort, labels, and opaque pagination |
| Search posts | `GET /v3/blogs/{blogId}/posts/search` | Provider-native query and order |
| Get comment | `GET /v3/blogs/{blogId}/posts/{postId}/comments/{commentId}` | Validates blog and post ownership |
| List post comments | `GET /v3/blogs/{blogId}/posts/{postId}/comments` | Typed status, dates, view, and opaque pagination |
| List blog comments | `GET /v3/blogs/{blogId}/comments` | Lists comments across a blog |
| Get page | `GET /v3/blogs/{blogId}/pages/{pageId}` | Static Blogger page resource |
| List pages | `GET /v3/blogs/{blogId}/pages` | Typed status, view, and opaque pagination |

The adapter intentionally excludes `UserInfo`, `PostUserInfo`, `BlogUserInfo`,
`PageViews`, and every write or moderation method. In particular, post and page
insert, update, patch, delete, publish, and revert operations have destructive
or state-transition semantics and no general idempotency-key contract.
Comment approval, spam marking, content removal, and deletion are likewise not
presented as a partial `Reactor` implementation.

`fetchUserInfo` is retained because it is a documented `blogs.listByUser`
query option. The provider-specific user information remains available only in
the sanitized list `Raw` value; this package does not claim a typed UserInfo
workflow.

## Authentication

The adapter accepts an externally managed OAuth 2.0 Bearer access token. The
least-privilege scope for this read-only surface is:

```text
https://www.googleapis.com/auth/blogger.readonly
```

Google's full Blogger management scope also authorizes reads:

```text
https://www.googleapis.com/auth/blogger
```

The management scope is not required or recommended for this adapter. When
`approval.scopes` is empty, capability approval is reported as unknown. When
scopes are recorded but contain neither Blogger scope, calls fail locally with
`approval_required` before an HTTP request.

The SDK does not implement the OAuth authorization redirect, authorization-code
exchange, refresh-token storage, or access-token refresh. `access_token_ref` is
resolved once at client creation through the configured
`socialhub.SecretResolver`.

```yaml
version: 1
platforms:
  - adapter: google/blogger-api-v3
    product: blogger-api
    settings:
      user_agent: social-hub/blogger
    accounts:
      - id: primary
        access_token_ref: env://BLOGGER_ACCESS_TOKEN
        approval:
          account_type: oauth2
          scopes:
            - https://www.googleapis.com/auth/blogger.readonly
```

Client IDs, app IDs, client-secret references, token stores, webhook settings,
and account-specific settings are outside this access-token contract and are
rejected from adapter configuration.

## Use

Importing the package registers `google/blogger-api-v3` with social-hub:

```go
import (
    "context"
    "fmt"

    "social-hub/adapters/blogger"
    "social-hub/pkg/socialhub"
)

func listRecentPosts(ctx context.Context) error {
    config := socialhub.AdapterConfig{
        Adapter: "google/blogger-api-v3",
        Product: "blogger-api",
        Accounts: []socialhub.AccountConfig{{
            ID:             "primary",
            AccessTokenRef: "env://BLOGGER_ACCESS_TOKEN",
            Approval: socialhub.ApprovalConfig{Scopes: []string{
                blogger.ScopeReadOnly,
            }},
        }},
    }

    adapter, err := socialhub.Open(ctx, "google/blogger-api-v3", config)
    if err != nil {
        return err
    }
    defer adapter.Close()

    generic, err := adapter.Client(ctx, "primary")
    if err != nil {
        return err
    }
    api := generic.(*blogger.Client).Blogger()

    posts, err := api.ListPosts(ctx, blogger.ListPostsRequest{
        BlogID:     "1234567890123456789",
        Status:     blogger.PostStatusLive,
        MaxResults: 25,
        OrderBy:    blogger.PostOrderPublished,
        Sort:       blogger.SortDescending,
    })
    if err != nil {
        return err
    }
    for _, post := range posts.Items {
        fmt.Println(post.ID, post.Title)
    }
    return nil
}
```

Boolean request fields use pointers so `false` is distinct from omission.
`startDate` and `endDate` must be RFC 3339 timestamps and are validated in
chronological order. `pageToken` and returned page tokens are opaque bounded
values: the adapter neither parses them nor automatically fetches another
page.

## Data, HTML, and links

Blogger post, page, and comment content can contain provider-supplied HTML.
This adapter preserves that content as data; applications must apply a
context-appropriate HTML sanitizer and escaping policy before rendering it.

Every entity and list retains bounded, recursively sanitized provider JSON in
`Raw`. `Raw` may contain private blog content and does not create a separate
authorization to cache, republish, or reuse Google user data. Applications
remain responsible for consent, retention, deletion, and Google API Services
User Data Policy obligations.

Provider fields such as `url`, `selfLink`, author links, and image URLs are
preserved as strings. The adapter does not follow or fetch any of those URLs.

## Quotas, errors, and transport security

Blogger quota is controlled by the caller's Google Cloud project and can vary
by project, endpoint, approval, and current Google policy. The Discovery
document does not define a stable numeric quota for this SDK to enforce.
Google Cloud Console, provider responses, and current official documentation
are authoritative. Optional quota and rate-limit response headers are retained
dynamically in `ResponseMeta.QuotaHeaders`; no remaining quota is inferred
when Google omits such headers.

`APIError` wraps a platform-neutral `socialhub.Error` while retaining Google's
standard `error.code`, `error.message`, `error.status`, typed `details`, and
legacy `errors` values. `Retry-After` supports delta seconds and HTTP dates;
`google.rpc.RetryInfo.retryDelay` is used when no usable header delay exists.
Permission failures include the required read-only scope and official
authorization documentation URL.

Successful responses must be bounded JSON objects with a JSON content type.
A top-level Google `error` object is treated as an error even with HTTP 200.
Resource kinds, IDs, blog/post ownership, and pagination tokens are checked
before values are returned. Provider JSON is recursively sanitized for the
exact configured access token and explicit credential keys without rewriting
ordinary article text that happens to discuss tokens or API keys.

The API origin is fixed to `https://blogger.googleapis.com/`. The adapter sends
the token only as `Authorization: Bearer`, clones the supplied HTTP client,
removes its cookie jar, and disables redirects so credentials cannot be
forwarded to a `Location` origin. Transport errors discard request URLs.

## Official sources

Official material reviewed on 2026-08-25:

- <https://blogger.googleapis.com/$discovery/rest?version=v3>
- <https://developers.google.com/blogger/docs/3.0/getting_started>
- <https://developers.google.com/blogger/docs/3.0/using>
- <https://developers.google.com/blogger/docs/3.0/reference>
- <https://github.com/googleapis/google-api-go-client/blob/main/blogger/v3/blogger-api.json>

The reviewed Discovery document identifies `blogger:v3`, revision `20260816`.
This package adds no third-party dependency.
