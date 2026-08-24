# Mailchimp Marketing API 3.0 adapter

Registration name: `mailchimp/marketing-api-v3.0`

`adapters/mailchimp` is a privacy-bounded, read-only adapter for Mailchimp
Marketing API 3.0. It exposes account-level audience/list metadata, campaign
metadata, and aggregate campaign reports. It does not expose contacts,
members, email addresses, individual activity, campaign content, sending, or
any mutation.

The official Swagger contract reviewed for this implementation reports
`info.version` `3.0.91` and `basePath` `/3.0`.

## Implemented surface

| social-hub method | Marketing API operation | Returned data |
| --- | --- | --- |
| `Marketing().ListAudiences` | `GET /3.0/audiences` | Audience ID/name, enabled channels, total contact count, `total_items` |
| `Marketing().GetAudience` | `GET /3.0/audiences/{audience_id}` | Same non-PII audience metadata |
| `Marketing().ListLists` | `GET /3.0/lists` | Legacy List identity, configuration flags, aggregate list stats, `total_items` |
| `Marketing().GetList` | `GET /3.0/lists/{list_id}` | Same privacy-bounded List metadata |
| `Marketing().ListCampaigns` | `GET /3.0/campaigns` | Campaign identity/state, aggregate recipients, selected settings/tracking/report summary |
| `Marketing().GetCampaign` | `GET /3.0/campaigns/{campaign_id}` | Same campaign metadata |
| `Marketing().ListReports` | `GET /3.0/reports` | Aggregate sent-campaign performance and `total_items` |
| `Marketing().GetReport` | `GET /3.0/reports/{campaign_id}` | Aggregate performance for one sent campaign |

Every request uses a fixed provider `fields` projection. The Lists projection
excludes footer contact/address/phone data, campaign defaults, notification
emails, signup URLs, and Email Beamer addresses. Campaigns exclude reply-to
addresses, sender identity, segment member rules, and content. Reports exclude
share passwords and member-level activity. The response decoder also rejects
these sensitive keys if Mailchimp unexpectedly returns them despite the
projection.

The following endpoint families are deliberately absent:

- `/lists/{list_id}/members` and all contact/member activity, event, note, tag,
  abuse, unsubscribe, or permanent-delete operations;
- `/audiences/{audience_id}/contacts` and archive/forget actions;
- report email activity, sent-to, open member details, click member details,
  unsubscribe details, and shared-report credentials;
- campaign content, send checklists, feedback, create/update/delete, schedule,
  send, pause, resume, cancel, replicate, and test actions;
- transactional messaging, batch operations, webhooks, templates, files, and
  every other Marketing API product family.

## API-key authentication and data center

This adapter supports an externally managed Mailchimp API key using HTTP Basic
Authentication. The Basic username is the non-secret constant `social-hub` and
the API key is the password. `secret_ref` is resolved once when the client is
created; key acquisition and rotation remain application responsibilities.

Mailchimp's account-specific API root is:

```text
https://<dc>.api.mailchimp.com/3.0/
```

The adapter derives `<dc>` from the validated API-key `key-dc` suffix. If a
credential has no usable suffix, `account.settings.data_center` may explicitly
supply a strictly validated `us<number>` value. If both sources exist, they
must match. There is no `base_url` option and no arbitrary host override.

```yaml
version: 1
platforms:
  - adapter: mailchimp/marketing-api-v3.0
    product: marketing-api
    settings:
      user_agent: social-hub/mailchimp
    accounts:
      - id: primary
        secret_ref: env://MAILCHIMP_API_KEY
        approval:
          account_type: api_key
        # Needed only when the key has no normal -usN suffix.
        settings:
          data_center: us6
```

OAuth 2 Bearer authentication, OAuth metadata lookup, authorization-code
exchange, token persistence, and refresh are intentionally not implemented.
Client IDs, access-token references, token stores, OAuth scopes, and webhook
credentials are rejected from configuration.

Capability approval is reported as `Granted` only when configuration records
`approval.account_type: api_key`; otherwise it remains `Unknown` because the
SDK cannot infer the creating user's current Mailchimp role from the key.

Mailchimp API access is tied to the role of the user that created the key. A
role change, user removal, or account deactivation can revoke access even when
the key format remains valid; those failures are returned as typed
authentication or permission errors.

## Use

```go
import (
    "context"
    "fmt"

    "social-hub/adapters/mailchimp"
    "social-hub/pkg/socialhub"
)

func listCampaigns(ctx context.Context) error {
    config := socialhub.AdapterConfig{
        Adapter: "mailchimp/marketing-api-v3.0",
        Product: "marketing-api",
        Accounts: []socialhub.AccountConfig{{
            ID:        "primary",
            SecretRef: "env://MAILCHIMP_API_KEY",
            Approval:  socialhub.ApprovalConfig{AccountType: "api_key"},
        }},
    }

    adapter, err := socialhub.Open(ctx, "mailchimp/marketing-api-v3.0", config)
    if err != nil {
        return err
    }
    defer adapter.Close()

    generic, err := adapter.Client(ctx, "primary")
    if err != nil {
        return err
    }
    api := generic.(*mailchimp.Client).Marketing()

    page, err := api.ListCampaigns(ctx, mailchimp.ListCampaignsRequest{
        Page:          mailchimp.Pagination{Count: 50, Offset: 0},
        Status:        mailchimp.CampaignStatusSent,
        SortField:     mailchimp.CampaignSortSendTime,
        SortDirection: mailchimp.SortDescending,
    })
    if err != nil {
        return err
    }
    for _, campaign := range page.Campaigns {
        fmt.Println(campaign.ID, campaign.Settings.Title, campaign.EmailsSent)
    }
    return nil
}
```

Collection requests use Mailchimp's typed `count`/`offset` pagination.
`count=0` omits the parameter and uses the provider default of 10; an explicit
count cannot exceed the Swagger maximum of 1000. Results retain the request
pagination in `Page` and the provider's matching collection size in
`TotalItems`. The adapter does not auto-page.

Date filters are RFC 3339/ISO 8601 timestamps. Paired `since` and `before`
values are checked for chronological order. Generic
`socialhub.WithFields`, caller request IDs, and idempotency keys are rejected;
only a per-call timeout is supported.

## Errors, concurrency, and data handling

Mailchimp documents a limit of 10 simultaneously processing Marketing API
requests per user, not per API key or client. Additional requests receive HTTP
429. At exceptionally high volume, Mailchimp may return HTTP 429 or 403 without
a JSON body. `DocumentedConcurrencyLimit` and every `ResponseMeta` expose the
current documented value of 10; 429 responses set `ConcurrencyLimited` and
map to retryable `rate_limited`. `Retry-After` supports delta seconds and HTTP
dates, and observed rate/concurrency response headers are retained in
`LimitHeaders`.

Mailchimp does not publish one stable fixed requests-per-window quota for this
surface, so this adapter does not invent or enforce one. Callers should cap
concurrency per Mailchimp user, honor `Retry-After`, and use current provider
responses and documentation as authoritative.

`APIError` preserves Mailchimp's Problem Detail document (`type`, `title`,
`status`, `detail`, `instance`, and optional field errors) while wrapping a
platform-neutral `socialhub.Error`. The problem `instance` is used as the
request/support ID when no request header is available. Empty non-JSON 429/403
bodies still receive status-based classification. Error `Raw` values are
independently bounded to 64 KiB, recursively redacted, and remain valid JSON
when the provider returns text or an oversized body.

Successful responses must be bounded JSON objects with JSON media types.
Entity IDs, campaign enums, aggregate counts, pagination totals, and detail ID
ownership are checked before values are returned. Each entity and collection
retains the fixed-field provider JSON in `Raw`. The exact configured API key,
the exact generated Basic Authorization value, and explicit credential keys
are recursively redacted from both success and error JSON.

`Raw` remains Mailchimp account data; it is not separate authorization to
retain, republish, or reuse that data. Applications remain responsible for
Mailchimp terms, consent, retention, deletion, and access control. Provider
archive URLs are preserved as strings but are never followed by the adapter.

The HTTP origin is always a validated Mailchimp data-center subdomain. The
adapter clones the supplied HTTP client, removes its cookie jar, and disables
redirects so Basic credentials cannot cross origins. Transport errors discard
request URLs.

## Official sources

Official contracts reviewed on 2026-08-25:

- <https://api.mailchimp.com/schema/3.0/Swagger.json>
- <https://mailchimp.com/developer/marketing/docs/fundamentals/>
- <https://mailchimp.com/developer/marketing/docs/methods-parameters/>
- <https://mailchimp.com/developer/marketing/docs/errors/>
- <https://mailchimp.com/developer/marketing/api/audiences/>
- <https://mailchimp.com/developer/marketing/api/lists/>
- <https://mailchimp.com/developer/marketing/api/campaigns/>
- <https://mailchimp.com/developer/marketing/api/reports/>

The Swagger root identifies `Mailchimp Marketing API`, version `3.0.91`, with
global HTTP Basic authentication. This package adds no third-party dependency.
