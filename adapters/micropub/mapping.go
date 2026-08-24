package micropub

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetUser(_ context.Context, userID string, _ ...socialhub.CallOption) (*socialhub.User, error) {
	if !client.supportsUpdate {
		return nil, unsupported("get_user", "endpoint is not configured with source-query support")
	}
	if userID != "" && userID != "me" && userID != client.siteURL {
		return nil, invalidArgument("get_user", "Micropub exposes only the configured site identity")
	}
	profileURL := client.siteURL
	accountType := "site"
	return &socialhub.User{
		Platform: platformName, AccountID: client.accountID, ID: client.siteURL,
		ProfileURL: &profileURL, AccountType: &accountType,
	}, nil
}

func (client *Client) GetPost(ctx context.Context, postURL string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	entry, err := client.Source(ctx, postURL, nil, options...)
	if err != nil {
		return nil, err
	}
	return client.mapEntry(postURL, entry), nil
}

func (client *Client) ListPosts(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Micropub does not define post listing")
}

func (client *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Micropub does not define comment listing")
}

func (client *Client) mapEntry(postURL string, entry *Entry) *socialhub.Post {
	visibility := "public"
	status := socialhub.PublishStatus{ID: postURL, State: socialhub.PublishStatePublished}
	post := &socialhub.Post{
		Platform: platformName, AccountID: client.accountID, ID: postURL, URL: &postURL,
		Visibility: &visibility, Status: &status,
		Extensions: map[string]json.RawMessage{"source": append(json.RawMessage(nil), entry.Raw...)},
	}
	if values := entry.Properties["content"]; len(values) != 0 {
		if text := sourceText(values[0]); text != "" {
			post.Text = &text
		}
	}
	if values := entry.Properties["published"]; len(values) != 0 {
		var value string
		if json.Unmarshal(values[0], &value) == nil {
			if published, err := time.Parse(time.RFC3339, value); err == nil {
				post.CreatedAt = &published
			}
		}
	}
	post.Media = append(post.Media, sourceMedia(entry.Properties["photo"], socialhub.MediaTypeImage)...)
	post.Media = append(post.Media, sourceMedia(entry.Properties["video"], socialhub.MediaTypeVideo)...)
	post.Media = append(post.Media, sourceMedia(entry.Properties["audio"], socialhub.MediaTypeAudio)...)
	post.Relations = append(post.Relations, sourceRelations(entry.Properties["in-reply-to"], socialhub.RelationReply)...)
	post.Relations = append(post.Relations, sourceRelations(entry.Properties["repost-of"], socialhub.RelationRepost)...)
	return post
}

func sourceText(value json.RawMessage) string {
	var text string
	if json.Unmarshal(value, &text) == nil {
		return text
	}
	var object struct {
		Text  string `json:"text"`
		HTML  string `json:"html"`
		Value string `json:"value"`
	}
	if json.Unmarshal(value, &object) != nil {
		return ""
	}
	return firstNonEmpty(object.Text, object.HTML, object.Value)
}

func sourceMedia(values []json.RawMessage, mediaType socialhub.MediaType) []socialhub.Media {
	result := make([]socialhub.Media, 0, len(values))
	for _, raw := range values {
		value := sourceText(raw)
		if !validAbsoluteURL(value) {
			continue
		}
		result = append(result, socialhub.Media{ID: value, URL: value, Type: mediaType, State: socialhub.MediaStateReady})
	}
	return result
}

func sourceRelations(values []json.RawMessage, relationType socialhub.RelationType) []socialhub.PostRelation {
	result := make([]socialhub.PostRelation, 0, len(values))
	for _, raw := range values {
		var value string
		if json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != "" {
			result = append(result, socialhub.PostRelation{Type: relationType, PostID: value})
		}
	}
	return result
}
