# Kuaishou OpenAPI adapter

Package `social-hub/adapters/kuaishou` implements the public Kuaishou website-app
OAuth and content APIs documented by Kuaishou Open Platform.

Implemented contracts:

- OAuth 2.0 authorization-code exchange and rotating refresh tokens;
- authorized-user public profile (`user_info`);
- direct video upload for files smaller than 10 MiB;
- zero-based fragment upload and completion for larger files;
- bounded local cover staging and multipart video publication;
- typed `video.Workflow` with a mandatory `CoverID`.

Video publication requires the `user_video_publish` scope and a completed cover
image smaller than 10 MiB. `PublishStatus` reports the status returned by the
publication call; the initial adapter does not guess an undocumented video-info
endpoint for later polling.

The upload gateway is supplied by Kuaishou's `start_upload` response. Production
requests accept only HTTPS hosts under `*.gifshow.com` or `*.kuaishou.com`.
`upload_scheme: http` is restricted to explicitly configured loopback hosts and
exists only for local contract tests.

Post reads, comments, reactions, messages, webhooks, and deletion are explicit
unsupported operations in this first public-API integration. No cookie, private
endpoint, or browser automation is used.
