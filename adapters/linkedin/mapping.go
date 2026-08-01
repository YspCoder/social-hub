package linkedin

import (
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input userInfo) *socialhub.User {
	memberURN := input.Sub
	if !strings.HasPrefix(memberURN, "urn:li:person:") {
		memberURN = "urn:li:person:" + memberURN
	}
	extension, _ := json.Marshal(struct {
		Subject       string          `json:"subject"`
		Email         string          `json:"email,omitempty"`
		EmailVerified bool            `json:"email_verified,omitempty"`
		Locale        json.RawMessage `json:"locale,omitempty"`
	}{Subject: input.Sub, Email: input.Email, EmailVerified: input.EmailVerified, Locale: input.Locale})
	return &socialhub.User{
		Platform: "linkedin", AccountID: accountID, ID: memberURN, DisplayName: stringPointer(input.Name),
		AvatarURL:   stringPointer(input.Picture),
		AccountType: stringPointer("member"), Extensions: map[string]json.RawMessage{"linkedin.oidc": extension},
	}
}

func mapPost(accountID socialhub.AccountID, input linkedInPost) *socialhub.Post {
	createdAt := milliseconds(input.PublishedAt)
	if createdAt == nil {
		createdAt = milliseconds(input.CreatedAt)
	}
	post := &socialhub.Post{
		Platform: "linkedin", AccountID: accountID, ID: input.ID, AuthorID: stringPointer(input.Author), Text: stringPointer(input.Commentary),
		CreatedAt: createdAt, URL: linkedInPostURL(input.ID), Visibility: stringPointer(strings.ToLower(input.Visibility)),
		Status: &socialhub.PublishStatus{ID: input.ID, State: publishState(input.Lifecycle), UpdatedAt: createdAt},
	}
	if input.Content.Media != nil {
		post.Media = append(post.Media, mediaFromURN(input.Content.Media.ID))
	}
	if input.Content.MultiImage != nil {
		for _, image := range input.Content.MultiImage.Images {
			post.Media = append(post.Media, mediaFromURN(image.ID))
		}
	}
	if input.Content.Article != nil {
		extension, _ := json.Marshal(input.Content.Article)
		post.Media = append(post.Media, socialhub.Media{ID: input.Content.Article.Thumbnail, URL: input.Content.Article.Source, Type: socialhub.MediaTypeDocument, State: socialhub.MediaStateReady, Extensions: map[string]json.RawMessage{"linkedin.article": extension}})
	}
	if input.ReshareContext != nil && input.ReshareContext.Parent != "" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationRepost, PostID: input.ReshareContext.Parent})
	}
	extension, _ := json.Marshal(struct {
		Lifecycle string `json:"lifecycle_state,omitempty"`
	}{Lifecycle: input.Lifecycle})
	post.Extensions = map[string]json.RawMessage{"linkedin.post": extension}
	return post
}

func mapComment(accountID socialhub.AccountID, postID string, input linkedInComment) socialhub.Comment {
	actor := firstNonEmpty(input.Actor, input.Created.Actor)
	return socialhub.Comment{
		Platform: "linkedin", AccountID: accountID, ID: string(input.ID), PostID: postID,
		AuthorID: stringPointer(actor), ParentID: stringPointer(input.ParentComment), Text: input.Message.Text, CreatedAt: milliseconds(input.Created.Time),
	}
}

func mediaFromURN(urn string) socialhub.Media {
	typeOfMedia := socialhub.MediaTypeDocument
	switch {
	case strings.HasPrefix(urn, "urn:li:image:"):
		typeOfMedia = socialhub.MediaTypeImage
	case strings.HasPrefix(urn, "urn:li:video:"):
		typeOfMedia = socialhub.MediaTypeVideo
	case strings.HasPrefix(urn, "urn:li:document:"):
		typeOfMedia = socialhub.MediaTypeDocument
	}
	return socialhub.Media{ID: urn, Type: typeOfMedia, State: socialhub.MediaStateReady}
}

func publishState(value string) socialhub.PublishState {
	if value == "PUBLISHED" {
		return socialhub.PublishStatePublished
	}
	if value == "PROCESSING" || value == "PUBLISH_REQUESTED" {
		return socialhub.PublishStatePending
	}
	return socialhub.PublishStateFailed
}

func milliseconds(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	result := time.UnixMilli(value).UTC()
	return &result
}

func linkedInPostURL(id string) *string {
	if id == "" {
		return nil
	}
	return stringPointer("https://www.linkedin.com/feed/update/" + id)
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
