# Admitad Publisher API adapter

Registration name: `admitad/publisher-api`

This package implements a deliberately bounded subset of the current,
unversioned Admitad Publisher API:

- ad-space affiliate program listing and detail;
- batch deeplink generation (up to 200 `ulp` values);
- ad-space coupon discovery;
- non-aggregate campaign statistics.

Official documentation is hosted in the [Mitgo Developer Center](https://developers.mitgo.com/hc/en-us/categories/34481291136402-Publisher-API).
The contract was rechecked on 2026-08-24.

## Authentication

The adapter accepts either an externally managed bearer token through
`access_token_ref`, or an Admitad application `client_id` and `secret_ref`.
Managed accounts use `POST /token/` with OAuth2 Client Credentials, persist the
returned access and refresh tokens through an optional `socialhub.TokenStore`,
and refresh before expiry. If Admitad reports that a refresh token is no longer
usable, the source obtains a new Client Credentials token.

Client Credentials requires at least one configured approval scope. The
implemented workflows use:

- `advcampaigns_for_website`
- `deeplink_generator`
- `coupons_for_website`
- `statistics`

```yaml
version: 1
platforms:
  - adapter: admitad/publisher-api
    product: publisher-api
    accounts:
      - id: publisher-main
        client_id: your-admitad-client-id
        secret_ref: env://ADMITAD_CLIENT_SECRET
        approval:
          account_type: publisher
          scopes:
            - advcampaigns_for_website
            - deeplink_generator
            - coupons_for_website
            - statistics
```

Website and campaign IDs are workflow parameters because one Publisher API
application can access multiple ad spaces. Provider numeric fields are exposed
as `ExactValue`; response objects and envelopes retain `Raw` JSON.

## Contract notes

- Pagination defaults to `limit=20&offset=0`; the documented maximum limit is
  500.
- Deeplink targets are repeated `ulp` query parameters. SubID lengths are
  validated at 50 characters for `subid` through `subid3` and 120 for `subid4`.
- `ListCampaignStatistics` always sends `total=0`. Admitad changes that endpoint
  to a top-level aggregate array when `total=1`, which is intentionally outside
  this list method.
- Admitad documents a 600 requests/minute/application quota for this Publisher
  API. Its legacy error code `4` can arrive with HTTP 503; the adapter maps that
  combination to `socialhub.CodeRateLimited` and `ClassRetryable`.
