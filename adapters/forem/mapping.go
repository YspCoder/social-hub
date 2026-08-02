package forem

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) mapUser(input wireUser) *socialhub.User {
	id := input.identifier()
	accountType := "member"
	return &socialhub.User{
		Platform: "forem", AccountID: client.accountID, ID: strconv.FormatInt(id, 10),
		Username: stringPointer(input.Username), DisplayName: stringPointer(input.Name),
		AvatarURL:  client.absoluteURL(firstNonEmpty(input.ProfileImage, input.ProfileImage90)),
		ProfileURL: client.absoluteURL(client.baseURL + "/" + url.PathEscape(input.Username)), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"forem.user": append(json.RawMessage(nil), input.Raw...)},
	}
}

func (client *Client) mapPost(input wireArticle) *socialhub.Post {
	id := strconv.FormatInt(input.ID, 10)
	published := input.Published || input.PublishedAt != nil || input.PublishedTimestamp != nil
	state, visibility := socialhub.PublishStatePending, "draft"
	if published {
		state, visibility = socialhub.PublishStatePublished, "public"
	}
	createdAt := input.PublishedAt
	if createdAt == nil {
		createdAt = input.PublishedTimestamp
	}
	if createdAt == nil {
		createdAt = input.CreatedAt
	}
	observedAt := client.clock.Now()
	post := &socialhub.Post{
		Platform: "forem", AccountID: client.accountID, ID: id,
		Text:      stringPointer(firstNonEmpty(input.BodyMarkdown, input.Description, input.Title)),
		CreatedAt: createdAt, URL: client.absoluteURL(firstNonEmpty(input.URL, input.Path)), Visibility: &visibility,
		Status: &socialhub.PublishStatus{ID: id, State: state, UpdatedAt: input.EditedAt},
		Metrics: []socialhub.Metric{
			{Name: "comments", Value: float64(input.CommentsCount), AsOf: observedAt, Definition: "Forem Article comment count"},
			{Name: "positive_reactions", Value: float64(input.PositiveReactionsCount), AsOf: observedAt, Definition: "Forem positive reaction count"},
			{Name: "public_reactions", Value: float64(input.PublicReactionsCount), AsOf: observedAt, Definition: "Forem public reaction count"},
			{Name: "page_views", Value: float64(input.PageViewsCount), AsOf: observedAt, Definition: "Forem Article page view count"},
		},
		Extensions: map[string]json.RawMessage{"forem.article": append(json.RawMessage(nil), input.Raw...)},
	}
	if authorID := input.User.identifier(); authorID > 0 {
		post.AuthorID = stringPointer(strconv.FormatInt(authorID, 10))
	}
	if image := client.absoluteURL(firstNonEmpty(input.CoverImage, input.SocialImage)); image != nil {
		post.Media = append(post.Media, socialhub.Media{URL: *image, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady})
	}
	return post
}

func (client *Client) mapArticle(input wireArticle) Article {
	post := client.mapPost(input)
	return Article{
		Post: *post, Title: input.Title, Description: input.Description, Slug: input.Slug,
		CanonicalURL: input.CanonicalURL, Tags: input.tags(), BodyMarkdown: input.BodyMarkdown, BodyHTML: input.BodyHTML,
		Published: post.Status.State == socialhub.PublishStatePublished, ReadingTimeMinutes: input.ReadingTimeMinutes,
		CommentsCount: input.CommentsCount, PositiveReactionsCount: input.PositiveReactionsCount,
		PublicReactionsCount: input.PublicReactionsCount, PageViewsCount: input.PageViewsCount,
		Raw: append(json.RawMessage(nil), input.Raw...),
	}
}

func (client *Client) appendComments(items *[]socialhub.Comment, postID, parentID string, input []wireComment) error {
	for _, comment := range input {
		if !validCommentID(comment.IDCode) {
			return platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		mapped := socialhub.Comment{
			Platform: "forem", AccountID: client.accountID, ID: comment.IDCode, PostID: postID,
			Text: comment.BodyHTML, CreatedAt: comment.CreatedAt,
			Extensions: map[string]json.RawMessage{"forem.comment": append(json.RawMessage(nil), comment.Raw...)},
		}
		if parentID != "" {
			mapped.ParentID = stringPointer(parentID)
		}
		if authorID := comment.User.identifier(); authorID > 0 {
			mapped.AuthorID = stringPointer(strconv.FormatInt(authorID, 10))
		}
		*items = append(*items, mapped)
		if err := client.appendComments(items, postID, comment.IDCode, comment.Children); err != nil {
			return err
		}
	}
	return nil
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
