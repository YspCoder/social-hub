package threads

import (
	"encoding/json"
	"strings"

	"social-hub/pkg/socialhub"
)

func mapProfile(accountID socialhub.AccountID, input graphProfile) *socialhub.User {
	extension, _ := json.Marshal(input)
	return &socialhub.User{
		Platform: "threads", AccountID: accountID, ID: input.ID,
		Username: stringPointer(input.Username), DisplayName: stringPointer(input.Name),
		AvatarURL: stringPointer(input.ProfilePictureURL), ProfileURL: stringPointer(profileURL(input.Username)),
		AccountType: stringPointer("person"), Extensions: map[string]json.RawMessage{"threads.profile": extension},
	}
}

func mapPost(accountID socialhub.AccountID, configuredUserID string, input graphPost) (*socialhub.Post, error) {
	if strings.TrimSpace(input.ID) == "" {
		return nil, platformError("map_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	authorID := input.Owner.ID
	if authorID == "" {
		authorID = configuredUserID
	}
	post := &socialhub.Post{
		Platform: "threads", AccountID: accountID, ID: input.ID, AuthorID: stringPointer(authorID),
		Text: stringPointer(input.Text), Media: mapPostMedia(input), CreatedAt: input.Timestamp.pointer(),
		URL: stringPointer(input.Permalink), Visibility: stringPointer("public"),
		Status: &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: input.Timestamp.pointer()},
	}
	if input.IsReply && input.RepliedTo.ID != "" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationReply, PostID: input.RepliedTo.ID})
	}
	if input.IsQuotePost && input.QuotedPost != nil && input.QuotedPost.ID != "" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationQuote, PostID: input.QuotedPost.ID})
	}
	if input.RepostedPost != nil && input.RepostedPost.ID != "" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationRepost, PostID: input.RepostedPost.ID})
	}
	extension, _ := json.Marshal(input)
	post.Extensions = map[string]json.RawMessage{"threads.post": extension}
	return post, nil
}

func mapPostMedia(input graphPost) []socialhub.Media {
	if len(input.Children.Data) > 0 {
		media := make([]socialhub.Media, 0, len(input.Children.Data))
		for _, child := range input.Children.Data {
			media = append(media, mapPostMedia(child)...)
		}
		return media
	}
	if input.GIFURL != "" {
		return []socialhub.Media{{
			ID: input.ID + "#gif", URL: input.GIFURL, Type: socialhub.MediaTypeAnimation, State: socialhub.MediaStateReady,
			Extensions: mediaExtension(input),
		}}
	}
	if input.MediaURL == "" {
		return nil
	}
	mediaType := socialhub.MediaTypeImage
	if strings.Contains(strings.ToUpper(input.MediaType), "VIDEO") {
		mediaType = socialhub.MediaTypeVideo
	}
	return []socialhub.Media{{
		ID: input.ID, URL: input.MediaURL, Type: mediaType, State: socialhub.MediaStateReady,
		Extensions: mediaExtension(input),
	}}
}

func mediaExtension(input graphPost) map[string]json.RawMessage {
	encoded, _ := json.Marshal(struct {
		MediaType string `json:"media_type,omitempty"`
		Thumbnail string `json:"thumbnail_url,omitempty"`
		AltText   string `json:"alt_text,omitempty"`
		Spoiler   bool   `json:"is_spoiler_media,omitempty"`
	}{input.MediaType, input.ThumbnailURL, input.AltText, input.IsSpoilerMedia})
	return map[string]json.RawMessage{"threads.media": encoded}
}

func mapPostPage(accountID socialhub.AccountID, configuredUserID string, response graphPostPage) (socialhub.Page[socialhub.Post], error) {
	items := make([]socialhub.Post, 0, len(response.Data))
	for _, item := range response.Data {
		post, err := mapPost(accountID, configuredUserID, item)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	next, previous := stringPointer(response.Paging.Cursors.After), stringPointer(response.Paging.Cursors.Before)
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func mapCommentPage(accountID socialhub.AccountID, rootPostID string, response graphPostPage) (socialhub.Page[socialhub.Comment], error) {
	items := make([]socialhub.Comment, 0, len(response.Data))
	for _, item := range response.Data {
		if item.ID == "" {
			return socialhub.Page[socialhub.Comment]{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		postID := rootPostID
		if item.RootPost.ID != "" {
			postID = item.RootPost.ID
		}
		extension, _ := json.Marshal(item)
		items = append(items, socialhub.Comment{
			Platform: "threads", AccountID: accountID, ID: item.ID, PostID: postID,
			AuthorID: stringPointer(item.Owner.ID), ParentID: stringPointer(item.RepliedTo.ID),
			Text: item.Text, CreatedAt: item.Timestamp.pointer(), Extensions: map[string]json.RawMessage{"threads.reply": extension},
		})
	}
	next, previous := stringPointer(response.Paging.Cursors.After), stringPointer(response.Paging.Cursors.Before)
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func mapInsights(response graphInsightPage) []Insight {
	items := make([]Insight, 0, len(response.Data))
	for _, input := range response.Data {
		item := Insight{
			ID: input.ID, Name: input.Name, Period: input.Period, Title: input.Title, Description: input.Description,
			Values: make([]InsightValue, 0, len(input.Values)),
		}
		for _, value := range input.Values {
			item.Values = append(item.Values, InsightValue{Value: append(json.RawMessage(nil), value.Value...), EndTime: value.EndTime.pointer()})
		}
		if input.TotalValue != nil {
			item.TotalValue = &InsightValue{
				Value: append(json.RawMessage(nil), input.TotalValue.Value...), EndTime: input.TotalValue.EndTime.pointer(),
			}
		}
		items = append(items, item)
	}
	return items
}

func profileURL(username string) string {
	if strings.TrimSpace(username) == "" {
		return ""
	}
	return "https://www.threads.com/@" + username
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}
