package dailymotion

import (
	"context"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	ScopeAccountRead     = "account.read"
	ScopeAccountManage   = "account.manage"
	ScopeProfileRead     = "profile.read"
	ScopeProfileManage   = "profile.manage"
	ScopeVideoRead       = "video.read"
	ScopeVideoManage     = "video.manage"
	ScopePlaylistRead    = "playlist.read"
	ScopePlaylistManage  = "playlist.manage"
	ScopeLiveRead        = "live.read"
	ScopeLiveManage      = "live.manage"
	ScopePlayerRead      = "player.read"
	ScopePlayerManage    = "player.manage"
	ScopeAnalyticsManage = "analytics.manage"

	BundlePublic       = "bundle.public"
	BundleUser         = "bundle.user"
	BundlePublisher    = "bundle.publisher"
	BundleOrganization = "bundle.organization"
)

// VideoListRequest selects videos owned by a profile.
type VideoListRequest struct {
	ProfileID     string
	Cursor        string
	MaxResults    int
	Sort          string
	Visibility    string
	IsExplicit    *bool
	IsForKids     *bool
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Tags          []string
}

// PlaylistListRequest selects playlists owned by a profile.
type PlaylistListRequest struct {
	ProfileID     string
	Cursor        string
	MaxResults    int
	Sort          string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
}

// CreateVideoRequest includes Dailymotion's mandatory publication semantics.
type CreateVideoRequest struct {
	ProfileID                 string
	Title                     string
	Description               string
	Category                  string
	Visibility                string
	IsForKids                 bool
	IsExplicit                bool
	Password                  string
	PublishedAt               *time.Time
	Language                  string
	Country                   string
	EngagementMessage         string
	Hashtags                  []string
	Tags                      []string
	IsAIAltered               bool
	EnableAIChapterGeneration bool
	EnableEmbed               *bool
	SourceURL                 string
}

// UpdateVideoRequest changes only non-nil fields.
type UpdateVideoRequest struct {
	Title                     *string
	Description               *string
	Category                  *string
	Visibility                *string
	IsForKids                 *bool
	IsExplicit                *bool
	Password                  *string
	PublishedAt               *time.Time
	Language                  *string
	Country                   *string
	EngagementMessage         *string
	Hashtags                  *[]string
	Tags                      *[]string
	IsAIAltered               *bool
	EnableAIChapterGeneration *bool
	EnableEmbed               *bool
	SourceURL                 *string
}

// UpdateProfileRequest changes profile metadata or webhook delivery settings.
// X-DM-Signature verification is intentionally outside this contract because
// Dailymotion does not publicly document its signing algorithm.
type UpdateProfileRequest struct {
	DisplayName *string
	Description *string
	SocialLinks *SocialLinks
	Webhook     *WebhookSettings
}

// CreatePlaylistRequest creates an empty profile playlist.
type CreatePlaylistRequest struct {
	ProfileID   string
	Title       string
	Description string
	Visibility  string
}

// UpdatePlaylistRequest changes only non-nil fields.
type UpdatePlaylistRequest struct {
	Title       *string
	Description *string
	Visibility  *string
}

// PlaylistVideosRequest selects one ordered playlist page.
type PlaylistVideosRequest struct {
	PlaylistID string
	Cursor     string
	MaxResults int
	Sort       string
}

// UploadSession identifies a process-local Dailymotion file upload.
type UploadSession struct {
	ID          string
	ProgressURL string
	Filename    string
	Size        int64
}

// UploadedFile contains the source URL returned by Dailymotion's upload host.
type UploadedFile struct {
	URL      string
	Name     string
	Format   string
	Duration string
	Size     string
	Checksum string
}

// ProfileWorkflow exposes identity, profile metadata, and webhook configuration.
type ProfileWorkflow interface {
	CurrentAccount(context.Context, ...socialhub.CallOption) (*Account, error)
	GetProfile(context.Context, string, ...socialhub.CallOption) (*Profile, error)
	UpdateProfile(context.Context, string, UpdateProfileRequest, ...socialhub.CallOption) error
}

// VideoWorkflow exposes typed video metadata operations.
type VideoWorkflow interface {
	GetVideo(context.Context, string, ...socialhub.CallOption) (*Video, error)
	ListVideos(context.Context, VideoListRequest, ...socialhub.CallOption) (socialhub.Page[Video], error)
	CreateVideo(context.Context, CreateVideoRequest, ...socialhub.CallOption) (*Video, error)
	UpdateVideo(context.Context, string, UpdateVideoRequest, ...socialhub.CallOption) error
	DeleteVideo(context.Context, string, ...socialhub.CallOption) error
}

// VideoUploadWorkflow exposes the upload session, file transfer, and publish lifecycle.
type VideoUploadWorkflow interface {
	Initialize(context.Context, string, int64, ...socialhub.CallOption) (*UploadSession, error)
	Upload(context.Context, string, io.Reader, ...socialhub.CallOption) (*UploadedFile, error)
	Publish(context.Context, string, CreateVideoRequest, ...socialhub.CallOption) (*Video, error)
	Abort(string) error
}

// PlaylistWorkflow exposes playlist CRUD and ordered membership operations.
type PlaylistWorkflow interface {
	GetPlaylist(context.Context, string, ...socialhub.CallOption) (*Playlist, error)
	ListPlaylists(context.Context, PlaylistListRequest, ...socialhub.CallOption) (socialhub.Page[Playlist], error)
	CreatePlaylist(context.Context, CreatePlaylistRequest, ...socialhub.CallOption) (*Playlist, error)
	UpdatePlaylist(context.Context, string, UpdatePlaylistRequest, ...socialhub.CallOption) error
	DeletePlaylist(context.Context, string, ...socialhub.CallOption) error
	ListPlaylistVideos(context.Context, PlaylistVideosRequest, ...socialhub.CallOption) (socialhub.Page[PlaylistVideo], error)
	AddPlaylistVideo(context.Context, string, string, string, ...socialhub.CallOption) (*PlaylistVideo, error)
	RemovePlaylistVideo(context.Context, string, string, ...socialhub.CallOption) error
}
