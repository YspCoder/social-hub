# Outbrain Amplify API v0.1 adapter

Package `social-hub/adapters/outbrain` implements marketer-scoped Outbrain
Amplify API v0.1 workflows. Budgets, Campaigns, PromotedLinks, and paid-media
reports intentionally remain separate from common social `Post`, `Fetcher`,
and `MediaUploader` operations.

## Configuration

Import the package for registration and configure one social-hub account per
Outbrain Marketer:

```go
import _ "social-hub/adapters/outbrain"
```

An externally managed token can be referenced directly:

```yaml
adapter: outbrain/amplify-api-v0.1
product: amplify-api
accounts:
  - id: native-us
    access_token_ref: secret://outbrain/token
    settings:
      marketer_id: 00f4b02153ee75f3c9dc4fc128ab041962
```

The adapter can instead obtain a token from `GET /login` with the API
username and password:

```yaml
adapter: outbrain/amplify-api-v0.1
product: amplify-api
accounts:
  - id: native-us
    secret_ref: secret://outbrain/password
    settings:
      marketer_id: 00f4b02153ee75f3c9dc4fc128ab041962
      username: api-user@example.com
```

Each account must configure exactly one credential mode. `base_url` overrides
are reserved for tests and controlled gateways. API and login redirects are
rejected so `OB-TOKEN-V1` tokens and Basic credentials cannot be forwarded to
another origin.

## Access and authentication

Amplify API access is granted by Outbrain on request. Ordinary dashboard access
does not by itself establish API access for a username or Marketer.

`LoginClient.Token` sends Basic authentication only to `/login` and reads the
`OB-TOKEN-V1` response field. The token is applied through the `OB-TOKEN-V1`
request header, not as a Bearer token. Tokens are valid for 30 days; creating a
new token does not invalidate older tokens, while password or email changes do.
The adapter caches login tokens in memory and optionally in
`socialhub.TokenStore`, serializes concurrent refreshes, and refreshes about 24
hours before expiry. This is important because login is limited to two requests
per hour per user.

## Supported workflows

- Marketer list/get and configured-Marketer access validation.
- Budget list/get, typed create, and typed update.
- Campaign list/get, disabled create, typed update, and explicit enable changes.
- PromotedLink list/get, hosted-image create, explicit enable changes, and
  bounded batch CPC updates.
- Campaign and Promoted Content performance reports.
- Periodic Campaign and per-Campaign Promoted Content reports.

Campaign creation always sends `enabled=false`, validates returned Marketer and
Budget ownership, and rejects a response that is enabled or on air.
PromotedLink creation is allowed only while the parent Campaign is disabled and
off air, always sends `enabled=false`, and validates returned Campaign ownership.

Enabling a Campaign requires at least one PromotedLink. Every PromotedLink must
be explicitly disabled, unarchived, and approved. The initial adapter refuses
to enable Campaigns with more than 500 PromotedLinks because one documented
page cannot prove every item's state. Enabling a PromotedLink is a separate
call that requires the parent Campaign to be enabled and the link to remain
approved and unarchived. This makes delivery a deliberate two-stage action.

The initial adapter does not implement local image multipart upload, carousel
sequences, audience or interest management, conversion pixels, publisher and
section bid controls, bulk Campaign mutations, or organic social capabilities.
Those surfaces require separately reviewed typed workflows rather than generic
map-based mutations.

## Limits and errors

Outbrain currently documents these limits:

- `/login`: 2 requests per hour per user.
- One token: 30 requests per second across the API.
- Performance Reporting: 10 requests per minute per Marketer.
- Real-time Performance Reporting: 50 requests per minute per Marketer.

HTTP 429 responses map to retryable `socialhub.ErrRateLimited` errors. The
`rate-limit-msec-left` header is interpreted as milliseconds, with standard
`Retry-After` as a fallback. The adapter preserves `AMPLIFY-REQUEST-ID`, bounds
response bodies, keeps logical workflow names in `socialhub.Error.Op`, and
redacts Basic credentials, tokens, and token-shaped platform messages.

The four report methods expose only documented filters. Campaign, Promoted
Content, Periodic Campaign, and Periodic Content endpoints each support their
documented `limit` and `offset`; the adapter does not invent pagination for
other report resources.

## Contract sources

The official [Amplify API v0.1 reference](https://amplifyv01.docs.apiary.io/),
[current authentication and rate-limit guide](https://www.outbrain.com/help/advertisers/amplify-api/),
and [API access request](https://www.outbrain.com/partner-api/) are the source
of truth. The Apiary reference was verified on 2026-08-09 and includes updates
through 2026-02-01 while retaining the v0.1 base URL.

No maintained official or community Go SDK was found with a sufficiently
complete current contract. The adapter therefore uses social-hub's bounded
shared transport without adding an Outbrain-specific dependency.
