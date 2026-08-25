# Yandex Direct API v5 adapter

Adapter name: `yandex/direct-api-v5`

This package implements advertiser-scoped paid-media workflows:

- classic Text Campaign list/get, metadata updates, suspend/resume, and guarded
  delete;
- classic Text Ad Group list/get, batch create/update, and guarded delete;
- Keyword list/get, batch create/update/delete, and explicit suspend/resume;
- production v501 management and online/offline Reports with bounded TSV
  streaming;
- JSON-RPC envelope errors, per-item warnings/errors, `RequestId`, `Units`, and
  `Units-Used-Login` metadata.

Organic Yandex content, Ads/creatives, images, portfolio strategies, agency
client provisioning, finance methods, and Unified Performance Campaign (UPC)
management are intentionally outside this initial adapter version. Production
services use the official v501 JSON endpoints, but UPC Campaign and Ad Group
resources still require distinct schemas that are not exposed here.

## Access and account boundary

Register the application in Yandex OAuth, request and receive Yandex Direct API
access, and obtain one OAuth token for each Direct user. The application needs
the `direct:api` scope. A debugging token can modify real advertising data when
used against the production endpoint; use the official Sandbox during
development.

The adapter sends the token as:

```text
Authorization: Bearer <OAuth token>
```

For an agency token, configure `client_login`. Every request then carries the
same `Client-Login` header and is bound to exactly one client advertiser. The
optional `use_operator_units` setting sends `Use-Operator-Units: true`, charging
the agency's point balance instead of the client's. It is rejected without a
`client_login`.

Token acquisition and replacement are intentionally external. Yandex does not
provide a Direct-specific refresh contract; revoked or expired tokens must be
re-authorized and the referenced secret replaced.

## Configuration

```yaml
version: 1
platforms:
  - adapter: yandex/direct-api-v5
    product: direct-api
    settings:
      accept_language: en
    accounts:
      - id: yandex-direct-ru
        access_token_ref: env://YANDEX_DIRECT_TOKEN
        approval:
          scopes: [direct:api]
        settings: {}

      - id: agency-client-a
        access_token_ref: env://YANDEX_DIRECT_AGENCY_TOKEN
        approval:
          scopes: [direct:api]
        settings:
          client_login: client-a-login
          use_operator_units: false
```

For the official Sandbox, select both Sandbox origins so management and
Reports stay isolated from production:

```yaml
settings:
  base_url: https://api-sandbox.direct.yandex.com/json/v5
  reports_base_url: https://api-sandbox.direct.yandex.com/json/v5
  accept_language: en
```

`client_id`, `app_id`, `secret_ref`, `token_store`, and webhook settings are
rejected because they are not part of the runtime Direct request contract.
Only the exact production v501 pair or official Sandbox v5 pair is accepted;
arbitrary gateways and mixed production/Sandbox pairs are rejected. Redirects
and the configured HTTP client's Cookie Jar are disabled so OAuth and
advertiser headers cannot cross origins.

## Campaign strategy boundary

Yandex has migrated manual campaigns from `DailyBudget` to weekly budgets.
The current `Campaigns.add` schema does not define
`TextCampaign.BiddingStrategy.Search.HighestPosition.WeeklySpendLimit`;
`WeeklySpendLimit` for `HIGHEST_POSITION` belongs to the separate `Strategies`
service and must be attached through `PackageBiddingStrategy`. Campaign
creation and strategy mutation are therefore deliberately outside this
adapter version: implementing them safely requires a typed Strategies workflow
and explicit reconciliation for a strategy created before a Campaign failure.

`UpdateCampaigns` only writes `Name`, `StartDate`, and `EndDate`. Reads retain
the current strategy type so callers can identify existing classic Text
Campaigns without the adapter inventing a legacy budget wire shape.

All monetary values exposed by Keyword and Report operations use Yandex's exact
integer micro-unit contract: account currency multiplied by `1,000,000`. The
adapter never converts money through `float64`.

Ad Group and Keyword creation first reads the parent Campaign and requires it
to be either explicitly `SUSPENDED` or newly `DRAFT`/`OFF`. Yandex has no Ad
Group-level suspend method. Keyword resume and Campaign resume remain explicit
operations because either can affect live delivery.

## Batch outcomes and errors

Yandex processes object arrays independently. Every mutation returns a
`BatchResult` containing the original `ActionResult` entries and response
metadata:

- a mixed successful/failed batch returns its result with
  `ErrPartialMutation`;
- a completely rejected batch returns the first typed provider error while all
  per-item errors remain available in the result;
- a transport failure, 5xx, malformed success, or missing per-item result for a
  mutation returns `ErrOutcomeUnknown`; reconcile by ID/provider state before
  retrying;
- provider warnings do not fail successful items and remain attached to them.

Provider and per-item failures can be inspected as `*yandexdirect.APIError`.
Its numeric platform code and `Metadata` retain sanitized `RequestId`, `Units`,
and `Units-Used-Login`, including on HTTP 200 business errors and
unknown-outcome mutation failures. Provider error text is restricted to valid
single-line UTF-8 and common credential markers are redacted before it enters
the platform-neutral error.

Yandex assigns `RequestId`; caller-generated request IDs and generic
idempotency keys are rejected. `WithCallTimeout` is supported, and each
caller-supplied `CallOption` is evaluated once even when a guarded mutation
performs multiple provider requests; its deadline covers the complete guarded
workflow instead of resetting for each preflight request. Typed get methods use fixed field lists
and return `LimitedBy`; pass it as the next `PageRequest.Offset`.

## Reports

`GenerateReport` posts the same definition to the configured Reports service
on every poll. Production defaults to `/json/v501/reports`; the official
Sandbox remains `/json/v5/reports`. In `auto` or `offline` mode it can return:

- `ReportReady` / HTTP 200: TSV was streamed to the supplied writer;
- `ReportQueued` / HTTP 201: repeat after `RetryAfter`;
- `ReportProcessing` / HTTP 202: repeat the identical request after
  `RetryAfter`.

Yandex documents the 201/202 status and retry headers, but does not guarantee
an empty entity body. The adapter consumes any such body with a 1 MiB bound and
still recognizes an explicit JSON error envelope.

`SEARCH_QUERY_PERFORMANCE_REPORT` is restricted to explicit `offline` mode.
Report monetary values remain integer micro-units because the adapter does not
send `returnMoneyInMicros: false`. Downloads reject redirects, compression, and
unexpected media types, and write at most `MaxBytes` (256 MiB by default).
Offline report names must remain unique per user when definitions differ.

## Limits and operations

- Management allows at most five concurrent requests per advertiser.
- Management uses advertiser/agency point balances. Each response's `Units`
  is parsed as `spent/remaining/daily-limit`; coordinate limiting across
  replicas and treat provider headers as authoritative.
- Campaign add/update are bounded to 10 items; Campaign
  suspend/resume/delete to 1,000; Ad Group add/update/delete to 1,000;
  same-group Keyword creation to 200; Keyword update to 1,000; and Keyword
  suspend/resume/delete to 10,000. The provider's account-specific
  `KEYWORDS_TOTAL_PER_ADGROUP` restriction remains authoritative.
- Reports allow 20 requests per 10 seconds per user and at most five queued
  offline reports; generated offline reports are retained for five hours.
- The application must have approved API access, the user must accept the API
  agreement, and account/IP/agency permissions still apply independently of
  OAuth token validity.

No real Yandex advertiser credentials were used for local verification.

## Sources and implementation reference

- [Yandex Direct API v5](https://yandex.com/dev/direct/doc/en/)
- [Production v501 Campaigns service](https://yandex.com/dev/direct/doc/en/campaigns/campaigns)
- [Official Sandbox v5 endpoints](https://yandex.com/dev/direct/doc/en/concepts/sandbox)
- [Access and authorization](https://yandex.com/dev/direct/doc/en/concepts/access)
- [JSON request/response format](https://yandex.com/dev/direct/doc/en/concepts/json)
- [Restrictions and points](https://yandex.com/dev/direct/doc/en/concepts/units)
- [Campaigns.add](https://yandex.com/dev/direct/doc/en/campaigns/add)
- [Current Campaigns.add budget migration notice](https://yandex.ru/dev/direct/doc/ru/campaigns/add)
- [AdGroups.add](https://yandex.com/dev/direct/doc/en/adgroups/add)
- [Keywords service](https://yandex.com/dev/direct/doc/en/objects/keyword)
- [Reports service](https://yandex.com/dev/direct/doc/en/reports)
- [Online/offline Reports](https://yandex.com/dev/direct/doc/en/mode)
- [API v5 changelog](https://yandex.com/dev/direct/doc/en/changelog)
- [biplane/yandex-direct](https://github.com/biplane/yandex-direct) - mature
  MIT-licensed PHP client reviewed for service separation and report-builder
  ergonomics; no runtime dependency or copied generated contract is used.

Official contracts and the reference project were reviewed on 2026-08-25.
