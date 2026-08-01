package microsoftteams

import (
	"context"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) SetReaction(ctx context.Context, ref MessageRef, reactionType string, options ...socialhub.CallOption) error {
	return c.changeReaction(ctx, "set_reaction", ref, reactionType, "setReaction", options...)
}

func (c *Client) UnsetReaction(ctx context.Context, ref MessageRef, reactionType string, options ...socialhub.CallOption) error {
	return c.changeReaction(ctx, "unset_reaction", ref, reactionType, "unsetReaction", options...)
}

func (c *Client) changeReaction(ctx context.Context, operation string, ref MessageRef, reactionType, action string, options ...socialhub.CallOption) error {
	if err := ref.validate(operation, true); err != nil {
		return err
	}
	if strings.TrimSpace(reactionType) == "" || len([]rune(reactionType)) > 64 || strings.ContainsAny(reactionType, "\r\n") {
		return invalidArgument(operation, "reaction_type must be a bounded Graph reaction or Unicode value")
	}
	if err := c.requireReaction(operation, ref.Target); err != nil {
		return err
	}
	return c.post(ctx, operation, messagePath(ref)+"/"+action, map[string]string{"reactionType": reactionType}, nil, options...)
}

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike {
		return unsupported("react", "common reactions map only to the Teams like reaction; use ReactionWorkflow for other values")
	}
	if err := c.validateReactionActor("react", input.ActorID); err != nil {
		return err
	}
	ref, err := ParseMessageRef(input.TargetID)
	if err != nil {
		return err
	}
	return c.SetReaction(ctx, ref, "like", options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike {
		return unsupported("remove_reaction", "common reactions map only to the Teams like reaction; use ReactionWorkflow for other values")
	}
	if err := c.validateReactionActor("remove_reaction", input.ActorID); err != nil {
		return err
	}
	ref, err := ParseMessageRef(input.TargetID)
	if err != nil {
		return err
	}
	return c.UnsetReaction(ctx, ref, "like", options...)
}

func (c *Client) validateReactionActor(operation, actorID string) error {
	if actorID == "" {
		return nil
	}
	if c.actorID == "" {
		return unsupported(operation, "Graph applies reactions as the delegated caller; configure actor_id to validate an explicit common actor")
	}
	if actorID != c.actorID {
		return invalidArgument(operation, "actor_id does not match the configured delegated caller")
	}
	return nil
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "non-empty text is required")
	}
	if input.ParentID != nil {
		return nil, unsupported("comment", "Teams supports one reply collection under a root message, not nested common comments")
	}
	parent, err := ParseMessageRef(input.PostID)
	if err != nil || parent.ReplyID != "" {
		return nil, invalidArgument("comment", "post_id must identify a root Teams message")
	}
	message, err := c.Reply(ctx, ReplyRequest{Parent: parent, Body: MessageBody{ContentType: "text", Content: input.Text}}, options...)
	if err != nil {
		return nil, err
	}
	comment := mapComment(c.accountID, MessageRef{Target: parent.Target, RootID: parent.RootID, ReplyID: message.ID}, *message)
	return &comment, nil
}

func (c *Client) DeleteComment(ctx context.Context, id string, options ...socialhub.CallOption) error {
	ref, err := ParseMessageRef(id)
	if err != nil || ref.ReplyID == "" {
		return invalidArgument("delete_comment", "comment ID must identify a Teams reply")
	}
	return c.SoftDelete(ctx, ref, options...)
}
