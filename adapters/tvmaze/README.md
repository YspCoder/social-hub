# TVmaze public API adapter

The `tvmaze/public-api` adapter provides typed, credential-free reads against
the unversioned [TVmaze API](https://www.tvmaze.com/api):

- show search, external-ID lookup, details, seasons, episodes, cast, and crew;
- broadcast and web-channel schedules;
- people search, details, and embedded show credits;
- show and people update timestamps for incremental synchronization.

```go
adapter, err := socialhub.Open(ctx, "tvmaze/public-api", socialhub.AdapterConfig{
    Adapter:  "tvmaze/public-api",
    Accounts: []socialhub.AccountConfig{{ID: "public"}},
})
if err != nil {
    return err
}
common, err := adapter.Client(ctx, "public")
if err != nil {
    return err
}
shows, err := common.(*tvmaze.Client).SearchShows(ctx, "Severance")
```

## Operational and licensing boundaries

- No API key or user token is required. Accounts accept only an SDK-local ID.
- Send a meaningful, unique `User-Agent`; the default is `social-hub/tvmaze`.
- TVmaze documents a limit of at least 20 requests per 10 seconds per IP. A
  `429` is returned when exceeded and callers should back off before retrying.
- API data is licensed under CC BY-SA. Products must provide attribution and
  comply with ShareAlike. [Enterprise plans](https://www.tvmaze.com/api/plans)
  offer alternative licensing and SLA terms.
- The multi-megabyte `/schedule/full` response is intentionally omitted so the
  adapter can retain the shared bounded-response policy.
- External-ID lookup refuses automatic redirects. It accepts only the documented
  `301` to a canonical positive show ID on the configured API origin and path.

## Go SDK assessment

No mature, reusable Go SDK was suitable for linking as of 2026-08-02. This
adapter therefore uses social-hub's shared transport without adding a dependency.

| Project | Assessment | Decision |
|---|---|---|
| [masnun/tvmaze](https://github.com/masnun/tvmaze) | Small, inactive, no declared license | Do not use |
| [tamnd/tvmaze-cli](https://github.com/tamnd/tvmaze-cli) | Apache-2.0 CLI/application, not an established SDK | Reference only |
| [patrickbathu/golang-tvmaze-api](https://github.com/patrickbathu/golang-tvmaze-api) | MIT, minimal surface and adoption | Reference only |
| [autobrr/qui](https://github.com/autobrr/qui) | Active product with an internal GPL-2.0 client | Cannot link into this SDK |
| [autobrr/upbrr](https://github.com/autobrr/upbrr) | Active product with an internal GPL-2.0 client | Cannot link into this SDK |

The authoritative API contract remains the [official documentation](https://www.tvmaze.com/api).
