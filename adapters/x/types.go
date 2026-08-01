package x

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

type dataResponse[T any] struct {
	Data T `json:"data"`
}

type xPost struct {
	ID             string           `json:"id"`
	Text           string           `json:"text"`
	AuthorID       string           `json:"author_id"`
	ConversationID string           `json:"conversation_id"`
	CreatedAt      *time.Time       `json:"created_at"`
	Attachments    xAttachments     `json:"attachments"`
	Referenced     []xPostReference `json:"referenced_tweets"`
	PublicMetrics  map[string]int64 `json:"public_metrics"`
}

type xAttachments struct {
	MediaKeys []string `json:"media_keys"`
}

type xPostReference struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type xUser struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Username        string `json:"username"`
	ProfileImageURL string `json:"profile_image_url"`
	URL             string `json:"url"`
}

type xMedia struct {
	MediaKey        string `json:"media_key"`
	Type            string `json:"type"`
	URL             string `json:"url"`
	PreviewImageURL string `json:"preview_image_url"`
	Width           *int   `json:"width"`
	Height          *int   `json:"height"`
	DurationMS      *int64 `json:"duration_ms"`
}

type xIncludes struct {
	Media []xMedia `json:"media"`
}

type responseMeta struct {
	NextToken     *string `json:"next_token"`
	PreviousToken *string `json:"previous_token"`
}

type postResponse struct {
	Data     xPost     `json:"data"`
	Includes xIncludes `json:"includes"`
}

type postsResponse struct {
	Data     []xPost      `json:"data"`
	Includes xIncludes    `json:"includes"`
	Meta     responseMeta `json:"meta"`
}

type createPostRequest struct {
	Text         *string          `json:"text,omitempty"`
	Media        *createPostMedia `json:"media,omitempty"`
	Reply        *createPostReply `json:"reply,omitempty"`
	QuoteTweetID *string          `json:"quote_tweet_id,omitempty"`
}

type createPostMedia struct {
	MediaIDs []string `json:"media_ids"`
}

type createPostReply struct {
	InReplyToTweetID string `json:"in_reply_to_tweet_id"`
}

func mapUser(accountID socialhub.AccountID, input xUser) *socialhub.User {
	return &socialhub.User{
		Platform:    "x",
		AccountID:   accountID,
		ID:          input.ID,
		Username:    stringPointer(input.Username),
		DisplayName: stringPointer(input.Name),
		AvatarURL:   stringPointer(input.ProfileImageURL),
		ProfileURL:  stringPointer(input.URL),
	}
}

func mapPost(accountID socialhub.AccountID, input xPost) *socialhub.Post {
	return mapPostWithIncludes(accountID, input, xIncludes{})
}

func mapPostWithIncludes(accountID socialhub.AccountID, input xPost, includes xIncludes) *socialhub.Post {
	postURL := "https://x.com/i/web/status/" + input.ID
	post := &socialhub.Post{
		Platform:  "x",
		AccountID: accountID,
		ID:        input.ID,
		Text:      stringPointer(input.Text),
		CreatedAt: input.CreatedAt,
		URL:       &postURL,
	}
	if input.AuthorID != "" {
		post.AuthorID = stringPointer(input.AuthorID)
	}
	for _, reference := range input.Referenced {
		relationType := socialhub.RelationType(reference.Type)
		switch reference.Type {
		case "replied_to":
			relationType = socialhub.RelationReply
		case "quoted":
			relationType = socialhub.RelationQuote
		case "retweeted":
			relationType = socialhub.RelationRepost
		}
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: relationType, PostID: reference.ID})
	}
	mediaByKey := make(map[string]xMedia, len(includes.Media))
	for _, media := range includes.Media {
		mediaByKey[media.MediaKey] = media
	}
	for _, mediaKey := range input.Attachments.MediaKeys {
		media, found := mediaByKey[mediaKey]
		if !found {
			post.Media = append(post.Media, socialhub.Media{ID: mediaKey, State: socialhub.MediaStateReady})
			continue
		}
		post.Media = append(post.Media, mapMedia(media))
	}
	observedAt := time.Now().UTC()
	for name, value := range input.PublicMetrics {
		post.Metrics = append(post.Metrics, socialhub.Metric{Name: name, Value: float64(value), AsOf: observedAt, Definition: "X public_metrics"})
	}
	return post
}

func mapMedia(input xMedia) socialhub.Media {
	mediaType := socialhub.MediaType(input.Type)
	switch input.Type {
	case "photo":
		mediaType = socialhub.MediaTypeImage
	case "animated_gif":
		mediaType = socialhub.MediaTypeAnimation
	case "video":
		mediaType = socialhub.MediaTypeVideo
	}
	mediaURL := input.URL
	if mediaURL == "" {
		mediaURL = input.PreviewImageURL
	}
	media := socialhub.Media{ID: input.MediaKey, URL: mediaURL, Type: mediaType, State: socialhub.MediaStateReady, Width: input.Width, Height: input.Height}
	if input.DurationMS != nil {
		duration := time.Duration(*input.DurationMS) * time.Millisecond
		media.Duration = &duration
	}
	return media
}

func mapComment(accountID socialhub.AccountID, postID string, input xPost) socialhub.Comment {
	comment := socialhub.Comment{Platform: "x", AccountID: accountID, ID: input.ID, PostID: postID, Text: input.Text, CreatedAt: input.CreatedAt}
	if input.AuthorID != "" {
		comment.AuthorID = stringPointer(input.AuthorID)
	}
	for _, reference := range input.Referenced {
		if reference.Type == "replied_to" {
			comment.ParentID = stringPointer(reference.ID)
			break
		}
	}
	return comment
}

func mapPostPage(accountID socialhub.AccountID, response postsResponse) socialhub.Page[socialhub.Post] {
	items := make([]socialhub.Post, 0, len(response.Data))
	for _, input := range response.Data {
		items = append(items, *mapPostWithIncludes(accountID, input, response.Includes))
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: response.Meta.NextToken, PrevCursor: response.Meta.PreviousToken, HasMore: response.Meta.NextToken != nil}
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

type xProblem struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Type   string `json:"type"`
	Status int    `json:"status"`
	Errors []struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"errors"`
}

func decodeError(status int, header http.Header, body []byte) error {
	var problem xProblem
	_ = json.Unmarshal(body, &problem)
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		code = socialhub.CodeInvalidArgument
	case http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		code = socialhub.CodeNotFound
	case http.StatusConflict:
		code = socialhub.CodeConflict
	case http.StatusTooManyRequests:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
	}
	message := problem.Detail
	if message == "" {
		message = problem.Title
	}
	if message == "" && len(problem.Errors) > 0 {
		message = problem.Errors[0].Message
	}
	platformCode := problem.Type
	if platformCode == "" && len(problem.Errors) > 0 {
		platformCode = problem.Errors[0].Type
	}
	return &socialhub.Error{
		Code:            code,
		Class:           class,
		Platform:        "x",
		Product:         "api",
		HTTPStatus:      status,
		PlatformCode:    platformCode,
		PlatformMessage: message,
		RequestID:       firstNonEmpty(header.Get("x-transaction-id"), header.Get("x-request-id")),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func parseRetryAfter(value string) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		delay := time.Until(at)
		if delay > 0 {
			return delay
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
