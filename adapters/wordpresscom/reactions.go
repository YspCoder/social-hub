package wordpresscom

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return client.postLike(ctx, input, []string{"new"}, true, options...)
}

func (client *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return client.postLike(ctx, input, []string{"mine", "delete"}, false, options...)
}

func (client *Client) postLike(ctx context.Context, input socialhub.ReactionRequest, suffix []string, expected bool, options ...socialhub.CallOption) error {
	if input.ActorID != "" && (client.userID == "" || input.ActorID != client.userID) {
		return invalidArgument("react", "actor must be the configured WordPress.com user")
	}
	if !validID(input.TargetID) || input.Kind != socialhub.ReactionLike {
		return invalidArgument("react", "target must be a positive Post ID and reaction must be LIKE")
	}
	api, err := client.requireUser("react")
	if err != nil {
		return err
	}
	if err := client.requireScopes("react", "posts"); err != nil {
		return err
	}
	var response likeResponse
	path := append([]string{"posts", input.TargetID, "likes"}, suffix...)
	if err := client.form(ctx, api, client.sitePath(path...), nil, &response, options...); err != nil {
		return err
	}
	if !response.Success || response.ILike != expected || response.PostID != mustID(input.TargetID) || response.SiteID <= 0 || !client.matchesSite(response.SiteID) {
		return platformError("react", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (client *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if !validID(input.PostID) || strings.TrimSpace(input.Text) == "" || !validText(input.Text) {
		return nil, invalidArgument("comment", "Post ID and valid non-empty comment text are required")
	}
	path := client.sitePath("posts", input.PostID, "replies", "new")
	if input.ParentID != nil {
		if !validID(*input.ParentID) {
			return nil, invalidArgument("comment", "parent comment ID must be a positive integer")
		}
		path = client.sitePath("comments", *input.ParentID, "replies", "new")
	}
	api, err := client.requireUser("comment")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("comment", "comments"); err != nil {
		return nil, err
	}
	var response wpComment
	if err := client.form(ctx, api, path, url.Values{"content": {input.Text}}, &response, options...); err != nil {
		return nil, err
	}
	postID := referenceID(response.Post)
	if response.ID <= 0 || postID != "" && postID != input.PostID {
		return nil, platformError("comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	mapped := mapComment(client.accountID, input.PostID, response, client.clock.Now())
	if input.ParentID != nil && mapped.ParentID == nil {
		mapped.ParentID = stringPointer(*input.ParentID)
	}
	return &mapped, nil
}

func (client *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	if !validID(commentID) {
		return invalidArgument("delete_comment", "comment ID must be a positive integer")
	}
	api, err := client.requireUser("delete_comment")
	if err != nil {
		return err
	}
	if err := client.requireScopes("delete_comment", "comments"); err != nil {
		return err
	}
	var response wpComment
	if err := client.form(ctx, api, client.sitePath("comments", commentID, "delete"), nil, &response, options...); err != nil {
		return err
	}
	if response.ID != mustID(commentID) {
		return platformError("delete_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}
