package stackexchange

import (
	"encoding/json"
	"html"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input UserDetails) *socialhub.User {
	extension, _ := json.Marshal(input)
	displayName := html.UnescapeString(input.DisplayName)
	return &socialhub.User{
		Platform: "stackexchange", AccountID: accountID, ID: strconv.FormatInt(input.UserID, 10),
		Username: stringPointer(displayName), DisplayName: stringPointer(displayName), AvatarURL: stringPointer(input.ProfileImage),
		ProfileURL: stringPointer(input.Link), AccountType: stringPointer(input.UserType),
		Extensions: map[string]json.RawMessage{"stackexchange.user": extension},
	}
}

func mapPost(accountID socialhub.AccountID, input PostDetails, observedAt time.Time) *socialhub.Post {
	postID := postIdentifier(input)
	id := strconv.FormatInt(postID, 10)
	createdAt := unixTimePointer(input.CreationDate)
	details, _ := json.Marshal(input)
	post := &socialhub.Post{
		Platform: "stackexchange", AccountID: accountID, ID: id, Text: stringPointer(firstNonEmpty(input.BodyMarkdown, input.Body)),
		CreatedAt: createdAt, URL: stringPointer(input.Link), Visibility: stringPointer("public"),
		Status:     &socialhub.PublishStatus{ID: id, State: socialhub.PublishStatePublished, UpdatedAt: unixTimePointer(firstPositive(input.LastActivityDate, input.CreationDate))},
		Extensions: map[string]json.RawMessage{"stackexchange.post": details},
	}
	if input.Owner.UserID > 0 {
		post.AuthorID = stringPointer(strconv.FormatInt(input.Owner.UserID, 10))
	}
	if input.QuestionID > 0 && (input.PostType == "answer" || input.AnswerID > 0) {
		post.Relations = []socialhub.PostRelation{{Type: socialhub.RelationReply, PostID: strconv.FormatInt(input.QuestionID, 10)}}
	}
	for name, metric := range map[string]struct {
		value      int64
		definition string
	}{
		"score": {input.Score, "Stack Exchange post score"}, "views": {input.ViewCount, "Stack Exchange question view count"},
		"answers": {input.AnswerCount, "Stack Exchange question answer count"}, "comments": {input.CommentCount, "Stack Exchange post comment count"},
		"favorites": {input.FavoriteCount, "Stack Exchange question favorite count"},
	} {
		post.Metrics = append(post.Metrics, socialhub.Metric{Name: name, Value: float64(metric.value), AsOf: observedAt, Definition: metric.definition})
	}
	return post
}

func postIdentifier(input PostDetails) int64 {
	if input.PostID > 0 {
		return input.PostID
	}
	if input.PostType == "answer" || input.AnswerID > 0 {
		return input.AnswerID
	}
	return input.QuestionID
}

func mapComment(accountID socialhub.AccountID, input CommentDetails, observedAt time.Time) *socialhub.Comment {
	extension, _ := json.Marshal(input)
	comment := &socialhub.Comment{
		Platform: "stackexchange", AccountID: accountID, ID: strconv.FormatInt(input.CommentID, 10),
		PostID: strconv.FormatInt(input.PostID, 10), Text: firstNonEmpty(input.BodyMarkdown, input.Body),
		CreatedAt:  unixTimePointer(input.CreationDate),
		Metrics:    []socialhub.Metric{{Name: "score", Value: float64(input.Score), AsOf: observedAt, Definition: "Stack Exchange comment score"}},
		Extensions: map[string]json.RawMessage{"stackexchange.comment": extension},
	}
	if input.Owner.UserID > 0 {
		comment.AuthorID = stringPointer(strconv.FormatInt(input.Owner.UserID, 10))
	}
	return comment
}

func unixTimePointer(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	value := time.Unix(seconds, 0).UTC()
	return &value
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func firstPositive(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
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
