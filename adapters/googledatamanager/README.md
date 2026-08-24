# Google Data Manager API v1 adapter

Adapter name: `google/data-manager-api-v1`

Package `social-hub/adapters/googledatamanager` sends typed online and offline
events to Google Ads, Google Analytics, Display & Video 360, and Floodlight
destinations through the stable Google Data Manager API v1. The REST contract
was checked against Discovery revision `20260811` on 2026-08-25.

The implemented operation is:

```text
POST https://datamanager.googleapis.com/v1/events:ingest
```

The adapter covers all current `Event` fields, request-level and event-level
consent, multiple destinations, row-level field warnings, exact JSON decimals,
user-data normalization, and optional GCP or AWS wrapped-key metadata. Audience
management, account management, and diagnostics queries are separate Data
Manager workflows and are not exposed by this package.

## Authentication

Configure exactly one credential mode per social-hub account:

1. A caller-managed bearer token through `access_token_ref`.
2. User OAuth 2.0 Authorization Code with offline refresh through `client_id`,
   `secret_ref`, and `settings.refresh_token_ref`.
3. Service Account JWT Bearer through `settings.service_account_email` and
   `settings.private_key_ref`.

Managed credentials require exactly this scope in `approval.scopes`:

```text
https://www.googleapis.com/auth/datamanager
```

For a caller-managed bearer token, `approval.scopes` may be omitted when the
token's grants are not known locally, or it must contain exactly that scope.
The OAuth helper requests least privilege and rejects a token response that
reports additional or different scopes.

Data Manager does not require a Google Ads developer token. The credential
must still have write access to every operating account. Data Partner accounts
and links require separate Google approval. Google recommends Application
Default Credentials or service-account impersonation for production workloads
where available; this initial lightweight REST adapter accepts explicit secret
references so it can use social-hub's existing `SecretResolver`, `TokenStore`,
error model, and redirect policy. It does not yet discover ADC automatically.

User OAuth configuration:

```yaml
adapter: google/data-manager-api-v1
product: data-manager-api
accounts:
  - id: conversions-production
    client_id: google-oauth-client-id
    secret_ref: env://GOOGLE_OAUTH_CLIENT_SECRET
    approval:
      scopes:
        - https://www.googleapis.com/auth/datamanager
    settings:
      refresh_token_ref: env://GOOGLE_DATA_MANAGER_REFRESH_TOKEN
```

Service Account configuration:

```yaml
adapter: google/data-manager-api-v1
product: data-manager-api
accounts:
  - id: conversions-backend
    approval:
      scopes:
        - https://www.googleapis.com/auth/datamanager
    settings:
      service_account_email: events@example-project.iam.gserviceaccount.com
      private_key_ref: env://GOOGLE_DATA_MANAGER_SERVICE_ACCOUNT_PRIVATE_KEY
```

OAuth and Service Account access tokens can be cached through
`socialhub.TokenStore`. All credential values are resolved at runtime through
`socialhub.SecretResolver`. Supply the token store through SDK options;
account-level `token_store` names are not resolved by this adapter and are
rejected. The API, authorization, and token endpoints are fixed to Google's
official HTTPS origins. Credential-bearing HTTP clients reject redirects and
discard cookie jars. OAuth callback URLs require HTTPS, except for HTTP
loopback callbacks used by local applications.

## Send events

Import the package to register the adapter and use its typed client:

```go
import datamanager "social-hub/adapters/googledatamanager"

client := common.(*datamanager.Client)
response, err := client.Events().IngestEvents(ctx, datamanager.IngestEventsRequest{
    Destinations: []datamanager.Destination{{
        Reference: "google-ads",
        OperatingAccount: datamanager.ProductAccount{
            AccountType: datamanager.AccountTypeGoogleAds,
            AccountID:   "1234567890",
        },
        ProductDestinationID: "987654321",
    }},
    Events: []datamanager.Event{{
        DestinationReferences: []string{"google-ads"},
        TransactionID:         order.ID,
        EventTimestamp:        order.CompletedAt,
        EventSource:           datamanager.EventSourceWeb,
        Currency:              "USD",
        ConversionValue:       "125.40",
        AdIdentifiers: &datamanager.AdIdentifiers{
            GCLID: order.GCLID,
        },
        UserData: &datamanager.UserData{UserIdentifiers: []datamanager.UserIdentifier{
            {EmailAddress: customer.Email},
            {PhoneNumber: customer.E164Phone},
        }},
    }},
    Encoding: datamanager.EncodingHex,
})
```

Google Ads conversion action IDs are plain numeric IDs, not resource names.
The adapter validates all events before opening the HTTP request, so one local
field error fails the whole batch. It also rejects duplicate or unresolved
destination references. `Item.Quantity` uses an integer string because the
Discovery schema declares `format: int64` on a JSON string; this matches
Google's generated Go client wire format.

Of the common `socialhub.CallOption` values, this method supports only
`WithCallTimeout`. Google assigns the response request ID, exposes no response
field selector for this method, and does not define a request-level
idempotency-key header.

An HTTP success and `response.RequestID` mean Google accepted the ingestion
request. They do not mean every downstream conversion was matched or processed.
Use Data Manager diagnostics and preserve the request ID for later operational
checks. `FieldWarnings` identify non-blocking fields that Google ignored or
adjusted.

## User data and privacy

When `UserData` or `ThirdPartyUserData` is present, `Encoding` must be `HEX` or
`BASE64`. Plain values are normalized and SHA-256 hashed in a temporary request
copy; already encoded 32-byte SHA-256 values are preserved.

- Email: remove all whitespace and lowercase. For `gmail.com` and
  `googlemail.com` only, remove local-part dots and the `+suffix`.
- Phone: require caller-normalized E.164 with a leading `+`.
- Address: hash given name, family name, and optional address line; preserve
  normalized region code, postal code, city, and administrative area as required
  by the API.

Inputs are never mutated. For encrypted uploads, set exactly one
`GCPWrappedKeyInfo` or `AWSWrappedKeyInfo`. Fields that normally require hashing
(email, phone, names, and address line) are then treated as caller-encrypted
ciphertext and validated against the selected outer encoding. Unhashed address
fields such as region, postal code, city, and administrative area remain
normalized plaintext, as required by Google. Coordinator keys are intentionally
unavailable because `events:ingest` does not support them.

Google Ads does not support IP-address matching for end users in the EEA, UK,
or Switzerland. The caller is responsible for geolocation-aware suppression,
lawful consent, notices, retention, and all applicable Google policy controls
before populating `DeviceInfo.IPAddress`.

## Limits and retry behavior

| Limit | Published value |
|---|---:|
| Events per request | 2,000 |
| Destinations per request | 10 |
| User identifiers per `UserData` | 10 |
| Ingestion requests per day | 100,000 |
| Ingestion requests per minute | 300 |

HTTP 429, `RESOURCE_EXHAUSTED`, and transient 5xx/google.rpc statuses map to
retryable social-hub errors. `Retry-After` and Google's request ID are preserved.
Daily quota exhaustion can require user action rather than immediate retry.
Provider-supplied free-form error descriptions are not exposed; the adapter
retains only recognized error codes/statuses and a fixed safe message. A retry
is safe only when every affected destination supports deduplication and the
event reuses a stable `transactionId`; the SDK does not claim request-level
idempotency.

Official references:

- [events.ingest REST reference](https://developers.google.com/data-manager/api/reference/rest/v1/events/ingest)
- [Set up API access](https://developers.google.com/data-manager/api/devguides/quickstart/set-up-access)
- [Send events guide](https://developers.google.com/data-manager/api/devguides/events/send-events)
- [User-data formatting](https://developers.google.com/data-manager/api/devguides/concepts/formatting)
- [Published limits](https://developers.google.com/data-manager/api/devguides/limits)
