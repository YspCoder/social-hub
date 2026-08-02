package discourse

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type discourseAction struct {
	ID      int  `json:"id"`
	Count   int  `json:"count"`
	Acted   bool `json:"acted"`
	CanUndo bool `json:"can_undo"`
	CanAct  bool `json:"can_act"`
}

type discoursePost struct {
	ID                int64             `json:"id"`
	Name              string            `json:"name"`
	Username          string            `json:"username"`
	AvatarTemplate    string            `json:"avatar_template"`
	CreatedAt         *time.Time        `json:"created_at"`
	UpdatedAt         *time.Time        `json:"updated_at"`
	RawText           string            `json:"raw"`
	Cooked            string            `json:"cooked"`
	PostNumber        int               `json:"post_number"`
	ReplyToPostNumber json.RawMessage   `json:"reply_to_post_number"`
	ReplyCount        int               `json:"reply_count"`
	Reads             int               `json:"reads"`
	ReadersCount      int               `json:"readers_count"`
	TopicID           int64             `json:"topic_id"`
	TopicSlug         string            `json:"topic_slug"`
	TopicTitle        string            `json:"topic_title"`
	UserID            int64             `json:"user_id"`
	DeletedAt         *time.Time        `json:"deleted_at"`
	Hidden            bool              `json:"hidden"`
	PostURL           string            `json:"post_url"`
	Actions           []discourseAction `json:"actions_summary"`
	Raw               json.RawMessage   `json:"-"`
}

func (post *discoursePost) UnmarshalJSON(data []byte) error {
	type alias discoursePost
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*post = discoursePost(decoded)
	post.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type discourseUser struct {
	ID             int64           `json:"id"`
	Username       string          `json:"username"`
	Name           string          `json:"name"`
	AvatarTemplate string          `json:"avatar_template"`
	CreatedAt      *time.Time      `json:"created_at"`
	LastPostedAt   *time.Time      `json:"last_posted_at"`
	LastSeenAt     *time.Time      `json:"last_seen_at"`
	TrustLevel     int             `json:"trust_level"`
	Moderator      bool            `json:"moderator"`
	Admin          bool            `json:"admin"`
	Title          *string         `json:"title"`
	PostCount      int             `json:"post_count"`
	TopicCount     int             `json:"topic_count"`
	Raw            json.RawMessage `json:"-"`
}

func (user *discourseUser) UnmarshalJSON(data []byte) error {
	type alias discourseUser
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*user = discourseUser(decoded)
	user.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type discourseUpload struct {
	ID               int64           `json:"id"`
	URL              string          `json:"url"`
	OriginalFilename string          `json:"original_filename"`
	FileSize         int64           `json:"filesize"`
	Width            int             `json:"width"`
	Height           int             `json:"height"`
	Extension        string          `json:"extension"`
	ShortURL         string          `json:"short_url"`
	ShortPath        string          `json:"short_path"`
	Raw              json.RawMessage `json:"-"`
}

func (upload *discourseUpload) UnmarshalJSON(data []byte) error {
	type alias discourseUpload
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*upload = discourseUpload(decoded)
	upload.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type topicResponse struct {
	ID           int64      `json:"id"`
	Title        string     `json:"title"`
	Slug         string     `json:"slug"`
	CategoryID   int64      `json:"category_id"`
	PostsCount   int        `json:"posts_count"`
	ReplyCount   int        `json:"reply_count"`
	Views        int        `json:"views"`
	LikeCount    int        `json:"like_count"`
	CreatedAt    *time.Time `json:"created_at"`
	LastPostedAt *time.Time `json:"last_posted_at"`
	Visible      bool       `json:"visible"`
	Closed       bool       `json:"closed"`
	Archived     bool       `json:"archived"`
	Archetype    string     `json:"archetype"`
	PostStream   struct {
		Posts  []discoursePost `json:"posts"`
		Stream []int64         `json:"stream"`
	} `json:"post_stream"`
	Raw json.RawMessage `json:"-"`
}

func (topic *topicResponse) UnmarshalJSON(data []byte) error {
	type alias topicResponse
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*topic = topicResponse(decoded)
	topic.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type latestPostsResponse struct {
	Posts []discoursePost `json:"latest_posts"`
}

type userResponse struct {
	User discourseUser `json:"user"`
}

type createPostPayload struct {
	Title             string `json:"title,omitempty"`
	Raw               string `json:"raw"`
	TopicID           int64  `json:"topic_id,omitempty"`
	Category          int64  `json:"category,omitempty"`
	TargetRecipients  string `json:"target_recipients,omitempty"`
	Archetype         string `json:"archetype,omitempty"`
	ReplyToPostNumber int    `json:"reply_to_post_number,omitempty"`
}

type postActionPayload struct {
	ID               int64 `json:"id"`
	PostActionTypeID int   `json:"post_action_type_id"`
}

// Topic preserves the Discourse topic fields that do not fit socialhub.Post.
type Topic struct {
	ID           string
	Title        string
	Slug         string
	CategoryID   string
	PostsCount   int
	ReplyCount   int
	Views        int
	LikeCount    int
	CreatedAt    *time.Time
	LastPostedAt *time.Time
	Visible      bool
	Closed       bool
	Archived     bool
	Archetype    string
	Posts        []socialhub.Post
	PostIDs      []string
	Raw          json.RawMessage
}

// CreateTopicRequest creates the first post in a public topic.
type CreateTopicRequest struct {
	Title      string
	Raw        string
	CategoryID string
}

// CreatePrivateMessageRequest creates a private-message topic.
type CreatePrivateMessageRequest struct {
	Title      string
	Raw        string
	Recipients []string
}

// PrivateMessage identifies a newly created private-message topic and its
// first post.
type PrivateMessage struct {
	TopicID    string
	Title      string
	Recipients []string
	FirstPost  socialhub.Post
}

// TopicWorkflow exposes Discourse topic semantics without flattening them into
// the platform-neutral post contract.
type TopicWorkflow interface {
	CreateTopic(context.Context, CreateTopicRequest, ...socialhub.CallOption) (*Topic, error)
	GetTopic(context.Context, string, ...socialhub.CallOption) (*Topic, error)
	ListLatestPosts(context.Context, string, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
}

// PrivateMessageWorkflow creates Discourse private-message topics.
type PrivateMessageWorkflow interface {
	CreatePrivateMessage(context.Context, CreatePrivateMessageRequest, ...socialhub.CallOption) (*PrivateMessage, error)
}
