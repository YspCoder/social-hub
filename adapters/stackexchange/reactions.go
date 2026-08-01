package stackexchange

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (client *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return client.vote(ctx, input, false, options...)
}

func (client *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return client.vote(ctx, input, true, options...)
}

func (client *Client) vote(ctx context.Context, input socialhub.ReactionRequest, undo bool, options ...socialhub.CallOption) error {
	operation := "upvote"
	pathSuffix := "/up"
	if undo {
		operation = "undo_upvote"
		pathSuffix = "/up/undo"
	}
	if !validID(input.TargetID) || input.Kind != socialhub.ReactionLike {
		return invalidArgument(operation, "target ID must be a positive integer and reaction must be LIKE")
	}
	if err := client.requireActor(operation, input.ActorID); err != nil {
		return err
	}
	if err := client.requireWrite(operation); err != nil {
		return err
	}
	_, err := call[json.RawMessage](client, ctx, operation, http.MethodPost, "/posts/"+input.TargetID+pathSuffix, nil, url.Values{}, options...)
	return err
}

func (client *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	textLength := utf8.RuneCountInString(strings.TrimSpace(input.Text))
	if !validID(input.PostID) || textLength < 15 || textLength > 600 {
		return nil, invalidArgument("comment", "post ID and comment text from 15 to 600 characters are required")
	}
	if input.ParentID != nil {
		return nil, invalidArgument("comment", "Stack Exchange comments are flat and do not accept parent_id")
	}
	if err := client.requireWrite("comment"); err != nil {
		return nil, err
	}
	response, err := call[CommentDetails](client, ctx, "comments_add", http.MethodPost, "/posts/"+input.PostID+"/comments/add", nil, url.Values{"body": {input.Text}}, options...)
	if err != nil {
		return nil, err
	}
	if len(response.Items) == 0 || response.Items[0].CommentID <= 0 {
		return nil, platformError("comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapComment(client.accountID, response.Items[0], client.clock.Now()), nil
}

func (client *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	if !validID(commentID) {
		return invalidArgument("delete_comment", "comment ID must be a positive integer")
	}
	if err := client.requireWrite("delete_comment"); err != nil {
		return err
	}
	_, err := call[json.RawMessage](client, ctx, "comments_delete", http.MethodPost, "/comments/"+commentID+"/delete", nil, url.Values{}, options...)
	return err
}
