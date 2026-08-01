# LinkedIn REST 202607 adapter

Package `social-hub/adapters/linkedin` targets LinkedIn's July 2026 Marketing
API version. Every `/rest` request carries `Linkedin-Version: 202607` and
`X-Restli-Protocol-Version: 2.0.0`; the adapter does not fall back to sunset
`ugcPosts` or Assets APIs.

Implemented contracts:

- OAuth 2.0 authorization-code exchange and partner-only programmatic refresh;
- OpenID Connect member profile reads through `/v2/userinfo`;
- Posts API text, existing media URN, multi-image, and reshare publication;
- post lookup, author pagination, status, and deletion;
- comment listing, top-level or nested comment creation, and LIKE reactions;
- Images API initialize, single-part binary upload, and status lookup;
- LinkedIn error, request ID, and `Retry-After` mapping.

The adapter intentionally does not expose messaging or event subscriptions.
Post reads and most organization operations require Community Management API
approval. `r_member_social` is restricted, and programmatic refresh tokens are
available only to selected partners. Record granted permissions in
`approval.scopes` to make missing access fail locally.

`DeleteComment` returns `ErrUnsupported`: LinkedIn requires both the root post
URN and actor in addition to the comment ID, while the common method accepts
only a comment ID. This avoids process-local hidden state that would fail after
a restart.

Example account settings:

```yaml
adapter: linkedin/rest-202607
accounts:
  - id: company
    client_id: "123456"
    secret_ref: env://LINKEDIN_CLIENT_SECRET
    access_token_ref: env://LINKEDIN_ACCESS_TOKEN
    settings:
      author_urn: "urn:li:organization:123456"
    approval:
      scopes:
        - openid
        - profile
        - r_organization_social
        - w_organization_social
```
