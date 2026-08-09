# Google AdSense Management API v2 adapter

Adapter name: `google/adsense-management-api-v2`

This adapter exposes publisher-account-bound, read-only workflows for the
[AdSense Management API v2](https://developers.google.com/adsense/management/reference/rest).
The contract was checked against Discovery revision `20260806` on 2026-08-10.

## Authentication

AdSense uses Google's OAuth 2.0 web-server authorization-code flow. Service
accounts are not supported. Managed OAuth requests offline access, preserves
the refresh token, rejects credential-bearing redirects, and can cache access
tokens through `socialhub.TokenStore`.

The default managed scope is:

```text
https://www.googleapis.com/auth/adsense.readonly
```

The full `https://www.googleapis.com/auth/adsense` scope is accepted when it
has been explicitly approved, but this adapter still exposes only the bounded
read workflows below.

```yaml
version: 1
platforms:
  - adapter: google/adsense-management-api-v2
    product: adsense-management-api
    accounts:
      - id: publisher-main
        client_id: google-oauth-client-id
        secret_ref: env://GOOGLE_OAUTH_CLIENT_SECRET
        approval:
          scopes:
            - https://www.googleapis.com/auth/adsense.readonly
        settings:
          publisher_id: pub-1234567890123456
          refresh_token_ref: env://GOOGLE_ADSENSE_REFRESH_TOKEN
```

A caller-managed access token can be configured with `access_token_ref`
instead of the client secret and refresh-token fields.

## Workflows

- Account details, direct child accounts, and the ad-blocking recovery tag.
- Ad clients and their AdSense code.
- Ad units, ad code, linked custom channels, custom channels, URL channels,
  and sites.
- Alerts, payments, and policy issues.
- Bounded JSON ad hoc reports and saved-report generation.

All caller-supplied IDs are converted to resource names beneath the configured
`accounts/pub-...` account. Returned primary resources are checked against the
same owner. Policy issues are the documented exception: associated host or
secondary ad clients can belong to a different account, so those references
are validated for shape without being treated as owned resources.

Ad hoc JSON reports default to 10,000 rows and are capped at the API maximum of
100,000 rows. Response headers must exactly match requested dimensions followed
by metrics, and row widths are verified. Use `Dimension("NEW_FIELD")` or
`Metric("NEW_FIELD")` for newly released uppercase Discovery fields before a
named SDK constant is added.

## Deliberate boundaries

The adapter does not expose `adunits.create`, `adunits.patch`, or custom-channel
mutation methods. Google documents those methods as restricted to eligible
AdSense for Platforms projects. More importantly, an `ARCHIVED` ad unit can
continue serving ads, so there is no generally available state transition that
this SDK can prove is non-delivering. This boundary avoids presenting a
misleading safe-mutation contract.

CSV report endpoints are not exposed in this layer. JSON is bounded and fully
validated by the shared transport; CSV can be added later with a separately
bounded streaming contract rather than buffering a possible one-million-row
response.

## Published quotas

Google currently documents 100 requests per minute per user/project, 500 per
minute per project, and 10,000 requests per day. The daily quota resets around
Pacific midnight. Report rows also consume a dynamic quota and can return HTTP
429 independently of request quotas. JSON reports are limited to 100,000 rows;
CSV reports are limited to 1,000,000 rows.

Runtime response headers, Google Cloud Console assignments, and dynamic report
row quotas remain authoritative. See the official
[limits](https://developers.google.com/adsense/management/appendix/limits),
[release notes](https://developers.google.com/adsense/management/release_notes),
and [direct request guide](https://developers.google.com/adsense/management/direct_requests).
