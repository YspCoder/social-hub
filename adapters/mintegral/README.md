# Mintegral AppGrowth Open API adapter

`adapters/mintegral` exposes the complete public service surface of
`github.com/jageros/mintegral-go` v0.1.1 through social-hub account lifecycle,
transport, secret-resolution, call-timeout, capability, and error contracts.

Official references:

- API integration: <https://helpcenter.mintegral.com/en/docs/api-integration>
- Token API: <https://helpcenter.mintegral.com/en/docs/token-api>
- Advanced Performance Report v2: <https://helpcenter.mintegral.com/en/docs/advanced-ad-delivery-report/>

The contract was reviewed on 2026-08-26. The adapter is registered as
`mintegral/appgrowth-open-api-v1`; Advanced Performance Report uses only
`GET /api/v2/reports/data` and its v2 state machine.

All three official pages returned HTTP 200 during the review. SHA-256 hashes
of the captured response bodies provide immutable evidence for the reviewed
state:

- API integration (modified 2025-07-02):
  `F12BEC44536C22B024288A7E05DDCBC941CD94E6CFC8CAFEF5497FBCD216C814`
- Token API (modified 2025-12-05):
  `39BD001302B98433CCC0D2704877E82D80933E3BE331E1507D88FAB507C024A6`
- Advanced Performance Reporting (modified 2026-06-16):
  `5257A6F0F01D8DD57BEBEEDF1243D3C658C50EB485049E9F594AA827C297E3C4`

The pinned SDK tag `v0.1.1` resolves to commit
`24767c0f99ca08268ea2393e6924fd9f0ceb89d4`. Its module checksum is
`h1:fn5Be4+fKtIbWC2FAxrogbadWCGQuLX4hmqs4O/lEzE=` and its `go.mod` checksum
is `h1:NjexFAFibyqckvSoVY8fse/MB4tolVBqdz+XQt924N8=`.

## Implemented surface

| Workflow | Methods | Purpose |
|---|---|---|
| `Accounts` | `Balance` | Advertiser balance and currency |
| `Campaigns` | `List`, `Create`, `Update` | Campaign lifecycle |
| `Apps` | `Names` | Target application-name lookup |
| `Offers` | `List`, `Create`, `Update`, `UpdateBids`, `UpdateBudget`, `SetStatus`, `UpdateTrafficDelivery`, `UpdateTracking`, `SetAudiences`, `UpdateTargetGoal`, `ApplyCreatives` | Offer delivery, targeting, optimization, and deprecated legacy creative association |
| `Events` | `BidGoalSupports` | Bid-goal event availability |
| `CreativeSets` | `List`, `Create`, `Update`, `Delete` | Creative combinations and outputs |
| `CreativeAds` | `List` | Ads generated from Creative Sets |
| `Assets` | `List`, `UploadMedia`, `UploadPlayable` | Asset library plus repeatable image, video, and playable uploads |
| `Reports` | `Status`, `Open`, `Consume` | Advanced Performance Report v2 generation and typed TSV streaming |
| `Audiences` | `List`, `PresignUpload`, `Upload`, `UploadFile`, `Create`, `Update`, `Delete` | Regional audience-file upload and audience lifecycle |

`ApplyCreatives` is retained only for complete upstream coverage. New work
should use `CreativeSets` because Mintegral has deprecated the legacy endpoint.

## Authentication and configuration

Mintegral Open API uses an Access Key and API Key, not OAuth. The upstream SDK
calculates the required request token as `md5(APIKey + md5(timestamp))` and
sends `access-key`, `timestamp`, and `token` headers. Configure the Access Key
as `client_id` and resolve the API Key from `secret_ref`:

```yaml
version: 1
platforms:
  - adapter: mintegral/appgrowth-open-api-v1
    product: appgrowth-open-api
    accounts:
      - id: mintegral-appgrowth-production
        client_id: "your-mintegral-access-key"
        secret_ref: env://MINTEGRAL_API_KEY
```

The adapter rejects OAuth token stores, webhook settings, and additional
account settings so credentials cannot be silently interpreted under the wrong
authentication model. `api_base_url` and `storage_base_url` are optional
adapter settings intended for controlled gateways and local contract
verification; production defaults come from the pinned SDK.

## Typed usage

Import this package for registration, open the configured account, and assert
the product-specific client before selecting a workflow:

```go
package main

import (
	"context"
	"time"

	mtg "github.com/jageros/mintegral-go"

	"social-hub/adapters/mintegral"
	"social-hub/pkg/socialhub"
)

func campaigns(ctx context.Context, config socialhub.AdapterConfig) (mtg.CampaignPage, error) {
	adapter, err := socialhub.Open(ctx, "mintegral/appgrowth-open-api-v1", config)
	if err != nil {
		return mtg.CampaignPage{}, err
	}
	defer adapter.Close()

	common, err := adapter.Client(ctx, "mintegral-appgrowth-production")
	if err != nil {
		return mtg.CampaignPage{}, err
	}
	client := common.(*mintegral.Client)

	return client.Campaigns().List(ctx, mtg.CampaignListRequest{}, socialhub.WithCallTimeout(15*time.Second))
}
```

Request and result models deliberately remain the strong types from the pinned
SDK. This preserves exact decimal values, ID widths, validation rules, page
metadata, upload integrity checks, and future-compatible report columns.

## Advanced Performance Report v2

The official report is asynchronous and has two phases on the same endpoint:

1. `type=1` creates or polls a report. Business codes `200`, `201`, and `202`
   are valid status responses.
2. `type=2` downloads TSV. Availability codes `203` through `205` make `Open`
   return to bounded polling instead of treating a not-yet-ready report as a
   permanent failure.

For status requests, `200` means complete, `201` means accepted and waiting,
and `202` means generation is in progress. For downloads, `203` means no
matching generation request exists, `204` means the data is not ready, and
`205` means the one-month retained result expired and is being regenerated.

`Status` exposes one status request. `Open` polls, opens the TSV response, and
returns a typed `ReportStream`; callers must close it. A
`socialhub.WithCallTimeout` passed to `Open` remains active until `Close`, while
`io.EOF` from `Next` remains the ordinary completion signal. `Consume` performs
bounded synchronous batching and reports parsed versus acknowledged rows.

Queries are limited to a closed range of seven days within the most recent six
months. The pinned SDK validates unsupported dimension and hourly combinations
before I/O. It preserves spend and rate cells as exact decimal text and keeps
unknown TSV columns in `ReportExtras` rather than discarding them.

If a later read or handler fails after at least one batch was acknowledged,
the adapter maps `mintegral.ErrPartialDelivery`. Do not transparently restart
the whole report without reconciling the already committed batches.

## Asset and audience uploads

Asset and audience methods accept `mintegral.UploadSource`, which must reopen
the same bytes on every call. Use `mintegral.NewFileUploadSource` for a stable
local file or construct a source with a known size and MD5. Media and playable
uploads are write operations and are not blindly retried after an uncertain
transport result.

Audience upload is intentionally separate from audience creation:

1. `PresignUpload` binds file name, MD5, byte size, and `area_type` to a
   short-lived upload plan.
2. `Upload` validates the source and sends it to the plan's storage provider.
   `UploadFile` only combines the first two steps.
3. Use the returned `DataPath` in `Create` or `Update`.

`area_type=1` uses an S3 raw `PUT` for non-mainland data. `area_type=2` uses an
OSS multipart `POST` for mainland China. Storage requests never receive
Mintegral management authentication headers. Presigned URLs, OSS policy,
signature, Access ID, and file contents are bearer secrets and must not be
logged. The SDK validates the complete source into a restricted temporary
snapshot, so allow temporary disk space near the file size; a still-valid plan
can replay that exact snapshot once for selected transient storage failures.

Audience files are limited to 5 GiB. New audiences can take up to roughly 12
hours to become targetable, and Mintegral prevents another update within 12
hours of creation or the previous modification.

## Error and retry boundaries

- The SDK performs limited retries only for replay-safe reads. It does not
  blindly repeat management writes or asset uploads.
- `mintegral.ErrOutcomeUnknown` is mapped as a user-action error: reconcile
  remote state before deciding whether to retry.
- `ErrRateLimited`, permission, authentication, invalid request, report
  timeout, upload expiry, and partial delivery retain structured social-hub
  classifications. `Retry-After` and numeric platform codes are preserved when
  available.
- Free-form provider messages, credentials, signatures, upload URLs, request
  bodies, and response bodies are not exposed through social-hub errors.
- AppGrowth advertising workflows are typed extensions. They are not reported
  as organic publishing, feed, media, reaction, messaging, or webhook support.
