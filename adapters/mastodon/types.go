package mastodon

import (
	"encoding/json"
	"time"
)

// TimelineRequest selects a page from the authenticated home timeline.
type TimelineRequest struct {
	Cursor     string
	MaxResults int
}

// InstanceInfo is the stable subset of /api/v2/instance used for feature and
// upload-limit discovery.
type InstanceInfo struct {
	Domain              string
	Title               string
	Version             string
	SourceURL           string
	MastodonAPIVersion  int
	MaxStatusCharacters int
	MaxMediaAttachments int
	ImageSizeLimit      int64
	VideoSizeLimit      int64
}

type mastodonAccount struct {
	ID             string          `json:"id"`
	Username       string          `json:"username"`
	Acct           string          `json:"acct"`
	DisplayName    string          `json:"display_name"`
	Locked         bool            `json:"locked"`
	Bot            bool            `json:"bot"`
	Discoverable   *bool           `json:"discoverable"`
	Group          bool            `json:"group"`
	CreatedAt      *time.Time      `json:"created_at"`
	Note           string          `json:"note"`
	URL            string          `json:"url"`
	URI            string          `json:"uri"`
	Avatar         string          `json:"avatar"`
	AvatarStatic   string          `json:"avatar_static"`
	Header         string          `json:"header"`
	HeaderStatic   string          `json:"header_static"`
	FollowersCount int64           `json:"followers_count"`
	FollowingCount int64           `json:"following_count"`
	StatusesCount  int64           `json:"statuses_count"`
	LastStatusAt   string          `json:"last_status_at"`
	Fields         json.RawMessage `json:"fields"`
}

type mastodonStatus struct {
	ID                 string               `json:"id"`
	URI                string               `json:"uri"`
	URL                string               `json:"url"`
	Account            mastodonAccount      `json:"account"`
	InReplyToID        *string              `json:"in_reply_to_id"`
	InReplyToAccountID *string              `json:"in_reply_to_account_id"`
	Reblog             *mastodonStatus      `json:"reblog"`
	QuotedStatusID     *string              `json:"quoted_status_id"`
	Quote              *mastodonQuote       `json:"quote"`
	Content            string               `json:"content"`
	CreatedAt          *time.Time           `json:"created_at"`
	EditedAt           *time.Time           `json:"edited_at"`
	RepliesCount       int64                `json:"replies_count"`
	ReblogsCount       int64                `json:"reblogs_count"`
	FavouritesCount    int64                `json:"favourites_count"`
	Reblogged          *bool                `json:"reblogged"`
	Favourited         *bool                `json:"favourited"`
	Bookmarked         *bool                `json:"bookmarked"`
	Sensitive          bool                 `json:"sensitive"`
	SpoilerText        string               `json:"spoiler_text"`
	Visibility         string               `json:"visibility"`
	Language           string               `json:"language"`
	MediaAttachments   []mastodonAttachment `json:"media_attachments"`
}

type mastodonQuote struct {
	State        string          `json:"state"`
	QuotedStatus *mastodonStatus `json:"quoted_status"`
}

type mastodonContext struct {
	Ancestors   []mastodonStatus `json:"ancestors"`
	Descendants []mastodonStatus `json:"descendants"`
}

type mastodonAttachment struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	URL         string                 `json:"url"`
	RemoteURL   string                 `json:"remote_url"`
	PreviewURL  string                 `json:"preview_url"`
	TextURL     string                 `json:"text_url"`
	Description string                 `json:"description"`
	BlurHash    string                 `json:"blurhash"`
	Meta        mastodonAttachmentMeta `json:"meta"`
}

type mastodonAttachmentMeta struct {
	Original mastodonAttachmentSize `json:"original"`
	Small    mastodonAttachmentSize `json:"small"`
	Focus    struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"focus"`
}

type mastodonAttachmentSize struct {
	Width     int64   `json:"width"`
	Height    int64   `json:"height"`
	Duration  float64 `json:"duration"`
	FrameRate string  `json:"frame_rate"`
	Bitrate   int64   `json:"bitrate"`
	Size      string  `json:"size"`
	Aspect    float64 `json:"aspect"`
}

type mastodonInstance struct {
	Domain      string `json:"domain"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	SourceURL   string `json:"source_url"`
	APIVersions struct {
		Mastodon int `json:"mastodon"`
	} `json:"api_versions"`
	Configuration struct {
		Statuses struct {
			MaxCharacters       int `json:"max_characters"`
			MaxMediaAttachments int `json:"max_media_attachments"`
		} `json:"statuses"`
		MediaAttachments struct {
			ImageSizeLimit int64 `json:"image_size_limit"`
			VideoSizeLimit int64 `json:"video_size_limit"`
		} `json:"media_attachments"`
	} `json:"configuration"`
}
