package dailymotion

import "time"

// Account identifies the authenticated API user and manageable profiles.
type Account struct {
	UserID   string           `json:"user_id"`
	Username string           `json:"username"`
	Profiles []ManagedProfile `json:"profiles"`
}

// ManagedProfile is the compact profile row returned by GET /me.
type ManagedProfile struct {
	ProfileID string `json:"profile_id"`
	Name      string `json:"name"`
}

// SocialLinks contains mutable profile links.
type SocialLinks struct {
	TwitterURL   *string `json:"twitter_url,omitempty"`
	InstagramURL *string `json:"instagram_url,omitempty"`
	FacebookURL  *string `json:"facebook_url,omitempty"`
	WebsiteURL   *string `json:"website_url,omitempty"`
}

// WebhookSettings configures API v2 video events on a profile.
type WebhookSettings struct {
	CallbackURL *string  `json:"callback_url,omitempty"`
	Events      []string `json:"events,omitempty"`
}

// Profile is a Dailymotion managed profile.
type Profile struct {
	ProfileID     string          `json:"profile_id"`
	Name          string          `json:"name"`
	DisplayName   *string         `json:"display_name,omitempty"`
	Description   *string         `json:"description,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	CanChangeName bool            `json:"can_change_name"`
	SocialLinks   SocialLinks     `json:"social_links"`
	Webhook       WebhookSettings `json:"webhook"`
}

// Processing describes Dailymotion's asynchronous encoding and publication state.
type Processing struct {
	EncodingStatus     string `json:"encoding_status"`
	EncodingProgress   int    `json:"encoding_progress"`
	PublishingProgress int    `json:"publishing_progress"`
}

// Source describes normalized source media metadata.
type Source struct {
	Duration    *float64 `json:"duration,omitempty"`
	Width       *int     `json:"width,omitempty"`
	Height      *int     `json:"height,omitempty"`
	AspectRatio *string  `json:"aspect_ratio,omitempty"`
	Checksum    *string  `json:"checksum,omitempty"`
}

// Thumbnail contains the API's generated thumbnail variants.
type Thumbnail struct {
	H1080URL *string `json:"h1080_url,omitempty"`
	H720URL  *string `json:"h720_url,omitempty"`
	H480URL  *string `json:"h480_url,omitempty"`
	H360URL  *string `json:"h360_url,omitempty"`
	H240URL  *string `json:"h240_url,omitempty"`
	H180URL  *string `json:"h180_url,omitempty"`
	H120URL  *string `json:"h120_url,omitempty"`
	H60URL   *string `json:"h60_url,omitempty"`
}

// Video is the typed API v2 video representation retained by this adapter.
type Video struct {
	VideoID                   string     `json:"video_id"`
	Title                     string     `json:"title"`
	Description               *string    `json:"description,omitempty"`
	Category                  string     `json:"category"`
	Visibility                string     `json:"visibility"`
	IsForKids                 *bool      `json:"is_for_kids,omitempty"`
	IsExplicit                bool       `json:"is_explicit"`
	CreatedAt                 time.Time  `json:"created_at"`
	Profile                   Profile    `json:"profile"`
	VideoURL                  string     `json:"video_url"`
	UpdatedAt                 time.Time  `json:"updated_at"`
	UploadedAt                time.Time  `json:"uploaded_at"`
	PublishedAt               *time.Time `json:"published_at,omitempty"`
	Language                  *string    `json:"language,omitempty"`
	Country                   *string    `json:"country,omitempty"`
	EngagementMessage         *string    `json:"engagement_message,omitempty"`
	Hashtags                  []string   `json:"hashtags"`
	Tags                      []string   `json:"tags"`
	IsPublished               bool       `json:"is_published"`
	Processing                Processing `json:"processing"`
	IsAIAltered               bool       `json:"is_ai_altered"`
	EnableAIChapterGeneration bool       `json:"enable_ai_chapter_generation"`
	Source                    Source     `json:"source"`
	Thumbnail                 Thumbnail  `json:"thumbnail"`
}

// Playlist is a Dailymotion profile playlist.
type Playlist struct {
	PlaylistID  string    `json:"playlist_id"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	Visibility  string    `json:"visibility"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Profile     Profile   `json:"profile"`
	PlaylistURL *string   `json:"playlist_url,omitempty"`
	EmbedURL    *string   `json:"embed_url,omitempty"`
}

// PlaylistVideo is the limited membership row guaranteed by API v2.
type PlaylistVideo struct {
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type apiPage[T any] struct {
	Data       []T `json:"data"`
	Pagination struct {
		Page     int     `json:"page"`
		PageSize int     `json:"page_size"`
		Total    *int    `json:"total"`
		Next     *string `json:"next"`
		Previous *string `json:"previous"`
	} `json:"pagination"`
}

type uploadSessionResponse struct {
	UploadURL   string `json:"upload_url"`
	ProgressURL string `json:"progress_url"`
}

type uploadFileResponse struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	Format   string `json:"format"`
	Duration string `json:"duration"`
	Size     string `json:"size"`
	Hash     string `json:"hash"`
}
