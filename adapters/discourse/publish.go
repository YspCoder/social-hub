package discourse

import (
	"context"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if input.Text == nil || !validText(*input.Text, 2<<20) || input.ReplyToID == nil || !validID(*input.ReplyToID) {
		return nil, invalidArgument("publish", "Discourse common publishing requires text and a reply_to_id post ID")
	}
	if len(input.MediaIDs) > 0 || input.QuotePostID != nil || input.Visibility != nil {
		return nil, unsupported("publish", "media_ids, quote_post_id, and visibility do not map to a Discourse reply")
	}
	post, err := client.createReply(ctx, *input.ReplyToID, *input.Text, options...)
	if err != nil {
		return nil, err
	}
	return client.mapPost(post), nil
}

func (client *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := client.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return post.Status, nil
}

func (client *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	api, err := client.requireAPI("delete_post")
	if err != nil {
		return err
	}
	if !validID(postID) {
		return invalidArgument("delete_post", "post ID must be a positive integer")
	}
	return api.JSON(ctx, http.MethodDelete, path("posts", postID), nil, nil, nil, options...)
}

func (client *Client) createReply(ctx context.Context, targetPostID, raw string, options ...socialhub.CallOption) (discoursePost, error) {
	target, err := client.getPost(ctx, targetPostID, options...)
	if err != nil {
		return discoursePost{}, err
	}
	if target.TopicID <= 0 || target.PostNumber <= 0 {
		return discoursePost{}, platformError("publish", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return client.createPost(ctx, createPostPayload{
		Raw: raw, TopicID: target.TopicID, ReplyToPostNumber: target.PostNumber,
	}, "publish", options...)
}

func (client *Client) createPost(ctx context.Context, input createPostPayload, operation string, options ...socialhub.CallOption) (discoursePost, error) {
	api, err := client.requireAPI(operation)
	if err != nil {
		return discoursePost{}, err
	}
	var response discoursePost
	if err := api.JSON(ctx, http.MethodPost, "/posts.json", nil, input, &response, options...); err != nil {
		return discoursePost{}, err
	}
	if response.ID <= 0 || response.TopicID <= 0 {
		return discoursePost{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response, nil
}

func (client *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if !validID(input.PostID) || !validText(input.Text, 2<<20) {
		return nil, invalidArgument("comment", "post ID and text are required")
	}
	target := input.PostID
	if input.ParentID != nil {
		if !validID(*input.ParentID) {
			return nil, invalidArgument("comment", "parent ID must be a positive post ID")
		}
		target = *input.ParentID
	}
	post, err := client.createReply(ctx, target, input.Text, options...)
	if err != nil {
		return nil, err
	}
	comment := client.mapComment(input.PostID, post)
	comment.ParentID = stringPointer(target)
	return &comment, nil
}

func (client *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	return client.DeletePost(ctx, commentID, options...)
}

func (client *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	api, err := client.requireAPI("react")
	if err != nil {
		return err
	}
	if !validID(input.TargetID) {
		return invalidArgument("react", "target ID must be a positive post ID")
	}
	if strings.TrimSpace(input.ActorID) != "" && input.ActorID != client.apiUsername {
		return invalidArgument("react", "actor must be the configured Discourse API username")
	}
	if input.Kind != socialhub.ReactionLike {
		return unsupported("react", "the official Discourse OpenAPI contract only documents post action type 2 (like)")
	}
	var response discoursePost
	if err := api.JSON(ctx, http.MethodPost, "/post_actions.json", nil, postActionPayload{
		ID: mustID(input.TargetID), PostActionTypeID: 2,
	}, &response, options...); err != nil {
		return err
	}
	if response.ID != mustID(input.TargetID) {
		return platformError("react", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (client *Client) RemoveReaction(context.Context, socialhub.ReactionRequest, ...socialhub.CallOption) error {
	return unsupported("remove_reaction", "unlike is not present in the official Discourse OpenAPI contract")
}
