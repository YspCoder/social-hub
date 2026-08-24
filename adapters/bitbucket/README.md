# Bitbucket Cloud REST API 2.0 adapter

`adapters/bitbucket` is a bounded, read-only adapter for the current Bitbucket
Cloud REST API 2.0 contract at `https://api.bitbucket.org/2.0`. It preserves
provider-native repository and pull-request semantics instead of forcing them
into social post models.

## Implemented surface

| Workflow | Endpoint | Legacy OAuth / current API-token or Forge scope |
| --- | --- | --- |
| Current account | `GET /user` | `account` / `read:user:bitbucket` |
| Accessible workspaces | `GET /user/workspaces` | `account` / `read:workspace:bitbucket` |
| Workspace detail | `GET /workspaces/{workspace}` | no legacy endpoint scope / `read:workspace:bitbucket` |
| Workspace repositories | `GET /repositories/{workspace}` | `repository` / `read:repository:bitbucket` |
| Repository detail | `GET /repositories/{workspace}/{repo_slug}` | `repository` / `read:repository:bitbucket` |
| Pull requests | `GET /repositories/{workspace}/{repo_slug}/pullrequests` | `pullrequest` / `read:pullrequest:bitbucket` |
| Pull request detail | `GET /repositories/{workspace}/{repo_slug}/pullrequests/{pull_request_id}` | `pullrequest` / `read:pullrequest:bitbucket` |
| Pull request comments | `GET /repositories/{workspace}/{repo_slug}/pullrequests/{pull_request_id}/comments` | `pullrequest` / `read:pullrequest:bitbucket` |
| Pull request comment detail | `GET /repositories/{workspace}/{repo_slug}/pullrequests/{pull_request_id}/comments/{comment_id}` | `pullrequest` / `read:pullrequest:bitbucket` |

The old `GET /workspaces` list operation is deprecated by Atlassian. This
adapter uses its documented replacement, `GET /user/workspaces`, whose values
are `WorkspaceAccess` objects containing both the workspace and the caller's
`administrator` flag.

`workspace` and `repo_slug` accept a slug or an API-compatible brace-wrapped
UUID. Pull request and comment IDs are positive integer `bitbucket.ID` values.

Issues are deliberately not implemented. The current official v3 OpenAPI
description contains no issue endpoints, even though older documentation and
some authentication material still mention an issue scope. `Repository.HasIssues`
is retained only because it remains part of the repository representation.

## Authentication

The adapter consumes an already issued credential through
`access_token_ref`. It does not exchange authorization codes, refresh OAuth
tokens, or rotate access tokens.

Two explicit account modes are supported:

```yaml
version: 1
platforms:
  - adapter: bitbucket/cloud-rest-api-v2
    product: cloud-rest-api
    settings:
      user_agent: social-hub/bitbucket
    accounts:
      - id: oauth-user
        access_token_ref: env://BITBUCKET_OAUTH_TOKEN
        approval:
          account_type: oauth2
          scopes: [account, repository, pullrequest]
        settings:
          auth_mode: bearer

      - id: api-token-user
        access_token_ref: env://BITBUCKET_API_TOKEN
        settings:
          auth_mode: basic_api_token
          email: developer@example.com
```

- `bearer` sends an OAuth 2.0 token, API token, or repository, project, or
  workspace access token in `Authorization: Bearer ...`.
- `basic_api_token` sends the Atlassian account email as the Basic username and
  the Atlassian API token referenced by `access_token_ref` as the password.
- Deprecated Bitbucket app passwords are not a supported credential mode.

Legacy OAuth consumers use the short scopes shown above. Scoped Atlassian API
tokens and Forge authorization use the corresponding current
`read:*:bitbucket` scopes from the official OpenAPI description. Capability
approval accepts either vocabulary rather than treating the two as cumulative.

OAuth access tokens are normally short lived. Bitbucket documents rotating
refresh tokens, including rotation after use. Credential acquisition and
refresh must therefore happen outside this adapter, followed by supplying the
current token through the configured `SecretResolver`.

## Use

Importing the package registers `bitbucket/cloud-rest-api-v2` with the root
registry:

```go
import (
    "context"
    "fmt"

    "social-hub/adapters/bitbucket"
    "social-hub/pkg/socialhub"
)

func listRepositories(ctx context.Context) error {
    config := socialhub.AdapterConfig{
        Adapter: "bitbucket/cloud-rest-api-v2",
        Product: "cloud-rest-api",
        Accounts: []socialhub.AccountConfig{{
            ID:             "primary",
            AccessTokenRef: "env://BITBUCKET_OAUTH_TOKEN",
            Approval: socialhub.ApprovalConfig{
                Scopes: []string{"account", "repository", "pullrequest"},
            },
            Settings: map[string]any{"auth_mode": "bearer"},
        }},
    }

    adapter, err := socialhub.Open(ctx, "bitbucket/cloud-rest-api-v2", config)
    if err != nil {
        return err
    }
    defer adapter.Close()

    generic, err := adapter.Client(ctx, "primary")
    if err != nil {
        return err
    }
    api := generic.(*bitbucket.Client).Bitbucket()

    page, err := api.ListRepositories(ctx, "example-workspace", bitbucket.ListRepositoriesRequest{
        Role: bitbucket.RepositoryRoleMember,
        Page: bitbucket.PageOptions{PageLength: 50},
    })
    if err != nil {
        return err
    }
    for _, repository := range page.Items {
        fmt.Println(repository.UUID, repository.FullName)
    }
    return nil
}
```

## Pagination and filtering

Bitbucket has both list-based and iterator-based pagination. A page can expose
`size`, `page`, `pagelen`, `next`, `previous`, and `values`, but only `values`
and a non-final page's `next` URL are reliable across endpoints. Iterator
continuations can contain unpredictable hashes, so callers must not derive a
next page themselves.

The adapter validates that a continuation uses HTTPS, the exact
`api.bitbucket.org` host, and the same endpoint path. It never follows the
absolute URL. Instead, it exposes the provider query as `Page.NextQuery`:

```go
next, err := api.ListRepositories(ctx, "example-workspace", bitbucket.ListRepositoriesRequest{
    Page: bitbucket.PageOptions{NextQuery: page.NextQuery},
})
```

`NextQuery` is mutually exclusive with `PageLength` and first-page filters.
First-page lengths are either omitted or between 1 and 100. `NextURL` and
`PreviousURL` are retained only as bounded provider evidence.

Repository, pull-request, and comment lists expose Bitbucket's `q` and `sort`
expressions without attempting to parse their provider-specific grammar.
Values are URL encoded, bounded, and rejected if they contain control
characters. Pull-request `state` values and repository `role` values use typed
enums.

## Errors, rate limits, and response boundaries

`APIError` wraps a platform-neutral `socialhub.Error` and preserves the
documented Bitbucket envelope:

```json
{
  "type": "error",
  "error": {
    "message": "Bad request",
    "fields": {"src": ["This field is required."]},
    "detail": "...",
    "id": "...",
    "data": {}
  }
}
```

Provider error JSON is recursively sanitized for credential-like keys and the
configured credential value before it is retained. HTTP `429` maps to
`CodeRateLimited`; request timeouts and `5xx` responses map to retryable
temporary failures. Authentication and permission failures link back to the
official authentication guidance.

Bitbucket limits use a one-hour rolling window and vary by resource and
authentication context. Anonymous requests are currently limited to 60 per
hour. Repository API resources normally start at 1,000 requests per hour and
can scale to 10,000 for qualifying workspaces and access-token or Forge app
authentication. Those values are operational policy, not a stable API
contract, so this adapter does not hard-code a client-side quota.

Every response preserves the currently documented scaled-limit headers when
present:

```text
X-RateLimit-Limit
X-RateLimit-Resource
X-RateLimit-NearLimit
Retry-After
```

It also retains `X-Request-Count`, an available request/trace identifier,
`ETag`, and standard lifecycle headers. `Retry-After` is parsed into a bounded
duration when valid.

Successful responses must be bounded JSON objects with a JSON content type.
Entity detail responses require their stable UUID or integer ID. Workspace,
repository, pull-request, and comment responses are checked against the
requested path selectors, including nested destination repository and
pull-request IDs. Every core entity and page retains bounded provider JSON,
which may contain private account or repository data.

The API origin is fixed to the official HTTPS host. The adapter clones the
supplied HTTP client, removes its cookie jar, and disables redirects so an
authorization header cannot be forwarded to a `Location` origin. Transport
errors discard request URLs.

## Official sources

Official material reviewed on 2026-08-25:

- <https://dac-static.atlassian.com/cloud/bitbucket/swagger.v3.json>
- <https://developer.atlassian.com/cloud/bitbucket/rest/intro/>
- <https://developer.atlassian.com/cloud/bitbucket/rest/intro/#pagination>
- <https://developer.atlassian.com/cloud/bitbucket/rest/api-group-users/>
- <https://developer.atlassian.com/cloud/bitbucket/rest/api-group-workspaces/>
- <https://developer.atlassian.com/cloud/bitbucket/rest/api-group-repositories/>
- <https://developer.atlassian.com/cloud/bitbucket/rest/api-group-pullrequests/>
- <https://support.atlassian.com/bitbucket-cloud/docs/api-request-limits/>

The reviewed official description is OpenAPI 3.0.0 for Bitbucket Cloud API
version 2.0. The package adds no third-party dependency.
