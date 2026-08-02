# Last.fm Web Services adapter

`lastfm` implements the official [Last.fm Web Services API 2.0](https://www.last.fm/api).
It exposes music discovery, public listening history, browser authentication,
now-playing updates, scrobbling, and track love state through typed workflows.
Music resources are intentionally not represented as social posts, and the
adapter does not download audio or use private Last.fm endpoints.

## Configuration

Import the adapter for registration:

```go
import _ "social-hub/adapters/lastfm"
```

Configure the API key as `client_id`. Store the API secret and authorized
session key behind secret references:

```yaml
version: 1
platforms:
  - adapter: lastfm/web-services-v2
    accounts:
      - id: listener
        client_id: 0123456789abcdef0123456789abcdef
        secret_ref: env://LASTFM_API_SECRET
        access_token_ref: env://LASTFM_SESSION_KEY
        settings:
          username: example-user
```

Public discovery and user endpoints require only `client_id`. The API secret
enables `AuthWorkflow`; the API secret plus session key enable
`ListeningWorkflow` and `LibraryWorkflow`. A Last.fm session key has no normal
expiry but remains revocable by the user.

## Browser authentication

Use the configured client to request a single-use token, send the user to the
approval URL, then exchange the approved token for a session:

```go
client := common.(*lastfm.Client)

token, err := client.RequestToken(ctx)
if err != nil {
	return err
}
approvalURL, err := client.AuthorizationURL(token, "https://app.example/lastfm/callback")
if err != nil {
	return err
}
// Redirect the user to approvalURL. After approval:
session, err := client.ExchangeSession(ctx, token)
```

Request tokens expire after 60 minutes and can be consumed once. Persist
`session.Key` in an encrypted secret store, then reference it with
`access_token_ref`. The adapter deliberately omits `auth.getMobileSession`
because that flow requires handling the user's Last.fm password.

## Typed workflows

- `AuthWorkflow`: request token, browser approval URL, session exchange
- `DiscoveryWorkflow`: Track, Artist, and Album info and search
- `UserWorkflow`: profile, recent tracks, top tracks, and loved tracks
- `ListeningWorkflow`: now playing and batches of up to 50 scrobbles
- `LibraryWorkflow`: love and unlove tracks

The scrobble signer follows Last.fm's ASCII parameter-name ordering, including
the documented `artist[10]` before `artist[1]` edge case. `format`, `callback`,
and `api_sig` are excluded from the signature. Write credentials are sent only
in form-encoded POST bodies.

Review the official [authentication guide](https://www.last.fm/api/authentication),
[scrobbling specification](https://www.last.fm/api/scrobbling), and
[API terms](https://www.last.fm/api/tos) before production use.

## Verification scope

Tests use deterministic local HTTP fixtures and do not contact Last.fm. The
adapter has not yet been validated with a live API application or user account.
