# W3C Micropub adapter

Adapter name: `indieweb/micropub`

This package implements the stable W3C Micropub Recommendation for publishing
to an IndieWeb site. It is a protocol adapter, not an adapter for one hosted
social network. A configured account targets one caller-selected Micropub
endpoint and one site identity.

## Implemented contracts

| Surface | Support |
|---|---|
| Common `Publisher` | Form-encoded `h-entry` text posts, URL replies, source-based status, and configured deletion |
| Common `Fetcher` | Configured site identity and `q=source` post reads when update support is enabled |
| Typed `EntryWorkflow` | JSON create, update replace/add/delete, delete, undelete, and source queries |
| Typed `QueryWorkflow` | `q=config` and `q=syndicate-to` |
| Typed `MediaWorkflow` | Exact-length streaming multipart upload to a discovered Media Endpoint |
| Common `MediaUploader` | Not exposed; a Media Endpoint upload is not a resumable multi-part session |
| Reactions/messages/webhooks | Not defined by Micropub |

Common `Publish` deliberately rejects media IDs, quote posts, and non-public
visibility because Micropub cannot infer the intended Microformats2 property
without losing meaning. Use `EntryWorkflow` for photo, video, audio, HTML,
like-of, repost-of, categories, syndication, or extension properties.

## Configuration

```yaml
adapter: indieweb/micropub
product: micropub
accounts:
  - id: personal-site
    access_token_ref: env://MICROPUB_ACCESS_TOKEN
    settings:
      endpoint: https://example.com/micropub?tenant=personal
      site_url: https://example.com/
      supports_update: true
      supports_delete: true
      supports_undelete: false
    approval:
      scopes: [create, update, delete]
```

`endpoint` and `site_url` must be absolute HTTP(S) URLs without credentials or
fragments. An endpoint's existing query string is preserved when the adapter
adds `q`, `url`, or `properties[]` query parameters.

Micropub does not define a capability query for update, delete, or undelete.
The corresponding settings are therefore explicit claims about the configured
server. `supports_undelete` requires `supports_delete`. When approval scopes
are configured, the adapter rejects missing `create`, `update`, or `delete`
permissions before sending a request. Empty scopes defer authorization to the
server because Micropub servers may define finer-grained scope names.

The adapter accepts a caller-managed bearer token through `access_token_ref`.
It does not implement IndieAuth authorization endpoint discovery or an OAuth
callback server; those have a separate lifecycle and are not part of the
Micropub Recommendation.

## Typed entry workflow

```go
common, err := adapter.Client(ctx, "personal-site")
if err != nil {
    return err
}
client := common.(*micropub.Client)

created, err := client.CreateEntry(ctx, micropub.EntryCreateRequest{
    Name:       "Release notes",
    Content:    micropub.Content{HTML: "<p>Version 1 is available.</p>"},
    Categories: []string{"go", "indieweb"},
    Photos: []micropub.Photo{{
        Value: "https://cdn.example.com/release.png",
        Alt:   "Release dashboard",
    }},
    SyndicateTo: []string{"archive-target"},
})
```

`ExtraProperties` accepts arrays of validated `json.RawMessage` values so
nested Microformats2 objects remain lossless. It rejects reserved names and
cannot overwrite typed fields. Update operations also use raw JSON arrays,
matching the protocol's requirement that every replacement/addition/removal
value be an array. Whole-property deletion and per-value deletion are separate
request fields because Micropub encodes those as different JSON shapes and
cannot combine both shapes in one request.

Create requires `201 Created` or `202 Accepted` plus an absolute `Location`.
Update/delete/undelete require `200`, `201`, or `204`; a `201` response also
requires `Location`. `shortlink` and `syndication` Link headers are preserved
in `EntryResult`.

## Queries and media

`Config` treats a `400` response or malformed successful response as an empty
configuration, as required for compatibility with servers that do not
implement `q=config`. Authentication, permission, rate-limit, and server
failures remain visible. `Source` supports repeated `properties[]` values and
returns lossless Microformats2 property arrays.

Use the `media-endpoint` URL returned by `Config` for uploads:

```go
config, err := client.Config(ctx)
if err != nil {
    return err
}
media, err := client.UploadMedia(ctx, micropub.MediaUploadRequest{
    Endpoint: config.MediaEndpoint,
    Filename: "release.png",
    MIME:     "image/png",
    Size:     size,
}, reader)
```

The upload streams one multipart field named `file`, enforces the declared
byte count, and requires `201 Created` plus an absolute `Location`. The Media
Endpoint may be on a different origin because the Recommendation explicitly
requires it to accept the same bearer token. All HTTP redirects are rejected,
including same-origin redirects, so credentials are never forwarded to a
redirect target.

## Errors and implementation references

Micropub errors `unauthorized`, `forbidden`, `insufficient_scope`, and
`invalid_request` map to the common authentication, permission, approval, and
validation categories. Response bodies are bounded and only sanitized error
codes/descriptions are retained.

The MIT-licensed `hacdias/indielib` and `hawx/tally-ho` projects were reviewed
for real-world server/parser behavior and contract ideas. Neither provides a
complete client matching this repository's capability, error, streaming, and
credential boundaries, so this adapter adds no external dependency.

Official references:

- <https://www.w3.org/TR/micropub/>
- <https://www.w3.org/TR/indieauth/>
- <https://micropub.rocks/>

Reviewed projects:

- <https://github.com/hacdias/indielib>
- <https://github.com/hawx/tally-ho>
