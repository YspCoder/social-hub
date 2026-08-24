# Steam Web API adapter

Adapter name: `steam/web-api`

This package implements a deliberately small, read-only subset of the public
Steam Web API host at `https://api.steampowered.com`.

| Typed workflow | Official endpoint | Authentication |
| --- | --- | --- |
| `GetPlayerSummaries` | `GET /ISteamUser/GetPlayerSummaries/v2/` | ordinary user Web API key |
| `GetNewsForApp` | `GET /ISteamNews/GetNewsForApp/v2/` | public; no key |

The adapter excludes publisher-only Web APIs and
`https://partner.steam-api.com`. Publisher keys are a separate credential
boundary, and the partner host rejects ordinary user keys. Friend lists,
owned games, recently played games, authenticated app news, writes, OAuth,
webhooks, and automatic pagination are also outside this package. The current
official pages reviewed for this implementation did not provide enough
response-schema detail to type the friend, owned-game, or recently-played
responses without guessing.

## Authentication and configuration

Configure an externally managed ordinary Steam user Web API key through
`access_token_ref`. The adapter resolves the secret at client construction and
sends it only in the `x-webapi-key` HTTPS request header. It never places the
key in a query parameter. `GetNewsForApp` always uses a separate anonymous
transport, including when the account also has a key.

```yaml
version: 1
platforms:
  - adapter: steam/web-api
    product: web-api
    accounts:
      - id: community-read
        access_token_ref: env://STEAM_WEB_API_KEY
      - id: public-news-only
```

The `public-news-only` account can call `GetNewsForApp`; attempting
`GetPlayerSummaries` returns `CodeUnauthenticated`. Client IDs, app IDs,
publisher secrets, token stores, OAuth scopes, account settings, webhook
settings, custom API origins, and redirects are rejected or disabled. Steam
recommends keeping Web API keys confidential and supports restricting a key to
known IP addresses.

## Usage

```go
package main

import (
	"context"
	"fmt"

	steamadapter "social-hub/adapters/steam"
	"social-hub/pkg/socialhub"
)

func playerNames(ctx context.Context, config socialhub.AdapterConfig) error {
	adapter, err := socialhub.Open(ctx, "steam/web-api", config)
	if err != nil {
		return err
	}
	defer adapter.Close()

	base, err := adapter.Client(ctx, "community-read")
	if err != nil {
		return err
	}
	client := base.(*steamadapter.Client)
	response, err := client.Steam().GetPlayerSummaries(ctx, steamadapter.GetPlayerSummariesRequest{
		SteamIDs: []steamadapter.SteamID{"76561197960435530"},
	})
	if err != nil {
		return err
	}
	for _, player := range response.Players {
		fmt.Println(player.SteamID, player.PersonaName)
	}
	return nil
}
```

Application news uses a strict `uint32` AppID:

```go
news, err := client.Steam().GetNewsForApp(ctx, steamadapter.GetNewsForAppRequest{
	AppID: 440,
	Count: 10,
	Tags:  []string{"patchnotes"},
})
```

## Data and input boundaries

- `SteamID` is retained as its original decimal string and validated against
  the complete positive `uint64` range. It is never converted through `int64`
  or a floating-point JSON number. A summaries request accepts 1 to 100 unique
  IDs, matching the official method limit.
- AppID is a positive `uint32`. The current v2 `feeds` and `tags` filters are
  supported. The adapter applies local safety limits of 100 news items, 65,536
  requested content characters, and 20 values per filter. These are SDK
  response-safety boundaries, not Valve quota claims.
- Successful responses are limited to 8 MiB and must preserve the documented
  `response.players` or `appnews.newsitems` envelope. Provider objects and the
  complete success envelope are retained in `Raw` after credential scanning.
- Redirects are refused and the copied HTTP client's cookie jar is removed.
  Error bodies are limited to 64 KiB and recursively redact credential-shaped
  JSON fields as well as the exact configured key.

## Rate limits and errors

The reviewed official Web API documentation does not define a current stable
QPS value, daily quota, or rate-limit response-header contract. This adapter
therefore does not hard-code the often-repeated historical daily quota. HTTP
`429` maps to retryable `CodeRateLimited`, and a valid `Retry-After` value is
preserved. `500` and `503` are retryable; `400`, `401`, `403`, `404`, and `405`
are permanent or user-action errors according to the official response guide.

Profile visibility still controls returned data. In particular, a private
resource can yield an authorization failure or omit data even when the key is
valid. Callers must not treat a valid key as permission to access private user
information.

## Official sources

Official material reviewed on 2026-08-25:

- <https://partner.steamgames.com/doc/webapi>
- <https://partner.steamgames.com/doc/webapi_overview>
- <https://partner.steamgames.com/doc/webapi_overview/auth>
- <https://partner.steamgames.com/doc/webapi_overview/responses>
- <https://partner.steamgames.com/doc/webapi/ISteamUser>
- <https://partner.steamgames.com/doc/webapi/IPlayerService>
- <https://partner.steamgames.com/doc/webapi/ISteamNews>
- <https://api.steampowered.com/ISteamWebAPIUtil/GetSupportedAPIList/v1/>

The selected methods use their current recommended `v2` forms. This package
has no third-party dependency.
