# Product Hunt API v2 adapter

Adapter name: `producthunt/graphql-v2`

This package implements a bounded, read-only surface of Product Hunt's
official API v2 GraphQL endpoint:

- list and look up posts;
- list comments on a post and look up one comment;
- list and look up topics;
- list and look up published collections;
- look up public users and read `viewer` with a user-context token.

The adapter deliberately does not implement mutations, token exchange, media
upload, unofficial webhooks, goals, or Maker-space objects. Product Hunt apps
are read-only with the `public` scope by default. Third-party write access is
granted only after Product Hunt approves the use case, so this package does
not imply that an access token is authorized to mutate data.

## Authentication and access

Every GraphQL request is sent to:

```text
POST https://api.producthunt.com/v2/api/graphql
Authorization: Bearer <access-token>
Content-Type: application/json
```

The API origin is fixed by the adapter. Redirect following and cookie-jar
authentication are disabled so the bearer token remains scoped to Product
Hunt's documented GraphQL endpoint.

The adapter consumes an externally managed token through `access_token_ref`.
It does not store a client secret or exchange credentials. Product Hunt
documents these token sources:

- OAuth 2.0 authorization code for user-context access;
- authorization code with PKCE (`S256`) for public clients;
- client credentials for public, application-context reads;
- a non-expiring, account-linked `developer_token` from the API dashboard.

The documented scopes are `public`, `private`, and `write`. `GetViewer`
requires user context; a client-credentials token is limited to public
endpoints. Product Hunt does not document a refresh-token flow, and the
dashboard developer token does not expire. Applications that manage OAuth
should rotate or reacquire tokens outside this adapter and update the secret
reference.

```yaml
version: 1
platforms:
  - adapter: producthunt/graphql-v2
    product: producthunt-api
    accounts:
      - id: producthunt-read
        access_token_ref: env://PRODUCTHUNT_ACCESS_TOKEN
        approval:
          scopes: [public]
```

Import the package to register its factory:

```go
package main

import (
	"context"
	"fmt"

	"social-hub/adapters/producthunt"
	"social-hub/pkg/socialhub"
)

func newestProducts(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "producthunt/graphql-v2", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "producthunt-read")
	if err != nil {
		return err
	}
	client := base.(*producthunt.Client)

	response, err := client.ProductHunt().ListPosts(ctx, producthunt.ListPostsRequest{
		Page:  producthunt.Pagination{First: 20},
		Order: producthunt.PostsOrderNewest,
	})
	if err != nil {
		return err
	}
	for _, edge := range response.Posts.Edges {
		fmt.Println(edge.Node.Name, edge.Node.URL)
	}
	return nil
}
```

## Cursor and response contract

List operations use Product Hunt's Relay-style `first`/`after` or
`last`/`before` pagination. A request may use only one direction. The official
reference does not publish a numeric maximum for `first` or `last`, so the
adapter validates positive values and cursor pairing without inventing a cap.
`PageInfo.StartCursor` and `PageInfo.EndCursor` are pointers because the schema
defines both as nullable.

GraphQL selections are fixed by each typed operation and all filter values are
sent as variables. Responses retain:

- the complete GraphQL envelope in `Raw` for successful operations;
- `X-Request-ID` in `Meta.RequestID`;
- `X-Rate-Limit-Limit`, `X-Rate-Limit-Remaining`, and
  `X-Rate-Limit-Reset` in `Meta`.

Product Hunt can return HTTP 200 with both partial `data` and `errors`. In that
case the adapter returns the decoded partial response and an `APIError`; callers
must inspect both and must not discard the error. `APIError.GraphQL` preserves
standard `message`, `locations`, `path`, and `extensions` fields, while also
accepting Product Hunt's OAuth-shaped `error` and `error_description` fields.
Error-path `Raw` values are structurally sanitized and capped at 64 KiB.

## Rate limits

Rate limiting is applied per application. The GraphQL endpoint has a budget of
6,250 complexity points per 15 minutes; request cost depends on selected
fields. The separate request-based limit of 450 requests per 15 minutes
applies to other `/v2/*` endpoints, not to this adapter's GraphQL calls.
Product Hunt may adjust limits based on traffic.

`X-Rate-Limit-Reset` is the number of seconds until reset. HTTP 429 and
GraphQL rate-limit failures map to retryable `socialhub.CodeRateLimited`
errors, with that value exposed as `RetryAfter` when valid. A distributed
application must coordinate the complexity budget across all processes that
share the Product Hunt application.

## Commercial use and attribution

Product Hunt prohibits commercial API use by default. Commercial integrations
must obtain permission from `hello@producthunt.com`. Product Hunt also asks
projects using the API to attribute and link back to Product Hunt. Supplying a
token or configuring `approval.scopes` does not grant a commercial license.

## Official sources

Official material reviewed on 2026-08-25:

- <https://api.producthunt.com/v2/docs>
- <https://api-v2-docs.producthunt.com/operation/query/>
- <https://api.producthunt.com/v2/docs/rate_limits/headers>
- <https://github.com/producthunt/producthunt-api>

The live GraphQL reference is authoritative for the implemented schema. It
currently includes fields such as connection `nodes`, post rank and review
fields, `productLinks`, and `posts(url:)` that are absent from the older schema
snapshot still present in the official starter repository. This adapter uses
the live reference and adds no third-party dependency.
