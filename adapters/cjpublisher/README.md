# CJ publisher GraphQL and Link Search v2 adapter

Registration name: `cj/publisher-graphql-link-search-v2`

CJ does not publish one versioned umbrella "Publisher API". Its current
publisher surface combines individually named, unversioned public GraphQL APIs
with the versioned REST Link Search v2 endpoint. The registration name and
metadata preserve that distinction instead of inventing an API version.

## Implemented workflows

| social-hub method | Official CJ operation | Notes |
|---|---|---|
| `CJ().SearchProductFeeds` | Product Feed GraphQL `productFeeds` | feed summary discovery; offset/limit pagination |
| `CJ().SearchProducts` | Product Feed GraphQL `products` | common product fields, exact money, offset or `nextPage` cursor pagination, optional PID-bound link code |
| `CJ().ListPublisherCommissions` | Commission Detail GraphQL `publisherCommissions` | near-real-time attribution, item detail, corrections, and `maxCommissionId` cursor |
| `CJ().ListProgramTerms` | Program Terms GraphQL `publisher.contracts` | current and historical terms, action terms, situations, item lists, PIDs, and commission rates |
| `CJ().SearchLinks` | `GET /v2/link-search` | creative/tracking-link discovery and deep-link eligibility; bounded XML decoding |

The adapter deliberately excludes the deprecated REST Commission Detail API,
advertiser-side Product Import mutations, full-feed subscriptions, private
GraphQL specs, Promotional Property mutations, and legacy developer-key query
authentication.

## Configuration and authentication

CJ Personal Access Tokens are created and revoked in the Developer Portal.
Requests use only `Authorization: Bearer <token>`. CJ exposes no authorization
code or refresh-token flow for these PATs, so a rejected or expired credential
must be rotated by the operator. Existing Developer Keys are deprecated and CJ
states that they will not work with future APIs.

```yaml
version: 1
platforms:
  - adapter: cj/publisher-graphql-link-search-v2
    product: publisher-apis
    accounts:
      - id: primary-publisher
        access_token_ref: env://CJ_PERSONAL_ACCESS_TOKEN
        approval:
          account_type: approved-cj-publisher
        settings:
          publisher_id: "1234567" # CJ company ID / CID
          website_id: "7654321"   # optional default promotional property / PID
```

Applications must import the package so its factory is registered:

```go
import _ "social-hub/adapters/cjpublisher"
```

CJ uses separate origins for Products, Commissions, Program Terms, and Link
Search. The four adapter-level URL overrides are intended only for a controlled
contract-verification gateway. Redirects are rejected so the PAT cannot move
to another origin.

## Contract guarantees

- GraphQL operations use variables rather than interpolating IDs or search
  values into query text. Product, commission, and program-term records retain
  their provider objects in `Raw`; response `Raw` retains the complete GraphQL
  envelope.
- CJ can return HTTP 200 with both partial `data` and GraphQL `errors`. The
  adapter decodes and returns the partial typed response, retains its complete
  envelope, and also returns an `APIError`. Callers must not discard the error.
- Provider decimals and long numeric values use `ExactValue`; the adapter does
  not coerce money or identifiers through `float64`.
- Product search defaults are left to CJ. `limit` is bounded to the documented
  maximum of 10,000. Offset and the opaque `nextPage` cursor are mutually
  exclusive. Responses larger than the shared 8 MiB transport bound must be
  retrieved with a smaller limit.
- Product `linkCode` is selected only when `IncludeLinkCode` is true. CJ's
  schema requires `pid: ID!`; the request PID overrides the account default,
  and an optional shopper ID is passed only as a GraphQL variable. CJ requires
  the PID to belong to the queried company and allows one PID per request.
- Commission date filters are normalized to UTC. Each complete date range is
  limited to 31 days. CJ permits a single bound and then applies its documented
  24-hour counterpart default. Corrections are delta records: `original=false`
  is not a replacement snapshot and shares an order correlation with the
  original transaction.
- Program Terms uses CJ's default page size of 10 when omitted and enforces the
  documented maximum of 100 records per request.
- Link Search requires a PID and at least one real search filter. Query
  encoding uses `url.Values`, preserving CJ's required `space -> +` and literal
  `+ -> %2B` behavior. XML responses and error bodies are bounded to 8 MiB.
  Legacy 401 responses may echo the rejected credential; `APIError.Raw` is
  therefore sanitized before it is exposed.

## Deep links

Link Search can restrict results to creatives where `allow-deep-linking=true`
and returns CJ tracking URLs/link code for joined advertisers. Product Search
can return a PID-bound `linkCode`. The current public CJ documentation does not
define a separate destination-rewrite or arbitrary deep-link creation request,
so this adapter does not construct one by modifying a tracking URL. Link code
is blank or unavailable when the publisher relationship or PID permission does
not allow it.

## Rate limits and limiter keys

| API family | Published limit | Other constraints |
|---|---:|---|
| Product Search GraphQL | 500 calls / 5 minutes | default 1,000, maximum 10,000 records; use `nextPage` beyond 10,000 |
| Commission Detail GraphQL | 200 calls / 5 minutes | 120 concurrent connections, 10,000 commissions per payload, 31-day date range |
| Link Search v2 | 25 calls / minute | publishers only; empty filter set returns no results |
| Program Terms GraphQL | no public call cap stated | default 10 and maximum 100 records per request |

These are distinct endpoint families and should not share one numeric bucket.
The same PAT can authorize more than one company, so application limiters should
key quotas by credential plus API family, not only by `publisher_id`. CJ does
not document stable rate-limit response headers for these APIs; HTTP 429 is
retryable even when no reset hint is supplied.

## Commercial access and data visibility

A CJ account and a PAT authorized for the requested company ID are required.
Link Search and Program Terms are publisher-only. Product search is available
to publishers and advertisers, but this adapter binds it to the configured
publisher company. Product feeds marked private by an advertiser are not
available through Product Search. Program Terms only includes advertisers with
which the publisher has or had a relationship. Link code and some commission
fields remain subject to relationship status, PID ownership, advertiser data
sharing, and the token user's account permissions.

The capability state is `ApprovalRequired` until configuration records a
non-empty `approval.account_type`; setting that field is an operator assertion,
not a substitute for CJ-side approval.

## Official contract evidence

Official CJ material reviewed on 2026-08-26:

- <https://developers.cj.com/authentication/overview>
- <https://developers.cj.com/graphql/reference/Product%20Feed>
- <https://developers.cj.com/graphql/reference/Commission%20Detail>
- <https://developers.cj.com/graphql/reference/Program%20Terms>
- <https://developers.cj.com/docs/rest-apis/link-search>
- <https://developers.cj.com/docs/rest-apis/overview>
- <https://www.cj.com/join>

The Developer Portal obtains its public GraphQL introspection documents from
CJ's production documentation asset store. The exact assets reviewed were:

| Contract asset | SHA-256 |
|---|---|
| `docs/api/graphql/public/Product Feed.json` | `b82502490627c863331ae408c86026eebcba0af8b5fb52f914a85ebbfb41a360` |
| `docs/api/graphql/public/Commission Detail.json` | `86d70706d263a725acf32ab44442d9b18879dc22072e54491b68fc6648b6a6d4` |
| `docs/api/graphql/public/Program Terms.json` | `747231b8d9c4e37841541e757edc381948ea2294ab7d5d305afe294bbf8c1939` |
| `REST APIs/Link Search.md` | `9ca6e630950be10be3d901c553fc82761a30bf783d0d073f2e27300216b84f86` |

No third-party schema package or GitHub runtime dependency is used. The public
introspection documents are sufficiently complete for the implemented fields,
and using older community clients would reintroduce the deprecated REST
Commission Detail or private Product Search contracts.
