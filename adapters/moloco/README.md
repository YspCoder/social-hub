# Moloco Ads Report API adapter

`adapters/moloco` implements the minimum official, read-only asynchronous
Report API surface from Moloco Ads Campaign Management API. It is registered
as `moloco/ads-campaign-management-v1.10` and pins the required
`Moloco-Cloud-Api-Version: v1.10` request header.

The implementation is intentionally limited to these documented endpoints:

- `GET /cm/v1/reports?ad_account_id=...`
- `GET /cm/v1/reports/{report_id}`
- `GET /cm/v1/reports/{report_id}/status`

It does not create or delete reports, manage campaigns, call the Analytics API,
parse report files, or download provider storage URLs. The official list
endpoint defines no pagination parameters, so this adapter does not invent a
cursor or page abstraction.

All four API endpoints accept only the documented HTTP `200`
success status with a non-empty JSON response. Redirects are disabled, and
the three report reads never leave the fixed `https://api.moloco.cloud`
origin.

## Configuration

Use `secret_ref` for the Moloco Ads API key and set the target ad account in
account settings:

```yaml
version: 1
platforms:
  - adapter: moloco/ads-campaign-management-v1.10
    product: ads-campaign-management
    accounts:
      - id: primary
        secret_ref: env://MOLOCO_API_KEY
        settings:
          ad_account_id: your-ad-account-id
```

Prefer an API key whose Moloco user has read-only permissions. The adapter
exchanges that key at `POST /cm/v1/auth/tokens`, caches the resulting bearer
token in memory, and refreshes it five minutes before the documented 16-hour
lifetime. The API key and bearer token must never be logged.
OAuth approval fields, token stores, webhook configuration, and unrelated
account identifiers are rejected rather than silently ignored.

After opening the configured adapter with `socialhub.Open`, access the typed
report workflow through its account client:

```go
base, err := adapter.Client(ctx, "primary")
if err != nil {
	return err
}
client, ok := base.(*moloco.Client)
if !ok {
	return errors.New("unexpected Moloco client type")
}

listed, err := client.Reports().ListReports(ctx)
```

## Rate limits and report locations

Moloco documents a general quota of 300 requests per five minutes for each ad
account. Responses expose `X-Rate-Limit-Quota`, `X-Rate-Limit-Remaining`, and
`X-Rate-Limit-Reset`; this adapter preserves those values in `ResponseMeta`.

A report in `READY` status may contain `location_json` and/or `location_csv`.
These are presigned URLs that expire after approximately one hour and may
contain sensitive query credentials. Do not log or persist the complete URL.
The adapter validates and returns the URL but deliberately does not follow it:
the storage host is provider-controlled and is outside the fixed Moloco API
origin trusted by this package.

The typed location fields remain available to the caller, but ordinary Go
string formatting is redacted and the status response deliberately keeps no
second `Raw` copy of those URLs. Provider errors and quota metadata are
bounded and redact the exact configured API key and current bearer token;
URL-, credential-, signature-, and policy-bearing error fields are also
suppressed.

## Official references

- [Getting started](https://developer.moloco.cloud/docs/getting-started.md)
- [API versioning](https://developer.moloco.cloud/docs/versioning.md)
- [Release schedule](https://developer.moloco.cloud/page/release-notes.md)
- [Rate limits](https://developer.moloco.cloud/docs/rate-limits.md)
- [Report API](https://developer.moloco.cloud/docs/report-api.md)
- [Create token](https://developer.moloco.cloud/reference/dspapi_createtoken-1.md)
- [List reports](https://developer.moloco.cloud/reference/dspapi_listreports.md)
- [Read report](https://developer.moloco.cloud/reference/dspapi_readreport.md)
- [Read report status](https://developer.moloco.cloud/reference/dspapi_readreportstatus.md)

All nine official Markdown pages returned HTTP 200 on 2026-08-26. SHA-256
hashes of the captured response bodies pin the reviewed contract:

| Page | Official `updatedAt` | SHA-256 |
|---|---|---|
| Getting started | 2025-09-18 | `F50117AAE2247B1F72A682A92E398DC9633E31AD9021AD891BE98ACC81645A4B` |
| API versioning | 2026-02-25 | `89D14946AF0E634C0D60DD8707FA78D6E22E5BD16B3424537AA25B3722F2482D` |
| Release schedule | 2026-03-03 | `1D14472FE2198BC9EA0DC66721B46AC6F41CC29351304122B855EF4466D321B6` |
| Rate limits | 2025-09-18 | `70D5C55F44A44A5A01755B606ADD813C579F5F3EB0232F1193DD7E2346121531` |
| Report API | 2025-09-18 | `798C1DC6058BBE319DAF4DD82EA0D721F05F061E995155F6FFDF03293327CCF8` |
| Create token | 2026-06-08 | `C2B163ABE6EC6606794609367A705DFB29854B04C161233D27FE1C0A0C789382` |
| List reports | 2026-06-08 | `A7A38F8535C64D8BDE14436886704BF6FBD42F78ABA87F2DB7A0027E880D6C9B` |
| Read report | 2026-06-08 | `BD8B44E2FCBF902769544DDDFF99C7AB5E390F77E753FE6C4FE4AAEA83E94685` |
| Read report status | 2026-06-08 | `3FC3B1308C7D2925348BEDEC26AC43F8DBDD20815B7A460E873B2F05E391BC49` |

The contract was reviewed against API version `v1.10`, released 2026-03-01
and scheduled for deprecation on 2027-04-01.
