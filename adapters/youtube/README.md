# YouTube Data API v3 adapter

Package `social-hub/adapters/youtube` targets the current YouTube Data API v3
and Google's web-server OAuth 2.0 flow.

Implemented contracts:

- channel and video lookup, channel video search pagination, and time filters;
- comment-thread listing with included replies;
- top-level comments, replies, deletion, and LIKE/none video ratings;
- typed `VideoUploadWorkflow` for metadata initialization, a complete resumable
  session PUT, status lookup, and video deletion;
- offline OAuth authorization-code exchange and refresh;
- Google error reason, request ID, quota, and `Retry-After` mapping.

The common `Publisher` and `MediaUploader` are unavailable because a YouTube
video resource is created together with its media upload. The first release
uses one complete PUT to a resumable session; it does not claim byte-range
resume after a process restart.

Unverified API projects created after July 28, 2020 have uploaded videos forced
to private until the project passes a YouTube audit. Set
`contains_synthetic_media` when realistic altered or synthetic content requires
disclosure. Search, insert, and mutation methods consume different quota units;
applications should monitor both daily quota and the separate search call cap.

Example account settings:

```yaml
adapter: youtube/data-v3
accounts:
  - id: channel
    client_id: "google-client-id"
    secret_ref: env://GOOGLE_CLIENT_SECRET
    access_token_ref: env://YOUTUBE_ACCESS_TOKEN
    settings:
      channel_id: "UCxxxxxxxx"
    approval:
      scopes:
        - https://www.googleapis.com/auth/youtube.readonly
        - https://www.googleapis.com/auth/youtube.upload
        - https://www.googleapis.com/auth/youtube.force-ssl
```
