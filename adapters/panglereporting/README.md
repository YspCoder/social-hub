# Pangle Publisher Reporting API 2.0 adapter

`adapters/panglereporting` implements Pangle Publisher Reporting API 2.0 for
revenue generated outside the Chinese mainland.

Official references:

- Reporting API 2.0: <https://www.pangleglobal.com/integration/reporting-api-v2>
- Publisher role and Security Key setup: <https://www.pangleglobal.com/integration/platform-configuration-for-mediation>
- Separate App and Ad Placement Management API: <https://www.pangleglobal.com/integration/management-api>

The official contract was reviewed on 2026-08-25. It was updated on 2026-02-09
to add the `app_id` request filter and improve empty `os` handling. GitHub
repository and code searches for Pangle API, the current API origin, and Pangle
Reporting API Go clients returned no reusable implementation, so this adapter
implements the official wire contract directly.

## Implemented surface

| Workflow | Endpoint | Notes |
|---|---|---|
| `IncomeReport` | `GET /union_pangle/open/api/rt/income` | One day per request; optional app, region, timezone, currency, and grouping filters |

The response model includes publisher, app, placement, delivery, bidding, and
estimated revenue metrics. `Decimal` preserves the exact JSON text for revenue,
eCPM, and rates instead of converting through `float64`. Pangle silently
truncates reports at 100,000 rows; `Report.MayBeTruncated` is therefore true
when the response reaches that boundary.

## Authentication and configuration

Pangle uses a master `user_id`, a permission-scoped `role_id`, and that role's
Security Key. This is not OAuth. The adapter resolves the Security Key only
from `secret_ref`, adds a current Unix timestamp, sorts all request parameters,
and applies Pangle's required MD5 signature algorithm. Timestamp drift greater
than ten minutes is rejected by Pangle.

```yaml
version: 1
platforms:
  - adapter: pangle/publisher-reporting-api-v2
    product: publisher-reporting-api
    accounts:
      - id: pangle-global-publisher
        secret_ref: env://PANGLE_REPORTING_SECURITY_KEY
        settings:
          user_id: "459"
          role_id: "459"
```

The role only receives data for apps it is authorized to access. Obtain the
Role ID and Security Key from Pangle under `Integrations -> SDK & API -> Pangle
Reporting API 2.0`.

## Contract and security boundaries

- The official origin is `https://open-api.pangleglobal.com`; it returns only
  revenue generated outside the Chinese mainland. The domestic 穿山甲 contract
  is a separate product and is not inferred from this API.
- Pangle allows 2 requests per second. `DefaultQuotaPolicy` also exposes the
  ten-minute timestamp skew and 100,000-row response boundary.
- Business code `100` is success. `PD0004` is a successful no-data result;
  signature, account, date, parameter, region, and QPS failures are mapped to
  the common error taxonomy.
- The Security Key, canonical signing input, response body, and signed request
  URL are never included in returned errors. Redirects and Cookie Jars are
  disabled, and the API origin cannot be overridden.
- JSON responses default to a 128 MiB limit, configurable from 1 MiB through
  512 MiB for controlled deployments. Signed query strings are limited to
  32 KiB.
- `time_zone` is optional. Omitting it preserves Pangle's UTC+8 default;
  explicitly pass `TimeZoneUTC` for UTC+0. Billing remains based on UTC+8.
- `currency` is optional and defaults to CNY. Pangle supports `cny` and `usd`.
- Per-call configuration accepts only `socialhub.WithCallTimeout`; caller
  request IDs, idempotency keys, and generic field selection are rejected.

Pangle's App and Ad Placement Management API uses a different SHA-1
nonce/timestamp signature and POST JSON contract. It should be implemented as
a separate typed adapter rather than sharing this reporting signature.
