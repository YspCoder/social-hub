# WeChat Mini Program server API adapter

Adapter name: `wechat/mini-program`

This package implements four Mini Program server-side workflows with the
official fixed origin `https://api.weixin.qq.com`:

- `GET /sns/jscode2session`;
- `POST /cgi-bin/stable_token` in ordinary and explicit force-refresh modes;
- `POST /cgi-bin/message/subscribe/send`;
- `POST /wxa/business/getuserphonenumber`.

The package does not implement Mini Program publishing, customer-service
messages, media, webhooks, third-party-platform `authorizer_access_token`, or
optional service-communication secondary encryption. It has no third-party
Go dependency.

## Configuration

Each account is one Mini Program AppID. The adapter resolves AppSecret at
runtime, retrieves stable access tokens itself, and does not accept an
externally managed access token.

```yaml
version: 1
platforms:
  - adapter: wechat/mini-program
    product: mini-program
    accounts:
      - id: primary
        app_id: wx0000000000000000
        secret_ref: env://WECHAT_MINI_PROGRAM_SECRET
```

The API origin cannot be overridden. The configured HTTP client is copied
with redirects disabled and its cookie jar removed. WeChat requires AppID,
AppSecret, and `js_code` in the code2Session query and requires access tokens
in other endpoint queries. Transport failures are stripped of their URL
wrapper so those query credentials cannot enter returned errors.

## Login state

`code2Session` is not OAuth. The Mini Program calls `wx.login`, sends its
short-lived code to the developer's server, and the server exchanges it for
OpenID and `session_key`; UnionID is returned only when the platform's UnionID
conditions are satisfied.

```go
base, err := adapter.Client(ctx, "primary")
if err != nil {
	return err
}
client := base.(*wechatminiprogram.Client)

session, err := client.Login().Code2Session(ctx, jsCode)
if err != nil {
	return err
}
```

OpenID, UnionID, `js_code`, and especially `session_key` are sensitive.
`session_key` must remain on the server and must not be returned as the
application's own login session. Create a separate application session after
binding the OpenID to the correct account and risk controls.

## Stable access token

Ordinary calls are coordinated per client and cached until five minutes
before provider expiry. The official contract currently states that the token
is valid for no more than 7,200 seconds, ordinary stable-token calls are
limited to 10,000 per minute and 500,000 per day, and force refresh is limited
to 20 per day with at least 30 seconds between calls. The adapter enforces the
30-second interval within one client but does not pretend that process-local
state can enforce provider-wide quotas across replicas.

```go
token, err := client.Credentials().GetStableAccessToken(ctx)
```

`token.Value` is a credential. Explicit
`ForceRefreshStableAccessToken` invalidates the previous provider token and
should be reserved for credential recovery. Multi-instance deployments should
centralize client construction or otherwise coordinate force refreshes.

## Subscription messages

Sending requires a template selected for the account and a matching grant
obtained from the user through `wx.requestSubscribeMessage`. One-time grants
can be consumed; error `43101` means no usable subscription remains. Account,
template, category, content, concurrency, and platform enforcement still
apply even though the adapter exposes the method.

```go
err := client.SubscriptionMessages().Send(ctx, wechatminiprogram.SubscriptionMessage{
	ToUser:     openID,
	TemplateID: templateID,
	Page:       "pages/orders/detail?id=123",
	Data: map[string]wechatminiprogram.TemplateValue{
		"thing01": {Value: "Order ready"},
		"date01":  {Value: "2026-08-25"},
	},
})
```

The adapter defaults `miniprogram_state` to `formal` and `lang` to `zh_CN`.
Template keyword types have different provider limits; the server remains
authoritative and `47003` maps to an invalid-argument error. This direct-AppID
adapter does not claim third-party-platform permission set 18.

## Phone number exchange and compliance

The current official component documentation limits the fast phone-number
capability to verified, non-individual Mini Program subjects, with only some
overseas regions supported, and describes it as a paid capability subject to
account quota or resource-package state. The WeChat console is authoritative
for current eligibility and commercial terms.

The user must explicitly consent through the `getPhoneNumber` button. Its code
is valid for five minutes, can be consumed once, and is different from a
`wx.login` code. It must not be reused or interchanged with code2Session.

```go
phone, err := client.PhoneNumbers().Exchange(ctx, wechatminiprogram.PhoneNumberRequest{
	Code:   phoneCode,
	OpenID: openID, // optional provider binding check
})
```

Phone numbers are personal information. Callers are responsible for a lawful,
clearly disclosed purpose, explicit consent where required, data minimization,
access control, encryption at rest, retention/deletion rules, incident
handling, and any applicable cross-border requirements. This adapter neither
stores phone data nor supplies a compliance determination.

## Error and sensitive-data behavior

WeChat commonly returns business errors as HTTP 200 JSON containing
`errcode`, `errmsg`, and sometimes `rid`. The adapter maps documented codes to
socialhub categories, including retryable `-1`, `45009`, and `45011`, login
risk block `40226`, and subscription errors `40037`, `43101`, `43107`,
`43108`, `45168`, and `47003`.

No response type has a `Raw` field. `APIError` keeps only numeric `errcode` and
a validated request ID; provider `errmsg` and response bodies are discarded
because they may echo AppID, AppSecret, codes, tokens, OpenIDs, or phone data.
Sensitive public values implement redacted `String` and `GoString`, but callers
must still avoid logging individual fields.

## Official sources

Official material reviewed on 2026-08-25:

- <https://developers.weixin.qq.com/miniprogram/dev/server/API/user-login/api_code2session.html>
- <https://developers.weixin.qq.com/miniprogram/dev/server/API/mp-access-token/api_getstableaccesstoken.html>
- <https://developers.weixin.qq.com/miniprogram/dev/server/API/mp-message-management/subscribe-message/api_sendmessage.html>
- <https://developers.weixin.qq.com/miniprogram/dev/server/API/user-info/phone-number/api_getphonenumber.html>
- <https://developers.weixin.qq.com/miniprogram/dev/framework/open-ability/subscribe-message.html>
- <https://developers.weixin.qq.com/miniprogram/dev/framework/open-ability/getPhoneNumber.html>
