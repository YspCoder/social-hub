package flickr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) mapPerson(input Person) (*socialhub.User, error) {
	if !validResourceID(input.NSID) {
		return nil, platformError("map_person", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("person response is missing nsid"))
	}
	displayName := input.RealName.Text
	if displayName == "" {
		displayName = input.Username.Text
	}
	profileURL := input.ProfileURL.Text
	if profileURL == "" {
		profileURL = "https://www.flickr.com/people/" + urlPathSegment(input.NSID) + "/"
	}
	extensions := map[string]json.RawMessage{}
	addExtension(extensions, "location", input.Location.Text)
	addExtension(extensions, "is_pro", input.IsPro.Bool())
	addExtension(extensions, "photo_count", input.Photos.Count.String())
	addExtension(extensions, "first_photo_at", unixTime(input.Photos.FirstDate))
	return &socialhub.User{
		Platform: "flickr", AccountID: c.accountID, ID: input.NSID,
		Username: optionalPointer(input.Username.Text), DisplayName: optionalPointer(displayName),
		AvatarURL: pointer(buddyIconURL(input)), ProfileURL: pointer(profileURL), Extensions: extensions,
	}, nil
}

func (c *Client) mapPhoto(input Photo) (*socialhub.Post, error) {
	if !validResourceID(input.ID) {
		return nil, platformError("map_photo", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("photo response is missing id"))
	}
	text := firstNonEmpty(input.Description.Text, input.Title.Text)
	mediaType := socialhub.MediaTypeImage
	if input.Media == "video" {
		mediaType = socialhub.MediaTypeVideo
	}
	media := socialhub.Media{ID: input.ID, URL: staticPhotoURL(input.Server, input.ID, input.Secret), Type: mediaType, State: socialhub.MediaStateReady}
	url := photoPageURL(input)
	createdAt := unixTime(input.Dates.Posted)
	metrics := make([]socialhub.Metric, 0, 2)
	if value, ok := input.Views.Int64(); ok {
		metrics = append(metrics, socialhub.Metric{Name: "views", Value: float64(value), Definition: "Flickr photo views"})
	}
	if value, ok := scalar(input.Comments.Text).Int64(); ok {
		metrics = append(metrics, socialhub.Metric{Name: "comments", Value: float64(value), Definition: "Flickr photo comments"})
	}
	tags := make([]string, 0, len(input.Tags.Tag))
	for _, tag := range input.Tags.Tag {
		tags = append(tags, firstNonEmpty(tag.Raw, tag.Content))
	}
	extensions := map[string]json.RawMessage{}
	addExtension(extensions, "title", input.Title.Text)
	addExtension(extensions, "license", input.License.String())
	addExtension(extensions, "safety_level", input.SafetyLevel.String())
	addExtension(extensions, "rotation", input.Rotation.String())
	addExtension(extensions, "tags", tags)
	addExtension(extensions, "is_favorite", input.IsFavorite.Bool())
	return &socialhub.Post{
		Platform: "flickr", AccountID: c.accountID, ID: input.ID, AuthorID: pointer(input.Owner.NSID),
		Text: optionalPointer(text), Media: []socialhub.Media{media}, CreatedAt: createdAt,
		URL: optionalPointer(url), Visibility: pointer(photoVisibility(input.Visibility.IsPublic, input.Visibility.IsFriend, input.Visibility.IsFamily)),
		Status:  &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: unixTime(input.Dates.LastUpdate)},
		Metrics: metrics, Extensions: extensions,
	}, nil
}

func (c *Client) mapPhotoSummary(input PhotoSummary) (*socialhub.Post, error) {
	if !validResourceID(input.ID) || !validResourceID(input.Owner) {
		return nil, platformError("map_photo_summary", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("photo list row is missing id or owner"))
	}
	text := firstNonEmpty(input.Description.Text, input.Title)
	mediaType := socialhub.MediaTypeImage
	if input.Media == "video" {
		mediaType = socialhub.MediaTypeVideo
	}
	media := socialhub.Media{ID: input.ID, URL: summaryMediaURL(input), Type: mediaType, State: socialhub.MediaStateReady}
	if width, ok := input.Width.Int(); ok && width > 0 {
		media.Width = &width
	}
	if height, ok := input.Height.Int(); ok && height > 0 {
		media.Height = &height
	}
	extensions := map[string]json.RawMessage{}
	addExtension(extensions, "title", input.Title)
	addExtension(extensions, "owner_name", input.OwnerName)
	addExtension(extensions, "tags", strings.Fields(input.Tags))
	metrics := []socialhub.Metric{}
	if views, ok := input.Views.Int64(); ok {
		metrics = append(metrics, socialhub.Metric{Name: "views", Value: float64(views), Definition: "Flickr photo views"})
	}
	return &socialhub.Post{
		Platform: "flickr", AccountID: c.accountID, ID: input.ID, AuthorID: pointer(input.Owner), Text: optionalPointer(text),
		Media: []socialhub.Media{media}, CreatedAt: unixTime(input.DateUpload),
		URL:        pointer("https://www.flickr.com/photos/" + urlPathSegment(input.Owner) + "/" + urlPathSegment(input.ID) + "/"),
		Visibility: pointer(photoVisibility(input.IsPublic, input.IsFriend, input.IsFamily)),
		Status:     &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: unixTime(input.LastUpdate)},
		Metrics:    metrics, Extensions: extensions,
	}, nil
}

func (c *Client) mapComment(photoID string, input PhotoComment) (*socialhub.Comment, error) {
	if !validResourceID(input.ID) || !validResourceID(photoID) {
		return nil, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("comment response is missing id or photo_id"))
	}
	extensions := map[string]json.RawMessage{}
	addExtension(extensions, "author_name", input.AuthorName)
	addExtension(extensions, "permalink", input.Permalink)
	return &socialhub.Comment{
		Platform: "flickr", AccountID: c.accountID, ID: input.ID, PostID: photoID,
		AuthorID: optionalPointer(input.Author), Text: input.Content, CreatedAt: unixTime(input.DateCreate), Extensions: extensions,
	}, nil
}

func photoVisibility(public, friend, family scalar) string {
	if public.Bool() {
		return "public"
	}
	if friend.Bool() && family.Bool() {
		return "friends_and_family"
	}
	if friend.Bool() {
		return "friends"
	}
	if family.Bool() {
		return "family"
	}
	return "private"
}

func photoPageURL(photo Photo) string {
	for _, candidate := range photo.URLs.URL {
		if candidate.Type == "photopage" {
			return candidate.Content
		}
	}
	if photo.Owner.NSID == "" {
		return ""
	}
	return "https://www.flickr.com/photos/" + urlPathSegment(photo.Owner.NSID) + "/" + urlPathSegment(photo.ID) + "/"
}

func summaryMediaURL(photo PhotoSummary) string {
	return firstNonEmpty(photo.URLOriginal, photo.URLLarge, photo.URLMedium, photo.URLSmall, staticPhotoURL(photo.Server, photo.ID, photo.Secret))
}

func staticPhotoURL(server, id, secret string) string {
	if server == "" || id == "" || secret == "" {
		return ""
	}
	return fmt.Sprintf("https://live.staticflickr.com/%s/%s_%s.jpg", server, id, secret)
}

func buddyIconURL(person Person) string {
	server, serverOK := person.IconServer.Int()
	farm, farmOK := person.IconFarm.Int()
	if !serverOK || !farmOK || server <= 0 || farm <= 0 {
		return "https://www.flickr.com/images/buddyicon.gif"
	}
	return fmt.Sprintf("https://farm%d.staticflickr.com/%d/buddyicons/%s.jpg", farm, server, urlPathSegment(person.NSID))
}

func pageCursors(pageValue, pagesValue scalar) (*string, *string, bool, error) {
	page, pageOK := pageValue.Int()
	pages, pagesOK := pagesValue.Int()
	if !pageOK || !pagesOK || page < 1 || pages < 0 || pages > 1_000_000 || pages > 0 && page > pages {
		return nil, nil, false, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("Flickr returned invalid pagination"))
	}
	var next, previous *string
	if page < pages {
		value := strconv.Itoa(page + 1)
		next = &value
	}
	if page > 1 {
		value := strconv.Itoa(page - 1)
		previous = &value
	}
	return next, previous, next != nil, nil
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

func urlPathSegment(value string) string {
	return url.PathEscape(value)
}
