package reddit

import "encoding/json"

type redditUser struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	IconImage    string  `json:"icon_img"`
	CreatedUTC   float64 `json:"created_utc"`
	TotalKarma   int64   `json:"total_karma"`
	LinkKarma    int64   `json:"link_karma"`
	CommentKarma int64   `json:"comment_karma"`
	IsGold       bool    `json:"is_gold"`
	IsMod        bool    `json:"is_mod"`
	Verified     bool    `json:"verified"`
}

type redditListing struct {
	Kind string `json:"kind"`
	Data struct {
		After    *string       `json:"after"`
		Before   *string       `json:"before"`
		Children []redditThing `json:"children"`
	} `json:"data"`
}

type redditThing struct {
	Kind string          `json:"kind"`
	Data redditThingData `json:"data"`
}

type redditThingData struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Author          string          `json:"author"`
	AuthorFullname  string          `json:"author_fullname"`
	Title           string          `json:"title"`
	SelfText        string          `json:"selftext"`
	Body            string          `json:"body"`
	URL             string          `json:"url"`
	Permalink       string          `json:"permalink"`
	Subreddit       string          `json:"subreddit"`
	SubredditID     string          `json:"subreddit_id"`
	ParentID        string          `json:"parent_id"`
	LinkID          string          `json:"link_id"`
	CrosspostParent string          `json:"crosspost_parent"`
	CreatedUTC      float64         `json:"created_utc"`
	Score           int64           `json:"score"`
	Ups             int64           `json:"ups"`
	UpvoteRatio     float64         `json:"upvote_ratio"`
	NumComments     int64           `json:"num_comments"`
	Over18          bool            `json:"over_18"`
	Spoiler         bool            `json:"spoiler"`
	Locked          bool            `json:"locked"`
	Stickied        bool            `json:"stickied"`
	IsSelf          bool            `json:"is_self"`
	PostHint        string          `json:"post_hint"`
	Thumbnail       string          `json:"thumbnail"`
	Replies         json.RawMessage `json:"replies"`
	Media           struct {
		RedditVideo *struct {
			FallbackURL       string `json:"fallback_url"`
			HLSURL            string `json:"hls_url"`
			DASHURL           string `json:"dash_url"`
			Width             int    `json:"width"`
			Height            int    `json:"height"`
			Duration          int    `json:"duration"`
			TranscodingStatus string `json:"transcoding_status"`
			IsGIF             bool   `json:"is_gif"`
		} `json:"reddit_video"`
	} `json:"media"`
}

type redditAPIResponse struct {
	JSON struct {
		Errors [][]json.RawMessage `json:"errors"`
		Data   struct {
			ID     string        `json:"id"`
			Name   string        `json:"name"`
			URL    string        `json:"url"`
			Things []redditThing `json:"things"`
		} `json:"data"`
	} `json:"json"`
}
