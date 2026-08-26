# Wikimedia MediaWiki REST adapter

`wikimedia/mediawiki-rest-v1` provides anonymous, read-only access to the
stable MediaWiki REST API v1 on Wikipedia language editions and Wikimedia
Commons. It uses the official `/w/rest.php/v1` endpoints directly instead of
the legacy RESTBase API, the Action API, or the API gateway.

The OpenAPI discovery route currently contains `specs/v0`, but that is the
version of the specification discovery module. It does not change the public
endpoint version implemented here, which is `v1`.

## Supported operations

| Method | MediaWiki REST endpoint | SDK method |
|---|---|---|
| `GET` | `/v1/search/page` | `SearchPages` |
| `GET` | `/v1/page/{title}/bare` | `GetPage` |
| `GET` | `/v1/page/{title}/links/media` | `ListPageMedia` |
| `GET` | `/v1/file/{title}` | `GetFile` |
| `GET` | `/v1/file/{title}/thumbnails` | `ListFileThumbnails` |

These endpoints require no token. Writes, OAuth, uploads, page HTML or source,
revision history, and webhooks are intentionally outside this adapter's
contract.

## Configuration

Wikimedia requires an identifying, contactable `User-Agent`. Use the
application name and version plus a URL or email address monitored by its
operator; do not use the example unchanged in production.

```yaml
version: 1
platforms:
  - adapter: wikimedia/mediawiki-rest-v1
    product: mediawiki-rest
    settings:
      user_agent: MyClient/1.0 (https://example.com/contact; admin@example.com) social-hub/0.1
    accounts:
      - id: english-wikipedia
        settings:
          project: wikipedia
          language: en
      - id: commons
        settings:
          project: commons
```

Wikipedia accounts require a lowercase language code. Commons accounts must
omit `language`. Credentials, token stores, approval declarations, webhook
secrets, arbitrary hosts, and redirects are rejected or disabled.

## Usage

```go
adapter, err := socialhub.Open(ctx, "wikimedia/mediawiki-rest-v1", socialhub.AdapterConfig{
    Adapter: "wikimedia/mediawiki-rest-v1",
    Product: "mediawiki-rest",
    Settings: map[string]any{
        "user_agent": "MyClient/1.0 (https://example.com/contact; admin@example.com)",
    },
    Accounts: []socialhub.AccountConfig{{
        ID: "english-wikipedia",
        Settings: map[string]any{"project": "wikipedia", "language": "en"},
    }},
})
if err != nil {
    return err
}

common, err := adapter.Client(ctx, "english-wikipedia")
if err != nil {
    return err
}
client := common.(*wikimedia.Client)
results, err := client.Knowledge().SearchPages(ctx, wikimedia.SearchPagesRequest{
    Query: "adapter pattern",
    Limit: 10,
})
```

File methods require the complete MediaWiki title, including the `File:`
namespace prefix. Page titles may contain spaces; the adapter normalizes them
to underscores and escapes the title as one URL path segment.

## Operational boundaries

- As verified on 2026-08-26, Wikimedia's experimental limits allow a compliant,
  identified client 200 requests per minute and recommend no more than three
  concurrent requests. Unidentified clients receive a much smaller allowance.
  These limits may change, so this adapter does not hard-code a rate limiter.
- `429` and `503` responses are retryable and preserve `Retry-After`. When the
  header is absent, the adapter reports Wikimedia's minimum five-second delay.
- Search accepts `limit=1..100` and defaults to 50. Page media returns at most
  100 files. None of the selected endpoints defines an offset, continuation
  token, or next cursor, so the adapter does not invent pagination metadata.
- Search excerpts can contain HTML, including `span.searchmatch`. Sanitize them
  for the target rendering context before displaying them.
- Provider URLs can be protocol-relative (`//...`). Resolve them against the
  configured wiki before using them as absolute URLs.
- Content and file licenses vary. Preserve the returned attribution and license
  fields; for files, `file_description_url` is the authoritative page for
  license and attribution metadata.
- Successful response bodies are bounded to 8 MiB, redirects are refused, and
  platform error bodies retained for diagnostics are bounded to 64 KiB.

Official references: [REST API reference](https://www.mediawiki.org/wiki/API:REST_API/Reference),
[REST API policies](https://www.mediawiki.org/wiki/API:REST_API/Policies),
[API stability](https://www.mediawiki.org/wiki/Wikimedia_APIs/Stability_policy),
[API access](https://www.mediawiki.org/wiki/Wikimedia_APIs/Access_policy),
[rate limits](https://www.mediawiki.org/wiki/Wikimedia_APIs/Rate_limits), and
[User-Agent policy](https://foundation.wikimedia.org/wiki/Policy:Wikimedia_Foundation_User-Agent_Policy).
