package misskey

import (
	"encoding/json"
	"mime"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) mapUser(input misskeyUser) (*socialhub.User, error) {
	if !validID(input.ID) || !validBoundedString(input.Username, 512) ||
		(input.AvatarURL != "" && !validHTTPURL(input.AvatarURL)) ||
		(input.URL != nil && *input.URL != "" && !validHTTPURL(*input.URL)) ||
		input.FollowersCount < 0 || input.FollowingCount < 0 || input.NotesCount < 0 {
		return nil, platformError("map_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	username := input.Username
	if input.Host != nil && *input.Host != "" {
		if !validBoundedString(*input.Host, 512) {
			return nil, platformError("map_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		username += "@" + *input.Host
	}
	displayName := input.Name
	if displayName != nil && !validOptionalString(*displayName, 4096) {
		return nil, platformError("map_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	profileURL := c.instanceURL + "/@" + input.Username
	if input.URL != nil && *input.URL != "" {
		profileURL = *input.URL
	}
	accountType := "person"
	if input.IsBot {
		accountType = "bot"
	}
	extension, _ := json.Marshal(struct {
		Host           *string    `json:"host,omitempty"`
		URI            *string    `json:"uri,omitempty"`
		CreatedAt      *time.Time `json:"created_at,omitempty"`
		Description    *string    `json:"description,omitempty"`
		Location       *string    `json:"location,omitempty"`
		Language       *string    `json:"language,omitempty"`
		BannerURL      *string    `json:"banner_url,omitempty"`
		IsCat          bool       `json:"is_cat,omitempty"`
		IsLocked       bool       `json:"is_locked,omitempty"`
		IsSilenced     bool       `json:"is_silenced,omitempty"`
		IsSuspended    bool       `json:"is_suspended,omitempty"`
		OnlineStatus   string     `json:"online_status,omitempty"`
		FollowersCount int64      `json:"followers_count"`
		FollowingCount int64      `json:"following_count"`
		NotesCount     int64      `json:"notes_count"`
	}{
		input.Host, input.URI, input.CreatedAt, input.Description, input.Location, input.Lang, input.BannerURL,
		input.IsCat, input.IsLocked, input.IsSilenced, input.IsSuspended, input.OnlineStatus,
		input.FollowersCount, input.FollowingCount, input.NotesCount,
	})
	return &socialhub.User{
		Platform: "misskey", AccountID: c.accountID, ID: input.ID, Username: stringPointer(username),
		DisplayName: displayName, AvatarURL: stringPointer(input.AvatarURL), ProfileURL: stringPointer(profileURL),
		AccountType: stringPointer(accountType), Extensions: map[string]json.RawMessage{"misskey.user": extension},
	}, nil
}

func (c *Client) mapNote(input misskeyNote) (*socialhub.Post, error) {
	if err := validateNote(input, true); err != nil {
		return nil, err
	}
	content := input
	pureRenote := input.RenoteID != nil && input.Text == nil && len(input.Files) == 0 && !hasPoll(input.Poll)
	if pureRenote && input.Renote != nil {
		content = *input.Renote
	}
	postURL := input.URL
	if postURL == "" {
		postURL = input.URI
	}
	if postURL == "" {
		postURL = c.instanceURL + "/notes/" + input.ID
	}
	post := &socialhub.Post{
		Platform: "misskey", AccountID: c.accountID, ID: input.ID, AuthorID: stringPointer(input.UserID),
		Text: content.Text, CreatedAt: input.CreatedAt, URL: stringPointer(postURL), Visibility: stringPointer(input.Visibility),
		Status: &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: input.CreatedAt},
		Metrics: []socialhub.Metric{
			{Name: "replies", Value: float64(input.RepliesCount), AsOf: c.clock.Now(), Definition: "Misskey note reply count"},
			{Name: "renotes", Value: float64(input.RenoteCount), AsOf: c.clock.Now(), Definition: "Misskey note Renote count"},
			{Name: "reactions", Value: float64(input.ReactionCount), AsOf: c.clock.Now(), Definition: "Misskey note reaction count"},
		},
	}
	if input.ReplyID != nil {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationReply, PostID: *input.ReplyID})
	}
	if input.RenoteID != nil {
		relation := socialhub.RelationQuote
		if pureRenote {
			relation = socialhub.RelationRepost
		}
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: relation, PostID: *input.RenoteID})
	}
	for _, file := range content.Files {
		media, err := mapDriveFile(file)
		if err != nil {
			return nil, err
		}
		post.Media = append(post.Media, *media)
	}
	extension, _ := json.Marshal(struct {
		ContentWarning     *string          `json:"content_warning,omitempty"`
		LocalOnly          bool             `json:"local_only,omitempty"`
		ChannelID          *string          `json:"channel_id,omitempty"`
		VisibleUserIDs     []string         `json:"visible_user_ids,omitempty"`
		Tags               []string         `json:"tags,omitempty"`
		Poll               json.RawMessage  `json:"poll,omitempty"`
		ReactionAcceptance *string          `json:"reaction_acceptance,omitempty"`
		Reactions          map[string]int64 `json:"reactions,omitempty"`
		MyReaction         *string          `json:"my_reaction,omitempty"`
	}{
		input.ContentWarning, input.LocalOnly, input.ChannelID, input.VisibleUserIDs, input.Tags,
		input.Poll, input.ReactionAcceptance, input.Reactions, input.MyReaction,
	})
	post.Extensions = map[string]json.RawMessage{"misskey.note": extension}
	return post, nil
}

func validateNote(input misskeyNote, validateRenote bool) error {
	if !validID(input.ID) || input.CreatedAt == nil || input.CreatedAt.IsZero() || !validID(input.UserID) ||
		!validVisibility(NoteVisibility(input.Visibility)) || input.ReactionCount < 0 || input.RenoteCount < 0 || input.RepliesCount < 0 ||
		(input.Text != nil && !validContentString(*input.Text, 1<<20)) ||
		(input.ContentWarning != nil && !validContentString(*input.ContentWarning, 4096)) ||
		(input.URL != "" && !validHTTPURL(input.URL)) || (input.URI != "" && !validHTTPURL(input.URI)) {
		return platformError("map_note", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	for _, id := range [](*string){input.ReplyID, input.RenoteID, input.ChannelID} {
		if id != nil && !validID(*id) {
			return platformError("map_note", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	if validateRenote && input.Renote != nil {
		return validateNote(*input.Renote, false)
	}
	return nil
}

func mapDriveFile(input misskeyDriveFile) (*socialhub.Media, error) {
	parsedMIME, _, mimeErr := mime.ParseMediaType(input.Type)
	if !validID(input.ID) || input.CreatedAt == nil || input.CreatedAt.IsZero() ||
		!validBoundedString(input.Name, 4096) || !validBoundedString(input.Type, 512) ||
		mimeErr != nil || !strings.Contains(parsedMIME, "/") || input.Size < 0 || !validHTTPURL(input.URL) ||
		input.Properties.Width < 0 || input.Properties.Height < 0 {
		return nil, platformError("map_media", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	mediaType := mediaTypeFromMIME(input.Type)
	size := input.Size
	extension, _ := json.Marshal(struct {
		Name         string  `json:"name"`
		MD5          string  `json:"md5,omitempty"`
		Sensitive    bool    `json:"sensitive,omitempty"`
		BlurHash     *string `json:"blurhash,omitempty"`
		ThumbnailURL *string `json:"thumbnail_url,omitempty"`
		Comment      *string `json:"comment,omitempty"`
		FolderID     *string `json:"folder_id,omitempty"`
		UserID       *string `json:"user_id,omitempty"`
	}{input.Name, input.MD5, input.Sensitive, input.BlurHash, input.ThumbnailURL, input.Comment, input.FolderID, input.UserID})
	return &socialhub.Media{
		ID: input.ID, URL: input.URL, MIME: input.Type, Type: mediaType, Size: &size,
		Width: intPointer(input.Properties.Width), Height: intPointer(input.Properties.Height),
		State: socialhub.MediaStateReady, Extensions: map[string]json.RawMessage{"misskey.drive_file": extension},
	}, nil
}

func mediaTypeFromMIME(value string) socialhub.MediaType {
	parsed, _, _ := mime.ParseMediaType(value)
	switch {
	case strings.HasPrefix(parsed, "image/gif"):
		return socialhub.MediaTypeAnimation
	case strings.HasPrefix(parsed, "image/"):
		return socialhub.MediaTypeImage
	case strings.HasPrefix(parsed, "video/"):
		return socialhub.MediaTypeVideo
	case strings.HasPrefix(parsed, "audio/"):
		return socialhub.MediaTypeAudio
	default:
		return socialhub.MediaTypeDocument
	}
}

func hasPoll(value json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(value))
	return trimmed != "" && trimmed != "null"
}

func (c *Client) mapComment(rootPostID string, input misskeyNote) (socialhub.Comment, error) {
	post, err := c.mapNote(input)
	if err != nil {
		return socialhub.Comment{}, err
	}
	text := ""
	if post.Text != nil {
		text = *post.Text
	}
	return socialhub.Comment{
		Platform: "misskey", AccountID: c.accountID, ID: post.ID, PostID: rootPostID,
		AuthorID: post.AuthorID, ParentID: input.ReplyID, Text: text, CreatedAt: post.CreatedAt,
		Metrics: post.Metrics, Extensions: post.Extensions,
	}, nil
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
