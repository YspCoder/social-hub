# Google Ad Manager API v1 adapter (Beta)

`adapters/admanager` implements publisher-side inventory, delivery, and
Interactive Report workflows for Google Ad Manager API v1. It is distinct from
the buyer and advertiser products already covered by social-hub:

- Google Ads manages search and other Google Ads campaigns.
- Display & Video 360 manages programmatic media buying.
- Campaign Manager 360 manages advertiser-side trafficking and ad serving.
- Google Ad Manager manages a publisher's inventory, Orders, LineItems, and
  reporting.

Adapter identifier:

```text
google/ad-manager-api-v1
```

The REST API is currently marked Beta by Google even though its wire version is
`v1`.

## Contract baseline

The adapter was verified on 2026-08-10 against:

- [Ad Manager API v1 REST reference](https://developers.google.com/ad-manager/api/beta/reference/rest)
- [Discovery document revision 20260806](https://admanager.googleapis.com/$discovery/rest?version=v1)
- [Official v1 protobuf contracts at commit cb8b758](https://github.com/googleapis/googleapis/tree/cb8b7583e76f7fb9c2666d8e1fc6a7e55b402400/google/ads/admanager/v1)
- [Authentication and scope guidance](https://developers.google.com/ad-manager/api/beta/authentication)
- [Interactive Report workflow](https://developers.google.com/ad-manager/api/beta/reports)
- [Filtering and ordering syntax](https://developers.google.com/ad-manager/api/beta/filters)
- [Ad Manager quota best practices](https://developers.google.com/ad-manager/api/bestpractices)

The official Proto and Discovery contracts are references rather than runtime
dependencies. The adapter uses social-hub's bounded shared transport and keeps
the opt-in package dependency surface unchanged.

## Supported workflows

| Resource | Operations | Required scope |
|---|---|---|
| Network | Get the configured network | `admanager` or `admanager.readonly` |
| Company | Get and paginated list | `admanager` or `admanager.readonly` |
| AdUnit | Get and paginated list | `admanager` or `admanager.readonly` |
| Order | Get and paginated list | `admanager` or `admanager.readonly` |
| LineItem | Get and paginated list | `admanager` or `admanager.readonly` |
| Report | Get/list and create forced-hidden reports | Read scope; create requires `admanager` |
| Report run | Start and poll a long-running report operation | Either scope |
| Report result | Fetch up to 10,000 typed rows per page | Either scope |

Order, LineItem, Company, and AdUnit mutations are deliberately absent. The v1
API exposes several of those methods, but the initial adapter does not have an
officially documented invariant that a partially configured resource cannot
become eligible to serve. Adding each mutation requires a separately reviewed,
typed safe-state workflow.

Report definitions initially cover dimensions, metrics, primary/comparison
date ranges, report type, time zone, currency, and expanded compatibility.
Filters, sorts, flags, scheduled delivery, and custom-field dimension slots are
not yet exposed rather than accepting untyped pass-through payloads.

## Access and OAuth

The caller needs an Ad Manager network, a Google Cloud project with the API
enabled, and an Ad Manager user or service-account user whose roles and teams
permit the requested resources. OAuth scopes do not bypass Ad Manager UI
permissions.

Supported scopes are:

```text
https://www.googleapis.com/auth/admanager
https://www.googleapis.com/auth/admanager.readonly
```

Managed OAuth defaults to `admanager.readonly`. Configure the full scope
explicitly when the application must call `CreateHiddenReport`. A caller that
manages service-account or other access tokens externally can supply the token
through `access_token_ref`.

## Configuration

One social-hub account binds to one Ad Manager network. Use separate account
entries for separate networks or credentials.

```yaml
adapters:
  - adapter: google/ad-manager-api-v1
    product: ad-manager-api
    accounts:
      - id: publisher-production
        client_id: ${GOOGLE_CLIENT_ID}
        secret_ref: env://GOOGLE_CLIENT_SECRET
        approval:
          scopes:
            - https://www.googleapis.com/auth/admanager.readonly
        settings:
          network_code: "123456789"
          refresh_token_ref: env://AD_MANAGER_REFRESH_TOKEN
```

For caller-managed token rotation, set `access_token_ref` and omit
`refresh_token_ref`. `client_id` and `secret_ref` may remain configured together
when the application also uses `Adapter.OAuth` to obtain new consent.

## Safety invariants

- Every resource, nested owner, report operation, and report result name must
  belong to the configured `network_code`.
- `CreateHiddenReport` always sends `visibility: HIDDEN` and rejects a response
  that is not hidden.
- All list calls are bounded to 1,000 resources and return one platform page.
  A fixed response field mask requests `totalSize` explicitly.
- Report result reads are bounded to 10,000 rows per page and never auto-drain.
- Full-scope operations are blocked locally when configured approval metadata
  contains only `admanager.readonly`.
- Credential-bearing HTTP clients reject redirects.
- Canonical Google status messages and asynchronous failure messages are
  bounded and scrubbed for common credential markers.

## Usage

Import the package for registration:

```go
import _ "social-hub/adapters/admanager"
```

Read publisher inventory:

```go
client := common.(*admanager.Client)

units, err := client.ListAdUnits(ctx, admanager.ListRequest{
    PageSize: 100,
    Filter:   `status = ACTIVE`,
    OrderBy:  `displayName`,
})
```

Create and run a report without making it visible in the Ad Manager UI:

```go
report, err := client.CreateHiddenReport(ctx, admanager.CreateReportRequest{
    DisplayName: "Yesterday by line item",
    Definition: admanager.ReportDefinition{
        Dimensions: []admanager.Dimension{
            admanager.DimensionLineItemName,
            admanager.DimensionLineItemID,
        },
        Metrics:   []admanager.Metric{admanager.MetricAdServerImpressions},
        DateRange: admanager.DateRange{Relative: admanager.RelativeYesterday},
        ReportType: admanager.ReportHistorical,
    },
})
operation, err := client.RunReport(ctx, report.ReportID)
```

Poll `GetReportOperation` with exponential backoff. When `Done` is true and
`Result` is present, pass `Result.ReportResult` to `FetchReportRows`. A completed
operation can instead contain `Failure`; the SDK does not retry long-running
operations implicitly.

## Quotas

Google's published Ad Manager guidance documents a network-level limit of 2
requests per second for standard accounts and 8 requests per second for Ad
Manager 360, plus separate reporting-system limits. `DefaultQuotaPolicy`
exposes those starting values, a five-second initial quota retry delay, and the
v1 list/report page maxima. Account-specific quotas, `Retry-After`,
`RESOURCE_EXHAUSTED`, and report job limits remain authoritative.
