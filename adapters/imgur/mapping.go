package imgur

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) mapAccount(input Account) (*socialhub.User, error) {
	identifier := string(input.ID)
	if identifier == "" {
		identifier = input.URL
	}
	if !validIdentifier(identifier) || !validIdentifier(input.URL) {
		return nil, platformError("map_account", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("Imgur account response is missing id or url"))
	}
	extensions := map[string]json.RawMessage{}
	addExtension(extensions, "imgur.bio", input.Bio)
	addExtension(extensions, "imgur.cover", input.Cover)
	addExtension(extensions, "imgur.reputation", input.Reputation)
	addExtension(extensions, "imgur.reputation_name", input.ReputationName)
	addExtension(extensions, "imgur.pro_expiration", input.ProExpiration)
	return &socialhub.User{
		Platform: "imgur", AccountID: client.accountID, ID: identifier,
		Username: pointer(input.URL), DisplayName: pointer(input.URL), AvatarURL: optionalPointer(input.Avatar),
		ProfileURL: pointer("https://imgur.com/user/" + input.URL), AccountType: pointer("imgur_account"), Extensions: extensions,
	}, nil
}

func (client *Client) mapImage(input Image) (*socialhub.Post, error) {
	if !validIdentifier(input.ID) || !validHTTPURL(input.Link) {
		return nil, platformError("map_image", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("Imgur image response is missing id or link"))
	}
	mediaType := socialhub.MediaTypeImage
	if strings.HasPrefix(strings.ToLower(input.MIME), "video/") {
		mediaType = socialhub.MediaTypeVideo
	} else if input.Animated {
		mediaType = socialhub.MediaTypeAnimation
	}
	media := socialhub.Media{ID: input.ID, URL: input.Link, MIME: input.MIME, Type: mediaType, State: socialhub.MediaStateReady}
	if input.Size > 0 {
		media.Size = &input.Size
	}
	if input.Width > 0 {
		media.Width = &input.Width
	}
	if input.Height > 0 {
		media.Height = &input.Height
	}
	createdAt := unixTime(input.Datetime)
	metrics := []socialhub.Metric{
		{Name: "views", Value: float64(input.Views), Definition: "Imgur image views"},
		{Name: "comments", Value: float64(input.CommentCount), Definition: "Imgur Gallery comments"},
		{Name: "favorites", Value: float64(input.FavoriteCount), Definition: "Imgur favorites"},
		{Name: "ups", Value: float64(input.Ups), Definition: "Imgur Gallery up-votes"},
		{Name: "downs", Value: float64(input.Downs), Definition: "Imgur Gallery down-votes"},
		{Name: "score", Value: float64(input.Score), Definition: "Imgur Gallery score"},
	}
	extensions := map[string]json.RawMessage{}
	addExtension(extensions, "imgur.title", input.Title)
	addExtension(extensions, "imgur.name", input.Name)
	addExtension(extensions, "imgur.deletehash", input.DeleteHash)
	addExtension(extensions, "imgur.animated", input.Animated)
	addExtension(extensions, "imgur.bandwidth", input.Bandwidth)
	addExtension(extensions, "imgur.vote", input.Vote)
	addExtension(extensions, "imgur.favorite", input.Favorite)
	addExtension(extensions, "imgur.nsfw", input.NSFW)
	addExtension(extensions, "imgur.section", input.Section)
	addExtension(extensions, "imgur.in_gallery", input.InGallery)
	addExtension(extensions, "imgur.in_most_viral", input.InMostViral)
	addExtension(extensions, "imgur.has_sound", input.HasSound)
	addExtension(extensions, "imgur.mp4", input.MP4)
	addExtension(extensions, "imgur.gifv", input.GIFV)
	addExtension(extensions, "imgur.hls", input.HLS)
	visibility := "hidden"
	if input.InGallery {
		visibility = "public_gallery"
	}
	return &socialhub.Post{
		Platform: "imgur", AccountID: client.accountID, ID: input.ID,
		AuthorID: optionalPointer(firstNonEmpty(string(input.AccountID), input.AccountURL)),
		Text:     optionalPointer(firstNonEmpty(input.Description, input.Title)), Media: []socialhub.Media{media}, CreatedAt: createdAt,
		URL: pointer("https://imgur.com/" + input.ID), Visibility: pointer(visibility),
		Status:  &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: createdAt},
		Metrics: metrics, Extensions: extensions,
	}, nil
}

func (client *Client) mapComments(postID string, input []Comment) ([]socialhub.Comment, error) {
	items := make([]socialhub.Comment, 0, len(input))
	var appendComment func(Comment, string) error
	appendComment = func(comment Comment, fallbackParent string) error {
		identifier := string(comment.ID)
		if !validIdentifier(identifier) {
			return platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("Imgur comment response is missing id"))
		}
		parentID := string(comment.ParentID)
		if parentID == "0" || parentID == "" {
			parentID = fallbackParent
		}
		extensions := map[string]json.RawMessage{}
		addExtension(extensions, "imgur.author", comment.Author)
		addExtension(extensions, "imgur.vote", comment.Vote)
		addExtension(extensions, "imgur.ups", comment.Ups)
		addExtension(extensions, "imgur.downs", comment.Downs)
		addExtension(extensions, "imgur.points", comment.Points)
		addExtension(extensions, "imgur.deleted", comment.Deleted)
		items = append(items, socialhub.Comment{
			Platform: "imgur", AccountID: client.accountID, ID: identifier, PostID: postID,
			AuthorID: optionalPointer(firstNonEmpty(string(comment.AuthorID), comment.Author)), ParentID: optionalPointer(parentID),
			Text: comment.Text, CreatedAt: unixTime(comment.Datetime), Extensions: extensions,
		})
		for _, child := range comment.Children {
			if err := appendComment(child, identifier); err != nil {
				return err
			}
		}
		return nil
	}
	for _, comment := range input {
		if err := appendComment(comment, ""); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func unixTime(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	value := time.Unix(seconds, 0).UTC()
	return &value
}

func addExtension(target map[string]json.RawMessage, key string, value any) {
	encoded, err := json.Marshal(value)
	if err == nil && string(encoded) != "null" && string(encoded) != `""` {
		target[key] = encoded
	}
}

func pointer(value string) *string { return &value }

func optionalPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
