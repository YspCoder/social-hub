# WeCom Corp API adapter

Package `social-hub/adapters/wecom` implements the public server APIs for a
WeCom self-built application.

Implemented contracts:

- Corp `access_token` retrieval, five-minute early refresh, optional
  `socialhub.TokenStore` persistence, and invalidation after token errors;
- member profile lookup through the common `Fetcher`;
- common text `Messenger` sends;
- typed text, Markdown, image, voice, video, and file application messages;
- member, department, tag, default, and `@all` recipient selection, including
  partial-delivery diagnostics;
- image, AMR voice, MP4 video, and ordinary-file temporary media through the
  common `MediaUploader`;
- encrypted callback URL challenge handling, SHA-1 signature verification,
  AES-256-CBC decryption, CorpID validation, and XML event decoding.

The common `Publisher` and `Reactor` are intentionally unavailable. A WeCom
application message is not a social post, and the self-built application API
does not expose a generic reaction contract.

## Configuration

`app_id` contains the WeCom CorpID. `secret_ref` contains the CorpSecret for
the self-built application identified by `settings.agent_id`.

```yaml
adapter: wecom/corp-api
product: corp-api
settings:
  base_url: https://qyapi.weixin.qq.com
accounts:
  - id: operations
    app_id: ww0123456789abcdef
    secret_ref: env://WECOM_CORP_SECRET
    webhook:
      token_ref: env://WECOM_WEBHOOK_TOKEN
      aes_key_ref: env://WECOM_ENCODING_AES_KEY
    settings:
      agent_id: 1000002
      default_user_ids: [alice, bob]
      default_party_ids: [2]
      default_tag_ids: [3]
```

Use `access_token_ref` instead of `secret_ref` when an external credential
service owns token retrieval and refresh. Secrets are always resolved through
`socialhub.SecretResolver`; do not place credential values directly in the
configuration file.

Webhook callbacks use WeCom's encrypted mode. Configure both
`webhook.token_ref` and `webhook.aes_key_ref`, route the GET challenge through
`socialhub.ChallengeHandler`, and call `Verify` before `Decode` for each POST
callback. Decryption rejects payloads whose embedded receive ID is not the
configured CorpID.

## Typed messages and media

Use `client.ApplicationMessages().SendApplicationMessage` for content that is
not representable by the common text messenger. `SendResult` preserves
`invaliduser`, `invalidparty`, `invalidtag`, and `unlicenseduser`; these fields
can report partial delivery even when WeCom returns `errcode: 0`.

Temporary media IDs expire after three days. The adapter enforces the
documented file contracts before upload: JPG/PNG up to 10 MB, AMR up to 2 MB,
MP4 up to 10 MB, and ordinary files up to 20 MB. Every upload must contain at
least six bytes.

API visibility, address-book fields, message recipients, and some operations
depend on the application's configured visibility range, permissions, WeCom
edition, and user licensing. The adapter surfaces documented permission and
license failures and does not fall back to cookies, scraping, browser
automation, private endpoints, or reverse-engineered protocols.

All current tests use deterministic local HTTP fixtures. The adapter has not
been validated with a real WeCom tenant.

Official documentation:

- <https://developer.work.weixin.qq.com/document/path/91039>
- <https://developer.work.weixin.qq.com/document/path/90196>
- <https://developer.work.weixin.qq.com/document/path/90236>
- <https://developer.work.weixin.qq.com/document/path/90253>
- <https://developer.work.weixin.qq.com/document/path/90930>
- <https://developer.work.weixin.qq.com/document/path/90313>

Last verified: 2026-08-01.
