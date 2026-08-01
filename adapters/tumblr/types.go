package tumblr

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

// NPFState is a Tumblr Neue Post Format publication state.
type NPFState string

const (
	NPFPublished  NPFState = "published"
	NPFQueue      NPFState = "queue"
	NPFDraft      NPFState = "draft"
	NPFPrivate    NPFState = "private"
	NPFUnapproved NPFState = "unapproved"
)

// NPFMediaUpload binds a stream to one NPF content block. The block's media
// field is replaced with Tumblr's multipart identifier.
type NPFMediaUpload struct {
	BlockIndex int
	Filename   string
	MIME       string
	Size       int64
	Reader     io.Reader
}

// NPFReblogTarget contains the three identifiers required for a genuine
// Tumblr reblog.
type NPFReblogTarget struct {
	BlogUUID          string
	PostID            string
	ReblogKey         string
	HideTrail         bool
	ExcludeTrailItems []int
}

// NPFPostRequest creates or replaces a post using evolving NPF JSON blocks.
// Content and Layout entries must each be complete JSON objects.
type NPFPostRequest struct {
	Content               []json.RawMessage
	Layout                []json.RawMessage
	State                 NPFState
	PublishOn             *time.Time
	Date                  *time.Time
	Tags                  []string
	SourceURL             string
	SendToTwitter         *bool
	IsPrivate             bool
	Slug                  string
	InteractabilityReblog string
	Reblog                *NPFReblogTarget
	Uploads               []NPFMediaUpload
}

// NPFResult identifies a created or edited post.
type NPFResult struct {
	ID    string
	State NPFState
}

// NPFPost preserves an editable NPF post without flattening its block schema.
type NPFPost struct {
	ID                    string
	BlogUUID              string
	BlogName              string
	PostURL               string
	ParentPostID          string
	ParentBlogUUID        string
	ReblogKey             string
	State                 NPFState
	QueuedState           string
	ScheduledPublishTime  *time.Time
	PublishOn             string
	InteractabilityReblog string
	Timestamp             *time.Time
	Tags                  []string
	Content               []json.RawMessage
	Layout                []json.RawMessage
	Trail                 []json.RawMessage
	Raw                   json.RawMessage
}

// NPFWorkflow exposes Tumblr's current post create, edit, fetch, and reblog
// contract, including inline multipart media.
type NPFWorkflow interface {
	CreateNPF(context.Context, string, NPFPostRequest, ...socialhub.CallOption) (*NPFResult, error)
	EditNPF(context.Context, string, string, NPFPostRequest, ...socialhub.CallOption) (*NPFResult, error)
	GetNPF(context.Context, string, string, ...socialhub.CallOption) (*NPFPost, error)
}

// PageRequest selects a Tumblr offset page.
type PageRequest struct {
	Cursor     string
	MaxResults int
}

// TaggedRequest selects public posts for one tag. Cursor is a Unix timestamp
// returned by the previous page.
type TaggedRequest struct {
	Tag        string
	Cursor     string
	MaxResults int
}

// NotesMode selects Tumblr's server-side note representation.
type NotesMode string

const (
	NotesAll             NotesMode = "all"
	NotesLikes           NotesMode = "likes"
	NotesConversation    NotesMode = "conversation"
	NotesRollup          NotesMode = "rollup"
	NotesReblogsWithTags NotesMode = "reblogs_with_tags"
)

// NotesRequest selects notes for a post on a specific blog.
type NotesRequest struct {
	BlogIdentifier string
	PostID         string
	Mode           NotesMode
	Cursor         string
}

// Note preserves the union of Tumblr reply, like, and reblog note fields.
type Note struct {
	Type      string
	Timestamp *time.Time
	BlogName  string
	BlogUUID  string
	BlogURL   string
	PostID    string
	ReplyID   string
	ReplyText string
	AddedText string
	Tags      []string
	Raw       json.RawMessage
}

// NotesPage contains typed notes and server-provided totals.
type NotesPage struct {
	Items        []Note
	Rollup       []Note
	NextCursor   *string
	TotalNotes   int64
	TotalLikes   int64
	TotalReblogs int64
}

// TimelineWorkflow exposes the authenticated dashboard, public tagged
// discovery, and note reads.
type TimelineWorkflow interface {
	Dashboard(context.Context, PageRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
	Tagged(context.Context, TaggedRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
	Notes(context.Context, NotesRequest, ...socialhub.CallOption) (NotesPage, error)
}

// EngagementWorkflow exposes Tumblr actions whose required reblog keys and
// blog URLs do not fit the common Reactor contract.
type EngagementWorkflow interface {
	Like(context.Context, string, string, ...socialhub.CallOption) error
	Unlike(context.Context, string, string, ...socialhub.CallOption) error
	Follow(context.Context, string, ...socialhub.CallOption) error
	Unfollow(context.Context, string, ...socialhub.CallOption) error
}

type flexString string

func (value *flexString) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*value = flexString(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*value = flexString(number.String())
	return nil
}

type tumblrAvatar struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type tumblrBlog struct {
	UUID           string          `json:"uuid"`
	Name           string          `json:"name"`
	Title          string          `json:"title"`
	URL            string          `json:"url"`
	Description    string          `json:"description"`
	Updated        int64           `json:"updated"`
	Posts          int64           `json:"posts"`
	Likes          int64           `json:"likes"`
	Ask            bool            `json:"ask"`
	AskAnon        bool            `json:"ask_anon"`
	Followed       bool            `json:"followed"`
	Avatar         []tumblrAvatar  `json:"avatar"`
	Theme          json.RawMessage `json:"theme"`
	Primary        bool            `json:"primary"`
	Admin          bool            `json:"admin"`
	CanMessage     bool            `json:"can_message"`
	CanSendFanMail bool            `json:"can_send_fan_mail"`
}

type tumblrMediaObject struct {
	URL      string  `json:"url"`
	Type     string  `json:"type"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Duration float64 `json:"duration"`
}

type tumblrContentBlock struct {
	Type   string              `json:"type"`
	Text   string              `json:"text"`
	URL    string              `json:"url"`
	Media  json.RawMessage     `json:"media"`
	Poster []tumblrMediaObject `json:"poster"`
}

type tumblrPost struct {
	ObjectType            string            `json:"object_type"`
	Type                  string            `json:"type"`
	ID                    flexString        `json:"id"`
	IDString              string            `json:"id_string"`
	Blog                  tumblrBlog        `json:"blog"`
	BlogName              string            `json:"blog_name"`
	TumblelogUUID         string            `json:"tumblelog_uuid"`
	PostURL               string            `json:"post_url"`
	ShortURL              string            `json:"short_url"`
	Timestamp             int64             `json:"timestamp"`
	FeaturedTimestamp     int64             `json:"featured_timestamp"`
	State                 string            `json:"state"`
	QueuedState           string            `json:"queued_state"`
	ScheduledPublishTime  int64             `json:"scheduled_publish_time"`
	PublishOn             string            `json:"publish_on"`
	ReblogKey             string            `json:"reblog_key"`
	Tags                  []string          `json:"tags"`
	Summary               string            `json:"summary"`
	Title                 string            `json:"title"`
	Body                  string            `json:"body"`
	Content               []json.RawMessage `json:"content"`
	Layout                []json.RawMessage `json:"layout"`
	Trail                 []json.RawMessage `json:"trail"`
	ParentPostID          flexString        `json:"parent_post_id"`
	ParentTumblelogUUID   string            `json:"parent_tumblelog_uuid"`
	InteractabilityReblog string            `json:"interactability_reblog"`
	NoteCount             int64             `json:"note_count"`
	Liked                 bool              `json:"liked"`
	IsBlocksPostFormat    bool              `json:"is_blocks_post_format"`
	Raw                   json.RawMessage   `json:"-"`
}

func (post *tumblrPost) UnmarshalJSON(data []byte) error {
	type alias tumblrPost
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*post = tumblrPost(decoded)
	post.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (post tumblrPost) identifier() string {
	if post.IDString != "" {
		return post.IDString
	}
	return string(post.ID)
}

type tumblrPostList struct {
	Blog       tumblrBlog   `json:"blog"`
	Posts      []tumblrPost `json:"posts"`
	TotalPosts int64        `json:"total_posts"`
	Links      tumblrLinks  `json:"_links"`
}

type tumblrLinks struct {
	Next *tumblrLink `json:"next"`
}

type tumblrLink struct {
	Type        string                     `json:"type"`
	Href        string                     `json:"href"`
	QueryParams map[string]json.RawMessage `json:"query_params"`
}

type tumblrNote struct {
	Type      string          `json:"type"`
	Timestamp float64         `json:"timestamp"`
	BlogName  string          `json:"blog_name"`
	BlogUUID  string          `json:"blog_uuid"`
	BlogURL   string          `json:"blog_url"`
	PostID    flexString      `json:"post_id"`
	ReplyID   flexString      `json:"reply_id"`
	ReplyText string          `json:"reply_text"`
	AddedText string          `json:"added_text"`
	Tags      []string        `json:"tags"`
	Raw       json.RawMessage `json:"-"`
}

func (note *tumblrNote) UnmarshalJSON(data []byte) error {
	type alias tumblrNote
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*note = tumblrNote(decoded)
	note.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type tumblrNotesResponse struct {
	Notes        []tumblrNote `json:"notes"`
	Rollup       []tumblrNote `json:"rollup_notes"`
	TotalNotes   int64        `json:"total_notes"`
	TotalLikes   int64        `json:"total_likes"`
	TotalReblogs int64        `json:"total_reblogs"`
	Links        tumblrLinks  `json:"_links"`
}

type tumblrIDResponse struct {
	ID flexString `json:"id"`
}

var _ NPFWorkflow = (*Client)(nil)
var _ TimelineWorkflow = (*Client)(nil)
var _ EngagementWorkflow = (*Client)(nil)
