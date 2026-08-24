# GitLab REST API v4 adapter

Adapter name: `gitlab/rest-api-v4`

This package implements a bounded, authenticated, read-only subset of the
GitLab REST API v4. GitLab entities retain their provider semantics and are not
coerced into generic social posts.

## Implemented surface

| Typed workflow | Endpoint | Boundary |
| --- | --- | --- |
| `GetAuthenticatedUser` | `GET /user` | user represented by the configured token |
| `GetUser` | `GET /users/:id` | signed-in-visible user by decimal global ID |
| `ListProjects` | `GET /projects` | one offset-paginated page visible to the token |
| `GetProject` | `GET /projects/:id` | project by decimal global ID |
| `ListProjectIssues` | `GET /projects/:id/issues` | one offset-paginated project issue page |
| `GetProjectIssue` | `GET /projects/:id/issues/:issue_iid` | project-local issue IID |
| `ListIssueNotes` | `GET /projects/:id/issues/:issue_iid/notes` | comments and system activity for an issue |
| `GetIssueNote` | `GET /projects/:id/issues/:issue_iid/notes/:note_id` | note by decimal global ID |

Writes, repositories, merge requests, pipelines, jobs, packages, releases,
GraphQL, OAuth authorization, token minting or refresh, automatic pagination,
keyset pagination, webhooks, and administrator-only user detail are outside
this adapter.

## API version and origin

GitLab's semantic REST API version is `v4`, and the default origin is:

```text
https://gitlab.com/api/v4
```

`settings.base_url` may target a controlled GitLab Self-Managed instance,
including an installation under a relative URL. It must use HTTPS, have no
query or fragment, have no trailing slash, and end in `/api/v4`.

The generated official OpenAPI v2 description identifies the API as `v4` and
declares `PRIVATE-TOKEN` as its default security header. GitLab's authentication
documentation additionally confirms that OAuth access tokens and personal,
project, or group access tokens can all use the OAuth-compatible
`Authorization: Bearer <token>` header. This adapter consistently uses that
Bearer form. `JOB-TOKEN`, deploy-token authentication, Basic authentication,
and token query parameters are not supported.

## Authentication and configuration

The adapter resolves one externally managed credential from
`access_token_ref`. It does not accept a client secret, exchange an OAuth code,
persist credentials, or refresh an OAuth token. GitLab currently documents a
two-hour lifetime and refresh-token flow for OAuth access tokens; credential
refresh must happen outside this package before creating a new client.

The token's type, scopes, resource ownership, role, SAML authorization, and the
visibility of confidential projects or issues remain GitLab policy. Typical
read access uses `read_api` or a broader compatible scope, but the exact scope
must be selected for the endpoints and token type in use.

```yaml
version: 1
platforms:
  - adapter: gitlab/rest-api-v4
    product: rest-api
    settings:
      base_url: https://gitlab.com/api/v4
      user_agent: social-hub/gitlab
    accounts:
      - id: engineering-read
        access_token_ref: env://GITLAB_ACCESS_TOKEN
        approval:
          scopes: [read_api]
```

Account-specific settings, webhook configuration, client or application IDs,
client secrets, and token-store configuration are rejected because they are
outside this adapter's externally supplied token contract.

## Usage

```go
package main

import (
	"context"
	"fmt"

	gitlabadapter "social-hub/adapters/gitlab"
	"social-hub/pkg/socialhub"
)

func listOpenIssues(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "gitlab/rest-api-v4", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "engineering-read")
	if err != nil {
		return err
	}
	api := base.(*gitlabadapter.Client).GitLab()

	page, err := api.ListProjectIssues(ctx, "278964", gitlabadapter.ListProjectIssuesRequest{
		State:   gitlabadapter.IssueStateOpened,
		OrderBy: gitlabadapter.IssueOrderUpdatedAt,
		Sort:    gitlabadapter.SortDescending,
		PerPage: 50,
	})
	if err != nil {
		return err
	}
	for _, issue := range page.Items {
		fmt.Println(issue.IID, issue.Title)
	}
	return nil
}
```

Importing the package registers the adapter with the root registry. The common
`Fetcher` remains unavailable because GitLab users, projects, issues, and notes
use provider-native identifiers and pagination; use the typed `GitLab()`
workflow instead.

## Identifier and response boundaries

GitLab numeric `id` and `iid` values decode into the decimal string type `ID`,
which prevents precision loss outside Go. Project IDs, user IDs, issue IIDs,
and note IDs supplied as path parameters must be positive decimal integers.
Although GitLab also documents URL-encoded namespaced project paths, this
adapter deliberately excludes them: the shared transport builds URL paths from
decoded components and would re-encode an embedded `%2F`. Use the project's
global decimal ID.

Project milestone ownership is optional in the provider representation. A
milestone can expose either `project_id` or `group_id`, so both are nullable
`*ID` fields. Each issue and note operation checks that returned project IDs,
issue IIDs, and detail IDs match the request.

Successful responses are limited to 8 MiB, must be valid JSON with the expected
object or array shape, and must use a JSON-compatible content type when one is
present. Core entities and pages retain a bounded copy of provider JSON in
`Raw`. That data can include private profile, project, confidential issue, or
internal note content and must be protected accordingly.

## Pagination

All implemented list methods expose offset pagination through `page` and
`per_page`; `per_page` is capped at GitLab's documented maximum of 100. The
provider default is normally 20. The adapter returns one page and never follows
an absolute `Link` URL automatically.

`Pagination` strictly parses `X-Page`, `X-Per-Page`, `X-Next-Page`,
`X-Prev-Page`, `X-Total`, and `X-Total-Pages`, while preserving the complete
bounded `Link` header. GitLab can omit total counts and the `rel="last"` link
when a query contains more than 10,000 records, so total fields are nullable.

GitLab supports keyset pagination for selected endpoints and recommends it for
large collections. The Projects API requires keyset pagination beyond its
documented offset range. This first adapter is offset-only; it does not claim
complete traversal of collections beyond the instance's configured maximum
offset.

## Redirect, error, and secret safety

The adapter copies the configured HTTP client, removes its cookie jar, and
disables redirects so a Bearer credential cannot be forwarded to a `Location`
origin. Only HTTPS API origins are accepted. Transport errors discard request
URLs, and bounded provider errors redact the configured token and common
credential-shaped fields before retaining diagnostic JSON.

Authentication and permission errors link to the personal-access-token page
on the configured instance, including its relative URL prefix, rather than
redirecting Self-Managed users to GitLab.com.

GitLab error responses may use either a string or a field-to-errors object in
`message`; both forms are preserved through `APIError`. HTTP authentication,
permission, not-found, rate-limit, transient, and permanent failures map to the
corresponding `socialhub.Error` code and class. GitLab commonly uses `404` both
for absence and for resources the token is not allowed to see, so
`CodeNotFound` deliberately preserves that ambiguity.

## Rate limits

GitLab.com currently documents a general authenticated API limit of 2,000
requests per minute, but endpoint limits can be more restrictive. In this
adapter's surface, the projects list is currently 2,000 requests per 10 minutes
and single-project retrieval is 400 requests per minute. GitLab Self-Managed
limits are administrator-configurable, and endpoint-specific limits can
supersede general user and IP limits. The adapter therefore does not hard-code
a client-side quota.

Every response preserves these values when GitLab emits them:

```text
RateLimit-Limit
RateLimit-Name
RateLimit-Observed
RateLimit-Remaining
RateLimit-Reset
RateLimit-ResetTime
Retry-After
X-Request-Id
X-GitLab-Meta
```

For `429 Too Many Requests`, retry delay is selected in this order:

1. `Retry-After` seconds or HTTP date.
2. `RateLimit-ResetTime` HTTP date.
3. `RateLimit-Reset` Unix epoch.
4. A conservative one-minute fallback.

GitLab specifically notes that rate-limited Projects, Groups, and Users API
responses might omit informational rate headers, which is why the fallback is
necessary. Provider headers and instance configuration remain authoritative.

## Official sources

Official material reviewed on 2026-08-25:

- <https://docs.gitlab.com/api/rest/>
- <https://docs.gitlab.com/api/rest/authentication/>
- <https://docs.gitlab.com/api/users/>
- <https://docs.gitlab.com/api/projects/>
- <https://docs.gitlab.com/api/issues/>
- <https://docs.gitlab.com/api/notes/>
- <https://docs.gitlab.com/api/rest/#pagination>
- <https://docs.gitlab.com/user/gitlab_com/#rate-limits-on-gitlabcom>
- <https://docs.gitlab.com/administration/settings/user_and_ip_rate_limits/>
- <https://gitlab.com/gitlab-org/gitlab/-/blob/3bb66ccbd0bf742c2f95c3c092c14dae1576a3df/doc/api/openapi/openapi_v2.yaml>

The reviewed official GitLab repository head was
`3bb66ccbd0bf742c2f95c3c092c14dae1576a3df` (2026-08-25). The generated
OpenAPI description covers the Projects and Issues operations and identifies
the REST contract as `v4`. The current generated description does not include
the `/user`, `/users/:id`, or core issue Notes operations, so those contracts
were checked against their official endpoint documentation. Live GitLab.com
responses independently confirmed the documented pagination, request-ID,
GitLab metadata, and rate-limit header shapes. This package adds no third-party
dependency.
