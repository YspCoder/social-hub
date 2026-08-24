# Amazon Creators API Catalog adapter

Registration name: `amazon/creators-api-catalog-v1`

This adapter implements the four current Amazon Creators API Catalog v1
operations for Associates product discovery and attributed URLs. Amazon has
retired Product Advertising API 5; old PA-API requests now fail with HTTP 403
and are intentionally not implemented here.

## Implemented workflows

| social-hub method | Amazon method | Notes |
|---|---|---|
| `Catalog().SearchItems` | `POST /catalog/v1/searchItems` | actor/artist/author/brand/keywords/title search, typed filters/resources, 1-10 items per page and pages 1-10 |
| `Catalog().GetItems` | `POST /catalog/v1/getItems` | 1-10 unique ASINs, localized item detail, images, ItemInfo, and OffersV2 |
| `Catalog().GetVariations` | `POST /catalog/v1/getVariations` | one ASIN, 1-10 variations per page, variation dimensions and price summary |
| `Catalog().GetBrowseNodes` | `POST /catalog/v1/getBrowseNodes` | 1-10 positive Browse Node IDs with optional ancestor/children resources |

The initial surface excludes Creators API feeds/reports, Associates commission
reporting, arbitrary URL conversion, and all legacy PA-API 5 SigV4 behavior.
Feeds and reports have distinct lifecycle and download contracts and should be
added as separate workflows.

## Configuration

Managed OAuth supports only current Credential Versions 3.1, 3.2, and 3.3.
The adapter sends the official JSON Client Credentials request with scope
`creatorsapi::default`, caches the roughly one-hour token in the configured
`socialhub.TokenStore`, and obtains a replacement before expiry. There is no
refresh token.

```yaml
version: 1
platforms:
  - adapter: amazon/creators-api-catalog-v1
    product: creators-api-catalog
    accounts:
      - id: us-store
        client_id: creators-credential-id
        secret_ref: env://AMAZON_CREATORS_CREDENTIAL_SECRET
        approval:
          account_type: approved-amazon-associates-store
          scopes:
            - creatorsapi::default
        settings:
          marketplace: www.amazon.com
          partner_tag: example-20
          credential_version: "3.1"
```

Credential Version selects the token endpoint independently of the request
marketplace. Current v3 credentials work globally, but each target marketplace
still requires its own valid Partner Tag:

| Version | Region | Token endpoint |
|---|---|---|
| `3.1` | North America | `https://api.amazon.com/auth/o2/token` |
| `3.2` | Europe | `https://api.amazon.co.uk/auth/o2/token` |
| `3.3` | Far East | `https://api.amazon.co.jp/auth/o2/token` |

An externally managed token can be supplied instead:

```yaml
      - id: japan-store
        access_token_ref: env://AMAZON_CREATORS_ACCESS_TOKEN
        settings:
          marketplace: www.amazon.co.jp
          partner_tag: example-22
```

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/amazoncreators"
```

Catalog and token endpoints are derived from Amazon's production contract and
cannot be overridden. Amazon does not document a public Creators API sandbox.
Redirects are rejected and caller cookie jars are ignored so credentials cannot
move to another origin or cookie context.

## Request and data guarantees

- Every Catalog call sends both the `x-marketplace` header and the required
  `marketplace` body field, plus the account's `partnerTag` body field.
- Search requires at least one of keywords, actor, artist, author, brand, or
  title. `itemCount` defaults to 10 and `itemPage` defaults to 1; each is
  bounded to 1-10. `GetItems` and `GetBrowseNodes` reject duplicates.
- Resource selectors are typed and checked per operation. Current OffersV2 is
  modeled; removed PA-API fields such as `PartnerType`, `Merchant`,
  `OfferCount`, and Offers V1 are not accepted.
- Money, savings, ratings, scores, and other JSON numbers that would otherwise
  lose decimal fidelity are exposed as raw-token-preserving `ExactValue`
  scalars.
- Items, result containers, envelopes, ItemInfo, Offer listings, and Browse
  Nodes retain their complete successful provider object in `Raw`.
- HTTP 200 partial failures remain in the typed `Errors` collection alongside
  successful data. A partial item failure does not turn the whole call into a
  Go error.
- `detailPageURL` and `searchURL` are returned exactly as Amazon supplies them.
  Callers must not remove or rewrite attribution parameters.
- Non-2xx responses map to `socialhub.Error`. `AssociateNotEligible` becomes
  `approval_required`; validation, authentication, permission, not-found,
  throttling, and 5xx failures retain their platform type/reason and retry
  semantics.

## Access, quota, and compliance

Creators API access requires a finally accepted Amazon Associates account in
the target region, an active store and Partner Tag for the marketplace, and at
least 10 qualifying sales in the preceding 30 days under the current onboarding
rules. Only the Associates primary account owner can apply. Each store can have
at most two applications and each application at most two credential sets.

New credentials receive at most 1 TPS and 8,640 transactions per day for their
first 30 days. Thereafter, Amazon derives the rolling quota from attributed
shipped revenue: one daily transaction per USD 0.05 and one TPS per USD 4,320,
up to 10 TPS. HTTP 429 is retryable. Rate-limit by credential/primary account,
not by marketplace alone. Thirty consecutive days without qualifying referral
sales can suspend API access.

Amazon's Associates IP License is stricter than ordinary API caching:

- ASINs may be retained while the license remains in force.
- Product Advertising Content must normally be refreshed within 24 hours.
- Image bytes must not be cached; image URLs may be cached for at most 24 hours.
- Price and availability displays require Amazon's timestamp/disclaimer rules.
- Product content must link back to the relevant Amazon page and must not be
  resold, redistributed, or used to train an LLM/ML model.
- Multi-tenant products should use customer-owned credentials and Partner Tags
  unless their Amazon agreement explicitly permits centralized redistribution.

Official contracts:

- <https://affiliate-program.amazon.com/creatorsapi/docs/en-us/introduction>
- <https://affiliate-program.amazon.com/creatorsapi/docs/en-us/onboarding/register-for-creators-api>
- <https://affiliate-program.amazon.com/creatorsapi/docs/en-us/paapiv5-deprecation>
- <https://affiliate-program.amazon.com/creatorsapi/docs/en-us/get-started/using-curl>
- <https://affiliate-program.amazon.com/creatorsapi/docs/en-us/concepts/common-request-headers-and-parameters>
- <https://affiliate-program.amazon.com/creatorsapi/docs/en-us/concepts/api-rates>
- <https://affiliate-program.amazon.com/creatorsapi/docs/en-us/api-reference/operations/search-items>
- <https://affiliate-program.amazon.com/creatorsapi/docs/en-us/api-reference/operations/get-items>
- <https://affiliate-program.amazon.com/creatorsapi/docs/en-us/api-reference/operations/get-variations>
- <https://affiliate-program.amazon.com/creatorsapi/docs/en-us/api-reference/operations/get-browse-nodes>
- <https://affiliate-program.amazon.com/help/operating/policies>

These contracts, the 21-marketplace reference (including Ireland), and the
production Catalog/LWA endpoints were rechecked on 2026-08-25.

Amazon's official SDK 1.2.0 (Apache-2.0) was reviewed for OAuth, model, and
operation contracts. `goark/pa-api` v0.17.1 at commit
`752cd0418e60da7403449fb7de84045e838eb6f4` (Apache-2.0) was reviewed only as a
Go modeling reference: its v3 OAuth request and marketplace payload differ from
the current official contract, so it is not added as a runtime dependency.
