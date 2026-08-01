package bluesky

import (
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	collectionPost   = "app.bsky.feed.post"
	collectionLike   = "app.bsky.feed.like"
	collectionRepost = "app.bsky.feed.repost"
)

// TimelineRequest selects a page from the authenticated home timeline.
type TimelineRequest struct {
	Cursor     string
	MaxResults int
	Algorithm  string
}

// PostMedia adds Bluesky-specific accessibility and aspect-ratio metadata to
// a blob returned by the common MediaUploader.
type PostMedia struct {
	MediaID string
	Alt     string
	Width   int
	Height  int
}

// PostRecordRequest creates an app.bsky.feed.post repository record.
type PostRecordRequest struct {
	Text       string
	Languages  []string
	Media      []PostMedia
	ReplyToURI string
	QuoteURI   string
	RecordKey  string
}

// SessionInfo contains the account identity and rotating legacy session
// tokens returned by a PDS.
type SessionInfo struct {
	DID            string
	Handle         string
	Email          string
	EmailConfirmed bool
	Active         bool
	Status         string
	Token          socialhub.Token
}

type strongRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

type blobLink struct {
	Link string `json:"$link"`
}

type blobRef struct {
	Type     string   `json:"$type"`
	Ref      blobLink `json:"ref"`
	MIMEType string   `json:"mimeType"`
	Size     int64    `json:"size"`
}

type bskyActor struct {
	DID            string          `json:"did"`
	Handle         string          `json:"handle"`
	DisplayName    string          `json:"displayName"`
	Description    string          `json:"description"`
	Pronouns       string          `json:"pronouns"`
	Website        string          `json:"website"`
	Avatar         string          `json:"avatar"`
	Banner         string          `json:"banner"`
	FollowersCount *int64          `json:"followersCount"`
	FollowsCount   *int64          `json:"followsCount"`
	PostsCount     *int64          `json:"postsCount"`
	CreatedAt      *time.Time      `json:"createdAt"`
	IndexedAt      *time.Time      `json:"indexedAt"`
	Associated     json.RawMessage `json:"associated"`
	Labels         json.RawMessage `json:"labels"`
	Verification   json.RawMessage `json:"verification"`
}

type bskyViewerState struct {
	Like       string `json:"like"`
	Repost     string `json:"repost"`
	Bookmarked *bool  `json:"bookmarked"`
}

type bskyPostView struct {
	URI           string          `json:"uri"`
	CID           string          `json:"cid"`
	Author        bskyActor       `json:"author"`
	Record        json.RawMessage `json:"record"`
	Embed         json.RawMessage `json:"embed"`
	BookmarkCount *int64          `json:"bookmarkCount"`
	ReplyCount    *int64          `json:"replyCount"`
	RepostCount   *int64          `json:"repostCount"`
	LikeCount     *int64          `json:"likeCount"`
	QuoteCount    *int64          `json:"quoteCount"`
	IndexedAt     *time.Time      `json:"indexedAt"`
	Viewer        bskyViewerState `json:"viewer"`
	Labels        json.RawMessage `json:"labels"`
}

type bskyPostRecord struct {
	Type      string          `json:"$type"`
	Text      string          `json:"text"`
	CreatedAt *time.Time      `json:"createdAt"`
	Languages []string        `json:"langs"`
	Reply     *bskyReplyRef   `json:"reply"`
	Embed     json.RawMessage `json:"embed"`
}

type bskyReplyRef struct {
	Root   strongRef `json:"root"`
	Parent strongRef `json:"parent"`
}

type bskyFeedReason struct {
	Type      string     `json:"$type"`
	By        bskyActor  `json:"by"`
	URI       string     `json:"uri"`
	CID       string     `json:"cid"`
	IndexedAt *time.Time `json:"indexedAt"`
}

type bskyFeedItem struct {
	Post   bskyPostView   `json:"post"`
	Reason bskyFeedReason `json:"reason"`
}

type bskyFeedResponse struct {
	Cursor string         `json:"cursor"`
	Feed   []bskyFeedItem `json:"feed"`
}

type bskyPostsResponse struct {
	Posts []bskyPostView `json:"posts"`
}

type bskyThreadResponse struct {
	Thread bskyThreadNode `json:"thread"`
}

type bskyThreadNode struct {
	Type     string           `json:"$type"`
	Post     bskyPostView     `json:"post"`
	Replies  []bskyThreadNode `json:"replies"`
	NotFound bool             `json:"notFound"`
	Blocked  bool             `json:"blocked"`
}

type createRecordRequest struct {
	Repo       string `json:"repo"`
	Collection string `json:"collection"`
	RecordKey  string `json:"rkey,omitempty"`
	Record     any    `json:"record"`
}

type createRecordResponse struct {
	URI              string `json:"uri"`
	CID              string `json:"cid"`
	ValidationStatus string `json:"validationStatus"`
}

type deleteRecordRequest struct {
	Repo       string `json:"repo"`
	Collection string `json:"collection"`
	RecordKey  string `json:"rkey"`
}

type imageEmbed struct {
	Type   string       `json:"$type"`
	Images []imageInput `json:"images"`
}

type imageInput struct {
	Alt         string       `json:"alt"`
	Image       blobRef      `json:"image"`
	AspectRatio *aspectRatio `json:"aspectRatio,omitempty"`
}

type videoEmbed struct {
	Type        string       `json:"$type"`
	Video       blobRef      `json:"video"`
	Alt         string       `json:"alt,omitempty"`
	AspectRatio *aspectRatio `json:"aspectRatio,omitempty"`
}

type aspectRatio struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type recordEmbed struct {
	Type   string    `json:"$type"`
	Record strongRef `json:"record"`
}

type recordWithMediaEmbed struct {
	Type   string      `json:"$type"`
	Record recordEmbed `json:"record"`
	Media  any         `json:"media"`
}

type postRecordInput struct {
	Type      string        `json:"$type"`
	Text      string        `json:"text"`
	CreatedAt string        `json:"createdAt"`
	Languages []string      `json:"langs,omitempty"`
	Reply     *bskyReplyRef `json:"reply,omitempty"`
	Embed     any           `json:"embed,omitempty"`
}
