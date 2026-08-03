# Instagram API with Instagram Login adapter

Package `social-hub/adapters/instagram` targets Graph API v26.0 through the
current Instagram Login product for professional accounts. Instagram Basic
Display API was retired on December 4, 2024 and is intentionally not exposed.

Implemented contracts:

- Instagram Login OAuth authorization-code exchange, long-lived token exchange,
  and long-lived token refresh;
- authorized professional-account profile and media reads;
- media detail, account media pagination, and comment pagination;
- comment replies and comment deletion;
- typed `ContainerWorkflow` for IMAGE, REELS, STORIES, and CAROUSEL creation,
  asynchronous status polling, and publication;
- common text `Messenger` sends and single-message lookup;
- typed `MessagingWorkflow` for text, image/audio/video HTTPS attachments,
  owned published-post sharing, and message react/unreact operations;
- typed `MessagingProfileWorkflow` for consented Instagram-scoped user
  profiles;
- `X-Hub-Signature-256` verification, subscription challenge handling, and
  normalized Instagram change and messaging events, including message echoes,
  reads, reactions, postbacks, referrals, and forward-compatible raw events;
- Graph error and rate-limit mapping with bounded response handling.

Content publication is deliberately not represented as the common
`Publisher`. Instagram first fetches an application-hosted HTTPS media URL into
an asynchronous container, then publishes the finished container. Use:

```text
ContainerWorkflow.Create -> ContainerWorkflow.Status -> ContainerWorkflow.Publish
```

The required permissions depend on the selected operations:

- `instagram_business_basic`
- `instagram_business_content_publish`
- `instagram_business_manage_comments`
- `instagram_business_manage_messages`

Instagram messaging is one-to-one and the Instagram user must initiate the
conversation. Standard automated replies are limited to Meta's 24-hour
messaging window. `GetMessage` can read details only for messages among the 20
most recent messages in the conversation. Human Agent, marketing/welcome
flows, generic templates, persistent menus, and attachment upload are outside
this adapter's first messaging release; no broader messaging entitlement is
implied.

Use an IGSID from a verified messaging webhook as `ConversationID`. Public
HTTPS attachments and owned published posts use `MessagingWorkflow`, because
the common `SendMessageRequest.MediaIDs` field cannot distinguish those two
Instagram wire formats. Profile picture URLs returned by
`GetMessagingUserProfile` expire after a few days and should not be treated as
durable media.

Apps normally need App Review and the account must be a supported professional
account. Configure granted permissions in `approval.scopes`; when that list is
present, the adapter fails locally with `ErrApprovalRequired` before issuing a
request for a missing permission.

Example account settings:

```yaml
adapter: instagram/login-v26
accounts:
  - id: brand
    client_id: "123456789"
    secret_ref: env://INSTAGRAM_APP_SECRET
    access_token_ref: env://INSTAGRAM_ACCESS_TOKEN
    settings:
      user_id: "17841400000000000"
    approval:
      scopes:
        - instagram_business_basic
        - instagram_business_content_publish
        - instagram_business_manage_comments
        - instagram_business_manage_messages
```
