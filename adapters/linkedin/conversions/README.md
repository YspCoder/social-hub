# LinkedIn Conversions API 202608 adapter

Package `social-hub/adapters/linkedin/conversions` streams server-side events
to one enabled LinkedIn Conversion Rule using Marketing API version `202608`.
It is separate from organic LinkedIn and paid-media Campaign management because
conversion events have their own product approval, matching identifiers,
retention window, and quotas.

## Configuration

Import the package for registration and typed event access:

```go
import conversions "social-hub/adapters/linkedin/conversions"
```

Configure one social-hub account per Conversion Rule:

```yaml
adapter: linkedin/conversions-api-202608
product: conversions-api
accounts:
  - id: north-america-purchase
    access_token_ref: secret://linkedin-conversions/purchase/access-token
    approval:
      scopes: [rw_conversions, r_ads]
    settings:
      ad_account_id: "5123456"
      conversion_id: "123456"
```

The adapter constructs
`urn:lla:llaPartnerConversion:{conversion_id}` itself, so a call cannot switch
to another rule. `ad_account_id` documents the intended owner; LinkedIn remains
authoritative for rule ownership and the authenticated member's Ad Account
role. The API origin is fixed at `https://api.linkedin.com/rest`, redirects are
rejected, and caller cookie jars are ignored so the Bearer token cannot be
forwarded to a different origin.

The Conversion Rule must already exist, use `conversionMethod=CONVERSIONS_API`,
be enabled, and be associated with the intended Campaigns. Rule creation and
Campaign association are outside this ingestion-only adapter and can be done in
Campaign Manager or through LinkedIn's Conversion Rule APIs.

## Authentication

LinkedIn supports two access paths:

- Advertisers can generate a non-expiring Direct API token in Campaign
  Manager's Signals Manager without creating a Developer App.
- Partners can request the Conversions API product for a verified Developer
  App and authorize member tokens with `rw_conversions` and `r_ads`. LinkedIn
  also requires a permitted Ad Account role.

For Direct API tokens, omit `approval.scopes`; capability approval is then
reported as unknown and LinkedIn enforces account ownership. Partner token
acquisition and refresh remain external to this package. Credential values are
always resolved through `socialhub.SecretResolver`.

## Submit events

```go
result, err := client.Events().SubmitEvents(ctx, conversions.SubmitEventsRequest{
    Events: []conversions.ConversionEvent{{
        ConversionHappenedAt: time.Now().Add(-time.Minute).UnixMilli(),
        EventID: "checkout-8c482",
        ConversionValue: &conversions.Money{
            CurrencyCode: "USD",
            Amount: "50.00",
        },
        User: conversions.User{
            Emails: []string{customer.Email},
            LinkedInFirstPartyTrackingUUIDs: []string{liFatID},
            Info: &conversions.UserInfo{
                FirstName: customer.FirstName,
                LastName: customer.LastName,
                CountryCode: "US",
            },
        },
    }},
})
```

A one-event request sends the event directly. Two or more events send
`{"elements":[...]}` with `X-RestLi-Method: BATCH_CREATE`. Batch validation is
atomic: one invalid local event prevents the request, and one LinkedIn batch
validation error rejects the whole remote batch. The maximum is 5,000 events.

`conversionHappenedAt` is Unix milliseconds and must be within the past 90
days. `Money.Amount` is a validated non-negative decimal string and remains a
JSON string, exactly as LinkedIn's schema requires. `eventId` is optional; use
the same ID on corresponding Insight Tag/Image Pixel and server events when
deduplication is configured.

## Matching and privacy

The typed user model covers all currently documented match identifiers:

- `SHA256_EMAIL`: plaintext email is lowercased, all whitespace is removed,
  and the result is SHA-256 hashed locally. Exact lowercase SHA-256 is accepted.
- `LINKEDIN_FIRST_PARTY_ADS_TRACKING_UUID`: the `li_fat_id` click/cookie value.
- `ACXIOM_ID`: a LiveRamp/Acxiom identifier.
- `PLAINTEXT_IP_ADDRESS` and `SHA256_IP_ADDRESS`: LinkedIn currently supports
  IPv4 only.
- `GOOGLE_AID`: a valid nonzero Android advertising UUID.
- Plaintext `userInfo`, one `externalId`, or a Lead Gen Form Response URN.

Uppercase SHA-256 and MD5-looking email values are rejected to prevent silent
double hashing. Input objects are never mutated. When matching uses only
`userInfo`, `externalIds`, or `lead`, the adapter still sends the required
`"userIds": []` field.

LinkedIn validation errors can echo `batchIndex` and rejected user values. The
adapter discards response messages and preserves only HTTP status, numeric
`serviceErrorCode`, request ID, retryability, and bounded `Retry-After`.

## Protocol and limits

Every request includes:

```text
Authorization: Bearer <access token>
Linkedin-Version: 202608
X-Restli-Protocol-Version: 2.0.0
```

The official limit is 600 requests per minute and 500,000 requests per day per
member access token. Applications should batch where appropriate and treat
live throttling responses and Developer Portal quotas as authoritative.

Official contracts verified on 2026-08-25. LinkedIn lists August 2026 (`202608`)
as the latest Marketing API version, and the Conversions ingestion contract
continues to expose the same event schema and limits:

- [Conversions API](https://learn.microsoft.com/en-us/linkedin/marketing/integrations/ads-reporting/conversions-api?view=li-lms-2026-08)
- [Conversions API schema](https://learn.microsoft.com/en-us/linkedin/marketing/integrations/ads-reporting/conversions-api-schema?view=li-lms-2026-08)
- [Getting access](https://learn.microsoft.com/en-us/linkedin/marketing/conversions/getting-access-conversions?view=li-lms-2026-08)
- [Deduplication](https://learn.microsoft.com/en-us/linkedin/marketing/conversions/deduplication?view=li-lms-2026-08)
- [Marketing API versioning](https://learn.microsoft.com/en-us/linkedin/marketing/versioning?view=li-lms-2026-08)
