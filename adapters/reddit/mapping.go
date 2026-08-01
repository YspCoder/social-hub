package reddit

import (
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input redditUser) *socialhub.User {
	extension, _ := json.Marshal(struct {
		CreatedUTC   float64 `json:"created_utc,omitempty"`
		TotalKarma   int64   `json:"total_karma,omitempty"`
		LinkKarma    int64   `json:"link_karma,omitempty"`
		CommentKarma int64   `json:"comment_karma,omitempty"`
		IsGold       bool    `json:"is_gold,omitempty"`
		IsMod        bool    `json:"is_mod,omitempty"`
		Verified     bool    `json:"verified,omitempty"`
	}{input.CreatedUTC, input.TotalKarma, input.LinkKarma, input.CommentKarma, input.IsGold, input.IsMod, input.Verified})
	return &socialhub.User{
		Platform: "reddit", AccountID: accountID, ID: fullname(input.ID, "t2_"),
		Username: stringPointer(input.Name), DisplayName: stringPointer(input.Name), AvatarURL: stringPointer(input.IconImage),
		ProfileURL: stringPointer("https://www.reddit.com/user/" + input.Name + "/"), AccountType: stringPointer("redditor"),
		Extensions: map[string]json.RawMessage{"reddit.account": extension},
	}
}

func mapPost(accountID socialhub.AccountID, input redditThingData, observedAt time.Time) *socialhub.Post {
	postID := fullname(input.Name, "t3_")
	createdAt := unixTimePointer(input.CreatedUTC)
	details, _ := json.Marshal(struct {
		Title       string  `json:"title,omitempty"`
		Subreddit   string  `json:"subreddit,omitempty"`
		SubredditID string  `json:"subreddit_id,omitempty"`
		Author      string  `json:"author,omitempty"`
		Score       int64   `json:"score,omitempty"`
		UpvoteRatio float64 `json:"upvote_ratio,omitempty"`
		Over18      bool    `json:"over_18,omitempty"`
		Spoiler     bool    `json:"spoiler,omitempty"`
		Locked      bool    `json:"locked,omitempty"`
		Stickied    bool    `json:"stickied,omitempty"`
		IsSelf      bool    `json:"is_self,omitempty"`
	}{input.Title, input.Subreddit, input.SubredditID, input.Author, input.Score, input.UpvoteRatio, input.Over18, input.Spoiler, input.Locked, input.Stickied, input.IsSelf})
	post := &socialhub.Post{
		Platform: "reddit", AccountID: accountID, ID: postID, AuthorID: stringPointer(input.AuthorFullname),
		Text: stringPointer(firstNonEmpty(input.SelfText, input.Title)), CreatedAt: createdAt,
		URL: stringPointer(redditPermalink(input.Permalink, input.URL)), Visibility: stringPointer("public"),
		Status:     &socialhub.PublishStatus{ID: postID, State: socialhub.PublishStatePublished, UpdatedAt: createdAt},
		Extensions: map[string]json.RawMessage{"reddit.post": details},
	}
	if input.CrosspostParent != "" {
		post.Relations = []socialhub.PostRelation{{Type: socialhub.RelationRepost, PostID: input.CrosspostParent}}
	}
	if input.Media.RedditVideo != nil {
		video := input.Media.RedditVideo
		state := socialhub.MediaStateReady
		if video.TranscodingStatus != "" && video.TranscodingStatus != "completed" {
			state = socialhub.MediaStateProcessing
		}
		duration := time.Duration(video.Duration) * time.Second
		post.Media = []socialhub.Media{{
			ID: postID, URL: firstNonEmpty(video.FallbackURL, video.HLSURL, video.DASHURL), Type: socialhub.MediaTypeVideo,
			Width: intPointer(video.Width), Height: intPointer(video.Height), Duration: durationIfPositive(duration), State: state,
		}}
	} else if input.PostHint == "image" && validHTTPURL(input.URL) {
		post.Media = []socialhub.Media{{ID: postID, URL: input.URL, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady}}
	}
	for name, value := range map[string]float64{
		"score": float64(input.Score), "upvotes": float64(input.Ups), "comments": float64(input.NumComments), "upvote_ratio": input.UpvoteRatio,
	} {
		post.Metrics = append(post.Metrics, socialhub.Metric{Name: name, Value: value, AsOf: observedAt, Definition: "Reddit submission " + name})
	}
	return post
}

func mapComments(accountID socialhub.AccountID, postID string, things []redditThing, observedAt time.Time) []socialhub.Comment {
	var comments []socialhub.Comment
	for _, thing := range things {
		if thing.Kind != "t1" {
			continue
		}
		data := thing.Data
		commentID := fullname(data.Name, "t1_")
		extension, _ := json.Marshal(struct {
			Author    string `json:"author,omitempty"`
			Subreddit string `json:"subreddit,omitempty"`
			Score     int64  `json:"score,omitempty"`
		}{data.Author, data.Subreddit, data.Score})
		comment := socialhub.Comment{
			Platform: "reddit", AccountID: accountID, ID: commentID, PostID: firstNonEmpty(data.LinkID, postID),
			AuthorID: stringPointer(data.AuthorFullname), ParentID: parentCommentPointer(data.ParentID), Text: data.Body,
			CreatedAt: unixTimePointer(data.CreatedUTC), Metrics: []socialhub.Metric{{Name: "score", Value: float64(data.Score), AsOf: observedAt, Definition: "Reddit comment score"}},
			Extensions: map[string]json.RawMessage{"reddit.comment": extension},
		}
		comments = append(comments, comment)
		if len(data.Replies) > 0 && string(data.Replies) != `""` {
			var replies redditListing
			if json.Unmarshal(data.Replies, &replies) == nil {
				comments = append(comments, mapComments(accountID, postID, replies.Data.Children, observedAt)...)
			}
		}
	}
	return comments
}

func fullname(value, prefix string) string {
	if value == "" || strings.HasPrefix(value, prefix) {
		return value
	}
	return prefix + value
}

func redditPermalink(permalink, fallback string) string {
	if strings.HasPrefix(permalink, "/") {
		return "https://www.reddit.com" + permalink
	}
	return fallback
}

func unixTimePointer(seconds float64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	nanoseconds := int64(seconds * float64(time.Second))
	value := time.Unix(0, nanoseconds).UTC()
	return &value
}

func parentCommentPointer(value string) *string {
	if !strings.HasPrefix(value, "t1_") {
		return nil
	}
	return stringPointer(value)
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

func durationIfPositive(value time.Duration) *time.Duration {
	if value <= 0 {
		return nil
	}
	return &value
}
