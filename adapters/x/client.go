package x

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the supported X API v2 capability interfaces.
type Client struct {
	accountID socialhub.AccountID
	transport *transport.Client
}

func (c *Client) Platform() socialhub.Platform { return "x" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"tweet.read", "tweet.write", "users.read"}, DocURL: "https://docs.x.com/x-api/posts/manage-tweets/introduction"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"tweet.read", "users.read"}, DocURL: "https://docs.x.com/x-api/posts/lookup/introduction"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"media.write", "tweet.write"}, DocURL: "https://docs.x.com/x-api/media/introduction"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"like.write", "tweet.write", "users.read"}, DocURL: "https://docs.x.com/x-api/likes/introduction"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "not implemented in the initial adapter"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "X Activity webhook is a separate product workflow"},
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
	if err := input.Validate(); err != nil {
		return nil, &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "x", Op: "publish", Cause: err}
	}
	body := createPostRequest{Text: input.Text, QuoteTweetID: input.QuotePostID}
	if len(input.MediaIDs) > 0 {
		body.Media = &createPostMedia{MediaIDs: input.MediaIDs}
	}
	if input.ReplyToID != nil {
		body.Reply = &createPostReply{InReplyToTweetID: *input.ReplyToID}
	}
	var response dataResponse[xPost]
	if err := c.transport.JSON(ctx, http.MethodPost, "/2/tweets", nil, body, &response, options...); err != nil {
		return nil, err
	}
	return mapPost(c.accountID, response.Data), nil
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
	var response dataResponse[struct {
		Deleted bool `json:"deleted"`
	}]
	if err := c.transport.JSON(ctx, http.MethodDelete, "/2/tweets/"+url.PathEscape(postID), nil, nil, &response, options...); err != nil {
		return err
	}
	if !response.Data.Deleted {
		return &socialhub.Error{Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: "x", Op: "delete_post", PlatformMessage: "platform did not confirm deletion"}
	}
	return nil
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "" {
		return nil, invalidArgument("get_user", "user ID is required")
	}
	query := url.Values{"user.fields": {"created_at,description,profile_image_url,url,username,verified"}}
	var response dataResponse[xUser]
	if err := c.transport.JSON(ctx, http.MethodGet, "/2/users/"+url.PathEscape(userID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	return mapUser(c.accountID, response.Data), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if postID == "" {
		return nil, invalidArgument("get_post", "post ID is required")
	}
	query := postFields()
	var response postResponse
	if err := c.transport.JSON(ctx, http.MethodGet, "/2/tweets/"+url.PathEscape(postID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	return mapPostWithIncludes(c.accountID, response.Data, response.Includes), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID == "" {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID is required")
	}
	query := postFields()
	setPageQuery(query, input.Cursor, input.MaxResults)
	if input.StartTime != nil {
		query.Set("start_time", input.StartTime.Format(timeFormat))
	}
	if input.EndTime != nil {
		query.Set("end_time", input.EndTime.Format(timeFormat))
	}
	var response postsResponse
	path := "/2/users/" + url.PathEscape(input.UserID) + "/tweets"
	if err := c.transport.JSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapPostPage(c.accountID, response), nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if input.PostID == "" {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID is required")
	}
	query := postFields()
	query.Set("query", "conversation_id:"+input.PostID)
	setPageQuery(query, input.Cursor, input.MaxResults)
	var response postsResponse
	if err := c.transport.JSON(ctx, http.MethodGet, "/2/tweets/search/recent", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	comments := make([]socialhub.Comment, 0, len(response.Data))
	for _, post := range response.Data {
		if post.ID == input.PostID {
			continue
		}
		comments = append(comments, mapComment(c.accountID, input.PostID, post))
	}
	return socialhub.Page[socialhub.Comment]{Items: comments, NextCursor: response.Meta.NextToken, HasMore: response.Meta.NextToken != nil}, nil
}

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, false, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, true, options...)
}

func (c *Client) setReaction(ctx context.Context, input socialhub.ReactionRequest, remove bool, options ...socialhub.CallOption) error {
	if input.ActorID == "" || input.TargetID == "" {
		return invalidArgument("react", "actor and target IDs are required")
	}
	var path string
	switch input.Kind {
	case socialhub.ReactionLike:
		path = "/2/users/" + url.PathEscape(input.ActorID) + "/likes"
	case socialhub.ReactionRepost:
		path = "/2/users/" + url.PathEscape(input.ActorID) + "/retweets"
	default:
		return invalidArgument("react", "unsupported reaction kind")
	}
	if remove {
		path += "/" + url.PathEscape(input.TargetID)
		return c.transport.JSON(ctx, http.MethodDelete, path, nil, nil, nil, options...)
	}
	body := map[string]string{"tweet_id": input.TargetID}
	return c.transport.JSON(ctx, http.MethodPost, path, nil, body, nil, options...)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if input.PostID == "" || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "post ID and text are required")
	}
	text := input.Text
	post, err := c.Publish(ctx, socialhub.CreatePostRequest{Text: &text, ReplyToID: &input.PostID}, options...)
	if err != nil {
		return nil, err
	}
	return &socialhub.Comment{Platform: "x", AccountID: c.accountID, ID: post.ID, PostID: input.PostID, Text: input.Text, CreatedAt: post.CreatedAt}, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	return c.DeletePost(ctx, commentID, options...)
}

func postFields() url.Values {
	return url.Values{
		"tweet.fields": {"attachments,author_id,conversation_id,created_at,public_metrics,referenced_tweets"},
		"expansions":   {"attachments.media_keys"},
		"media.fields": {"duration_ms,height,media_key,preview_image_url,type,url,width"},
	}
}

func setPageQuery(query url.Values, cursor string, maxResults int) {
	if cursor != "" {
		query.Set("pagination_token", cursor)
	}
	if maxResults > 0 {
		query.Set("max_results", strconv.Itoa(maxResults))
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "x", Op: operation, PlatformMessage: message}
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)

const timeFormat = "2006-01-02T15:04:05Z07:00"
