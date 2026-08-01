package flickr

import (
	"context"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

// PhotoListRequest selects a Flickr member's photostream page.
type PhotoListRequest struct {
	UserID     string
	Cursor     string
	MaxResults int
	StartTime  *time.Time
	EndTime    *time.Time
	SafeSearch int
	Privacy    int
}

// UpdatePhotoRequest changes title or description. Non-nil empty values clear fields.
type UpdatePhotoRequest struct {
	Title       *string
	Description *string
}

// UploadPhotoRequest carries Flickr Upload API metadata.
type UploadPhotoRequest struct {
	Filename    string
	MIME        string
	Size        int64
	Title       string
	Description string
	Tags        []string
	IsPublic    *bool
	IsFriend    *bool
	IsFamily    *bool
	SafetyLevel int
	ContentType int
	Hidden      int
}

// UploadResult identifies the resource created by Flickr's direct upload.
type UploadResult struct {
	PhotoID string
}

// AlbumListRequest selects a Flickr member's photoset page.
type AlbumListRequest struct {
	UserID     string
	Cursor     string
	MaxResults int
}

// AlbumPhotosRequest selects one photoset page.
type AlbumPhotosRequest struct {
	AlbumID    string
	OwnerID    string
	Cursor     string
	MaxResults int
	Privacy    int
	Media      string
}

// CreateAlbumRequest creates a photoset around an existing owned photo.
type CreateAlbumRequest struct {
	Title          string
	Description    string
	PrimaryPhotoID string
}

// PhotoWorkflow exposes typed photo operations.
type PhotoWorkflow interface {
	GetPhoto(context.Context, string, ...socialhub.CallOption) (*Photo, error)
	ListPhotos(context.Context, PhotoListRequest, ...socialhub.CallOption) (socialhub.Page[PhotoSummary], error)
	UpdatePhoto(context.Context, string, UpdatePhotoRequest, ...socialhub.CallOption) error
	DeletePhoto(context.Context, string, ...socialhub.CallOption) error
}

// PhotoUploadWorkflow exposes Flickr's single-request upload lifecycle.
type PhotoUploadWorkflow interface {
	Upload(context.Context, UploadPhotoRequest, io.Reader, ...socialhub.CallOption) (*UploadResult, error)
}

// AlbumWorkflow exposes Flickr photoset reads, creation, and membership.
type AlbumWorkflow interface {
	GetAlbum(context.Context, string, string, ...socialhub.CallOption) (*Album, error)
	ListAlbums(context.Context, AlbumListRequest, ...socialhub.CallOption) (socialhub.Page[Album], error)
	ListAlbumPhotos(context.Context, AlbumPhotosRequest, ...socialhub.CallOption) (socialhub.Page[PhotoSummary], error)
	CreateAlbum(context.Context, CreateAlbumRequest, ...socialhub.CallOption) (*AlbumReference, error)
	AddAlbumPhoto(context.Context, string, string, ...socialhub.CallOption) error
	RemoveAlbumPhoto(context.Context, string, string, ...socialhub.CallOption) error
}
