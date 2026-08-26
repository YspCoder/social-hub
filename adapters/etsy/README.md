# Etsy Open API v3 adapter

Registration name: `etsy/open-api-v3`

This adapter implements a bounded Etsy seller surface for shop and listing
reads, draft listing creation, listing images, and listing inventory. Etsy
commerce resources intentionally do not implement social-hub's organic
`Publisher`, `Fetcher`, `MediaUploader`, `Reactor`, or `Messenger` contracts.

## Implemented workflows

| social-hub method | Etsy operation | Authorization |
|---|---|---|
| `Listings().GetShop` | `GET /v3/application/shops/{shop_id}` | `x-api-key` |
| `Listings().GetListing` | `GET /v3/application/listings/{listing_id}` | `x-api-key`; OAuth is needed for owner-only fields |
| `Listings().ListShopListings` | `GET /v3/application/shops/{shop_id}/listings` | `listings_r` |
| `Listings().CreateDraftListing` | `POST /v3/application/shops/{shop_id}/listings` | `listings_w` |
| `Listings().ListListingImages` | `GET /v3/application/listings/{listing_id}/images` | `x-api-key` |
| `Listings().UploadListingImage` | `POST /v3/application/shops/{shop_id}/listings/{listing_id}/images` | `listings_w` |
| `Listings().GetListingInventory` | `GET /v3/application/listings/{listing_id}/inventory` | `listings_r` |
| `Listings().UpdateListingInventory` | `PUT /v3/application/listings/{listing_id}/inventory` | `listings_w` |

The initial surface excludes listing activation/update/delete, digital files,
videos, taxonomy discovery, personalization, translations, shipping and return
policy management, orders, payments, reviews, favorites, and webhooks. Those
contracts should be added only from their current OpenAPI operations.

## Configuration

Every Etsy v3 request requires the application keystring and shared secret as
`x-api-key: keystring:shared_secret`. `client_id` stores the keystring and
`secret_ref` resolves the shared secret. The following configuration uses a
refresh token; the adapter refreshes the one-hour access token before expiry
and persists rotated credentials through the configured `socialhub.TokenStore`.

```yaml
version: 1
platforms:
  - adapter: etsy/open-api-v3
    product: open-api
    accounts:
      - id: primary-shop
        client_id: etsy-keystring
        secret_ref: env://ETSY_SHARED_SECRET
        approval:
          account_type: seller-app
          scopes:
            - listings_r
            - listings_w
        settings:
          shop_id: 12345678
          refresh_token_ref: env://ETSY_REFRESH_TOKEN
```

An externally managed OAuth token uses `access_token_ref` instead of
`settings.refresh_token_ref`. Omitting both token references creates an
API-key-only client: public shop, listing, and image reads work, while scoped
methods return `socialhub.CodeApprovalRequired` before sending a request.

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/etsy"
```

## OAuth 2.0 and PKCE

`Adapter.OAuth` exposes the documented Authorization Code and Refresh grants.
Use `GeneratePKCE`, retain the verifier server-side, and pass the challenge to
`OAuthClient.AuthorizationURL`. The helper always sends
`code_challenge_method=S256` and requires a non-empty, single-use `state` even
though Etsy allows OAuth 2.1 clients relying on PKCE to omit it. Redirect URIs
must be registered, HTTPS, and exact, including case and trailing slash.

Authorization endpoint: `https://www.etsy.com/oauth/connect`

Token endpoint: `https://openapi.etsy.com/v3/public/oauth/token`

Etsy access tokens include the numeric user ID prefix and are valid for one
hour. Refresh tokens are valid for 90 days and retain the original scopes;
changing scopes requires a new authorization-code flow. Production token
stores must encrypt credentials at rest.

The OAuth helper accepts the complete scope vocabulary in the current OpenAPI
contract: `address_r`, `address_w`, `billing_r`, `cart_r`, `cart_w`, `email_r`,
`favorites_r`, `favorites_w`, `feedback_r`, `listings_d`, `listings_r`,
`listings_w`, `profile_r`, `profile_w`, `recommend_r`, `recommend_w`, `shops_r`,
`shops_w`, `transactions_r`, and `transactions_w`.

## Request and data guarantees

- `CreateDraftListing` calls only Etsy's `createDraftListing` operation and
  requires the documented HTTP 201 response, the configured shop ID, a
  positive listing ID, and `draft` state. This adapter has no activation
  method, so listing publication cannot happen implicitly.
- Prices use `ExactDecimal`, which is emitted as a JSON/form number without a
  `float64` round trip. Response money retains Etsy's integer `amount`,
  `divisor`, and `currency_code` fields.
- `UpdateListingInventory` is a complete inventory replacement, not a patch.
  Callers must send every product and offering that should remain. Empty
  `*_on_property` slices are sent as arrays so callers can clear variation
  dimensions; an empty product set is rejected locally.
- Image assignment accepts exactly one binary image or deleted
  `listing_image_id`. Binary input is read through a 25 MiB SDK safety limit;
  multipart requests and all success/error responses are bounded.
- Redirects are rejected so `x-api-key`, bearer tokens, authorization codes,
  and refresh tokens cannot move to another origin. Successful responses must
  use the exact documented 200/201 status and contain JSON. Shop, listing,
  image, inventory, and collection identities are checked against the request;
  decoded values and the complete provider object in `Raw` remain available
  when a post-decode contract check fails.
- Provider failures map to `socialhub.Error`; 429 and transient 5xx responses
  are retryable, and `Retry-After` is capped at 24 hours. The adapter does not
  retry write operations automatically. `CreateDraftListing`,
  `UploadListingImage`, and `UpdateListingInventory` have no documented
  idempotency key: transport failures, 408/5xx responses, or invalid success
  responses return `ErrOutcomeUnknown` with `socialhub.CodeConflict`. Reconcile
  Etsy shop state before retrying; definitive provider 4xx responses retain
  their normal classification.

## Access, quotas, and compliance

Etsy currently has three access paths:

- **Seller App**: for one active shop in good standing; approval is generally
  automated and OAuth access is restricted to that registered shop.
- **Personal App**: reviewed access for limited-scale uses beyond the
  developer's own shop.
- **Commercial Access**: manual upgrade from an approved Personal App for
  broader applications used by any seller who grants OAuth consent.

Configuring `approval.account_type` records deployment intent; it does not
prove Etsy approval. A clear platform permission/approval error is returned
when Etsy rejects the key, account, or scope.

Rate limits are application/API-key based and combine Queries Per Second (QPS)
with Queries Per Day (QPD). QPD uses a rolling 24-hour window rather than a
midnight reset. Current allocations are visible in Etsy's Developer Portal and
must not be hard-coded. `ResponseMeta` preserves the documented
`x-limit-per-second`, remaining-second, `x-limit-per-day`, and
`x-remaining-today` values. Honor HTTP 429 and `Retry-After`; higher limits
require an Etsy quota request with the use case and estimated QPS/QPD.

Official contracts verified on 2026-08-26:

- <https://developers.etsy.com/documentation/>
- <https://developers.etsy.com/documentation/reference/>
- <https://www.etsy.com/openapi/generated/oas/3.0.0.json>
- <https://developers.etsy.com/documentation/essentials/authentication/>
- <https://developers.etsy.com/documentation/essentials/requests/>
- <https://developers.etsy.com/documentation/essentials/rate-limits/>
- <https://developers.etsy.com/documentation/tutorials/listings/>
- <https://github.com/etsy/open-api/tree/2ecce66e07627358a074d46188586f0283c9a6cf>

The machine-readable contract reports OpenAPI 3.0.2, title `Etsy Open API v3`,
and version 3.0.0. Because the direct Etsy contract host was unavailable from
the verification environment, the same source was checked through the
read-only `api-evangelist/etsy` mirror at commit
[`3824defde9f8f46941abc9c34c49783ff5282b8a`](https://github.com/api-evangelist/etsy/blob/3824defde9f8f46941abc9c34c49783ff5282b8a/openapi/_original/etsy-openapi-original.yml).
The verified file SHA-256 is
`a5d28aa6f6791d9026daa4bc854de3f8e2157e77758c9fb5deca2c3dd636762e`.
No third-party runtime dependency is added.
