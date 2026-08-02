package lemmy

import (
	"context"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike {
		return unsupported("react", "common Lemmy reactions support only post upvotes; use VoteWorkflow for downvotes")
	}
	if err := client.validateActor(ctx, input.ActorID, options...); err != nil {
		return err
	}
	return client.VotePost(ctx, input.TargetID, 1, options...)
}

func (client *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike {
		return unsupported("remove_reaction", "common Lemmy reactions support only post vote removal")
	}
	if err := client.validateActor(ctx, input.ActorID, options...); err != nil {
		return err
	}
	return client.VotePost(ctx, input.TargetID, 0, options...)
}

func (client *Client) validateActor(ctx context.Context, actorID string, options ...socialhub.CallOption) error {
	if actorID == "" {
		return nil
	}
	if !validID(actorID) {
		return invalidArgument("react", "actor ID must be a positive integer")
	}
	user, err := client.GetUser(ctx, "", options...)
	if err != nil {
		return err
	}
	if user.ID != actorID {
		return invalidArgument("react", "actor must be the configured Lemmy user")
	}
	return nil
}

func (client *Client) VotePost(ctx context.Context, postID string, score int, options ...socialhub.CallOption) error {
	if !validID(postID) || !validVote(score) {
		return invalidArgument("vote_post", "post ID and score -1, 0, or 1 are required")
	}
	payload := struct {
		PostID int64 `json:"post_id"`
		Score  int   `json:"score"`
	}{PostID: mustID(postID), Score: score}
	var response postResponse
	if err := client.requestJSON(ctx, http.MethodPost, "/post/like", nil, payload, &response, options...); err != nil {
		return err
	}
	if !validPostView(response.PostView) || response.PostView.Post.ID != mustID(postID) {
		return platformError("vote_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (client *Client) VoteComment(ctx context.Context, commentID string, score int, options ...socialhub.CallOption) error {
	if !validID(commentID) || !validVote(score) {
		return invalidArgument("vote_comment", "comment ID and score -1, 0, or 1 are required")
	}
	payload := struct {
		CommentID int64 `json:"comment_id"`
		Score     int   `json:"score"`
	}{CommentID: mustID(commentID), Score: score}
	var response commentResponse
	if err := client.requestJSON(ctx, http.MethodPost, "/comment/like", nil, payload, &response, options...); err != nil {
		return err
	}
	if response.CommentView.Comment.ID != mustID(commentID) {
		return platformError("vote_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func validVote(score int) bool { return score >= -1 && score <= 1 }

func (client *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if !validID(input.PostID) || strings.TrimSpace(input.Text) == "" || !validBody(input.Text, 10000) {
		return nil, invalidArgument("comment", "post ID and non-empty content within the Lemmy limit are required")
	}
	parentID, err := optionalCommentID(input.ParentID)
	if err != nil {
		return nil, err
	}
	payload := struct {
		Content  string `json:"content"`
		PostID   int64  `json:"post_id"`
		ParentID *int64 `json:"parent_id,omitempty"`
	}{Content: input.Text, PostID: mustID(input.PostID), ParentID: parentID}
	var response commentResponse
	if err := client.requestJSON(ctx, http.MethodPost, "/comment", nil, payload, &response, options...); err != nil {
		return nil, err
	}
	if response.CommentView.Comment.PostID != mustID(input.PostID) {
		return nil, platformError("comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	mapped, err := client.mapComment(response.CommentView)
	if err != nil {
		return nil, err
	}
	if input.ParentID != nil && (mapped.ParentID == nil || *mapped.ParentID != *input.ParentID) {
		return nil, platformError("comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &mapped, nil
}

func optionalCommentID(value *string) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	if !validID(*value) {
		return nil, invalidArgument("comment", "parent ID must be a positive integer")
	}
	return int64Pointer(mustID(*value)), nil
}

func (client *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	if !validID(commentID) {
		return invalidArgument("delete_comment", "comment ID must be a positive integer")
	}
	payload := struct {
		CommentID int64 `json:"comment_id"`
		Deleted   bool  `json:"deleted"`
	}{CommentID: mustID(commentID), Deleted: true}
	var response commentResponse
	if err := client.requestJSON(ctx, http.MethodPost, "/comment/delete", nil, payload, &response, options...); err != nil {
		return err
	}
	if response.CommentView.Comment.ID != mustID(commentID) || !response.CommentView.Comment.Deleted {
		return platformError("delete_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}
