# Zhihu Data API adapter

Package `social-hub/adapters/zhihu` implements the documented Zhihu Data Open
Platform v1 APIs.

Implemented contracts:

- Bearer `Access Secret` authentication with a second-level
  `X-Request-Timestamp`;
- typed Zhihu site search and current hot-list workflows;
- the current or OAuth-authorized user's public-range content list, mapped to
  the common `Fetcher.ListPosts` model;
- OAuth 2.0 authorization-code URL generation and token exchange for separately
  approved third-party applications;
- documented business-error mapping for parameter, authentication, frequency,
  quota, and internal failures.

`access_token_ref` must resolve to the Data Open Platform Access Secret. An
optional account `settings.oauth_token_ref` resolves to a user's OAuth access
token and is sent only as `X-OAuth-Token` on user-data requests. These
credentials are not interchangeable.

Data API access is currently in invite testing. Configure
`account.settings.approved: true` only after an Access Secret has been granted;
otherwise data operations return `ApprovalRequired` without making a request.
Third-party OAuth credentials require a separate application by email. The
current OAuth documentation defines neither refresh-token rotation nor a
`state` parameter, so this adapter does not invent either contract.

The official Data Open Platform does not document content publication, media
upload, reactions, comment writes, private messages, or webhooks. Single-content
detail, arbitrary user detail, and pageable comment reads are also explicit
unsupported operations. No cookie, private community endpoint, scraping, or
browser automation is used.
