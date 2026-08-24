# GitHub REST API adapter

Adapter name: `github/rest-api`

This package implements a bounded, read-only subset of the GitHub REST API:

| Typed workflow | GitHub endpoint | Boundary |
| --- | --- | --- |
| `GetAuthenticatedUser` | `GET /user` | requires a user-context access token |
| `GetUser` | `GET /users/{username}` | public profile; Enterprise Managed User visibility still depends on access |
| `ListAuthenticatedRepositories` | `GET /user/repos` | repositories explicitly visible to the authenticated user |
| `ListRepositoriesForUser` | `GET /users/{username}/repos` | public repositories only |
| `GetRepository` | `GET /repos/{owner}/{repo}` | fields vary with repository permission |
| `ListIssues` | `GET /repos/{owner}/{repo}/issues` | includes pull requests as issue representations |
| `GetIssue` | `GET /repos/{owner}/{repo}/issues/{issue_number}` | issue numbers are repository-local string IDs |
| `ListIssueComments` | `GET /repos/{owner}/{repo}/issues/{issue_number}/comments` | issue and pull-request conversation comments |
| `GetIssueComment` | `GET /repos/{owner}/{repo}/issues/comments/{comment_id}` | comment IDs are string-preserved int64 values |

Writes, search, commits, contents, releases, Actions, checks, notifications,
GraphQL, OAuth authorization, token creation or refresh, GitHub App JWT and
installation-token minting, webhooks, and automatic pagination are outside
this adapter. The implemented entities retain their complete bounded JSON in
`Raw`; they are not coerced into generic social posts.

## API version and headers

GitHub's current REST API version is `2026-03-10`. Every request explicitly
sends:

```text
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2026-03-10
User-Agent: social-hub/github
Authorization: Bearer <access-token>
```

The adapter verifies a non-empty `X-GitHub-Api-Version-Selected` response
against the requested version. `ResponseMeta` also preserves `Deprecation`,
`Sunset`, `Warning`, `ETag`, `X-GitHub-Media-Type`, and request-ID headers so a
closing API version is observable before GitHub begins returning `410 Gone`.

GitHub still supports `2022-11-28` through 2028-03-10, but it is not the latest
version and this adapter does not silently fall back to it.

## Authentication and permissions

The adapter resolves one externally managed access token from
`access_token_ref`. Suitable tokens include a personal access token, a
user-to-server GitHub App token, an installation access token for compatible
repository endpoints, or a workflow `GITHUB_TOKEN`. Endpoint compatibility,
fine-grained permissions, classic OAuth scopes, repository selection, SAML
SSO authorization, and field visibility remain properties of that token.

`GET /user` specifically needs user context; an installation token is not a
substitute. Public endpoints need no fine-grained permission for public data,
while private repositories, issues, and comments require the read permissions
listed by each endpoint's current documentation. The adapter does not infer
permission from a token string. Configured approval scopes and GitHub's
`X-OAuth-Scopes`, `X-Accepted-OAuth-Scopes`, and `X-GitHub-SSO` response headers
remain available for diagnostics.

```yaml
version: 1
platforms:
  - adapter: github/rest-api
    product: rest-api
    accounts:
      - id: engineering-read
        access_token_ref: env://GITHUB_TOKEN
```

The package does not accept client secrets, persist tokens, exchange OAuth
codes, or refresh access tokens.

## Usage

```go
package main

import (
	"context"
	"fmt"

	githubadapter "social-hub/adapters/github"
	"social-hub/pkg/socialhub"
)

func openIssues(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "github/rest-api", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "engineering-read")
	if err != nil {
		return err
	}
	client := base.(*githubadapter.Client)
	page, err := client.GitHub().ListIssues(ctx, "octocat", "Hello-World", githubadapter.ListIssuesRequest{
		State: githubadapter.IssueStateOpen,
		PerPage: 50,
	})
	if err != nil {
		return err
	}
	for _, issue := range page.Items {
		fmt.Println(issue.Number, issue.Title, issue.PullRequest != nil)
	}
	return nil
}
```

## Input and response boundaries

Owner, login, repository, issue-number, and comment-ID path inputs are
validated before URL construction. Numeric GitHub `id` and `number` JSON
fields decode into the decimal string type `ID`; `node_id` remains a separate
opaque string. This avoids precision loss outside Go.

All implemented lists accept GitHub's documented `page` and `per_page`
parameters, with `per_page` capped at 100. `ListAuthenticatedRepositories`
rejects `type` when `visibility` or `affiliation` is also set because GitHub
documents that combination as a `422`. Times are encoded as UTC RFC 3339.

`Page.Links` parses only positive `page` values for `next`, `prev`, `first`,
and `last` relations. GitHub may also include internal `before` or `after`
values in a Link URL. The complete bounded header remains available as text,
but the adapter never follows an absolute provider URL and never accepts one
as a request target.

Successful JSON is limited to 8 MiB and must have the expected object or array
shape. Each entity and page retains a copy of the provider JSON. That JSON may
contain private profile or repository data and must be protected according to
the application's GitHub data-handling obligations.

## Redirect and secret safety

The copied HTTP client has its cookie jar removed and redirects disabled.
GitHub documents `301 Moved Permanently` for renamed repositories and
transferred issues; those responses map to a permanent `CodeConflict` instead
of forwarding `Authorization` to a `Location` origin. Callers should resolve
the canonical owner, repository, or issue themselves.

The API origin is fixed to `https://api.github.com`; caller-supplied origins
and API gateways are rejected so credentials cannot be redirected to another
host. Credentials are sent only in the Authorization header, transport errors
discard request URLs, and bounded error bodies redact the configured token and
common credential keys. Non-JSON and oversized error bodies remain valid JSON
after sanitization. GitHub's `404` can mean either that a resource does not
exist or that the token is not allowed to see it; the adapter preserves that
deliberate ambiguity as `CodeNotFound`.

## Rate limits and errors

GitHub's primary limit depends on authentication context. The general user
limit is 5,000 requests per hour; Enterprise Cloud app contexts may receive
15,000, installation limits can scale, and workflow `GITHUB_TOKEN` uses a
repository-specific budget. The adapter therefore does not hard-code a quota.

Every response exposes these values unchanged and also derives bounded time
values where possible:

```text
X-RateLimit-Limit
X-RateLimit-Remaining
X-RateLimit-Used
X-RateLimit-Reset
X-RateLimit-Resource
Retry-After
X-GitHub-Request-Id
```

Primary exhaustion is a `403` or `429` with remaining `0`; retry delay comes
from the UTC epoch reset. Secondary limits also use `403` or `429`; the adapter
prefers `Retry-After` and otherwise reports GitHub's documented minimum
one-minute delay. GitHub currently documents at most 100 concurrent requests
and 900 REST points per minute, with most reads costing one point, but secondary
rules can change without notice. Provider headers are authoritative.

## Official sources

Official material reviewed on 2026-08-24:

- <https://docs.github.com/en/rest/about-the-rest-api/api-versions>
- <https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api>
- <https://docs.github.com/en/rest/using-the-rest-api/using-pagination-in-the-rest-api>
- <https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api>
- <https://docs.github.com/en/rest/users/users>
- <https://docs.github.com/en/rest/repos/repos>
- <https://docs.github.com/en/rest/issues/issues>
- <https://docs.github.com/en/rest/issues/comments>
- <https://github.com/github/rest-api-description/tree/main/descriptions/api.github.com>

The reviewed official OpenAPI repository head was
`d77b7dde24f7b3a52b3532b1337d4be8a60fb34d` (2026-08-20). Live GitHub API
responses independently confirmed version selection and the documented Link,
request-ID, media-type, and rate-limit headers. This package has no third-party
dependency.
