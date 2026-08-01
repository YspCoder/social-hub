package vimeo

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) mapUser(input vimeoUser) (*socialhub.User, error) {
	id, err := resourceID(input.URI, "users")
	if err != nil {
		return nil, platformError("map_user", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	extension, _ := json.Marshal(struct {
		Location    string `json:"location,omitempty"`
		Bio         string `json:"bio,omitempty"`
		ResourceKey string `json:"resource_key,omitempty"`
		Websites    any    `json:"websites,omitempty"`
	}{input.Location, input.Bio, input.ResourceKey, input.Websites})
	return &socialhub.User{
		Platform: "vimeo", AccountID: c.accountID, ID: id,
		DisplayName: stringPointer(input.Name), AvatarURL: stringPointer(bestPicture(input.Pictures).Link),
		ProfileURL: stringPointer(input.Link), AccountType: stringPointer(input.Account),
		Extensions: map[string]json.RawMessage{"vimeo.user": extension},
	}, nil
}

func (c *Client) mapVideo(input vimeoVideo) (*socialhub.Post, error) {
	id, err := resourceID(input.URI, "videos")
	if err != nil {
		return nil, platformError("map_video", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	authorID, _ := resourceID(input.User.URI, "users")
	state := videoPublishState(input)
	duration := time.Duration(input.Duration) * time.Second
	mediaExtension, _ := json.Marshal(struct {
		ThumbnailURL    string `json:"thumbnail_url,omitempty"`
		UploadStatus    string `json:"upload_status,omitempty"`
		TranscodeStatus string `json:"transcode_status,omitempty"`
	}{bestPicture(input.Pictures).Link, input.Upload.Status, input.Transcode.Status})
	postExtension, _ := json.Marshal(struct {
		Name        string `json:"name,omitempty"`
		Status      string `json:"status,omitempty"`
		ResourceKey string `json:"resource_key,omitempty"`
	}{input.Name, input.Status, input.ResourceKey})
	createdAt := input.ReleaseTime
	if createdAt == nil {
		createdAt = input.CreatedTime
	}
	post := &socialhub.Post{
		Platform: "vimeo", AccountID: c.accountID, ID: id, AuthorID: stringPointer(authorID),
		Text: stringPointer(firstNonEmpty(input.Description, input.Name)), CreatedAt: createdAt,
		URL: stringPointer(input.Link), Visibility: stringPointer(input.Privacy.View),
		Status: &socialhub.PublishStatus{ID: id, State: state, UpdatedAt: input.ModifiedTime},
		Media: []socialhub.Media{{
			ID: id, Type: socialhub.MediaTypeVideo, Width: intPointer(input.Width), Height: intPointer(input.Height),
			Duration: durationPointer(duration), State: mediaState(state),
			Extensions: map[string]json.RawMessage{"vimeo.video": mediaExtension},
		}},
		Extensions: map[string]json.RawMessage{"vimeo.video": postExtension},
	}
	observedAt := c.clock.Now()
	for name, value := range map[string]*int64{
		"plays":    input.Stats.Plays,
		"comments": input.Metadata.Connections.Comments.Total,
		"likes":    input.Metadata.Connections.Likes.Total,
	} {
		if value != nil {
			post.Metrics = append(post.Metrics, socialhub.Metric{Name: name, Value: float64(*value), AsOf: observedAt, Definition: "Vimeo video " + name + " total"})
		}
	}
	return post, nil
}

func (c *Client) mapActivity(input vimeoActivity) (*socialhub.Post, error) {
	post, err := c.mapVideo(input.Clip)
	if err != nil {
		return nil, err
	}
	actorID, _ := resourceID(input.User.URI, "users")
	extension, _ := json.Marshal(struct {
		Type    string     `json:"type,omitempty"`
		Time    *time.Time `json:"time,omitempty"`
		ActorID string     `json:"actor_id,omitempty"`
	}{input.Type, input.Time, actorID})
	if post.Extensions == nil {
		post.Extensions = make(map[string]json.RawMessage)
	}
	post.Extensions["vimeo.activity"] = extension
	return post, nil
}

func (c *Client) mapComment(videoID string, parentID *string, input vimeoComment) (socialhub.Comment, error) {
	commentID, err := resourceID(input.URI, "comments")
	if err != nil {
		return socialhub.Comment{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	author := input.Metadata.Connections.User
	if author.URI == "" {
		author = input.User
	}
	authorID, _ := resourceID(author.URI, "users")
	compositeID := joinCommentID(videoID, commentID)
	var compositeParent *string
	if parentID != nil {
		value := joinCommentID(videoID, *parentID)
		compositeParent = &value
	}
	extension, _ := json.Marshal(struct {
		Type        string `json:"type,omitempty"`
		ResourceKey string `json:"resource_key,omitempty"`
	}{input.Type, input.ResourceKey})
	return socialhub.Comment{
		Platform: "vimeo", AccountID: c.accountID, ID: compositeID, PostID: videoID,
		AuthorID: stringPointer(authorID), ParentID: compositeParent, Text: input.Text,
		CreatedAt: input.CreatedOn, Extensions: map[string]json.RawMessage{"vimeo.comment": extension},
	}, nil
}

func videoPublishState(input vimeoVideo) socialhub.PublishState {
	switch input.Status {
	case "available":
		return socialhub.PublishStatePublished
	case "uploading_error", "transcoding_error", "quota_exceeded":
		return socialhub.PublishStateFailed
	case "uploading", "transcoding":
		return socialhub.PublishStatePending
	}
	if input.Upload.Status == "error" || input.Transcode.Status == "error" {
		return socialhub.PublishStateFailed
	}
	return socialhub.PublishStatePending
}

func mediaState(state socialhub.PublishState) socialhub.MediaState {
	switch state {
	case socialhub.PublishStatePublished:
		return socialhub.MediaStateReady
	case socialhub.PublishStateFailed:
		return socialhub.MediaStateFailed
	default:
		return socialhub.MediaStateProcessing
	}
}

func resourceID(uri, collection string) (string, error) {
	parts := strings.Split(strings.Trim(uri, "/"), "/")
	for index := len(parts) - 2; index >= 0; index-- {
		if parts[index] == collection && index+1 < len(parts) && validResourceID(parts[index+1]) {
			return parts[index+1], nil
		}
	}
	return "", fmt.Errorf("invalid Vimeo %s URI", collection)
}

func joinCommentID(videoID, commentID string) string { return videoID + "/" + commentID }

func splitCommentID(value string) (string, string, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 || !validResourceID(parts[0]) || !validResourceID(parts[1]) {
		return "", "", fmt.Errorf("invalid composite comment ID")
	}
	return parts[0], parts[1], nil
}

func bestPicture(input vimeoPicture) struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Link   string `json:"link"`
} {
	var best struct {
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Link   string `json:"link"`
	}
	for _, size := range input.Sizes {
		if size.Width*size.Height > best.Width*best.Height {
			best = size
		}
	}
	return best
}

func pageCursors(paging struct {
	Next     string `json:"next"`
	Previous string `json:"previous"`
}) (*string, *string, error) {
	next, err := pageCursor(paging.Next)
	if err != nil {
		return nil, nil, err
	}
	previous, err := pageCursor(paging.Previous)
	if err != nil {
		return nil, nil, err
	}
	return next, previous, nil
}

func pageCursor(value string) (*string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid paging URL")
	}
	page := parsed.Query().Get("page")
	if number, err := strconv.Atoi(page); err != nil || number <= 0 {
		return nil, fmt.Errorf("invalid paging page")
	}
	return &page, nil
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

func durationPointer(value time.Duration) *time.Duration {
	if value <= 0 {
		return nil
	}
	copy := value
	return &copy
}
