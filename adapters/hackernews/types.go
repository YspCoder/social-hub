package hackernews

import (
	"context"

	"social-hub/pkg/socialhub"
)

// Feed identifies one official ranked or chronological story list.
type Feed string

const (
	FeedTop  Feed = "topstories"
	FeedNew  Feed = "newstories"
	FeedBest Feed = "beststories"
	FeedAsk  Feed = "askstories"
	FeedShow Feed = "showstories"
	FeedJob  Feed = "jobstories"
)

// ItemType is Hacker News' closed item discriminator.
type ItemType string

const (
	ItemJob     ItemType = "job"
	ItemStory   ItemType = "story"
	ItemComment ItemType = "comment"
	ItemPoll    ItemType = "poll"
	ItemPollOpt ItemType = "pollopt"
)

// Item preserves the official API's story, comment, job, and poll model.
type Item struct {
	ID          int64    `json:"id"`
	Deleted     bool     `json:"deleted,omitempty"`
	Type        ItemType `json:"type,omitempty"`
	By          string   `json:"by,omitempty"`
	Time        int64    `json:"time,omitempty"`
	Text        string   `json:"text,omitempty"`
	Dead        bool     `json:"dead,omitempty"`
	Parent      int64    `json:"parent,omitempty"`
	Poll        int64    `json:"poll,omitempty"`
	Kids        []int64  `json:"kids,omitempty"`
	URL         string   `json:"url,omitempty"`
	Score       int64    `json:"score,omitempty"`
	Title       string   `json:"title,omitempty"`
	Parts       []int64  `json:"parts,omitempty"`
	Descendants int64    `json:"descendants,omitempty"`
}

// User preserves one public Hacker News profile.
type User struct {
	ID        string  `json:"id"`
	Created   int64   `json:"created"`
	Karma     int64   `json:"karma"`
	About     string  `json:"about,omitempty"`
	Submitted []int64 `json:"submitted,omitempty"`
}

// Updates contains recently changed item IDs and profile names.
type Updates struct {
	Items    []int64  `json:"items"`
	Profiles []string `json:"profiles"`
}

// FeedRequest selects and paginates an official feed.
type FeedRequest struct {
	Feed       Feed
	Cursor     string
	MaxResults int
}

// ChildrenRequest selects direct children of any item, preserving comment-tree shape.
type ChildrenRequest struct {
	ParentID   int64
	Cursor     string
	MaxResults int
}

// ItemWorkflow exposes raw items, direct children, and the current maximum ID.
type ItemWorkflow interface {
	GetItem(context.Context, int64, ...socialhub.CallOption) (*Item, error)
	ListChildren(context.Context, ChildrenRequest, ...socialhub.CallOption) (socialhub.Page[Item], error)
	MaxItemID(context.Context, ...socialhub.CallOption) (int64, error)
}

// FeedWorkflow exposes all six official story feeds.
type FeedWorkflow interface {
	ListFeed(context.Context, FeedRequest, ...socialhub.CallOption) (socialhub.Page[Item], error)
}

// UserWorkflow exposes the complete public profile model.
type UserWorkflow interface {
	GetUserProfile(context.Context, string, ...socialhub.CallOption) (*User, error)
}

// UpdatesWorkflow exposes the polling-oriented changed-items endpoint.
type UpdatesWorkflow interface {
	GetUpdates(context.Context, ...socialhub.CallOption) (*Updates, error)
}
