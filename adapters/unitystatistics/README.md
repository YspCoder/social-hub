# Unity Advertising Statistics API v2 adapter

`adapters/unitystatistics` implements Unity's organization-bound Advertising
Statistics API v2. Unity labels the current OpenAPI 3.0 contract `v2.0 latest`;
production routes are below `/advertise/stats/v2`.

Official references:

- Statistics API v2: <https://services.docs.unity.com/statistics/v2/>
- OpenAPI contract: <https://services.docs.unity.com/specs/v2/73746174697374696373.yml>
- Service Account authentication: <https://services.docs.unity.com/docs/service-account-auth/>
- Errors: <https://services.docs.unity.com/docs/errors/>
- Response headers: <https://services.docs.unity.com/docs/headers/>

## Implemented workflows

Both non-deprecated operations in the current contract are implemented:

- Acquisition reports through `DownloadAcquisitionsReport`
- SKAdNetwork reports through `DownloadSKANReport`

Acquisition reports expose all 114 current metrics: nine pre-install metrics
and 15 post-install metric families for day 0, 1, 3, 7, 14, 21, and 28. This
includes the Payer, Payer Rate, and Cost Per Payer families added in the
2026-06-23 contract change. Acquisition requests support all 12 breakdowns and
all documented filters. SKAN requests expose all seven metrics, four
breakdowns, and their three documented filters.

The report schema changes with the selected metrics and breakdowns, so the
adapter streams the response to a caller-provided `io.Writer` instead of
pretending that every row has one fixed Go shape. Both CSV and JSON are
supported, as are identity, gzip, and deflate transfer encodings. The default
limit is 256 MiB of decompressed output and can be changed per download.

For CSV, set `EOFMarker` to make Unity append `#__EOF__,rows=N,...`. The adapter
then parses the CSV stream, including quoted newlines, and rejects missing or
duplicate markers, inconsistent column counts, incorrect row counts, and data
after the marker. `EOFMarker` is deliberately rejected for JSON. HTTP 204 is
returned as `ReportResult.NoData` without writing output.

## Access requirements and authentication

Create a Unity Organization Service Account and assign the least-privileged
role required by the account:

- `Advertise Stats API Viewer` for organization reporting
- `Advertise Stats API MMP Viewer` for an MMP integration

Use the numeric **Organization Core ID** shown in the Acquire Dashboard. Other
Unity organization identifiers are not interchangeable with this value.

The adapter accepts exactly one authentication mode:

- HTTP Basic: `client_id` is the Service Account Key ID and `secret_ref`
  resolves the Secret Key. The SDK sends `KEY_ID:SECRET_KEY` using HTTP Basic.
- Long-lived Bearer: `access_token_ref` resolves a Unity Service Account bearer
  token. Unity documents these tokens as not requiring refresh.

Credential values are resolved through `socialhub.SecretResolver`; config files
store references only.

```yaml
version: 1
platforms:
  - adapter: unity/advertising-statistics-api-v2
    product: advertising-statistics-api
    accounts:
      - id: unity-statistics-primary
        client_id: ${UNITY_SERVICE_ACCOUNT_KEY_ID}
        secret_ref: env://UNITY_SERVICE_ACCOUNT_SECRET_KEY
        settings:
          organization_id: "5772916123937"
```

For long-lived Bearer authentication, replace `client_id` and `secret_ref`
with:

```yaml
        access_token_ref: env://UNITY_SERVICE_ACCOUNT_BEARER_TOKEN
```

## Typed usage

Import the package for registration, open the configured adapter, and stream a
report to any `io.Writer`:

```go
package main

import (
	"context"
	"io"
	"time"

	"social-hub/adapters/unitystatistics"
	"social-hub/pkg/socialhub"
)

func downloadDailyAcquisitionReport(
	ctx context.Context,
	config socialhub.AdapterConfig,
	output io.Writer,
	start time.Time,
) error {
	adapter, err := socialhub.Open(
		ctx,
		"unity/advertising-statistics-api-v2",
		config,
	)
	if err != nil {
		return err
	}
	defer adapter.Close()

	common, err := adapter.Client(ctx, "unity-statistics-primary")
	if err != nil {
		return err
	}
	client := common.(*unitystatistics.Client)

	_, err = client.Reports().DownloadAcquisitionsReport(
		ctx,
		unitystatistics.AcquisitionsReportRequest{
			Start:      start,
			End:        start.Add(24 * time.Hour),
			Scale:      unitystatistics.ScaleDay,
			Metrics:    []unitystatistics.AcquisitionMetric{
				unitystatistics.MetricClicks,
				unitystatistics.MetricInstalls,
				unitystatistics.MetricSpend,
			},
			Breakdowns: []unitystatistics.AcquisitionBreakdown{
				unitystatistics.BreakdownCampaign,
				unitystatistics.BreakdownCountry,
			},
			Format:    unitystatistics.FormatCSV,
			EOFMarker: true,
		},
		output,
		unitystatistics.DownloadOptions{
			Compression: unitystatistics.CompressionGzip,
		},
	)
	return err
}
```

Unity retains at most two years of report data, beginning no earlier than
2024-05-21. A report can execute server-side for up to ten minutes. Query one
day at a time when using high-cardinality breakdowns such as `sourceAppId` or
`country`, and split larger backfills into caller-managed jobs.

## Rate limits and errors

`DefaultQuotaPolicy` records the current aggregate limits:

| Window | Limit |
|---|---:|
| One second | 1 request |
| Thirty minutes | 30 requests |

Unity counts both limits independently by `organization_id` and by
`ip_address`; the first exhausted dimension blocks the request. Coordinate
calls through social-hub's shared limiter rather than creating a limiter per
client.

`ReportResult` and `APIError` preserve bounded `RateLimit-Policy`, `RateLimit`,
and `Unity-RateLimit` values. `APIError` also retains sanitized Unity problem
details, request IDs, and `Retry-After`. Retry only errors whose `Retryable()`
method returns true. The adapter rejects redirects so credentials cannot be
forwarded to another origin.
