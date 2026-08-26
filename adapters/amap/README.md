# Amap Web Service Place Search v5 adapter

Adapter name: `amap/web-service-place-v5`

This package implements the current Amap Place Search 2.0 Web Service API. It
is deliberately limited to the official HTTPS origin and three read-only
routes:

| Typed workflow | Official endpoint | Local boundary |
| --- | --- | --- |
| `SearchText` | `GET /v5/place/text` | `page_size` 1..25, first 200 results |
| `SearchAround` | `GET /v5/place/around` | `0` uses the provider default; otherwise up to 50,000 m, first 200 results |
| `GetDetails` | `GET /v5/place/detail` | 1..10 POI IDs |

Legacy `/v3/place/*`, polygon search, input tips, coordinate conversion, route
planning, writes, callbacks, and automatic pagination are outside this
adapter.

## Configuration and signing

`access_token_ref` resolves a Web Service API key. If digital signing is
enabled for that key in the Amap console, `secret_ref` must resolve the private
key paired with it. Omitting `secret_ref` deliberately sends no `sig` and is
valid only for a key whose digital-signature switch is disabled.

```yaml
version: 1
platforms:
  - adapter: amap/web-service-place-v5
    product: web-service-place
    accounts:
      - id: mainland-search
        access_token_ref: env://AMAP_WEB_SERVICE_KEY
        secret_ref: env://AMAP_WEB_SERVICE_SIGNING_SECRET
      - id: research-key-without-signing
        access_token_ref: env://AMAP_RESEARCH_KEY
```

For a signed request the adapter adds `key`, sorts every request parameter by
parameter name, concatenates original values as `key=value&...`, appends the
private key without a delimiter, computes UTF-8 MD5, and sends the lowercase
hex digest as `sig`. Query encoding happens after signing, including for a
literal plus sign. Redirects and cookies are disabled. Transport errors omit
the request URL. Provider error text and raw error bodies are not retained;
errors expose only the bounded numeric `infocode`, HTTP status, retry delay,
and a fixed local message.

Each configured account resolves its own key and optional signing secret, so
multiple Amap applications can coexist in one adapter configuration.

## Coordinates, regions, and POI types

Amap documents its mainland-China coordinates as GCJ-02. `Coordinate` is
always longitude first and latitude second; requests are rounded to at most
six decimal places as required by Place Search v5. Convert WGS84, BD-09, or
other coordinate systems before calling `SearchAround`.

`region` accepts an administrative name, `citycode`, or `adcode`.
`city_limit=true` requires `region` and strictly limits recall to that region.
For precise district-level filtering, prefer `adcode`. `TypeCodes` accepts
unique six-digit Amap POI typecodes. English results require Amap's advanced
multilingual service approval.

Optional `show_fields` groups are `children`, `business`, `indoor`, `navi`,
and `photos`. The adapter requests only groups selected by the caller.

## Quotas and errors

Amap does not publish one fixed allowance applicable to every account and
key. The official quota guide directs developers to Console > Traffic
Analysis > Quota Management for current call and QPS allowances. The adapter
therefore exports `QuotaDocumentationURL` but does not invent quota counters
or a static limiter.

Place APIs normally report business errors with HTTP 200 and JSON fields
`status`, `info`, and `infocode`. The adapter treats only `status=1` with
`infocode=10000` as success. Minute/QPS errors are retryable; daily, account,
manual-unblock, balance, signature, key, IP allowlist, and permission failures
are user-action errors. Provider `info` text and raw error JSON are deliberately
discarded because they are not a stable or safe SDK contract.

## Scope fit

This package is a platform-specific location extension, not a social-media
adapter. It exposes only `PlacesWorkflow`; the shared publish, fetch, media,
reaction, messaging, and webhook capabilities remain unsupported. It should
therefore not be used as the representative adapter for validating social-hub's
cross-platform social capability model.

## Certification and use restrictions

Creating a Web Service key requires an Amap developer account and developer
certification. Under the current Amap Open Platform Service Agreement,
individual research use and organizational or commercial use have different
license requirements. A legal person or organization must obtain the
applicable technical-service license before commercial use; commissioned
systems can require licenses for both the developer and operator. Configuration
of a key does not prove that this external approval exists.

The agreement also restricts direct storage, caching, scraping, downloading,
resale, sublicensing, model training, vehicle use, and use outside mainland
China without the required written permission. This SDK performs no cache or
persistence; callers remain responsible for display, retention, attribution,
privacy, and license compliance.

## Official sources

Official material reviewed on 2026-08-26:

- <https://lbs.amap.com/api/webservice/guide/api-advanced/newpoisearch>
- <https://lbs.amap.com/api/webservice/guide/create-project/get-key>
- <https://lbs.amap.com/faq/quota-key/key/41181/>
- <https://lbs.amap.com/api/webservice/guide/tools/info>
- <https://lbs.amap.com/api/webservice/guide/tools/flowlevel>
- <https://lbs.amap.com/api/javascript-api/guide/transform/convertfrom>
- <https://lbs.amap.com/home/terms/>

This package adds no dependency beyond social-hub's existing core and shared
transport.
