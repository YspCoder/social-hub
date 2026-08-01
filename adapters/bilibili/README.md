# Bilibili Open Platform adapter

Package `social-hub/adapters/bilibili` implements the documented Bilibili Open
Platform APIs for approved applications and authorized creators.

Implemented contracts:

- OAuth 2.0 PC web authorization, code exchange, and single-use refresh-token
  rotation;
- v2 request signing with body MD5 and HMAC-SHA256;
- authorized-user public profile (`USER_INFO`);
- archive initialization and single-file video upload up to 100 MB;
- JPEG/PNG cover upload up to 5 MB;
- archive submission, status/detail lookup, list pagination, and deletion;
- typed `SubmissionWorkflow` for `tid`, tags, copyright, source, and reprint
  controls, plus `video.Workflow` backed by account defaults.

`expires_in` in the OAuth response is an absolute UTC timestamp, not a relative
duration. Each refresh token can be used only once, so callers must persist the
new token pair atomically.

The public `Publisher` requires `default_tid` and `default_tags` in account
settings. Use `SubmissionWorkflow` when metadata varies per archive. Files above
100 MB require Bilibili's separate fragment-upload protocol and return an
explicit `Unsupported` error in the initial adapter.

Access requires Bilibili Open Platform identity verification, application
approval, creator authorization, and the relevant `USER_INFO` / `ARC_BASE`
scopes. Comments, reactions, private messages, and separately approved event
push products are not exposed by this adapter. No cookie, private main-site API,
or browser automation is used.
