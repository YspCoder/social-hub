package microsoftteams

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type graphMessagePage struct {
	Value    []json.RawMessage `json:"value"`
	NextLink string            `json:"@odata.nextLink"`
}

func (c *Client) List(ctx context.Context, input ListMessagesRequest, options ...socialhub.CallOption) (MessagePage, error) {
	const operation = "list_messages"
	if err := input.Target.validate(operation); err != nil {
		return MessagePage{}, err
	}
	if err := c.requireRead(operation, input.Target); err != nil {
		return MessagePage{}, err
	}
	limit, err := pageLimit(operation, input.MaxResults)
	if err != nil {
		return MessagePage{}, err
	}
	query := url.Values{"$top": {strconv.Itoa(limit)}}
	path, query, err := c.pageRequest(input.Cursor, targetCollectionPath(input.Target), query)
	if err != nil {
		return MessagePage{}, err
	}
	var response graphMessagePage
	if err := c.get(ctx, operation, path, query, &response, options...); err != nil {
		return MessagePage{}, err
	}
	return decodePage(response, "")
}

func (c *Client) ListReplies(ctx context.Context, input ListRepliesRequest, options ...socialhub.CallOption) (MessagePage, error) {
	const operation = "list_replies"
	if err := input.Parent.validate(operation, false); err != nil {
		return MessagePage{}, err
	}
	if err := c.requireRead(operation, input.Parent.Target); err != nil {
		return MessagePage{}, err
	}
	limit, err := pageLimit(operation, input.MaxResults)
	if err != nil {
		return MessagePage{}, err
	}
	query := url.Values{"$top": {strconv.Itoa(limit)}}
	path, query, err := c.pageRequest(input.Cursor, repliesPath(input.Parent), query)
	if err != nil {
		return MessagePage{}, err
	}
	var response graphMessagePage
	if err := c.get(ctx, operation, path, query, &response, options...); err != nil {
		return MessagePage{}, err
	}
	return decodePage(response, input.Parent.RootID)
}

func decodePage(response graphMessagePage, rootID string) (MessagePage, error) {
	page := MessagePage{Items: make([]ChatMessage, 0, len(response.Value)), NextCursor: response.NextLink, HasMore: response.NextLink != ""}
	for _, raw := range response.Value {
		message, err := decodeMessage(raw)
		if err != nil {
			return MessagePage{}, err
		}
		if rootID != "" && message.ReplyToID != "" && message.ReplyToID != rootID {
			return MessagePage{}, platformError("list_replies", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		page.Items = append(page.Items, *message)
	}
	return page, nil
}

func pageLimit(operation string, requested int) (int, error) {
	if requested < 0 {
		return 0, invalidArgument(operation, "max_results must not be negative")
	}
	if requested == 0 {
		return 50, nil
	}
	if requested > 50 {
		return 50, nil
	}
	return requested, nil
}

func (c *Client) GetUser(context.Context, string, ...socialhub.CallOption) (*socialhub.User, error) {
	return nil, unsupported("get_user", "this Teams messaging adapter does not expose Microsoft Entra user lookup")
}

func (c *Client) GetPost(ctx context.Context, id string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	ref, err := ParseMessageRef(id)
	if err != nil {
		return nil, err
	}
	message, err := c.Get(ctx, ref, options...)
	if err != nil {
		return nil, err
	}
	post := mapPost(c.accountID, ref, *message)
	return &post, nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "time-range filtering is not exposed by Teams message list endpoints")
	}
	var target Target
	if strings.TrimSpace(input.UserID) == "" {
		if c.defaultTarget == nil {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user_id must contain a Teams conversation reference when no default target is configured")
		}
		target = *c.defaultTarget
	} else {
		var err error
		target, err = ParseConversationRef(input.UserID)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
	}
	page, err := c.List(ctx, ListMessagesRequest{Target: target, Cursor: input.Cursor, MaxResults: input.MaxResults}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	result := socialhub.Page[socialhub.Post]{Items: make([]socialhub.Post, 0, len(page.Items)), HasMore: page.HasMore}
	if page.NextCursor != "" {
		result.NextCursor = stringPointer(page.NextCursor)
	}
	for _, message := range page.Items {
		result.Items = append(result.Items, mapPost(c.accountID, MessageRef{Target: target, RootID: message.ID}, message))
	}
	return result, nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	ref, err := ParseMessageRef(input.PostID)
	if err != nil || ref.ReplyID != "" {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post_id must identify a root Teams message")
	}
	page, err := c.ListReplies(ctx, ListRepliesRequest{Parent: ref, Cursor: input.Cursor, MaxResults: input.MaxResults}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	result := socialhub.Page[socialhub.Comment]{Items: make([]socialhub.Comment, 0, len(page.Items)), HasMore: page.HasMore}
	if page.NextCursor != "" {
		result.NextCursor = stringPointer(page.NextCursor)
	}
	for _, message := range page.Items {
		result.Items = append(result.Items, mapComment(c.accountID, MessageRef{Target: ref.Target, RootID: ref.RootID, ReplyID: message.ID}, message))
	}
	return result, nil
}
