package lark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const commonLikeEmoji = "THUMBSUP"

func (c *Client) AddReaction(ctx context.Context, messageID, emojiType string, options ...socialhub.CallOption) (*Reaction, error) {
	if err := c.requireScopes("im.message.reaction.create", "im:message.reactions:write_only"); err != nil {
		return nil, err
	}
	messageID, emojiType = strings.TrimSpace(messageID), strings.TrimSpace(emojiType)
	if !validMessageID(messageID) || !validText(emojiType, 64) {
		return nil, invalidArgument("im.message.reaction.create", "message_id and a bounded emoji_type are required")
	}
	var response struct {
		Data wireReaction `json:"data"`
	}
	path := "/open-apis/im/v1/messages/" + url.PathEscape(messageID) + "/reactions"
	if err := c.call(ctx, "im.message.reaction.create", http.MethodPost, path, nil, struct {
		ReactionType struct {
			EmojiType string `json:"emoji_type"`
		} `json:"reaction_type"`
	}{ReactionType: struct {
		EmojiType string `json:"emoji_type"`
	}{EmojiType: emojiType}}, &response, false, options...); err != nil {
		return nil, err
	}
	if !validOpaqueID(response.Data.ReactionID, 512) || response.Data.ReactionType.EmojiType != emojiType {
		return nil, platformError("im.message.reaction.create", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &Reaction{
		ID: response.Data.ReactionID, MessageID: messageID, EmojiType: emojiType,
		ActorID: response.Data.Operator.OperatorID,
	}, nil
}

func (c *Client) DeleteReaction(ctx context.Context, messageID, reactionID string, options ...socialhub.CallOption) error {
	if err := c.requireScopes("im.message.reaction.delete", "im:message.reactions:write_only"); err != nil {
		return err
	}
	messageID, reactionID = strings.TrimSpace(messageID), strings.TrimSpace(reactionID)
	if !validMessageID(messageID) || !validOpaqueID(reactionID, 512) {
		return invalidArgument("im.message.reaction.delete", "message_id and reaction_id are required")
	}
	path := "/open-apis/im/v1/messages/" + url.PathEscape(messageID) + "/reactions/" + url.PathEscape(reactionID)
	return c.call(ctx, "im.message.reaction.delete", http.MethodDelete, path, nil, nil, nil, false, options...)
}

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike {
		return unsupported("react", "the common Lark reactor maps only like to THUMBSUP")
	}
	if input.ActorID != "" && input.ActorID != c.actorID {
		return invalidArgument("react", "actor_id must match account.settings.actor_id")
	}
	_, err := c.AddReaction(ctx, input.TargetID, commonLikeEmoji, options...)
	return err
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike {
		return unsupported("remove_reaction", "the common Lark reactor maps only like to THUMBSUP")
	}
	if c.actorID == "" || (input.ActorID != "" && input.ActorID != c.actorID) {
		return invalidArgument("remove_reaction", "configured actor_id must identify the reaction owner")
	}
	if err := c.requireScopes("im.message.reaction.list", "im:message.reactions:read", "im:message.reactions:write_only"); err != nil {
		return err
	}
	messageID := strings.TrimSpace(input.TargetID)
	if !validMessageID(messageID) {
		return invalidArgument("remove_reaction", "target_id must be a Lark message ID")
	}
	cursor := ""
	for page := 0; page < 20; page++ {
		query := url.Values{"reaction_type": {commonLikeEmoji}, "page_size": {"50"}}
		if cursor != "" {
			query.Set("page_token", cursor)
		}
		var response struct {
			Data struct {
				Items     []wireReaction `json:"items"`
				HasMore   bool           `json:"has_more"`
				PageToken string         `json:"page_token"`
			} `json:"data"`
		}
		path := "/open-apis/im/v1/messages/" + url.PathEscape(messageID) + "/reactions"
		if err := c.get(ctx, "im.message.reaction.list", path, query, &response, options...); err != nil {
			return err
		}
		for _, reaction := range response.Data.Items {
			if reaction.Operator.OperatorID == c.actorID && reaction.ReactionType.EmojiType == commonLikeEmoji {
				return c.DeleteReaction(ctx, messageID, reaction.ReactionID, options...)
			}
		}
		if !response.Data.HasMore || response.Data.PageToken == "" || response.Data.PageToken == cursor {
			break
		}
		cursor = response.Data.PageToken
	}
	return platformError("remove_reaction", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "comment text is required")
	}
	targetID := input.PostID
	var parentID *string
	if input.ParentID != nil {
		targetID = strings.TrimSpace(*input.ParentID)
		parentID = stringPointer(targetID)
	}
	encoded, _ := json.Marshal(map[string]string{"text": input.Text})
	message, err := c.Reply(ctx, ReplyRequest{MessageID: targetID, MessageType: "text", Content: encoded}, options...)
	if err != nil {
		return nil, err
	}
	comment := socialhub.Comment{
		Platform: "lark", AccountID: c.accountID, ID: message.ID, PostID: input.PostID,
		AuthorID: message.SenderID, ParentID: parentID, Text: input.Text, CreatedAt: message.SentAt, Extensions: message.Extensions,
	}
	return &comment, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	return c.Delete(ctx, commentID, options...)
}
