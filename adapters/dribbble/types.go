package dribbble

import (
	"context"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

// User is Dribbble's authorized-user representation.
type User struct {
	ID            int64             `json:"id"`
	Name          string            `json:"name"`
	Login         string            `json:"login"`
	HTMLURL       string            `json:"html_url"`
	AvatarURL     string            `json:"avatar_url"`
	Bio           string            `json:"bio"`
	Location      string            `json:"location"`
	Links         map[string]string `json:"links"`
	CanUploadShot bool              `json:"can_upload_shot"`
	Pro           bool              `json:"pro"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// Images contains Dribbble's image renditions.
type Images struct {
	HiDPI  string `json:"hidpi"`
	Normal string `json:"normal"`
	Teaser string `json:"teaser"`
}

// Video contains an optional Shot video and previews.
type Video struct {
	ID               int64     `json:"id"`
	Duration         int64     `json:"duration"`
	Filename         string    `json:"video_file_name"`
	Size             int64     `json:"video_file_size"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	Silent           bool      `json:"silent"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	URL              string    `json:"url"`
	SmallPreviewURL  string    `json:"small_preview_url"`
	LargePreviewURL  string    `json:"large_preview_url"`
	XLargePreviewURL string    `json:"xlarge_preview_url"`
}

// Attachment is a file attached to a Shot.
type Attachment struct {
	ID           int64     `json:"id"`
	URL          string    `json:"url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type"`
	CreatedAt    time.Time `json:"created_at"`
}

// Project groups Shots belonging to the authorized user.
type Project struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ShotsCount  int64     `json:"shots_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Shot is Dribbble's published design object.
type Shot struct {
	ID               int64        `json:"id"`
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	DescriptionText  string       `json:"description_text"`
	Width            int          `json:"width"`
	Height           int          `json:"height"`
	Images           Images       `json:"images"`
	PublishedAt      time.Time    `json:"published_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	HTMLURL          string       `json:"html_url"`
	Animated         bool         `json:"animated"`
	Tags             []string     `json:"tags"`
	Attachments      []Attachment `json:"attachments"`
	Projects         []Project    `json:"projects"`
	Team             *User        `json:"team"`
	User             *User        `json:"user"`
	Video            *Video       `json:"video"`
	LowProfile       bool         `json:"low_profile"`
	ViewsCount       int64        `json:"views_count"`
	LikesCount       int64        `json:"likes_count"`
	CommentsCount    int64        `json:"comments_count"`
	AttachmentsCount int64        `json:"attachments_count"`
}

// RateLimit is the latest X-RateLimit header snapshot.
type RateLimit struct {
	Limit      int64
	Remaining  int64
	ResetAt    time.Time
	ObservedAt time.Time
}

// PendingResource describes an accepted asynchronous Dribbble operation.
type PendingResource struct {
	ID       string
	Location string
	State    socialhub.PublishState
}

// CreateShotRequest describes Dribbble's image-only Shot upload.
type CreateShotRequest struct {
	Filename        string
	MIME            string
	Size            int64
	Title           string
	Description     string
	LowProfile      bool
	ReboundSourceID string
	ScheduledFor    *time.Time
	Tags            []string
	TeamID          string
}

// UpdateShotRequest contains mutable Shot metadata.
type UpdateShotRequest struct {
	Title        *string
	Description  *string
	LowProfile   *bool
	ScheduledFor *time.Time
	Tags         []string
	TeamID       *string
}

// ShotWorkflow exposes Dribbble's publishing-specific Shot lifecycle.
type ShotWorkflow interface {
	CreateShot(context.Context, CreateShotRequest, io.Reader, ...socialhub.CallOption) (*PendingResource, error)
	UpdateShot(context.Context, string, UpdateShotRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	DeleteShot(context.Context, string, ...socialhub.CallOption) error
}

// CreateProjectRequest creates a named Project.
type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpdateProjectRequest contains mutable Project fields.
type UpdateProjectRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ProjectWorkflow manages the authorized user's Projects.
type ProjectWorkflow interface {
	ListProjects(context.Context, string, int, ...socialhub.CallOption) (socialhub.Page[Project], error)
	CreateProject(context.Context, CreateProjectRequest, ...socialhub.CallOption) (*Project, error)
	UpdateProject(context.Context, string, UpdateProjectRequest, ...socialhub.CallOption) (*Project, error)
	DeleteProject(context.Context, string, ...socialhub.CallOption) error
}

// AttachmentUploadRequest describes an exact-length file attached to a Shot.
type AttachmentUploadRequest struct {
	ShotID   string
	Filename string
	MIME     string
	Size     int64
}

// AttachmentUpload reports asynchronous acceptance; v2 does not return an ID.
type AttachmentUpload struct {
	ShotID string
	State  socialhub.PublishState
}

// AttachmentWorkflow uploads and deletes Shot attachments.
type AttachmentWorkflow interface {
	UploadAttachment(context.Context, AttachmentUploadRequest, io.Reader, ...socialhub.CallOption) (*AttachmentUpload, error)
	DeleteAttachment(context.Context, string, string, ...socialhub.CallOption) error
}

var _ ShotWorkflow = (*Client)(nil)
var _ ProjectWorkflow = (*Client)(nil)
var _ AttachmentWorkflow = (*Client)(nil)
