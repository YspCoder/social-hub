# Xiaohongshu Share JS adapter

Package `social-hub/adapters/xiaohongshu` implements the documented server
handoff for Xiaohongshu's Share JS SDK 1.0.1.

Implemented contracts:

- application access-token acquisition with the documented SHA-256 signature;
- serialized, cached access tokens whose `expires_in` value is treated as an
  absolute millisecond timestamp;
- media-only `xhs.share` handoff signatures for normal image and video shares;
- an explicit `client_share` capability and typed `ShareWorkflow`.

The SDK no longer permits automatic title, body, or topic prefill. Media must
already be available at application-controlled HTTPS URLs, and the returned
payload must be passed to the official browser SDK for an interactive share.
This adapter does not implement server-side note publication, note/feed reads,
media hosting, comments, messages, or webhooks.

New Share Open Platform onboarding is currently shown as paused in the official
documentation. Existing approved applications must set account
`settings.approved: true`; otherwise `Prepare` returns `ApprovalRequired`.
Requesting a new application token invalidates the previous token, so production
deployments should share an encrypted `TokenStore` and avoid independent token
refreshers. A static externally managed token can be provided with
`access_token_ref`.

No cookie, private API, note-publishing automation, or browser automation is
used.
