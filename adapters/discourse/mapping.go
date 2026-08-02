package discourse

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) mapUser(input discourseUser) *socialhub.User {
	accountType := "member"
	if input.Admin {
		accountType = "admin"
	} else if input.Moderator {
		accountType = "moderator"
	}
	return &socialhub.User{
		Platform: "discourse", AccountID: client.accountID, ID: strconv.FormatInt(input.ID, 10),
		Username: stringPointer(input.Username), DisplayName: stringPointer(input.Name),
		AvatarURL:  client.absoluteURL(strings.ReplaceAll(input.AvatarTemplate, "{size}", "120")),
		ProfileURL: client.absoluteURL(client.baseURL + "/u/" + url.PathEscape(input.Username)), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"discourse.user": append(json.RawMessage(nil), input.Raw...)},
	}
}

func (client *Client) mapPost(input discoursePost) *socialhub.Post {
	id := strconv.FormatInt(input.ID, 10)
	observedAt := client.clock.Now()
	state := socialhub.PublishStatePublished
	if input.DeletedAt != nil {
		state = socialhub.PublishStateFailed
	}
	post := &socialhub.Post{
		Platform: "discourse", AccountID: client.accountID, ID: id,
		Text: stringPointer(firstNonEmpty(input.RawText, input.Cooked)), CreatedAt: input.CreatedAt,
		URL: client.absoluteURL(input.PostURL), Status: &socialhub.PublishStatus{ID: id, State: state, UpdatedAt: input.UpdatedAt},
		Metrics: []socialhub.Metric{
			{Name: "replies", Value: float64(input.ReplyCount), AsOf: observedAt, Definition: "Discourse direct reply count"},
			{Name: "likes", Value: float64(likeCount(input.Actions)), AsOf: observedAt, Definition: "Discourse post action type 2 count"},
			{Name: "reads", Value: float64(input.Reads), AsOf: observedAt, Definition: "Discourse post read count"},
		},
		Extensions: map[string]json.RawMessage{"discourse.post": append(json.RawMessage(nil), input.Raw...)},
	}
	if input.UserID > 0 {
		post.AuthorID = stringPointer(strconv.FormatInt(input.UserID, 10))
	}
	return post
}

func (client *Client) mapComment(rootPostID string, input discoursePost) socialhub.Comment {
	post := client.mapPost(input)
	comment := socialhub.Comment{
		Platform: "discourse", AccountID: client.accountID, ID: post.ID, PostID: rootPostID,
		Text: firstNonEmpty(input.RawText, input.Cooked), CreatedAt: input.CreatedAt,
		ParentID: stringPointer(rootPostID), Metrics: post.Metrics,
		Extensions: map[string]json.RawMessage{"discourse.post": append(json.RawMessage(nil), input.Raw...)},
	}
	if input.UserID > 0 {
		comment.AuthorID = stringPointer(strconv.FormatInt(input.UserID, 10))
	}
	return comment
}

func (client *Client) mapTopic(input topicResponse) *Topic {
	topic := &Topic{
		ID: strconv.FormatInt(input.ID, 10), Title: input.Title, Slug: input.Slug,
		CategoryID: strconv.FormatInt(input.CategoryID, 10), PostsCount: input.PostsCount, ReplyCount: input.ReplyCount,
		Views: input.Views, LikeCount: input.LikeCount, CreatedAt: input.CreatedAt, LastPostedAt: input.LastPostedAt,
		Visible: input.Visible, Closed: input.Closed, Archived: input.Archived, Archetype: input.Archetype,
		Raw: append(json.RawMessage(nil), input.Raw...),
	}
	for _, post := range input.PostStream.Posts {
		topic.Posts = append(topic.Posts, *client.mapPost(post))
	}
	for _, id := range input.PostStream.Stream {
		if id > 0 {
			topic.PostIDs = append(topic.PostIDs, strconv.FormatInt(id, 10))
		}
	}
	return topic
}

func (client *Client) mapUpload(input discourseUpload, request socialhub.BeginUploadRequest) socialhub.Media {
	media := socialhub.Media{
		ID: strconv.FormatInt(input.ID, 10), MIME: request.MIME, Type: request.Type, State: socialhub.MediaStateReady,
		Extensions: map[string]json.RawMessage{"discourse.upload": append(json.RawMessage(nil), input.Raw...)},
	}
	if resolved := client.absoluteURL(input.URL); resolved != nil {
		media.URL = *resolved
	}
	size := input.FileSize
	if size <= 0 {
		size = request.Size
	}
	media.Size = &size
	if input.Width > 0 {
		media.Width = intPointer(input.Width)
	}
	if input.Height > 0 {
		media.Height = intPointer(input.Height)
	}
	return media
}

func likeCount(actions []discourseAction) int {
	for _, action := range actions {
		if action.ID == 2 {
			return action.Count
		}
	}
	return 0
}

func (client *Client) absoluteURL(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	base, err := url.Parse(client.baseURL + "/")
	if err != nil {
		return nil
	}
	reference, err := url.Parse(value)
	if err != nil {
		return nil
	}
	resolved := base.ResolveReference(reference)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return nil
	}
	result := resolved.String()
	return &result
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func intPointer(value int) *int {
	copy := value
	return &copy
}
