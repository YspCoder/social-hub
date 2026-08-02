package discourse

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) CreateTopic(ctx context.Context, input CreateTopicRequest, options ...socialhub.CallOption) (*Topic, error) {
	if !validText(input.Title, 1024) || !validText(input.Raw, 2<<20) {
		return nil, invalidArgument("create_topic", "title and raw body are required")
	}
	categoryID := int64(0)
	if input.CategoryID != "" {
		if !validID(input.CategoryID) {
			return nil, invalidArgument("create_topic", "category ID must be a positive integer")
		}
		categoryID = mustID(input.CategoryID)
	}
	post, err := client.createPost(ctx, createPostPayload{
		Title: input.Title, Raw: input.Raw, Category: categoryID,
	}, "create_topic", options...)
	if err != nil {
		return nil, err
	}
	mapped := client.mapPost(post)
	return &Topic{
		ID: strconv.FormatInt(post.TopicID, 10), Title: input.Title, CategoryID: input.CategoryID,
		PostsCount: 1, ReplyCount: 0, Visible: true, Archetype: "regular", Posts: []socialhub.Post{*mapped},
		PostIDs: []string{mapped.ID},
	}, nil
}

func (client *Client) GetTopic(ctx context.Context, topicID string, options ...socialhub.CallOption) (*Topic, error) {
	api, err := client.requireAPI("get_topic")
	if err != nil {
		return nil, err
	}
	if !validID(topicID) {
		return nil, invalidArgument("get_topic", "topic ID must be a positive integer")
	}
	var response topicResponse
	if err := api.JSON(ctx, http.MethodGet, path("t", topicID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != mustID(topicID) {
		return nil, platformError("get_topic", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return client.mapTopic(response), nil
}

func (client *Client) CreatePrivateMessage(ctx context.Context, input CreatePrivateMessageRequest, options ...socialhub.CallOption) (*PrivateMessage, error) {
	if !validText(input.Title, 1024) || !validText(input.Raw, 2<<20) || len(input.Recipients) == 0 {
		return nil, invalidArgument("create_private_message", "title, raw body, and recipients are required")
	}
	recipients := make([]string, 0, len(input.Recipients))
	seen := make(map[string]struct{}, len(input.Recipients))
	for _, recipient := range input.Recipients {
		recipient = strings.TrimSpace(recipient)
		if !validUsername(recipient) {
			return nil, invalidArgument("create_private_message", "recipient username is invalid")
		}
		if _, exists := seen[recipient]; exists {
			continue
		}
		seen[recipient] = struct{}{}
		recipients = append(recipients, recipient)
	}
	post, err := client.createPost(ctx, createPostPayload{
		Title: input.Title, Raw: input.Raw, TargetRecipients: strings.Join(recipients, ","), Archetype: "private_message",
	}, "create_private_message", options...)
	if err != nil {
		return nil, err
	}
	return &PrivateMessage{
		TopicID: strconv.FormatInt(post.TopicID, 10), Title: input.Title,
		Recipients: append([]string(nil), recipients...), FirstPost: *client.mapPost(post),
	}, nil
}
