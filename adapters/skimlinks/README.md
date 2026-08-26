# Skimlinks publisher APIs adapter

Registration name:

```text
skimlinks/publisher-apis-v4
```

This package implements a bounded publisher-facing surface across three
official Skimlinks products. Merchant API is versioned as v4; Reporting API and
Link Wrapper do not publish an overall version. The registration version
therefore follows the only versioned resource family and does not imply that
the other products are v4.

## Implemented workflows

| Method | Official operation | Contract |
| --- | --- | --- |
| `ListMerchants` | `GET /v4/publisher/{publisher_id}/merchants` | site-specific merchant/rate discovery; offset filters; default limit 200 |
| `ListDomains` | `GET /v4/publisher/{publisher_id}/domains` | monetizable merchant domains |
| `WrapLink` | `https://go.skimresources.com/?id=...&url=...` | local Link Wrapper URL construction; optional `xcust` and `sref` |
| `ListCommissions` | `GET /publisher/{publisher_id}/commission-report` | individual commission records; offset pagination; limit 1-5000 |
| `GetPerformanceReport` | `GET /publisher/{publisher_id}/reports` | aggregated reports by date, merchant, page, link, domain, country, device, or payout type; limit 1-600 |

The four remote workflows expose typed fields plus successful provider JSON in
bounded `Raw`. Primary merchant, domain, commission, and report-row entities
also retain their own provider object. `ExactValue` preserves identifiers,
amounts, rates, counts, and nullable scalars without `float64` coercion.

The adapter enforces each documented success status: HTTP `201` for Merchant
list, HTTP `200` for Domains, Individual Commissions, Aggregated Reports, and
token issuance. Unexpected 2xx responses are provider contract errors rather
than silently accepted alternatives. `ResponseMeta` retains the actual status
and a bounded provider request ID.

Successful response contracts also require:

- Merchant collections whose `num_returned` matches the decoded rows, whose
  size respects the requested/default limit, and whose advertiser and merchant
  program identities are valid, unique, and consistent with requested filters.
  Present legacy `id` values must match the documented `advertiser_id`, and
  returned publisher-domain stats must identify the selected site.
- A present Domains collection, valid continuation metadata when `has_more` is
  true, and unique positive domain, merchant, and advertiser IDs. The official
  Domains example abbreviates two objects while reporting `num_returned` as
  26,324, so the adapter requires that count to be no smaller than the decoded
  collection instead of requiring equality.
- Present commission and report collections, structurally valid pagination,
  documented row limits, unique commission IDs, the configured publisher ID,
  and returned commission identity fields that agree with supplied filters.
  Aggregated-report `count` must not be smaller than the decoded report set.

On HTTP, decode, or response-contract failures, returned envelopes retain the
available response metadata and a bounded diagnostic `Raw`. Exact configured
client credentials and the current temporary token are redacted longest-first,
including entity-level `Raw` values decoded before a later failure. Structured
HTTP failures expose the same bounded diagnostic body on `APIError.Raw`.

## Authentication and configuration

Merchant and Reporting use the same temporary timestamp-based access token.
Managed mode sends the official JSON client-credentials request to
`POST https://authentication.skimapis.com/access_token`:

```json
{
  "client_id": "...",
  "client_secret": "...",
  "grant_type": "client_credentials"
}
```

The response supplies `access_token`, `timestamp`, and `expiry_timestamp`.
Skimlinks requires the token in the `access_token` query parameter, not in an
Authorization header. A configured `socialhub.TokenStore` caches it only when
it has a nonzero documented expiry timestamp. If a persistent write fails, the
process cache retains the newly issued token and retries that write on the next
request. Externally managed temporary tokens can instead be supplied through
`access_token_ref`; `token_store` is rejected in that mode. Skimlinks client
credentials do not define OAuth scopes, so configured approval scopes are also
rejected.

```yaml
version: 1
platforms:
  - adapter: skimlinks/publisher-apis-v4
    product: publisher-apis
    accounts:
      - id: primary-publisher
        client_id: your-api-client-id
        secret_ref: env://SKIMLINKS_CLIENT_SECRET
        token_store: encrypted-tokens
        approval:
          account_type: approved-skimlinks-publisher
        settings:
          publisher_id: 12345
          publisher_domain_id: 67890
          site_id: 123X456
```

`publisher_domain_id` selects the registered site whose merchant availability
and rates are returned. Although the Merchant blueprint says omission falls
back to the first registered domain, this adapter requires an explicit value
to avoid silently querying the wrong site. `site_id` is the domain-specific
Link Wrapper `id` shown in Publisher Hub Setup; it is not assumed to be a
numeric publisher domain ID.

All configured HTTP clients are cloned with redirects disabled. This prevents
query credentials from being forwarded after a platform redirect. `WrapLink`
only validates and constructs the official redirect URL; it never requests the
destination or follows the redirect. Endpoint overrides are intended only for
controlled contract-verification gateways.

## Pagination, limits, and access

- Merchant API says some endpoints are limited to 40 requests per minute and
  1,000 per hour per API key, returning HTTP 429. It does not assign that quota
  explicitly to every v4 endpoint.
- Individual commissions are limited to 40 requests/minute and 300/hour per
  API key. Same-IP limits are 80/minute, 500/hour, and 1,000/day.
- Aggregated reports are limited to 40 requests/minute and 500/hour per API
  key. Same-IP limits are 80/minute, 500/hour, and 2,000/day.
- The public Link Wrapper and authentication descriptions do not publish a
  numeric quota. HTTP 429 remains mapped to retryable `rate_limited` and
  `Retry-After` is preserved when present and capped at 24 hours.
- Merchant lists use `has_more`, `next_val`, and offset inputs. Commission
  responses contain `pagination.has_next`, `total_count`, `offset`, and
  `limit`. Aggregated reports use offset inputs and return `count`, `reports`,
  and `totals`.

The commission endpoint's formal parameter table sets `limit` to 1-5,000,
while a Python example later in the same official blueprint comments that the
maximum is 600. The adapter follows the formal endpoint parameter contract and
accepts up to 5,000; callers that need conservative cross-version behavior can
cap requests at 600.

API credentials require a Skimlinks publisher account and are retrieved from
Publisher Hub under Toolbox > APIs > API Authentication credentials. Site IDs
come from the Hub Setup section. The reviewed public documentation states no
general country restriction; merchant coverage and rates remain specific to
the registered publisher domain and optional country filters. Managed
publishers who exceed Reporting API limits are directed to contact their
account manager about Data Pipe access.

## Excluded surface

The initial package deliberately excludes Merchant offers and vertical
taxonomies, Product Key, Trending Products, product-bought/payment/deactivated
merchant reports, multi-aggregated NDJSON streams, Data Pipe, Skimlinks
JavaScript, signup/account mutation, and webhooks. Merchant v3 resources are
excluded because the current official blueprint groups and labels them as
`OLD version`.

## Official sources verified 2026-08-26

- <https://developers.skimlinks.com/>
- <https://developers.skimlinks.com/merchant.html>
- <https://skimlinksmerchantapi.docs.apiary.io/>
- <https://skimlinksmerchantapi.docs.apiary.io/api-description-document>
- <https://developers.skimlinks.com/link.html>
- <https://skimlinkslinkapi.docs.apiary.io/>
- <https://jsapi.apiary.io/apis/skimlinkslinkapi>
- <https://developers.skimlinks.com/reporting.html>
- <https://skimlinksreporting.docs.apiary.io/>
- <https://skimlinksreporting.docs.apiary.io/api-description-document>

The official outer documentation files were last modified 2026-07-15. Apiary
reports Merchant API updated 2025-06-19, Reporting API updated 2026-01-19, and
Link Wrapper updated 2020-11-03. Link Wrapper's raw blueprint download is no
longer exposed, but the official page still embeds `skimlinkslinkapi`; Apiary's
public parsed document supplies the production host, authentication prose,
parameter table, and example URL used here.

The review used immutable local captures with these SHA-256 hashes:

| Capture | SHA-256 |
| --- | --- |
| Merchant API Blueprint | `4A0BC95CA7CE6A03158F81F767F098C08BEB8B057784039E3E686762424A02A3` |
| Reporting API Blueprint | `C60CF24D56BFF82C9330B9EF653FBB4DEB4157D7705782346B2F046BE87C385B` |
| Link Wrapper parsed Apiary document | `3B4BE987560155DFC2F65D094EDCF17D02E2ADB0E999AE50257DFC69317E8DB7` |
| Merchant outer documentation | `6DC04ACD32675F957CECBC55F1D1A980C2B05C34F94241497E86CC7D1874C82C` |
| Reporting outer documentation | `9D97800C2A083C72BBDC17367967DFA113CC9154C2FC60B0234061B39DDE3AEE` |
| Link Wrapper outer documentation | `EEC261841A29E6A42E2CFC51D060456B815705C8667AACBD443EADAE8359C3F5` |

No mature public Go SDK covering this exact current suite was identified or
used. The adapter has no new runtime dependency.
