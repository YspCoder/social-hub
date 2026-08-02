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
| `vimeo/api-v3.4` | OAuth2, users/videos/feeds/comments/likes, typed TUS video upload | Local contract tests |
| `dailymotion/api-v2` | OAuth2 Client Credentials, managed profiles/videos/playlists, typed multipart video upload | Local contract tests |
| `flickr/services-api` | OAuth1 HMAC-SHA1, profiles/photos/comments/favorites, photosets, typed streaming upload | Local contract tests |
| `giphy/v1` | API-key GIF/Sticker discovery, Random ID, analytics pingbacks, typed streaming upload | Local contract tests |
| `imgur/v3` | Client-ID/OAuth2, profiles/images/comments/Gallery votes, albums, typed streaming upload | Local contract tests |
| `soundcloud/public-api-v1` | OAuth2.1 PKCE, users/tracks/comments/likes/reposts, typed activity feed and streaming track upload | Local contract tests |
| `mixcloud/api` | OAuth2 browser flow, users/Cloudcasts/comments/search, Favourite/Repost/Listen Later/Follow, typed streaming MP3 upload | Local contract tests |
| `spotify/web-api-v1` | OAuth2 PKCE, profiles/catalog/library/playlists, Premium-gated Spotify Connect playback | Local contract tests |
| `applemusic/api` | ES256 developer tokens, storefront/catalog/library/playlists, recently played history | Local contract tests |
| `lastfm/web-services-v2` | Signed browser auth, track/artist/album discovery, profiles/listening history, now playing, scrobbling, love state | Local contract tests |
| `musicbrainz/ws-v2` | Credential-free artist/release/recording/work metadata, typed lookup and browse, shared public-service request gate | Local contract tests; public WS/2 smoke |
| `listenbrainz/api-v1` | Public listening history, playing now, feedback and JSPF playlists; token-authorized scrobbling, feedback and deletion | Local contract tests; public API v1 smoke |
| `trakt/v2` | OAuth2 browser/device flows, movie/TV catalog, history/watchlist/ratings sync, scrobbling, comments | Local contract tests |
| `tmdb/v3` | Bearer/API-key app auth, movie/TV/person catalog, user sessions, favorites/watchlist/ratings | Local contract tests |
| `letterboxd/api-v0` | OAuth2/OIDC, film search/catalog, members/activity/watchlists, diary/review log entries, likes/ratings/watched state | Local contract tests; access-request beta |
| `myanimelist/v2` | OAuth2 plain PKCE, anime/manga catalog and rankings, profiles, personal list reads and mutations | Local contract tests; API v2 beta |
| `anilist/graphql-v2` | OAuth2, anime/manga discovery, profiles, media-list tracking, text/list activities, replies and likes | Local contract tests; public GraphQL smoke |
| `kitsu/edge` | Caller-managed OAuth2 tokens, JSON:API anime/manga discovery, profiles, library tracking, posts and comments | Local contract tests; public edge API smoke |
| `simkl/v1` | OAuth2 confidential/PKCE/PIN flows, movie/TV/anime catalog and trending, incremental library sync, batched history/ratings, scrobbling | Local contract tests; public CDN smoke |
| `tvmaze/public-api` | Credential-free show/episode/season catalog, broadcast and web schedules, people/credits, incremental updates | Local contract tests; public API smoke |
| `pinterest/v5` | OAuth2, owned Pins/account reads, typed board-aware Pin and video upload workflow | Local contract tests |
| `reddit/data-api` | OAuth2, profiles/submissions/comments, typed subreddit submission, human-initiated votes | Local contract tests |
| `stackexchange/api-v2.3` | API key/OAuth2 PKCE, users/questions/answers/comments, typed Q&A workflow, human-initiated votes | Local contract tests |
| `hackernews/firebase-v0` | Credential-free users/items, six story feeds, direct comment trees, max item and incremental updates | Local contract tests; public API v0 smoke |
| `discourse/rest-api` | Per-instance API keys, users/Posts/replies, typed Topics/private messages, uploads, likes, signed webhooks | Local contract tests |
| `forem/api-v1` | Per-instance API keys, users/Articles/threaded comments, typed Article publishing and reactions | Local contract tests |
| `lemmy/api-v3` | Per-instance JWTs, people/Posts/comments, typed community publishing/votes/private messages, Pictrs image upload | Local contract tests |
| `nostr/nip-01` | Multi-relay signed events, profiles/notes/threads, NIP-09 deletion, NIP-18 reposts, NIP-25 reactions, NIP-92 media metadata | Local WebSocket contract tests |
| `dribbble/v2` | OAuth2 Publishing API, owned profiles/Shots, typed image publishing, Projects and Attachments | Local contract tests |
| `deviantart/api-v1-20240701` | OAuth2.1 PKCE, users/Deviations/galleries/comments, text Status publishing, favourites | Local contract tests |
| `snapchat/public-profile-v1` | OAuth2, typed read-only Public Profile discovery and Spotlight workflow | Local contract tests |
| `mastodon/rest` | Per-instance OAuth2, profiles/statuses/home timeline, media, favourites/boosts, instance discovery | Local contract tests |
| `matrix/client-server-v1.19` | Per-homeserver bearer tokens, profiles/room events, threads/reactions, raw media upload, incremental sync | Local contract tests |
| `misskey/api` | Per-instance tokens/MiAuth, users/Notes/home timeline, Drive media, emoji reactions, instance discovery | Local contract tests |
| `bluesky/atproto` | Per-PDS sessions, profiles/posts/feeds/threads, repo records, blobs, likes/reposts | Local contract tests |
| `threads/api` | OAuth2, text/reply/quote publishing, remote media containers, replies, insights, discovery, moderation, reposts | Local contract tests |
| `twitch/helix` | OAuth2 user/app tokens, users/VODs, streams, channels, schedules, clips, chat, EventSub webhooks | Local contract tests |
| `kick/public-api-v2` | OAuth2.1 user/app tokens, typed channels/V2 livestream discovery/chat, event subscriptions, RSA webhooks | Local contract tests |
| `peertube/rest-v1` | Per-instance OAuth2 password/refresh grants, accounts/videos/comments, ratings, channels, typed streaming video upload | Local contract tests |
| `whatsapp/cloud-v25` | User/system-user tokens, text/media/template messages, media lifecycle, business profiles, signed webhooks | Local contract tests |
| `tumblr/v2` | API-key/OAuth2, NPF posts, inline media, blogs/dashboard/tagged feeds, notes, likes/follows | Local contract tests |
| `wordpress.com/rest-v1.1` | OAuth2, WordPress.com/Jetpack sites, Posts, Comments/Likes, typed publishing and streaming Media | Local contract tests |
| `patreon/api-v2` | OAuth2 refresh, creator identity/Campaigns/Posts/Members, signed webhooks | Local contract tests |
| `line/messaging-api` | Channel tokens, typed push/reply/multicast messages, profiles, inbound content, quotas, signed webhooks | Local contract tests |
| `kakao/login-talk-rest` | OAuth2, authorized-user profiles, approved friend discovery, self/friend Talk messages | Local contract tests |
| `vk/v5.199` | User/community/service tokens, walls/reposts, profiles, photos, comments/likes, messages, Callback API | Local contract tests |
| `telegram/bot-api` | Bot messages, channel text posts, typed media sends, webhook registration/verification | Local contract tests |
| `discord/v10` | Bot messages, users/channel history, replies/reactions, Gateway discovery | Local contract tests |
| `slack/web-api` | Bot/user tokens, channel messages/threads, reactions, external files, signed Events API | Local contract tests |
| `lark/openapi` | Feishu/Lark dual-region tokens, chats/threads, reactions, IM resources, encrypted events | Local contract tests |
| `microsoft-teams/graph-v1` | Global/national-cloud Graph v1.0, chat/channel threads, hosted content, reactions, basic change notifications | Local contract tests |
| `wechat/official-account` | App token, follower profiles, customer-service messages, drafts, materials, XML/AES webhooks | Local contract tests |
| `wecom/corp-api` | Corp tokens, members, typed application messages, temporary media, encrypted callbacks | Local contract tests |
| `dingtalk/openapi-v1.0` | Corp-scoped app tokens, UnionID contact reads, typed application-robot group and OTO batch messages | Local contract tests |
| `qq/bot-api` | App tokens, C2C/group/channel messages, scene-bound URL media, Ed25519 callbacks | Local contract tests |
| `weibo/v2` | OAuth2, posts/reposts, users/timelines, comments/likes, image upload | Local contract tests |
| `douyin/openapi` | OAuth2 user/client tokens, users/videos/comments, direct/chunked upload, webhooks | Local contract tests |
| `toutiao/openapi` | OAuth2 user/client tokens, authorized profiles, owned videos, direct/chunked upload | Local contract tests |
| `xigua/openapi` | OAuth2 user/client tokens, authorized profiles, owned videos, 16 GiB multipart workflow | Local contract tests |
| `kuaishou/openapi` | OAuth2, user profiles, direct/fragment video upload, mandatory-cover publication | Local contract tests |
| `bilibili/open-platform` | OAuth2, v2 request signing, creator profiles, video/cover upload, archive management | Local contract tests |
| `xiaohongshu/share-js` | Approved-app token signing and media-only client Share JS handoff | Local contract tests |
| `zhihu/data-api` | Access Secret auth, site search, hot list, authorized-user content reads, OAuth2 | Local contract tests |

No credentialed adapter has been validated against a real platform account yet.
Deterministic local fixtures remain the required baseline; opt-in public read
smoke tests are identified in the table above.

## Development

```powershell
go test ./...
go test -race ./...
go vet ./...
```

See [the implementation blueprint](docs/social-hub-blueprint.md) for the
supported-platform plan, architecture, and delivery milestones.
