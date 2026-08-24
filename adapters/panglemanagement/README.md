# Pangle App and Ad Placement Management API adapter

`adapters/panglemanagement` implements the complete public Pangle App and Ad
Placement Management API surface for publisher inventory outside the Chinese
mainland.

Official reference:

- Management API: <https://www.pangleglobal.com/integration/management-api>

The official contract was reviewed on 2026-08-25. The latest published
revision is 1.1.13, while requests continue to send wire version `1.0` as
required by Pangle. GitHub repository and code searches did not find a mature
Go client for these endpoints, so the adapter implements the official wire
contract directly.

## Implemented surface

| Workflow | Endpoint | Notes |
|---|---|---|
| `CreateApp` | `POST /union/media/open_api/site/create` | Production and sandbox status rules; verification timeout is an accepted asynchronous result |
| `UpdateApp` | `POST /union/media/open_api/site/update` | Typed partial updates and explicit empty blocking-rule lists |
| `ListApps` | `POST /union/media/open_api/site/query` | Page size 1-500; ID, name, OS, and status filters |
| `CreatePlacement` | `POST /union/media/open_api/code/create` | Native, Banner, App Open, Rewarded Video, and Interstitial specifications |
| `UpdatePlacement` | `POST /union/media/open_api/code/update` | Typed partial updates, pause/resume, rewards, blocking rules, and regional CPM |
| `ListPlacements` | `POST /union/media/open_api/code/query` | Page size 1-500 and all documented filters |
| `UpdateExpectedCPM` | `POST /union_pangle/open/api/code/cpm` | Update or delete placement CPM with cooldown-aware errors |

IDs are stored as decimal strings in Go and encoded as JSON numbers. This
avoids `float64` precision loss while preserving Pangle's integer wire format.
They must be canonical positive values within signed 64-bit range; leading
zeroes are rejected because they are not valid JSON number syntax.
`Money` is an exact decimal string on requests, and `Decimal` preserves exact
response text.

## Authentication and configuration

Pangle uses a master `user_id`, a permission-scoped `role_id`, and the role's
Security Key. It is not OAuth. Every request generates a cryptographically
random nonce and current Unix timestamp, sorts the Security Key, timestamp,
and nonce as strings, concatenates them, and applies Pangle's required SHA-1
signature.

```yaml
version: 1
platforms:
  - adapter: pangle/app-placement-management-api-v1.1.13
    product: app-placement-management-api
    accounts:
      - id: pangle-global-publisher
        secret_ref: env://PANGLE_MANAGEMENT_SECURITY_KEY
        settings:
          user_id: "459"
          role_id: "459"
```

The role must be a super admin, admin, or a custom role with the matching App,
Ad Placement, and blocking-rule permissions. Feature-specific placement and
bidding modes can additionally require an account allowlist. Production is
fixed to `https://open-api.pangleglobal.com`; custom origins are rejected so
publisher signatures cannot be redirected across products or regions.

## Sandbox

Set `sandbox: true` in adapter settings to use Pangle's isolated HTTP sandbox
and send `X-Tt-Env: open_api_sandbox`:

```yaml
settings:
  sandbox: true
```

Sandbox app creation requires `AppStatusTest`; production creation requires
`AppStatusLive`. Pangle's official sandbox contract is fixed to plaintext HTTP,
so `sandbox: true` must be used only with disposable non-production test
credentials. Redirects and Cookie Jars remain disabled in both environments.

## Contract and safety boundaries

- App creation or verification-sensitive updates can return business code
  `50007`. The call returns successfully with `PendingReview=true`; final
  verification arrives as a Pangle in-site message. This is not a completed
  approval.
- The current 1.1.13 request tables expose app category, name, download URL,
  blocking rules, COPPA, and status as applicable. Removed legacy request
  parameters such as package, OS, and APK signature are not sent; they remain
  available only on query responses where Pangle documents them.
- Native, Banner, App Open, Rewarded Video, and Interstitial creation use
  distinct typed specs. Invalid render modes, sizes, orientations, material
  types, reward values, and callback combinations are rejected before I/O.
- `mask_rule_id` and `mask_rule_ids` are mutually exclusive. `IDList()` can
  create a non-nil empty list when the caller intentionally clears bindings.
- Regional CPM is restricted to Pangle's documented country allowlist and
  fixed-CPM bidding. CPM values retain decimal precision.
- App and placement deletion are not part of the public Management API.
  Placement pause uses status `3`, and resume uses the explicit command status
  `-1`. Expected-CPM deletion requires `Delete=true` together with the
  provider-required CPM, app, placement, and currency fields.
- Pangle does not define idempotency or caller request-ID headers. Only
  `socialhub.WithCallTimeout` is accepted. If a mutation has a transport
  failure, HTTP 5xx, or an undecodable 2xx response, the adapter returns
  `ErrOutcomeUnknown`; query the App or Placement state before retrying,
  especially after a status switch or expected-CPM deletion.
- Pangle does not publish a numeric general request quota for this API.
  Business code `47005` is mapped to rate limiting; expected-CPM code `121`
  represents its update cooldown. Callers should honor `Retry-After` when it is
  returned and otherwise apply bounded exponential backoff.
- Request JSON is limited to 1 MiB and response JSON to 8 MiB. Security Keys,
  signatures, request bodies, response bodies, and free-form platform messages
  are not returned in errors. Provider request IDs and `Retry-After` are kept
  only after UTF-8, control-character, length, and credential checks.
- The Publisher Reporting API uses a different MD5 query signature and lives
  in `adapters/panglereporting`. The domestic Chuanshanjia API is a separate
  contract and is not inferred from this global Pangle adapter.
