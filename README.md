# social-hub

`social-hub` is a Go SDK that exposes capability-oriented adapters for social
media APIs. Applications import only the platform adapters they use and work
against the common interfaces in `pkg/socialhub`.

The project is under active development. The public API is not stable before
the first tagged alpha release.

## Implemented adapters

| Adapter | Capabilities | Status |
|---|---|---|
| `x/v2` | OAuth2 PKCE, posts, users/timelines, replies, reactions, media upload | Local contract tests |
| `facebook/page` | OAuth2, Page posts/feed, comments/likes, photo upload, webhooks | Local contract tests |
| `instagram/login-v26` | OAuth2, professional profiles/media/comments, container publishing, webhooks | Local contract tests |
| `linkedin/rest-202607` | OAuth2/OIDC, versioned posts, comments/reactions, image upload | Local contract tests |
| `tiktok/v2` | Login Kit OAuth2, Display API, typed Direct Post workflow, webhooks | Local contract tests |
| `youtube/data-v3` | Google OAuth2, channels/videos/comments/ratings, typed video upload | Local contract tests |
| `pinterest/v5` | OAuth2, owned Pins/account reads, typed board-aware Pin and video upload workflow | Local contract tests |
| `reddit/data-api` | OAuth2, profiles/submissions/comments, typed subreddit submission, human-initiated votes | Local contract tests |
| `snapchat/public-profile-v1` | OAuth2, typed read-only Public Profile discovery and Spotlight workflow | Local contract tests |
| `mastodon/rest` | Per-instance OAuth2, profiles/statuses/home timeline, media, favourites/boosts, instance discovery | Local contract tests |
| `bluesky/atproto` | Per-PDS sessions, profiles/posts/feeds/threads, repo records, blobs, likes/reposts | Local contract tests |
| `threads/api` | OAuth2, text/reply/quote publishing, remote media containers, replies, insights, discovery, moderation, reposts | Local contract tests |
| `twitch/helix` | OAuth2 user/app tokens, users/VODs, streams, channels, schedules, clips, chat, EventSub webhooks | Local contract tests |
| `whatsapp/cloud-v25` | User/system-user tokens, text/media/template messages, media lifecycle, business profiles, signed webhooks | Local contract tests |
| `tumblr/v2` | API-key/OAuth2, NPF posts, inline media, blogs/dashboard/tagged feeds, notes, likes/follows | Local contract tests |
| `line/messaging-api` | Channel tokens, typed push/reply/multicast messages, profiles, inbound content, quotas, signed webhooks | Local contract tests |
| `vk/v5.199` | User/community/service tokens, walls/reposts, profiles, photos, comments/likes, messages, Callback API | Local contract tests |
| `telegram/bot-api` | Bot messages, channel text posts, typed media sends, webhook registration/verification | Local contract tests |
| `discord/v10` | Bot messages, users/channel history, replies/reactions, Gateway discovery | Local contract tests |
| `slack/web-api` | Bot/user tokens, channel messages/threads, reactions, external files, signed Events API | Local contract tests |
| `lark/openapi` | Feishu/Lark dual-region tokens, chats/threads, reactions, IM resources, encrypted events | Local contract tests |
| `microsoft-teams/graph-v1` | Global/national-cloud Graph v1.0, chat/channel threads, hosted content, reactions, basic change notifications | Local contract tests |
| `wechat/official-account` | App token, follower profiles, customer-service messages, drafts, materials, XML/AES webhooks | Local contract tests |
| `wecom/corp-api` | Corp tokens, members, typed application messages, temporary media, encrypted callbacks | Local contract tests |
| `weibo/v2` | OAuth2, posts/reposts, users/timelines, comments/likes, image upload | Local contract tests |
| `douyin/openapi` | OAuth2 user/client tokens, users/videos/comments, direct/chunked upload, webhooks | Local contract tests |
| `kuaishou/openapi` | OAuth2, user profiles, direct/fragment video upload, mandatory-cover publication | Local contract tests |
| `bilibili/open-platform` | OAuth2, v2 request signing, creator profiles, video/cover upload, archive management | Local contract tests |
| `xiaohongshu/share-js` | Approved-app token signing and media-only client Share JS handoff | Local contract tests |
| `zhihu/data-api` | Access Secret auth, site search, hot list, authorized-user content reads, OAuth2 | Local contract tests |

No adapter has been validated against a real platform account yet. The initial
development phase intentionally uses deterministic local HTTP fixtures only.

## Development

```powershell
go test ./...
go test -race ./...
go vet ./...
```

See [the implementation blueprint](docs/social-hub-blueprint.md) for the
supported-platform plan, architecture, and delivery milestones.
