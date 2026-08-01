# Reddit Data API adapter

Package `social-hub/adapters/reddit` targets Reddit's official, unversioned
OAuth Data API at `oauth.reddit.com`.

Implemented contracts:

- authorized profile lookup, own submitted listing pagination, submission
  lookup, and included comment-tree flattening;
- typed `SubmissionWorkflow` for subreddit-aware self/link submission and
  deletion;
- comments, replies, deletion, and human-initiated upvote/unvote;
- web-app OAuth authorization-code exchange with permanent refresh tokens;
- mandatory identifiable `User-Agent`, Reddit API errors, and a thread-safe
  snapshot of `x-ratelimit-used`, `x-ratelimit-remaining`, and
  `x-ratelimit-reset`.

The common `Publisher` is unavailable because `CreatePostRequest` cannot carry
Reddit's required subreddit, title, and submission kind. `ListComments` returns
the comment tree included in the response; the first release does not claim
automatic `/api/morechildren` expansion. Vote calls must proxy a human action
one-for-one and must not be used for automated voting or amplification.

Reddit requires approved OAuth access. Eligible free access currently permits
100 queries per minute per OAuth client ID, averaged over an approximately
10-minute window. Commercial use, research beyond limits, or other unpermitted
uses require a separate agreement with Reddit. Treat these as policy and
runtime constraints, not constants in a limiter.

Example account settings:

```yaml
adapter: reddit/data-api
settings:
  user_agent: "server:com.example.socialhub:v0.1.0 (by /u/api_contact)"
accounts:
  - id: redditor
    client_id: "reddit-client-id"
    secret_ref: env://REDDIT_CLIENT_SECRET
    access_token_ref: env://REDDIT_ACCESS_TOKEN
    settings:
      user_id: "t2_abc123"
      username: "api_contact"
    approval:
      scopes: [identity, read, history, submit, edit, vote]
```
