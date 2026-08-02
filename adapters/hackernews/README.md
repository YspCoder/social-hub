# Hacker News Firebase API adapter

`hackernews/firebase-v0` integrates the official, public Hacker News API v0.
It requires no API key, OAuth flow, commercial authorization, or enterprise
qualification.

## Supported surface

| Surface | Operations |
|---|---|
| Common `Fetcher` | Public profiles, stories/jobs/polls, configured feed, user submissions filtered to posts, direct post comments |
| `ItemWorkflow` | Raw item lookup, direct-child pagination, current maximum item ID |
| `FeedWorkflow` | Top, new, best, Ask HN, Show HN, and job feeds |
| `UserWorkflow` | Full profile including karma, HTML about text, and submitted item IDs |
| `UpdatesWorkflow` | Recently changed item IDs and profile names |

The official API is read-only. Publishing, voting, media uploads, direct
messages, and webhooks are therefore declared unsupported instead of being
implemented through undocumented website endpoints.

## Configuration

```yaml
version: 1
platforms:
  - adapter: hackernews/firebase-v0
    settings:
      user_agent: social-hub/my-service (+https://example.com/contact)
    accounts:
      - id: public
        settings:
          default_feed: topstories
```

`default_feed` controls common `ListPosts` calls and accepts `topstories`,
`newstories`, `beststories`, `askstories`, `showstories`, or `jobstories`.
It defaults to `topstories`. Credentials, token stores, approval declarations,
and webhook secrets are rejected because they are not part of this API.

## Usage

```go
feed, err := client.FeedWorkflow().ListFeed(ctx, hackernews.FeedRequest{
    Feed:       hackernews.FeedBest,
    MaxResults: 25,
})

comments, err := client.ListComments(ctx, socialhub.ListCommentsRequest{
    PostID:     "8863",
    MaxResults: 20,
})
```

Feed and child cursors are stable decimal offsets into the ID snapshot returned
for that call. A later call can observe a changed feed, so consumers requiring a
strict snapshot should persist the raw IDs themselves.

Hacker News returns HTML in item `title`/`text` and user `about` fields. The
adapter preserves it rather than silently sanitizing or converting it. Common
posts join a non-empty title and body with a blank line; raw values remain in
`hackernews.item` extensions. Deleted and dead posts map to `visibility=removed`.

The common user-post listing filters the profile's mixed `submitted` list,
which contains stories, polls, jobs, and comments. It scans at most 500 IDs per
page and may therefore return an empty page with `has_more=true`; continue with
`next_cursor` to avoid unbounded fan-out. Common comment listing returns direct
children only. Use `ItemWorkflow.ListChildren` recursively when the entire
comment tree is required.

Firebase represents a missing item or profile as HTTP 200 with JSON `null`; the
adapter maps that response to `socialhub.ErrNotFound`. HTTP 429 and 5xx responses
remain retryable and preserve bounded `Retry-After` and request identifiers.
Redirects are refused.

## Rate behavior and live verification

The official v0 documentation currently states that there is no rate limit.
The adapter does not invent a fixed throttle, but callers should cache feed ID
lists, page conservatively, and avoid repeatedly walking large comment trees.

Run the opt-in public smoke test with:

```powershell
$env:HACKERNEWS_LIVE_TEST='1'
go test ./adapters/hackernews -run TestPublicLiveSmoke -count=1
```

## GitHub assessment

| Project | Assessment |
|---|---|
| [`HackerNews/API`](https://github.com/HackerNews/API) | Authoritative MIT-licensed v0 documentation and field contract; used as the primary source. |
| [`alexferrari88/GoHN`](https://github.com/alexferrari88/GoHN) | The most complete current Go wrapper found, with feeds, users, updates, and concurrent comment traversal. Used to cross-check models and workflow coverage. |
| [`peterhellberg/hn`](https://github.com/peterhellberg/hn) | Small established Go wrapper used to cross-check endpoint paths and fallback discussion URLs; its last code update is older. |

No upstream client dependency was added. For this adapter the API calls are
small, while an external wrapper would bypass `social-hub`'s bounded transport,
call options, redirect policy, and common error model. A future full-tree helper
can still adopt GoHN's bounded-worker approach without changing the public raw
item contract.
