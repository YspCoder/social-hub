# Forem API V1 adapter

Adapter name: `forem/api-v1`

This adapter targets the official Forem API V1 contract (`1.0.0`). Each
configured account represents one API key on one Forem instance. An omitted
`account.settings.base_url` targets DEV Community at `https://dev.to`.

## Authentication and configuration

Forem uses an `api-key` header rather than OAuth. Generate the key in the user
settings of the target Forem instance and place its secret reference in
`access_token_ref`.

```yaml
version: 1
platforms:
  - adapter: forem/api-v1
    product: api
    accounts:
      - id: dev-author
        access_token_ref: env://DEV_API_KEY
      - id: company-author
        access_token_ref: env://COMPANY_FOREM_API_KEY
        settings:
          base_url: https://community.example.com
```

Every request sends:

- `api-key: <resolved key>`
- `Accept: application/vnd.forem.api-v1+json`
- `User-Agent: social-hub/forem`

Use HTTPS outside local development. The adapter allows same-origin redirects
but rejects cross-origin redirects so an API key cannot follow a redirect to a
different host.

## Common capabilities

| Capability | Coverage |
|---|---|
| `Fetcher` | Current user or user by ID/username, Article by numeric ID, authenticated-account or username Article pages, and flattened threaded Article comments |
| `Reactor` | Idempotent Article `like`; an optional actor ID must match the authenticated user |
| `Publisher` | Not exposed because the common request has no required Forem Article title or Article metadata |
| `MediaUploader` | Not exposed; Article publishing accepts remote image URLs but V1 has no general-purpose public media upload contract |
| `Messenger` | Not exposed; V1 has no direct-message contract |
| `WebhookHandler` | Not exposed; V1 publishes no signed inbound webhook verification contract |

Comments are read-only in the current public API. `RemoveReaction` is also
unsupported: Forem documents idempotent create and explicit toggle operations,
but not an idempotent delete operation. The adapter does not use toggle to
pretend that a reaction was removed.

## Typed workflows

Article publishing requires fields that do not exist in
`socialhub.CreatePostRequest`, so it remains an explicit typed workflow:

```go
common, err := adapter.Client(ctx, "dev-author")
if err != nil {
    return err
}
client := common.(*forem.Client)

article, err := client.ArticleWorkflow().CreateArticle(ctx, forem.CreateArticleRequest{
    Title:        "Shipping social-hub",
    BodyMarkdown: "# Release notes\n\nThe first build is ready.",
    Published:    true,
    Tags:         []string{"go", "opensource"},
})
```

`ArticleWorkflow` provides create/get/update, all/published/unpublished account
pages, and unpublish. Forem accepts no more than four Article tags. The V1
request schema uses a comma-separated string, while older responses can return
`tags` or `tag_list` as either strings or arrays; the adapter accepts both.

`UpdateArticleRequest` omits nil fields and serializes non-nil values verbatim.
It does not represent the distinct JSON `null` state used to remove nullable
metadata such as `series`. For that operation, send the complete updated
`BodyMarkdown` with revised YAML front matter, as required by Forem's update
contract.

All documented Forem reaction categories and target types are available through
`ReactionWorkflow`:

```go
err = client.ReactionWorkflow().CreateForemReaction(ctx, forem.ForemReactionRequest{
    Category: forem.ReactionFire,
    TargetID: "12345",
    Type:     forem.ReactableArticle,
})
```

Categories are `like`, `unicorn`, `exploding_head`, `raised_hands`, and `fire`.
Targets are `Article`, `Comment`, and `User`. Use `CreateForemReaction` for
idempotent creation and `ToggleForemReaction` only when toggle semantics are
intended.

## Rate limits and API maturity

The current V1 specification does not publish one global quota. The legacy V0
documentation records endpoint-specific limits of 10 Article creates per 30
seconds and 30 Article updates per 30 seconds. Self-hosted instances may differ.
The adapter therefore treats HTTP `429` as retryable and preserves
`Retry-After`; callers should let the shared rate-limit layer adapt to observed
responses instead of assuming one fixed instance-wide budget.

The official documentation warns that the API changes rapidly and may be ahead
of or behind deployed instances. Raw Article, User, and Comment JSON is retained
in model extensions to preserve fields outside the common contract.

No external Go SDK is used. `ShiraazMoollatjie/gophorem` was last updated in
2020, and `karvounis/dev-client-go` targets the older beta `0.9.7` contract.
Neither matches the current V1 media type and schemas, so this adapter reuses the
repository's bounded authenticated transport and implements only documented
endpoints.

Official references:

- <https://developers.forem.com/api/>
- <https://developers.forem.com/api/v1>
- <https://developers.forem.com/api/v0>

All tests use deterministic local HTTP fixtures. No real Forem or DEV account
has been used for validation yet.
