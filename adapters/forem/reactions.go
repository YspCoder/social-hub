package forem

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike {
		return unsupported("react", "common Forem reactions support only Article likes; use ReactionWorkflow for other categories")
	}
	if input.ActorID != "" {
		user, err := client.GetUser(ctx, "", options...)
		if err != nil {
			return err
		}
		if user.ID != input.ActorID {
			return invalidArgument("react", "actor must be the authenticated Forem user")
		}
	}
	return client.CreateForemReaction(ctx, ForemReactionRequest{
		Category: ReactionLike, TargetID: input.TargetID, Type: ReactableArticle,
	}, options...)
}

func (client *Client) RemoveReaction(context.Context, socialhub.ReactionRequest, ...socialhub.CallOption) error {
	return unsupported("remove_reaction", "Forem API V1 exposes create and toggle, but no idempotent reaction removal endpoint")
}

func (client *Client) Comment(context.Context, socialhub.CreateCommentRequest, ...socialhub.CallOption) (*socialhub.Comment, error) {
	return nil, unsupported("comment", "Forem API V1 does not expose public comment creation")
}

func (client *Client) DeleteComment(context.Context, string, ...socialhub.CallOption) error {
	return unsupported("delete_comment", "Forem API V1 does not expose public comment deletion")
}

func (client *Client) CreateForemReaction(ctx context.Context, input ForemReactionRequest, options ...socialhub.CallOption) error {
	return client.foremReaction(ctx, "/api/reactions", input, options...)
}

func (client *Client) ToggleForemReaction(ctx context.Context, input ForemReactionRequest, options ...socialhub.CallOption) error {
	return client.foremReaction(ctx, "/api/reactions/toggle", input, options...)
}

func (client *Client) foremReaction(ctx context.Context, path string, input ForemReactionRequest, options ...socialhub.CallOption) error {
	if !validReaction(input) {
		return invalidArgument("forem_reaction", "category, positive target ID, and reactable type are required")
	}
	query := url.Values{
		"category": {string(input.Category)}, "reactable_id": {input.TargetID}, "reactable_type": {string(input.Type)},
	}
	return client.requestJSON(ctx, http.MethodPost, path, query, nil, nil, options...)
}

func validReaction(input ForemReactionRequest) bool {
	if !validID(input.TargetID) {
		return false
	}
	switch input.Category {
	case ReactionLike, ReactionUnicorn, ReactionExplodingHead, ReactionRaisedHands, ReactionFire:
	default:
		return false
	}
	switch input.Type {
	case ReactableArticle, ReactableComment, ReactableUser:
		return true
	default:
		return false
	}
}
