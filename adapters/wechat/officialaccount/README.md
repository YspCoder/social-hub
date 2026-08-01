# WeChat Official Account adapter

Package `social-hub/adapters/wechat/officialaccount` implements the public
WeChat Official Account APIs used by the first social-hub release.

## Supported surface

- app `access_token` retrieval and in-memory refresh coordination
- follower profile lookup
- customer-service text and image messages
- temporary media through the common `MediaUploader`
- temporary and permanent material through `material.Provider`
- typed multi-article draft and free-publish workflow
- plaintext and AES XML webhook verification and decoding

The common `Publisher` is intentionally unavailable because a WeChat draft is
a list of rich-text articles and cannot be represented without loss by one
common post request. Use `(*officialaccount.Client).Drafts()` instead.

Capabilities depend on account type, verification, API permissions, customer
service conversation windows, and platform policy. This adapter does not fall
back to private APIs, cookies, or browser automation.

## Configuration

```yaml
adapter: wechat/official-account
product: official-account
settings:
  base_url: https://api.weixin.qq.com
accounts:
  - id: primary
    app_id: wx123
    secret_ref: env://WECHAT_APP_SECRET
    webhook:
      token_ref: env://WECHAT_WEBHOOK_TOKEN
      aes_key_ref: env://WECHAT_ENCODING_AES_KEY
```

For externally managed app tokens, set `access_token_ref` instead of
`secret_ref`. Secrets are resolved through `socialhub.SecretResolver` and must
not be stored inline in configuration.

Official documentation:

- <https://developers.weixin.qq.com/doc/offiaccount/Getting_Started/Overview.html>
- <https://developers.weixin.qq.com/doc/offiaccount/Asset_Management/New_temporary_materials.html>
- <https://developers.weixin.qq.com/doc/offiaccount/Draft_Box/Add_draft.html>
