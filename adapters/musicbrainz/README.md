# MusicBrainz WS/2 adapter

The `musicbrainz/ws-v2` adapter provides typed, credential-free access to the
[MusicBrainz Web Service](https://musicbrainz.org/doc/MusicBrainz_API):

- Artist, Release Group, Release, Recording, and Work search and lookup;
- Artist release-group and recording browse workflows;
- offset pagination mapped to `socialhub.Page` cursors;
- fixed include sets for aliases, credits, genres, labels, media, and relations.

```go
adapter, err := socialhub.Open(ctx, "musicbrainz/ws-v2", socialhub.AdapterConfig{
    Adapter:  "musicbrainz/ws-v2",
    Accounts: []socialhub.AccountConfig{{ID: "public"}},
})
if err != nil {
    return err
}
common, err := adapter.Client(ctx, "public")
if err != nil {
    return err
}
artists, err := common.(*musicbrainz.Client).SearchArtists(ctx, musicbrainz.SearchRequest{
    Query: `artist:"The Beatles"`, Limit: 10,
})
```

## Operational and licensing boundaries

- No API key or token is required. Accounts accept only an SDK-local ID.
- MusicBrainz requires a meaningful, contactable `User-Agent`; the default points
  to the social-hub repository and should be replaced by downstream applications.
- The public service allows an average of one request per second per source IP
  unless another arrangement exists. A shared adapter-level gate defaults to
  `1.1s` to leave boundary margin, and throttling `503` responses map to
  retryable rate-limit errors.
- `request_interval` may be changed explicitly for a local mirror or a separately
  agreed service level. Multiple clients from one adapter share the same gate.
- Core database data is CC0. Supplementary data is CC BY-NC-SA 3.0; consumers
  must determine which fields they retain and comply with the applicable license.
  Commercial users should review MetaBrainz licensing rather than assuming every
  response field is unrestricted.
- Cover art belongs to the separate Cover Art Archive and is not exposed here.
- Automated metadata polling is intentionally absent; MusicBrainz explicitly
  discourages polling to detect changes.

Official references: [API](https://musicbrainz.org/doc/MusicBrainz_API),
[rate limiting](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting), and
[data licenses](https://musicbrainz.org/doc/About/Data_License).

## Go SDK assessment

No mature reusable Go SDK met this module's maintenance and licensing bar as of
2026-08-02, so the adapter uses social-hub's bounded transport without a new dependency.

| Project | Assessment | Decision |
|---|---|---|
| [michiwend/gomusicbrainz](https://github.com/michiwend/gomusicbrainz) | MIT, 63 stars, explicitly WIP; core code last pushed in 2023 | Reference only |
| [sentriz/gonic](https://github.com/sentriz/gonic) | Mature GPL-3.0 application with an internal client | Cannot link into this SDK |
| [sentriz/wrtag](https://github.com/sentriz/wrtag) | Active GPL-3.0 tagging application, not an SDK | Reference only |
| [jpdillingham/brainz](https://github.com/jpdillingham/brainz) | Small MIT CLI, not an established client library | Reference only |
