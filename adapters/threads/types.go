package threads

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

// ContainerType identifies a Threads publication container media type.
type ContainerType string

const (
	ContainerText     ContainerType = "TEXT"
	ContainerImage    ContainerType = "IMAGE"
	ContainerVideo    ContainerType = "VIDEO"
	ContainerCarousel ContainerType = "CAROUSEL"
)

// ReplyControl selects who may reply to a Threads post.
type ReplyControl string

const (
	ReplyEveryone             ReplyControl = "everyone"
	ReplyAccountsYouFollow    ReplyControl = "accounts_you_follow"
	ReplyMentionedOnly        ReplyControl = "mentioned_only"
	ReplyParentPostAuthorOnly ReplyControl = "parent_post_author_only"
	ReplyFollowersOnly        ReplyControl = "followers_only"
)

// PollAttachment defines the two to four choices on a text-post poll.
type PollAttachment struct {
	OptionA string `json:"option_a"`
	OptionB string `json:"option_b"`
	OptionC string `json:"option_c,omitempty"`
	OptionD string `json:"option_d,omitempty"`
}

// ContainerRequest creates one remote-media or text publication container.
type ContainerRequest struct {
	Type                 ContainerType
	Text                 string
	ImageURL             string
	VideoURL             string
	AltText              string
	Children             []string
	CarouselItem         bool
	ReplyToID            string
	QuotePostID          string
	ReplyControl         ReplyControl
	TopicTag             string
	LocationID           string
	LinkAttachmentURL    string
	Poll                 *PollAttachment
	SpoilerMedia         bool
	GhostPost            bool
	EnableReplyApprovals bool
}

// ContainerStatus reports Threads media processing state.
type ContainerStatus struct {
	ID           string
	Status       string
	ErrorMessage string
}

// ContainerWorkflow exposes Threads create, status, and publish lifecycle.
type ContainerWorkflow interface {
	CreateContainer(context.Context, ContainerRequest, ...socialhub.CallOption) (*ContainerStatus, error)
	ContainerStatus(context.Context, string, ...socialhub.CallOption) (*ContainerStatus, error)
	PublishContainer(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error)
}

// InsightValue preserves scalar or structured metric values from Graph.
type InsightValue struct {
	Value   json.RawMessage
	EndTime *time.Time
}

// Insight is one post- or account-level Threads metric.
type Insight struct {
	ID          string
	Name        string
	Period      string
	Title       string
	Description string
	Values      []InsightValue
	TotalValue  *InsightValue
}

// InsightsWorkflow exposes post and account insights without flattening
// demographic objects into misleading scalar metrics.
type InsightsWorkflow interface {
	PostInsights(context.Context, string, []string, ...socialhub.CallOption) ([]Insight, error)
	AccountInsights(context.Context, []string, ...socialhub.CallOption) ([]Insight, error)
}

// PageRequest selects one cursor page for a typed workflow.
type PageRequest struct {
	Cursor     string
	MaxResults int
}

// KeywordSearchType selects top or recent Threads search results.
type KeywordSearchType string

const (
	KeywordSearchTop    KeywordSearchType = "TOP"
	KeywordSearchRecent KeywordSearchType = "RECENT"
)

// KeywordSearchRequest selects a public keyword result page.
type KeywordSearchRequest struct {
	Query      string
	Type       KeywordSearchType
	Cursor     string
	MaxResults int
}

// DiscoveryWorkflow exposes approved public-profile, keyword, and mention reads.
type DiscoveryWorkflow interface {
	LookupProfile(context.Context, string, ...socialhub.CallOption) (*socialhub.User, error)
	ProfilePosts(context.Context, string, PageRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
	KeywordSearch(context.Context, KeywordSearchRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
	Mentions(context.Context, PageRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
}

// ModerationWorkflow exposes reply hide and pending-approval actions.
type ModerationWorkflow interface {
	SetReplyHidden(context.Context, string, bool, ...socialhub.CallOption) error
	ReviewPendingReply(context.Context, string, bool, ...socialhub.CallOption) error
}

// RepostWorkflow exposes repost creation and returns its independently
// deletable Threads post ID.
type RepostWorkflow interface {
	Repost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error)
}

// QuotaConfig describes one server-provided rolling quota window.
type QuotaConfig struct {
	Total           int64 `json:"quota_total"`
	DurationSeconds int64 `json:"quota_duration"`
}

// PublishingQuota reports quota use instead of hard-coding hosted defaults.
type PublishingQuota struct {
	PostUsage           int64       `json:"quota_usage"`
	PostConfig          QuotaConfig `json:"config"`
	ReplyUsage          int64       `json:"reply_quota_usage"`
	ReplyConfig         QuotaConfig `json:"reply_config"`
	DeleteUsage         int64       `json:"delete_quota_usage"`
	DeleteConfig        QuotaConfig `json:"delete_config"`
	LocationSearchUsage int64       `json:"location_search_quota_usage"`
	LocationSearch      QuotaConfig `json:"location_search_config"`
}

// QuotaWorkflow reads the account's current publishing-related quota.
type QuotaWorkflow interface {
	PublishingQuota(context.Context, ...socialhub.CallOption) (*PublishingQuota, error)
}

type graphTime struct{ time.Time }

func (t *graphTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999-0700", "2006-01-02T15:04:05-0700"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("threads: invalid timestamp")
}

func (t graphTime) pointer() *time.Time {
	if t.IsZero() {
		return nil
	}
	value := t.Time
	return &value
}

type idReference struct {
	ID string `json:"id"`
}

type graphProfile struct {
	ID                   string          `json:"id"`
	Username             string          `json:"username"`
	Name                 string          `json:"name"`
	Verified             bool            `json:"is_verified"`
	ProfilePictureURL    string          `json:"threads_profile_picture_url"`
	Biography            string          `json:"threads_biography"`
	RecentlySearched     json.RawMessage `json:"recently_searched_keywords"`
	EligibleForGeoGating *bool           `json:"is_eligible_for_geo_gating"`
}

type graphPost struct {
	ID                 string          `json:"id"`
	MediaProductType   string          `json:"media_product_type"`
	MediaType          string          `json:"media_type"`
	MediaURL           string          `json:"media_url"`
	GIFURL             string          `json:"gif_url"`
	Permalink          string          `json:"permalink"`
	Owner              idReference     `json:"owner"`
	Username           string          `json:"username"`
	Text               string          `json:"text"`
	Timestamp          graphTime       `json:"timestamp"`
	Shortcode          string          `json:"shortcode"`
	ThumbnailURL       string          `json:"thumbnail_url"`
	Children           graphPostPage   `json:"children"`
	IsQuotePost        bool            `json:"is_quote_post"`
	QuotedPost         *graphPost      `json:"quoted_post"`
	RepostedPost       *graphPost      `json:"reposted_post"`
	HasReplies         bool            `json:"has_replies"`
	AltText            string          `json:"alt_text"`
	LinkAttachmentURL  string          `json:"link_attachment_url"`
	PollAttachment     json.RawMessage `json:"poll_attachment"`
	TextEntities       json.RawMessage `json:"text_entities"`
	LocationID         string          `json:"location_id"`
	TopicTag           string          `json:"topic_tag"`
	IsSpoilerMedia     bool            `json:"is_spoiler_media"`
	GhostPostStatus    string          `json:"ghost_post_status"`
	Verified           bool            `json:"is_verified"`
	ProfilePictureURL  string          `json:"profile_picture_url"`
	IsReply            bool            `json:"is_reply"`
	IsReplyOwnedByMe   bool            `json:"is_reply_owned_by_me"`
	RootPost           idReference     `json:"root_post"`
	RepliedTo          idReference     `json:"replied_to"`
	HideStatus         string          `json:"hide_status"`
	ReplyAudience      string          `json:"reply_audience"`
	ReplyApprovalState string          `json:"reply_approval_status"`
}

type graphCursors struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type graphPaging struct {
	Cursors  graphCursors `json:"cursors"`
	Next     string       `json:"next"`
	Previous string       `json:"previous"`
}

type graphPostPage struct {
	Data   []graphPost `json:"data"`
	Paging graphPaging `json:"paging"`
}

type idResponse struct {
	ID string `json:"id"`
}

type successResponse struct {
	Success   bool   `json:"success"`
	DeletedID string `json:"deleted_id"`
}

type containerStatusResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
}

type graphInsightValue struct {
	Value   json.RawMessage `json:"value"`
	EndTime graphTime       `json:"end_time"`
}

type graphInsight struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Period      string              `json:"period"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Values      []graphInsightValue `json:"values"`
	TotalValue  *graphInsightValue  `json:"total_value"`
}

type graphInsightPage struct {
	Data []graphInsight `json:"data"`
}

type graphQuotaPage struct {
	Data []PublishingQuota `json:"data"`
}

func validMetricNames(metrics []string) bool {
	if len(metrics) == 0 {
		return false
	}
	for _, metric := range metrics {
		if !validToken(metric) || strings.TrimSpace(metric) == "" {
			return false
		}
	}
	return true
}

var _ ContainerWorkflow = (*Client)(nil)
var _ InsightsWorkflow = (*Client)(nil)
var _ DiscoveryWorkflow = (*Client)(nil)
var _ ModerationWorkflow = (*Client)(nil)
var _ RepostWorkflow = (*Client)(nil)
var _ QuotaWorkflow = (*Client)(nil)
