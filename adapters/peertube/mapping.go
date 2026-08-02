package peertube

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) mapAccount(input Account) (*socialhub.User, error) {
	if input.ID < 1 || input.Name == "" {
		return nil, platformError("map_account", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	extension, _ := json.Marshal(input)
	username := input.Name
	if input.Host != "" && !strings.Contains(username, "@") {
		username += "@" + input.Host
	}
	return &socialhub.User{
		Platform: "peertube", AccountID: c.accountID, ID: strconv.FormatInt(input.ID, 10),
		Username: stringPointer(username), DisplayName: stringPointer(input.DisplayName),
		AvatarURL: stringPointer(c.actorImageURL(input.Avatars)), ProfileURL: stringPointer(c.resolveURL(input.URL)),
		AccountType: stringPointer("federated_account"), Extensions: map[string]json.RawMessage{"peertube.account": extension},
	}, nil
}

func (c *Client) mapVideo(input Video) (*socialhub.Post, error) {
	postID := input.UUID
	if postID == "" && input.ID > 0 {
		postID = strconv.FormatInt(input.ID, 10)
	}
	if !validResourceID(postID) || input.Name == "" {
		return nil, platformError("map_video", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	text := input.Name
	if input.Description != nil && *input.Description != "" {
		text = *input.Description
	} else if input.TruncatedDescription != nil && *input.TruncatedDescription != "" {
		text = *input.TruncatedDescription
	}
	mediaState, publishState := videoStates(input.State.ID)
	media := socialhub.Media{ID: postID, Type: socialhub.MediaTypeVideo, State: mediaState}
	if input.Duration > 0 {
		duration := time.Duration(input.Duration) * time.Second
		media.Duration = &duration
	}
	if file := preferredVideoFile(input); file != nil {
		media.URL = c.resolveURL(file.FileURL)
		if file.Size > 0 {
			media.Size = &file.Size
		}
		if file.Width > 0 {
			width := int(file.Width)
			media.Width = &width
		}
		if file.Height > 0 {
			height := int(file.Height)
			media.Height = &height
		}
	} else if len(input.StreamingPlaylists) > 0 {
		media.URL = c.resolveURL(input.StreamingPlaylists[0].PlaylistURL)
	}
	extension, _ := json.Marshal(input)
	post := &socialhub.Post{
		Platform: "peertube", AccountID: c.accountID, ID: postID, Text: &text,
		Media: []socialhub.Media{media}, CreatedAt: firstTime(input.PublishedAt, input.CreatedAt),
		Visibility: stringPointer(privacyName(input.Privacy)),
		Status:     &socialhub.PublishStatus{ID: postID, State: publishState, UpdatedAt: input.UpdatedAt},
		Extensions: map[string]json.RawMessage{"peertube.video": extension},
		Metrics: []socialhub.Metric{
			{Name: "views", Value: float64(input.Views), AsOf: c.clock.Now(), Definition: "PeerTube video view count"},
			{Name: "likes", Value: float64(input.Likes), AsOf: c.clock.Now(), Definition: "PeerTube video like count"},
			{Name: "dislikes", Value: float64(input.Dislikes), AsOf: c.clock.Now(), Definition: "PeerTube video dislike count"},
			{Name: "comments", Value: float64(input.Comments), AsOf: c.clock.Now(), Definition: "PeerTube video comment count"},
		},
	}
	if input.Account.ID > 0 {
		post.AuthorID = stringPointer(strconv.FormatInt(input.Account.ID, 10))
	}
	if input.URL != "" {
		post.URL = stringPointer(c.resolveURL(input.URL))
	}
	return post, nil
}

func (c *Client) mapComment(postID string, input VideoComment) (socialhub.Comment, error) {
	if input.ID < 1 {
		return socialhub.Comment{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	extension, _ := json.Marshal(input)
	comment := socialhub.Comment{
		Platform: "peertube", AccountID: c.accountID, ID: strconv.FormatInt(input.ID, 10), PostID: postID,
		Text: input.Text, CreatedAt: input.CreatedAt, Extensions: map[string]json.RawMessage{"peertube.comment": extension},
		Metrics: []socialhub.Metric{{Name: "replies", Value: float64(input.TotalReplies), AsOf: c.clock.Now(), Definition: "PeerTube comment reply count"}},
	}
	if input.Account.ID > 0 {
		comment.AuthorID = stringPointer(strconv.FormatInt(input.Account.ID, 10))
	}
	if input.InReplyToCommentID != nil {
		parent := strconv.FormatInt(*input.InReplyToCommentID, 10)
		comment.ParentID = &parent
	}
	return comment, nil
}

func videoStates(state int) (socialhub.MediaState, socialhub.PublishState) {
	switch state {
	case 1, 5:
		return socialhub.MediaStateReady, socialhub.PublishStatePublished
	case 3:
		return socialhub.MediaStateUploading, socialhub.PublishStatePending
	case 4:
		return socialhub.MediaStateCreated, socialhub.PublishStatePending
	case 7, 8:
		return socialhub.MediaStateFailed, socialhub.PublishStateFailed
	default:
		return socialhub.MediaStateProcessing, socialhub.PublishStatePending
	}
}

func privacyName(value NumberConstant) string {
	if value.Label != "" {
		return strings.ToLower(strings.ReplaceAll(value.Label, " ", "_"))
	}
	switch value.ID {
	case 1:
		return "public"
	case 2:
		return "unlisted"
	case 3:
		return "private"
	case 4:
		return "internal"
	case 5:
		return "password_protected"
	default:
		return ""
	}
}

func preferredVideoFile(input Video) *VideoFile {
	var selected *VideoFile
	consider := func(file *VideoFile) {
		if file == nil || file.FileURL == "" {
			return
		}
		if selected == nil || file.Width*file.Height > selected.Width*selected.Height {
			copy := *file
			selected = &copy
		}
	}
	for i := range input.Files {
		consider(&input.Files[i])
	}
	for i := range input.StreamingPlaylists {
		for j := range input.StreamingPlaylists[i].Files {
			consider(&input.StreamingPlaylists[i].Files[j])
		}
	}
	return selected
}

func (c *Client) actorImageURL(images []ActorImage) string {
	for _, image := range images {
		if value := firstNonEmpty(image.FileURL, image.Path); value != "" {
			return c.resolveURL(value)
		}
	}
	return ""
}

func (c *Client) resolveURL(value string) string {
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, _ := url.Parse(c.instanceURL)
	return base.ResolveReference(parsed).String()
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func firstTime(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil && !value.IsZero() {
			return value
		}
	}
	return nil
}
