package youtube

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.rate(ctx, input, "like", options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.rate(ctx, input, "none", options...)
}

func (c *Client) rate(ctx context.Context, input socialhub.ReactionRequest, rating string, options ...socialhub.CallOption) error {
	if input.TargetID == "" || (input.ActorID != "" && input.ActorID != c.channelID) {
		return invalidArgument("rate", "target video is required and actor must match the configured channel")
	}
	if input.Kind != socialhub.ReactionLike {
		return unsupported("rate", "YouTube common reactions map only LIKE")
	}
	if err := c.requireScope("rate", "https://www.googleapis.com/auth/youtube.force-ssl"); err != nil {
		return err
	}
	return c.transport.JSON(ctx, http.MethodPost, "/videos/rate", url.Values{"id": {input.TargetID}, "rating": {rating}}, nil, nil, options...)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if input.PostID == "" || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "video ID and comment text are required")
	}
	if err := c.requireScope("comment", "https://www.googleapis.com/auth/youtube.force-ssl"); err != nil {
		return nil, err
	}
	if input.ParentID != nil {
		body := youtubeComment{}
		body.Snippet.ParentID = *input.ParentID
		body.Snippet.TextOriginal = input.Text
		var response youtubeComment
		if err := c.transport.JSON(ctx, http.MethodPost, "/comments", url.Values{"part": {"snippet"}}, body, &response, options...); err != nil {
			return nil, err
		}
		return commentFromCreate(c.accountID, input, response), nil
	}
	body := commentThread{}
	body.Snippet.VideoID = input.PostID
	body.Snippet.TopLevelComment.Snippet.TextOriginal = input.Text
	var response commentThread
	if err := c.transport.JSON(ctx, http.MethodPost, "/commentThreads", url.Values{"part": {"snippet"}}, body, &response, options...); err != nil {
		return nil, err
	}
	return commentFromCreate(c.accountID, input, response.Snippet.TopLevelComment), nil
}

func commentFromCreate(accountID socialhub.AccountID, input socialhub.CreateCommentRequest, response youtubeComment) *socialhub.Comment {
	return &socialhub.Comment{
		Platform: "youtube", AccountID: accountID, ID: response.ID, PostID: input.PostID,
		AuthorID: stringPointer(response.Snippet.AuthorChannelID.Value), ParentID: input.ParentID,
		Text: firstNonEmpty(response.Snippet.TextOriginal, input.Text), CreatedAt: response.Snippet.PublishedAt,
	}
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	if commentID == "" {
		return invalidArgument("delete_comment", "comment ID is required")
	}
	if err := c.requireScope("delete_comment", "https://www.googleapis.com/auth/youtube.force-ssl"); err != nil {
		return err
	}
	return c.transport.JSON(ctx, http.MethodDelete, "/comments", url.Values{"id": {commentID}}, nil, nil, options...)
}
