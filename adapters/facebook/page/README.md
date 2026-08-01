# Facebook Page adapter

Registry name: `facebook/page`

Implemented capabilities:

- Meta OAuth authorization-code exchange, long-lived user token exchange, and
  managed Page token discovery
- Page feed publication, retrieval, and deletion
- Page and post lookup, feed pagination, comments, replies, and likes
- Unpublished Page photo upload through the common media workflow
- `X-Hub-Signature-256` webhook verification, Page event decoding, and
  subscription challenge handling

The first version intentionally does not expose Messenger, Reels, or Page video
upload. These have separate permission and asynchronous publication workflows.

Official documentation: <https://developers.facebook.com/docs/pages-api>

Graph API version: `v26.0`. Last verified: 2026-08-01.
