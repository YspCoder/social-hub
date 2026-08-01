package mastodon

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapAccount(accountID socialhub.AccountID, input mastodonAccount) *socialhub.User {
	extension, _ := json.Marshal(struct {
		Acct           string          `json:"acct,omitempty"`
		URI            string          `json:"uri,omitempty"`
		NoteHTML       string          `json:"note_html,omitempty"`
		Header         string          `json:"header,omitempty"`
		HeaderStatic   string          `json:"header_static,omitempty"`
		Locked         bool            `json:"locked,omitempty"`
		Bot            bool            `json:"bot,omitempty"`
		Discoverable   *bool           `json:"discoverable,omitempty"`
		Group          bool            `json:"group,omitempty"`
		CreatedAt      *time.Time      `json:"created_at,omitempty"`
		FollowersCount int64           `json:"followers_count,omitempty"`
		FollowingCount int64           `json:"following_count,omitempty"`
		StatusesCount  int64           `json:"statuses_count,omitempty"`
		LastStatusAt   string          `json:"last_status_at,omitempty"`
		Fields         json.RawMessage `json:"fields,omitempty"`
	}{
		input.Acct, input.URI, input.Note, input.Header, input.HeaderStatic, input.Locked, input.Bot, input.Discoverable,
		input.Group, input.CreatedAt, input.FollowersCount, input.FollowingCount, input.StatusesCount, input.LastStatusAt, input.Fields,
	})
	accountType := "person"
	if input.Bot {
		accountType = "bot"
	} else if input.Group {
		accountType = "group"
	}
	return &socialhub.User{
		Platform: "mastodon", AccountID: accountID, ID: input.ID, Username: stringPointer(input.Acct),
		DisplayName: stringPointer(input.DisplayName), AvatarURL: stringPointer(firstNonEmpty(input.Avatar, input.AvatarStatic)),
		ProfileURL: stringPointer(input.URL), AccountType: stringPointer(accountType),
		Extensions: map[string]json.RawMessage{"mastodon.account": extension},
	}
}

func mapStatus(accountID socialhub.AccountID, input mastodonStatus, observedAt time.Time) *socialhub.Post {
	content := input
	post := &socialhub.Post{
		Platform: "mastodon", AccountID: accountID, ID: input.ID, AuthorID: stringPointer(input.Account.ID),
		CreatedAt: input.CreatedAt, URL: stringPointer(firstNonEmpty(input.URL, input.URI)), Visibility: stringPointer(input.Visibility),
		Status: &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: input.EditedAt},
	}
	if input.Reblog != nil {
		content = *input.Reblog
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationRepost, PostID: content.ID})
	}
	post.Text = stringPointer(content.Content)
	for _, attachment := range content.MediaAttachments {
		post.Media = append(post.Media, mapAttachment(attachment))
	}
	if input.InReplyToID != nil && *input.InReplyToID != "" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationReply, PostID: *input.InReplyToID})
	}
	quotedID := input.QuotedStatusID
	if quotedID == nil && input.Quote != nil && input.Quote.QuotedStatus != nil {
		quotedID = &input.Quote.QuotedStatus.ID
	}
	if quotedID != nil && *quotedID != "" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationQuote, PostID: *quotedID})
	}
	extension, _ := json.Marshal(struct {
		URI         string     `json:"uri,omitempty"`
		ContentHTML string     `json:"content_html,omitempty"`
		SpoilerText string     `json:"spoiler_text,omitempty"`
		Language    string     `json:"language,omitempty"`
		Sensitive   bool       `json:"sensitive,omitempty"`
		EditedAt    *time.Time `json:"edited_at,omitempty"`
		Favourited  *bool      `json:"favourited,omitempty"`
		Reblogged   *bool      `json:"reblogged,omitempty"`
		Bookmarked  *bool      `json:"bookmarked,omitempty"`
	}{input.URI, content.Content, input.SpoilerText, input.Language, input.Sensitive, input.EditedAt, input.Favourited, input.Reblogged, input.Bookmarked})
	post.Extensions = map[string]json.RawMessage{"mastodon.status": extension}
	post.Metrics = []socialhub.Metric{
		{Name: "replies", Value: float64(input.RepliesCount), AsOf: observedAt, Definition: "Mastodon status reply count"},
		{Name: "boosts", Value: float64(input.ReblogsCount), AsOf: observedAt, Definition: "Mastodon status reblog count"},
		{Name: "favourites", Value: float64(input.FavouritesCount), AsOf: observedAt, Definition: "Mastodon status favourite count"},
	}
	return post
}

func mapAttachment(input mastodonAttachment) socialhub.Media {
	mediaType := socialhub.MediaTypeDocument
	switch input.Type {
	case "image":
		mediaType = socialhub.MediaTypeImage
	case "video":
		mediaType = socialhub.MediaTypeVideo
	case "gifv":
		mediaType = socialhub.MediaTypeAnimation
	case "audio":
		mediaType = socialhub.MediaTypeAudio
	}
	state := socialhub.MediaStateProcessing
	if input.URL != "" {
		state = socialhub.MediaStateReady
	}
	var duration *time.Duration
	if input.Meta.Original.Duration > 0 {
		value := time.Duration(input.Meta.Original.Duration * float64(time.Second))
		duration = &value
	}
	extension, _ := json.Marshal(struct {
		RemoteURL   string                 `json:"remote_url,omitempty"`
		PreviewURL  string                 `json:"preview_url,omitempty"`
		TextURL     string                 `json:"text_url,omitempty"`
		Description string                 `json:"description,omitempty"`
		BlurHash    string                 `json:"blurhash,omitempty"`
		Meta        mastodonAttachmentMeta `json:"meta,omitempty"`
	}{input.RemoteURL, input.PreviewURL, input.TextURL, input.Description, input.BlurHash, input.Meta})
	return socialhub.Media{
		ID: input.ID, URL: input.URL, Type: mediaType, Width: intPointer(input.Meta.Original.Width), Height: intPointer(input.Meta.Original.Height),
		Duration: duration, State: state, Extensions: map[string]json.RawMessage{"mastodon.attachment": extension},
	}
}

func mapComment(accountID socialhub.AccountID, rootPostID string, input mastodonStatus) socialhub.Comment {
	return socialhub.Comment{
		Platform: "mastodon", AccountID: accountID, ID: input.ID, PostID: rootPostID,
		AuthorID: stringPointer(input.Account.ID), ParentID: input.InReplyToID, Text: input.Content, CreatedAt: input.CreatedAt,
	}
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func intPointer(value int64) *int {
	if value <= 0 || (strconv.IntSize == 32 && value > int64(^uint(0)>>1)) {
		return nil
	}
	converted := int(value)
	return &converted
}
