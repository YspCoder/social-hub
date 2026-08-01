package bluesky

import (
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapActor(accountID socialhub.AccountID, input bskyActor) *socialhub.User {
	extension, _ := json.Marshal(struct {
		Description    string          `json:"description,omitempty"`
		Pronouns       string          `json:"pronouns,omitempty"`
		Website        string          `json:"website,omitempty"`
		Banner         string          `json:"banner,omitempty"`
		FollowersCount *int64          `json:"followers_count,omitempty"`
		FollowsCount   *int64          `json:"follows_count,omitempty"`
		PostsCount     *int64          `json:"posts_count,omitempty"`
		CreatedAt      *time.Time      `json:"created_at,omitempty"`
		IndexedAt      *time.Time      `json:"indexed_at,omitempty"`
		Associated     json.RawMessage `json:"associated,omitempty"`
		Labels         json.RawMessage `json:"labels,omitempty"`
		Verification   json.RawMessage `json:"verification,omitempty"`
	}{
		input.Description, input.Pronouns, input.Website, input.Banner, input.FollowersCount, input.FollowsCount,
		input.PostsCount, input.CreatedAt, input.IndexedAt, input.Associated, input.Labels, input.Verification,
	})
	return &socialhub.User{
		Platform: "bluesky", AccountID: accountID, ID: input.DID,
		Username: stringPointer(input.Handle), DisplayName: stringPointer(input.DisplayName), AvatarURL: stringPointer(input.Avatar),
		ProfileURL: stringPointer(profileURL(input.Handle)), AccountType: stringPointer("person"),
		Extensions: map[string]json.RawMessage{"bluesky.profile": extension},
	}
}

func mapPost(accountID socialhub.AccountID, input bskyPostView, observedAt time.Time) (*socialhub.Post, error) {
	var record bskyPostRecord
	if err := json.Unmarshal(input.Record, &record); err != nil {
		return nil, platformError("map_post", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if input.URI == "" || input.CID == "" || input.Author.DID == "" {
		return nil, platformError("map_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	createdAt := record.CreatedAt
	if createdAt == nil {
		createdAt = input.IndexedAt
	}
	post := &socialhub.Post{
		Platform: "bluesky", AccountID: accountID, ID: input.URI, AuthorID: stringPointer(input.Author.DID),
		Text: stringPointer(record.Text), CreatedAt: createdAt, URL: stringPointer(postURL(input.Author.Handle, input.URI)),
		Visibility: stringPointer("public"),
		Status:     &socialhub.PublishStatus{ID: input.URI, State: socialhub.PublishStatePublished, UpdatedAt: input.IndexedAt},
	}
	if record.Reply != nil && record.Reply.Parent.URI != "" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationReply, PostID: record.Reply.Parent.URI})
	}
	media, quoteURI := mapEmbed(input.Embed)
	post.Media = media
	if quoteURI != "" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationQuote, PostID: quoteURI})
	}
	post.Metrics = appendMetric(post.Metrics, "replies", input.ReplyCount, observedAt, "Bluesky post reply count")
	post.Metrics = appendMetric(post.Metrics, "reposts", input.RepostCount, observedAt, "Bluesky post repost count")
	post.Metrics = appendMetric(post.Metrics, "likes", input.LikeCount, observedAt, "Bluesky post like count")
	post.Metrics = appendMetric(post.Metrics, "quotes", input.QuoteCount, observedAt, "Bluesky post quote count")
	post.Metrics = appendMetric(post.Metrics, "bookmarks", input.BookmarkCount, observedAt, "Viewer-independent bookmark count when exposed")
	extension, _ := json.Marshal(struct {
		CID       string          `json:"cid"`
		IndexedAt *time.Time      `json:"indexed_at,omitempty"`
		Record    json.RawMessage `json:"record"`
		Embed     json.RawMessage `json:"embed,omitempty"`
		Viewer    bskyViewerState `json:"viewer,omitempty"`
		Labels    json.RawMessage `json:"labels,omitempty"`
	}{input.CID, input.IndexedAt, input.Record, input.Embed, input.Viewer, input.Labels})
	post.Extensions = map[string]json.RawMessage{"bluesky.post": extension}
	return post, nil
}

func mapFeedItem(accountID socialhub.AccountID, item bskyFeedItem, observedAt time.Time) (*socialhub.Post, error) {
	post, err := mapPost(accountID, item.Post, observedAt)
	if err != nil {
		return nil, err
	}
	if item.Reason.Type != "app.bsky.feed.defs#reasonRepost" || item.Reason.URI == "" {
		return post, nil
	}
	originalID := post.ID
	post.ID = item.Reason.URI
	post.AuthorID = stringPointer(item.Reason.By.DID)
	post.CreatedAt = item.Reason.IndexedAt
	post.URL = nil
	post.Status = &socialhub.PublishStatus{ID: item.Reason.URI, State: socialhub.PublishStatePublished, UpdatedAt: item.Reason.IndexedAt}
	post.Relations = append([]socialhub.PostRelation{{Type: socialhub.RelationRepost, PostID: originalID}}, post.Relations...)
	reason, _ := json.Marshal(item.Reason)
	post.Extensions["bluesky.feed_reason"] = reason
	return post, nil
}

func mapFeedPage(accountID socialhub.AccountID, response bskyFeedResponse, observedAt time.Time) (socialhub.Page[socialhub.Post], error) {
	items := make([]socialhub.Post, 0, len(response.Feed))
	for _, item := range response.Feed {
		post, err := mapFeedItem(accountID, item, observedAt)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	next := pageCursor(response.Cursor)
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func mapComment(accountID socialhub.AccountID, rootURI string, input bskyPostView) (socialhub.Comment, error) {
	var record bskyPostRecord
	if err := json.Unmarshal(input.Record, &record); err != nil {
		return socialhub.Comment{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	comment := socialhub.Comment{
		Platform: "bluesky", AccountID: accountID, ID: input.URI, PostID: rootURI,
		AuthorID: stringPointer(input.Author.DID), Text: record.Text, CreatedAt: record.CreatedAt,
	}
	if record.Reply != nil {
		comment.ParentID = stringPointer(record.Reply.Parent.URI)
	}
	return comment, nil
}

func mapEmbed(raw json.RawMessage) ([]socialhub.Media, string) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, ""
	}
	var envelope struct {
		Type      string          `json:"$type"`
		Images    []imageView     `json:"images"`
		Items     []galleryView   `json:"items"`
		CID       string          `json:"cid"`
		Playlist  string          `json:"playlist"`
		Thumbnail string          `json:"thumbnail"`
		Alt       string          `json:"alt"`
		Record    json.RawMessage `json:"record"`
		Media     json.RawMessage `json:"media"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return nil, ""
	}
	switch envelope.Type {
	case "app.bsky.embed.images#view":
		return mapImageViews(envelope.Images), ""
	case "app.bsky.embed.gallery#view":
		media := make([]socialhub.Media, 0, len(envelope.Items))
		for _, item := range envelope.Items {
			media = append(media, mapImageView(item.imageView()))
		}
		return media, ""
	case "app.bsky.embed.video#view":
		extension, _ := json.Marshal(struct {
			Thumbnail string `json:"thumbnail,omitempty"`
			Alt       string `json:"alt,omitempty"`
		}{envelope.Thumbnail, envelope.Alt})
		return []socialhub.Media{{
			ID: envelope.CID, URL: envelope.Playlist, Type: socialhub.MediaTypeVideo, State: socialhub.MediaStateReady,
			Extensions: map[string]json.RawMessage{"bluesky.video": extension},
		}}, ""
	case "app.bsky.embed.record#view":
		return nil, embeddedRecordURI(envelope.Record)
	case "app.bsky.embed.recordWithMedia#view":
		media, _ := mapEmbed(envelope.Media)
		return media, embeddedRecordURI(envelope.Record)
	default:
		return nil, ""
	}
}

type imageView struct {
	Thumb       string       `json:"thumb"`
	Fullsize    string       `json:"fullsize"`
	Alt         string       `json:"alt"`
	AspectRatio *aspectRatio `json:"aspectRatio"`
}

type galleryView struct {
	Type        string       `json:"$type"`
	Thumbnail   string       `json:"thumbnail"`
	Fullsize    string       `json:"fullsize"`
	Alt         string       `json:"alt"`
	AspectRatio *aspectRatio `json:"aspectRatio"`
}

func (g galleryView) imageView() imageView {
	return imageView{Thumb: g.Thumbnail, Fullsize: g.Fullsize, Alt: g.Alt, AspectRatio: g.AspectRatio}
}

func mapImageViews(images []imageView) []socialhub.Media {
	media := make([]socialhub.Media, 0, len(images))
	for _, image := range images {
		media = append(media, mapImageView(image))
	}
	return media
}

func mapImageView(input imageView) socialhub.Media {
	extension, _ := json.Marshal(struct {
		Thumbnail   string       `json:"thumbnail,omitempty"`
		Alt         string       `json:"alt,omitempty"`
		AspectRatio *aspectRatio `json:"aspect_ratio,omitempty"`
	}{input.Thumb, input.Alt, input.AspectRatio})
	media := socialhub.Media{
		ID: input.Fullsize, URL: input.Fullsize, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady,
		Extensions: map[string]json.RawMessage{"bluesky.image": extension},
	}
	if input.AspectRatio != nil {
		media.Width, media.Height = intPointer(input.AspectRatio.Width), intPointer(input.AspectRatio.Height)
	}
	return media
}

func embeddedRecordURI(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var record struct {
		URI    string          `json:"uri"`
		Record json.RawMessage `json:"record"`
	}
	if json.Unmarshal(raw, &record) != nil {
		return ""
	}
	if record.URI != "" {
		return record.URI
	}
	return embeddedRecordURI(record.Record)
}

func appendMetric(metrics []socialhub.Metric, name string, value *int64, observedAt time.Time, definition string) []socialhub.Metric {
	if value == nil {
		return metrics
	}
	return append(metrics, socialhub.Metric{Name: name, Value: float64(*value), AsOf: observedAt, Definition: definition})
}

func profileURL(handle string) string {
	if handle == "" {
		return ""
	}
	return "https://bsky.app/profile/" + handle
}

func postURL(handle, uri string) string {
	if handle == "" {
		return ""
	}
	parsed, err := parseRecordURI(uri)
	if err != nil || parsed.Collection != collectionPost {
		return ""
	}
	return profileURL(handle) + "/post/" + parsed.RecordKey
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func intPointer(value int) *int {
	if value <= 0 {
		return nil
	}
	copy := value
	return &copy
}
