package weibo

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the supported Weibo capability interfaces.
type Client struct {
	accountID socialhub.AccountID
	transport *transport.Client
	clock     socialhub.Clock
	sourceIP  string

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
}

func (c *Client) Platform() socialhub.Platform { return "weibo" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	writeReason := "write APIs require platform approval and account.settings.source_ip"
	if c.sourceIP != "" {
		writeReason = "write APIs require platform approval; source_ip is configured"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: writeReason, DocURL: "https://open.weibo.com/wiki/2/statuses/update"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "user timelines and comments are limited to data visible to the authorized application", DocURL: "https://open.weibo.com/wiki/2/statuses/user_timeline"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "initial common uploader supports one image per upload_pic call", DocURL: "https://open.weibo.com/wiki/2/statuses/upload_pic"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "likes and comments are supported; reposts are independent posts created through Publisher", DocURL: "https://open.weibo.com/wiki/2/attitudes/create"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "no general public direct-message API is exposed by this adapter"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Weibo realtime callbacks require a separate approved product"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return c, true }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (c *Client) Close() error { return nil }

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if input.ReplyToID != nil {
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "weibo", Product: "open-api", Op: "publish", PlatformMessage: "the public statuses API does not support reply posts; use comments"}
	}
	if err := c.requireSourceIP("publish"); err != nil {
		return nil, err
	}
	form := url.Values{"rip": {c.sourceIP}}
	path := "/2/statuses/update.json"
	if input.QuotePostID != nil {
		if *input.QuotePostID == "" || len(input.MediaIDs) > 0 {
			return nil, invalidArgument("publish", "a repost requires a target ID and cannot include media IDs")
		}
		path = "/2/statuses/repost.json"
		form.Set("id", *input.QuotePostID)
		if input.Text != nil && *input.Text != "" {
			form.Set("status", *input.Text)
		}
	} else {
		if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
			return nil, invalidArgument("publish", "text is required")
		}
		form.Set("status", *input.Text)
		if utf8.RuneCountInString(*input.Text) > 140 {
			form.Set("is_longtext", "1")
		}
		if len(input.MediaIDs) > 9 {
			return nil, invalidArgument("publish", "Weibo supports at most nine image IDs")
		}
		if len(input.MediaIDs) > 0 {
			path = "/2/statuses/upload_url_text.json"
			form.Set("pic_id", strings.Join(input.MediaIDs, ","))
		}
		if input.Visibility != nil {
			switch *input.Visibility {
			case "public":
				form.Set("visible", "0")
			case "private":
				form.Set("visible", "1")
			default:
				return nil, invalidArgument("publish", "visibility must be public or private")
			}
		}
	}
	var response weiboStatus
	if err := c.form(ctx, path, form, &response, options...); err != nil {
		return nil, err
	}
	if err := response.APIError.Err("publish", http.StatusOK, nil); err != nil {
		return nil, err
	}
	post := mapStatus(c.accountID, response, c.clock.Now())
	if input.QuotePostID != nil && !hasRelation(post.Relations, socialhub.RelationRepost, *input.QuotePostID) {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationRepost, PostID: *input.QuotePostID})
	}
	return post, nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	if postID == "" {
		return nil, invalidArgument("publish_status", "post ID is required")
	}
	if _, err := c.GetPost(ctx, postID, options...); err != nil {
		return nil, err
	}
	return &socialhub.PublishStatus{ID: postID, State: socialhub.PublishStatePublished}, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if postID == "" {
		return invalidArgument("delete_post", "post ID is required")
	}
	var response weiboStatus
	if err := c.form(ctx, "/2/statuses/destroy.json", url.Values{"id": {postID}}, &response, options...); err != nil {
		return err
	}
	return response.APIError.Err("delete_post", http.StatusOK, nil)
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "" {
		return nil, invalidArgument("get_user", "user ID is required")
	}
	var response weiboUser
	if err := c.transport.JSON(ctx, http.MethodGet, "/2/users/show.json", url.Values{"uid": {userID}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := response.APIError.Err("get_user", http.StatusOK, nil); err != nil {
		return nil, err
	}
	return mapUser(c.accountID, response), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if postID == "" {
		return nil, invalidArgument("get_post", "post ID is required")
	}
	var response weiboStatus
	if err := c.transport.JSON(ctx, http.MethodGet, "/2/statuses/show.json", url.Values{"id": {postID}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := response.APIError.Err("get_post", http.StatusOK, nil); err != nil {
		return nil, err
	}
	return mapStatus(c.accountID, response, c.clock.Now()), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "weibo", Product: "open-api", Op: "list_posts", PlatformMessage: "Weibo user_timeline does not accept time-range filters"}
	}
	page, count, err := pageParameters(input.Cursor, input.MaxResults, 20, 100)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"page": {strconv.Itoa(page)}, "count": {strconv.Itoa(count)}}
	if input.UserID != "" {
		query.Set("uid", input.UserID)
	}
	var response statusListResponse
	if err := c.transport.JSON(ctx, http.MethodGet, "/2/statuses/user_timeline.json", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if err := response.APIError.Err("list_posts", http.StatusOK, nil); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Statuses))
	observedAt := c.clock.Now()
	for _, status := range response.Statuses {
		items = append(items, *mapStatus(c.accountID, status, observedAt))
	}
	return paged(items, page, count, response.TotalNumber), nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if input.PostID == "" {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID is required")
	}
	page, count, err := pageParameters(input.Cursor, input.MaxResults, 50, 200)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query := url.Values{"id": {input.PostID}, "page": {strconv.Itoa(page)}, "count": {strconv.Itoa(count)}}
	var response commentListResponse
	if err := c.transport.JSON(ctx, http.MethodGet, "/2/comments/show.json", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if err := response.APIError.Err("list_comments", http.StatusOK, nil); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Comments))
	for _, comment := range response.Comments {
		items = append(items, mapComment(c.accountID, input.PostID, comment))
	}
	return paged(items, page, count, response.TotalNumber), nil
}

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, false, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, true, options...)
}

func (c *Client) setReaction(ctx context.Context, input socialhub.ReactionRequest, remove bool, options ...socialhub.CallOption) error {
	if input.TargetID == "" {
		return invalidArgument("react", "target ID is required")
	}
	if input.Kind != socialhub.ReactionLike {
		return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "weibo", Product: "open-api", Op: "react", PlatformMessage: "a Weibo repost is an independent post; create it through Publisher"}
	}
	path := "/2/attitudes/create.json"
	if remove {
		path = "/2/attitudes/destroy.json"
	}
	var response struct{ APIError }
	if err := c.form(ctx, path, url.Values{"id": {input.TargetID}}, &response, options...); err != nil {
		return err
	}
	return response.APIError.Err("react", http.StatusOK, nil)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if input.PostID == "" || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "post ID and text are required")
	}
	if err := c.requireSourceIP("comment"); err != nil {
		return nil, err
	}
	path := "/2/comments/create.json"
	form := url.Values{"id": {input.PostID}, "comment": {input.Text}, "rip": {c.sourceIP}}
	if input.ParentID != nil {
		if *input.ParentID == "" {
			return nil, invalidArgument("comment", "parent comment ID must not be empty")
		}
		path = "/2/comments/reply.json"
		form.Set("cid", *input.ParentID)
	}
	var response weiboComment
	if err := c.form(ctx, path, form, &response, options...); err != nil {
		return nil, err
	}
	if err := response.APIError.Err("comment", http.StatusOK, nil); err != nil {
		return nil, err
	}
	comment := mapComment(c.accountID, input.PostID, response)
	if comment.ParentID == nil && input.ParentID != nil {
		comment.ParentID = stringPointer(*input.ParentID)
	}
	return &comment, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	if commentID == "" {
		return invalidArgument("delete_comment", "comment ID is required")
	}
	var response weiboComment
	if err := c.form(ctx, "/2/comments/destroy.json", url.Values{"cid": {commentID}}, &response, options...); err != nil {
		return err
	}
	return response.APIError.Err("delete_comment", http.StatusOK, nil)
}

func (c *Client) SendMessage(context.Context, socialhub.SendMessageRequest, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, socialhub.UnsupportedError("weibo", socialhub.CapMessage)
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, socialhub.UnsupportedError("weibo", socialhub.CapMessage)
}

func (c *Client) form(ctx context.Context, path string, form url.Values, output any, options ...socialhub.CallOption) error {
	request, err := c.transport.NewRequest(ctx, http.MethodPost, path, nil, strings.NewReader(form.Encode()), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.transport.Do(request, output)
}

func (c *Client) requireSourceIP(operation string) error {
	if c.sourceIP == "" {
		return invalidArgument(operation, "account.settings.source_ip is required by Weibo write APIs")
	}
	return nil
}

func pageParameters(cursor string, requested, fallback, maximum int) (int, int, error) {
	page := 1
	var err error
	if cursor != "" {
		page, err = strconv.Atoi(cursor)
		if err != nil || page < 1 {
			return 0, 0, invalidArgument("pagination", "cursor must be a positive page number")
		}
	}
	count := requested
	if count <= 0 {
		count = fallback
	}
	if count > maximum {
		count = maximum
	}
	return page, count, nil
}

func paged[T any](items []T, page, count, total int) socialhub.Page[T] {
	var next, previous *string
	if page*count < total {
		value := strconv.Itoa(page + 1)
		next = &value
	}
	if page > 1 {
		value := strconv.Itoa(page - 1)
		previous = &value
	}
	return socialhub.Page[T]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}
}

func hasRelation(relations []socialhub.PostRelation, kind socialhub.RelationType, postID string) bool {
	for _, relation := range relations {
		if relation.Type == kind && relation.PostID == postID {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
