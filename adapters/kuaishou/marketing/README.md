# Kuaishou Magnetic Engine Marketing API adapter

Package `social-hub/adapters/kuaishou/marketing` implements advertiser-scoped
workflows for the Kuaishou Magnetic Engine Marketing API (MAPI). Paid-media
resources intentionally remain separate from the organic
`social-hub/adapters/kuaishou` adapter and are not exposed as common social
`Post` or `Fetcher` operations.

## Configuration

Import the package for registration, configure one account per advertiser, and
resolve credentials through the application's `socialhub.SecretResolver`:

```go
import _ "social-hub/adapters/kuaishou/marketing"
```

```yaml
adapter: kuaishou/magnetic-engine-marketing-api
product: magnetic-engine-marketing-api
accounts:
  - id: brand-cn
    app_id: "123456"
    secret_ref: secret://kuaishou/app-secret
    access_token_ref: secret://kuaishou/brand-cn/access-token
    settings:
      advertiser_id: 987654321
```

`app_id` and `secret_ref` are needed by `Adapter.OAuth`; API calls use the
resolved `access_token_ref` in the `Access-Token` header. Redirects are rejected
so that credentials cannot be forwarded to another origin.

## Access and OAuth

MAPI registration requires corporate qualifications and platform review. The
official onboarding guide currently states a review target of seven business
days and allows up to ten App IDs for one developer. Authorization starts at
`https://developers.e.kuaishou.com/tools/authorize` and supports the
`advertiser`, `agent`, `ad_social`, and `series` authorization types. Request
only roles that the application actually needs.

The authorization code expires after 10 minutes. The returned access token is
valid for 24 hours and the refresh token for 30 days. `OAuthClient` provides
authorization URL construction, code exchange, and token refresh; applications
remain responsible for atomic, encrypted token persistence and rotation.

## Supported workflows

- Advertiser information lookup.
- Campaign create, update, list, and status changes.
- Unit create, list, and status changes.
- Creative create, list, and status changes.
- Account, Campaign, Unit, and Creative real-time reports.
- Browser authorization, code exchange, and refresh-token rotation.

Campaigns and Units are created with `put_status=2` (paused). Creative creation
has no equivalent pause field, so callers must keep the parent Unit paused until
the full object graph has been reviewed. Some marketing goals and periodic
delivery combinations do not permit paused Campaign creation; the API returns a
typed user-action error and the caller must use an approved platform workflow.
The adapter does not silently create a delivering Campaign.

Complex provider fields that do not yet justify stable typed fields can be sent
through `Fields`. Identity, ownership, status, and other protected keys cannot
be overridden. Media asset upload, asynchronous report jobs, audience tooling,
and organic Kuaishou publishing are outside this package's current scope.

## Versions and limits

Kuaishou does not publish one global `v2.2` MAPI contract. The implemented API
surface mixes `/gw/dsp/...` and `/v1/...` routes, and documentation revisions
are maintained independently per endpoint (for example, Campaign create and
update have separate revision numbers). The adapter therefore registers as
`kuaishou/magnetic-engine-marketing-api` and reports `continuous` as its API
version. This describes a contract verified against the official documentation
on 2026-08-09, not an official platform-wide semantic version.

The documented developer-level limit is 3,000 calls per minute. Limits may also
vary by endpoint and account, so callers should honor platform errors and use
the shared social-hub rate-limit and retry hooks. Real-time reports accept at
most seven inclusive calendar dates. Asynchronous reports cover up to the most
recent six months and permit up to 50 tasks per day, but are not implemented in
this initial adapter.

## Contract sources

The official [Kuaishou MAPI documentation](https://developers.e.kuaishou.com/docs)
is the source of truth. The public
[`bububa/kwai-marketing-api`](https://github.com/bububa/kwai-marketing-api)
project was audited at tag `v1.10.1` (commit
`3c6f3360368c69b21ac6d6aeaa8b0f9556336408`) as an Apache-2.0-licensed model and
endpoint reference. Its source was not imported; where it differs from current
official documentation, the official contract takes precedence.
