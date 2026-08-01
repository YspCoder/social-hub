package page

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the Facebook Page capability interfaces.
type Client struct {
	accountID     socialhub.AccountID
	pageID        string
	transport     *transport.Client
	webhookSecret string
	webhookToken  string

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
}

func (c *Client) Platform() socialhub.Platform { return "facebook" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhookSupported := c.webhookSecret != ""
	webhookReason := ""
	if !webhookSupported {
		webhookReason = "webhook.secret_ref is not configured"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"pages_manage_posts", "pages_read_engagement"}, DocURL: docURL},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"pages_read_engagement", "pages_read_user_content"}, DocURL: docURL},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"pages_manage_posts"}, Reason: "initial adapter supports unpublished Page photos only", DocURL: docURL},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"pages_manage_engagement"}, DocURL: docURL},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: webhookSupported, Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: "https://developers.facebook.com/docs/graph-api/webhooks"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Messenger Platform is not part of the initial Page adapter"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return c, true }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.webhookSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	form := url.Values{}
	if input.Text != nil {
		form.Set("message", *input.Text)
	}
	for index, mediaID := range input.MediaIDs {
		attached, _ := json.Marshal(map[string]string{"media_fbid": mediaID})
		form.Set("attached_media["+strconv.Itoa(index)+"]", string(attached))
	}
	var response idResponse
	if err := c.form(ctx, http.MethodPost, "/"+url.PathEscape(c.pageID)+"/feed", form, &response, options...); err != nil {
		return nil, err
	}
	return &socialhub.Post{Platform: "facebook", AccountID: c.accountID, ID: response.ID, Text: input.Text, Media: mediaReferences(input.MediaIDs)}, nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return &socialhub.PublishStatus{ID: post.ID, State: socialhub.PublishStatePublished, UpdatedAt: post.CreatedAt}, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if postID == "" {
		return invalidArgument("delete_post", "post ID is required")
	}
	var response successResponse
	if err := c.form(ctx, http.MethodDelete, "/"+url.PathEscape(postID), nil, &response, options...); err != nil {
		return err
	}
	if !response.Success {
		return platformError("delete_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) GetUser(ctx context.Context, pageID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if pageID == "" {
		pageID = c.pageID
	}
	query := url.Values{"fields": {"id,name,link,picture.type(large)"}}
	var response graphPage
	if err := c.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(pageID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	return mapPage(c.accountID, response), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if postID == "" {
		return nil, invalidArgument("get_post", "post ID is required")
	}
	query := url.Values{"fields": {postFieldList}}
	var response graphPost
	if err := c.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(postID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	return mapPost(c.accountID, response), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	pageID := input.UserID
	if pageID == "" {
		pageID = c.pageID
	}
	query := url.Values{"fields": {postFieldList}}
	setPaging(query, input.Cursor, input.MaxResults)
	if input.StartTime != nil {
		query.Set("since", strconv.FormatInt(input.StartTime.Unix(), 10))
	}
	if input.EndTime != nil {
		query.Set("until", strconv.FormatInt(input.EndTime.Unix(), 10))
	}
	var response graphPosts
	if err := c.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(pageID)+"/feed", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapPostPage(c.accountID, response), nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if input.PostID == "" {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID is required")
	}
	query := url.Values{"fields": {"id,message,created_time,from,parent"}}
	setPaging(query, input.Cursor, input.MaxResults)
	var response graphComments
	if err := c.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(input.PostID)+"/comments", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	return mapCommentPage(c.accountID, input.PostID, response), nil
}

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.TargetID == "" {
		return invalidArgument("react", "target ID is required")
	}
	if input.Kind != socialhub.ReactionLike {
		return socialhub.UnsupportedError("facebook", socialhub.CapReact)
	}
	var response successResponse
	if err := c.form(ctx, http.MethodPost, "/"+url.PathEscape(input.TargetID)+"/likes", nil, &response, options...); err != nil {
		return err
	}
	if !response.Success {
		return platformError("react", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.TargetID == "" {
		return invalidArgument("remove_reaction", "target ID is required")
	}
	if input.Kind != socialhub.ReactionLike {
		return socialhub.UnsupportedError("facebook", socialhub.CapReact)
	}
	var response successResponse
	if err := c.form(ctx, http.MethodDelete, "/"+url.PathEscape(input.TargetID)+"/likes", nil, &response, options...); err != nil {
		return err
	}
	if !response.Success {
		return platformError("remove_reaction", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if input.PostID == "" || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "post ID and text are required")
	}
	targetID := input.PostID
	if input.ParentID != nil && *input.ParentID != "" {
		targetID = *input.ParentID
	}
	var response idResponse
	if err := c.form(ctx, http.MethodPost, "/"+url.PathEscape(targetID)+"/comments", url.Values{"message": {input.Text}}, &response, options...); err != nil {
		return nil, err
	}
	return &socialhub.Comment{Platform: "facebook", AccountID: c.accountID, ID: response.ID, PostID: input.PostID, ParentID: input.ParentID, Text: input.Text}, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	if commentID == "" {
		return invalidArgument("delete_comment", "comment ID is required")
	}
	var response successResponse
	if err := c.form(ctx, http.MethodDelete, "/"+url.PathEscape(commentID), nil, &response, options...); err != nil {
		return err
	}
	if !response.Success {
		return platformError("delete_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) form(ctx context.Context, method, path string, form url.Values, output any, options ...socialhub.CallOption) error {
	encoded := ""
	if form != nil {
		encoded = form.Encode()
	}
	request, err := c.transport.NewRequest(ctx, method, path, nil, strings.NewReader(encoded), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.transport.Do(request, output)
}

func setPaging(query url.Values, cursor string, limit int) {
	if cursor != "" {
		query.Set("after", cursor)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
}

func mediaReferences(ids []string) []socialhub.Media {
	media := make([]socialhub.Media, 0, len(ids))
	for _, id := range ids {
		media = append(media, socialhub.Media{ID: id, State: socialhub.MediaStateReady})
	}
	return media
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)

const postFieldList = "id,message,created_time,permalink_url,from,attachments{media,type,url,target}"
