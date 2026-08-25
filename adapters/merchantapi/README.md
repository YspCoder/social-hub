# Google Merchant API v1

Package `social-hub/adapters/merchantapi` integrates Merchant Center product
data and reporting used by Shopping Ads and other Google commerce surfaces.
Importing the package registers `google/merchant-api-v1`.

This adapter intentionally remains separate from Google Ads campaign
management and Data Manager conversion ingestion. Merchant API controls the
catalog and eligibility data that those products consume; it does not set
campaign bids or budgets.

## Supported stable v1 surface

- Merchant Account identity and account-level issues;
- Data Source list and get;
- processed Product list and get, including `productStatus`;
- ProductInput insert/upsert, explicit-mask patch, and delete;
- paginated Merchant Query Language reports;
- dynamic method-group quota usage and Product account limits.

Account creation, user and relationship management, Program enable/disable,
shipping, return policies, Data Source mutation, and Notifications are outside
this adapter version.

## Configuration

```yaml
version: 1
platforms:
  - adapter: google/merchant-api-v1
    product: merchant-api
    accounts:
      - id: merchant-main
        client_id: 123456789.apps.googleusercontent.com
        secret_ref: secret://google/merchant/client-secret
        access_token_ref: secret://google/merchant/access-token
        approval:
          scopes:
            - https://www.googleapis.com/auth/content
        settings:
          merchant_account_id: "123456789"
```

The Google Cloud project must enable Merchant API. The OAuth principal or
service identity represented by the supplied token must also be a user of the
configured Merchant Center account. OAuth consent and Merchant Center account
roles are separate authorization gates.

Merchant REST requests use the fixed official origin
`https://merchantapi.googleapis.com`. Authorization and token exchange use
Google's fixed HTTPS OAuth origins. The adapter does not accept endpoint
overrides; cloned HTTP clients do not follow redirects or retain a cookie jar.

## Product input

Product writes always identify an existing API Data Source. Common
Shopping attributes are strongly typed; newly added or specialist v1 fields
can be supplied through `ProductAttributes.Extra` as exact `json.RawMessage`
values without overriding a typed field.

```go
import (
    "context"

    "social-hub/adapters/merchantapi"
    "social-hub/pkg/socialhub"
)

func upsertProduct(ctx context.Context, adapter socialhub.Adapter) error {
    common, err := adapter.Client(ctx, "merchant-main")
    if err != nil {
        return err
    }
    client := common.(*merchantapi.Client)
    _, err = client.Products().InsertProductInput(ctx, merchantapi.InsertProductInputRequest{
        DataSource: "accounts/123456789/dataSources/104628",
        Input: merchantapi.ProductInput{
            OfferID:         "SKU12345",
            ContentLanguage: "en",
            FeedLabel:       "US",
            VersionNumber:   "42",
            ProductAttributes: &merchantapi.ProductAttributes{
                Title:        "Classic Cotton T-Shirt",
                Description:  "A durable cotton t-shirt.",
                Link:         "https://shop.example/products/SKU12345",
                ImageLink:    "https://shop.example/images/SKU12345.jpg",
                Availability: merchantapi.AvailabilityInStock,
                Condition:    merchantapi.ConditionNew,
                Price: &merchantapi.Price{
                    AmountMicros: "15990000",
                    CurrencyCode: "USD",
                },
                GTINs: []string{"9780007350896"},
            },
        },
    })
    return err
}
```

`VersionNumber` prevents an older primary-feed insertion from overwriting a
newer one. It is not accepted for supplemental-source inserts or patch calls;
the caller is responsible for selecting it only where Merchant API permits it.

Use `ProductResourceName` and `ProductInputResourceName` to create the
recommended unpadded base64url resource names:

```go
name, err := merchantapi.ProductResourceName(
    "123456789", "en", "US", "sku/with/reserved~characters", false,
)
```

Response `Name` fields remain human-readable and can contain reserved
characters from the offer ID. Use the corresponding `Base64EncodedName` field
or these helpers when passing a resource name back to a REST method.

## Reporting and eligibility

```go
page, err := client.Reports().SearchReports(ctx, merchantapi.ReportRequest{
    Query: `SELECT product_view.id,
                   product_view.title,
                   product_view.aggregated_reporting_context_status
            FROM product_view
            WHERE product_view.aggregated_reporting_context_status != 'ELIGIBLE'`,
    PageSize: 1000,
})
if err != nil {
    return err
}
for _, row := range page.Rows {
    var view struct {
        ID     string `json:"id"`
        Title  string `json:"title"`
        Status string `json:"aggregatedReportingContextStatus"`
    }
    if err := row.DecodeField("productView", &view); err != nil {
        return err
    }
}
```

Report rows retain the single populated report view as exact JSON. This avoids
float conversion for micros, ratios, and int64 counters while keeping the
adapter forward-compatible with new v1 report fields.

## Mutation and consistency boundaries

- `productInputs.insert` is an upsert. If the product currently belongs to a
  different Data Source, supplying another source can move its ownership.
- ProductInput responses describe submitted input. The processed Product may
  differ after rules and supplemental sources are applied.
- Product processing is asynchronous. A Product read can remain stale or
  unavailable for several minutes after insert, patch, or delete.
- `PatchProductInput` requires a non-empty explicit update mask and rejects
  immutable identifiers. Mask paths use
  `product_attributes.<top_level_field>` or
  `custom_attribute.<attribute_name>`; nested attribute paths, `*`, and
  ProductInput identity fields are rejected. A mask path without a body value
  deletes that field, following the platform contract.
- Deleting a ProductInput from a primary Data Source removes the processed
  Product and all supplemental inputs. Deleting from a supplemental source
  removes only attributes contributed by that source.
- A successful insert does not mean the Product is approved for Shopping Ads.
  Inspect `Product.ProductStatus`, account issues, or `product_view` reports.

## OAuth, quotas, and limits

- `Adapter.OAuth` supports Google's Authorization Code exchange and refresh
  using `https://www.googleapis.com/auth/content`. `OAuthClient.Close` clears
  the client secret and prevents new authorization work.
- REST clients use a snapshot of the access token resolved during `Client`.
  They do not refresh or persist it through `TokenStore`; after OAuth refresh,
  update the referenced secret and create a new client. `Client.Close` clears
  its token and prevents subsequent authenticated requests.
- Per-call options support only `Timeout`. Caller request IDs, idempotency keys,
  and generic partial-response fields are rejected because the stable methods
  do not define those mutation headers and account validation requires identity
  fields.
- `ErrOutcomeUnknown` marks a write whose request may have reached Google but
  whose result could not be confirmed. Reconcile the ProductInput in Merchant
  Center before retrying.
- Merchant quotas are dynamic per account and method group. Call
  `ListQuotaGroups`; values include current daily usage, daily limit, minute
  limit, and all methods sharing the group.
- Daily quota usage resets at 12:00 PM UTC, not midnight.
- `ListProductLimits` uses the required `type = "products"` filter and returns
  separate EEA and non-EEA Shopping Ads catalog limits when configured.
- List pages are bounded to each official v1 maximum. Report requests may ask
  for up to 100,000 rows, but the shared transport still limits an individual
  REST response to 8 MiB; use smaller pages for wide reports.

## Official and mature references

- [Merchant API REST reference](https://developers.google.com/merchant/api/reference/rest)
- [Product management guide](https://developers.google.com/merchant/api/guides/products/add-manage)
- [Product compatibility guide](https://developers.google.com/merchant/api/guides/compatibility/products)
- [Official generated REST Go clients](https://github.com/googleapis/google-api-go-client/tree/main/merchantapi)
- [Official Cloud Go Merchant clients](https://github.com/googleapis/google-cloud-go/tree/main/shopping/merchant)
- [Google OAuth 2.0 web-server flow](https://developers.google.com/identity/protocols/oauth2/web-server)

The official generated stable v1 Discovery revisions reviewed on 2026-08-25
were Accounts `20260807`, Products `20260729`, Data Sources `20260719`, Reports
`20260715`, and Quota `20260713`. Their generated Go clients were used as
contract references without adding their large runtime dependency graph to
social-hub.
