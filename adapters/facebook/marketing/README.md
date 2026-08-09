# Meta Marketing API v25 adapter

`facebook/marketing-api-v25` provides a typed, account-scoped client for the
Meta Marketing API. It covers the minimum usable media-buying chain without
pretending that advertising resources are social posts:

- Ad Account details;
- Campaign create/read/list/update;
- Ad Set create/read/list/update with typed targeting and promoted objects;
- existing-post or link Ad Creative create/read/list;
- Ad create/read/list/update; and
- synchronous account, Campaign, Ad Set, or Ad Insights.

Every Campaign, Ad Set, and Ad is created with `PAUSED`. Activation, archival,
or deletion requires a separate, explicit update call. The adapter never
silently starts spend.

## Access and authentication

Production access depends on the Meta app's Marketing API product, App Review,
the token subject's ad-account role, and the requested operation. Management
uses `ads_management`; read-only reporting may use `ads_read`. A user token can
be obtained with `Adapter.OAuth`, including the long-lived user-token exchange.
Business system-user tokens should normally be provisioned outside the SDK and
referenced through `access_token_ref`.

When `secret_ref` is configured, every Graph request includes an HMAC-SHA256
`appsecret_proof` while continuing to send the access token in the
`Authorization` header. Redirects are rejected so credentials cannot be
forwarded to another origin.

```yaml
version: 1
platforms:
  - adapter: facebook/marketing-api-v25
    product: marketing-api
    accounts:
      - id: brand-ads
        client_id: "1234567890"
        secret_ref: env://META_APP_SECRET
        access_token_ref: env://META_SYSTEM_USER_TOKEN
        approval:
          scopes: [ads_management, business_management]
        settings:
          # Numeric ID only. The adapter adds act_ to account endpoints.
          ad_account_id: "9876543210"
```

Credential values are never accepted inline. If `approval.scopes` is present,
the client fails locally with `approval_required` when the declared grants do
not authorize the requested workflow. An empty scope list means the caller is
managing approval state externally.

## Usage

Importing the package registers the adapter. Advertising methods are available
from the typed client after the common client has been opened:

```go
import (
    "context"

    marketing "social-hub/adapters/facebook/marketing"
    "social-hub/pkg/socialhub"
)

func createPausedCampaign(ctx context.Context, common socialhub.Client) (string, error) {
    client := common.(*marketing.Client)
    campaign, err := client.Management().CreateCampaign(ctx, marketing.CreateCampaignRequest{
        Name:        "Autumn launch",
        Objective:   marketing.ObjectiveTraffic,
        DailyBudget: 5000, // Meta account-currency minor units.
    })
    if err != nil {
        return "", err
    }
    return campaign.ID, nil
}
```

Ad Set targeting uses typed geo, audience, placement, device, and flexible
targeting fields. A basic existing-post creative uses `ObjectStoryID`; a link
creative uses `ObjectStorySpec`. Ads reference the returned creative ID.

Insights values remain strings because Meta returns currency and metric values
as decimal strings. This preserves exact API values and avoids imposing a
currency exponent or floating-point policy on callers.

## Rate limits and response contracts

Meta Marketing API limits are dynamic and depend on business use case, app,
ad account, access tier, and recent error rate. A fixed requests-per-second
constant would be misleading. Applications should feed `X-Ad-Account-Usage`
and `X-Business-Use-Case-Usage` into their limiter when shared response-metadata
hooks are added. This adapter already maps Graph codes `4`, `17`, `32`, and
`613`, HTTP `429`, and transient Graph errors into retryable social-hub errors.

Responses are bounded by the shared transport. Resource reads and creations
must return valid numeric IDs, update responses must explicitly confirm the
mutation, pagination follows opaque `after` cursors, and both RFC3339 and Meta's
numeric-offset timestamps are accepted.

## Deliberate exclusions

- image/video asset upload and dynamic creative generation;
- asynchronous Insights report jobs;
- Custom Audiences, Pixels, Conversions API, and Offline Conversions;
- Lead Ads webhooks; and
- automatic retry of spend-affecting mutations.

These products have distinct privacy, data-retention, idempotency, or webhook
contracts and should be added as separate typed workflows.

## Reference audit

The implementation was checked against Meta's official Marketing API resources,
official Business SDK behavior, and the MIT-licensed
`justwatch/facebook-marketing-api-golang-sdk`. The latter is active and useful
for entity and pagination semantics, but its generated Marketing package was at
v24 during this implementation and brings its own transport, logger, retry, and
error abstractions. It is therefore referenced rather than imported; this
adapter remains aligned with social-hub's bounded transport and error model.

No credentialed request has been run against a real ad account. Deterministic
local contract tests are the current verification baseline.
