// Package video defines resumable short-video publication workflows.
package video

import (
	"context"
	"io"
	"time"
)

// State is the normalized state of a video publication workflow.
type State string

const (
	StateCreated        State = "created"
	StateUploading      State = "uploading"
	StateUploaded       State = "uploaded"
	StateProcessing     State = "processing"
	StatePublishPending State = "publish_pending"
	StatePublished      State = "published"
	StateFailed         State = "failed"
	StateExpired        State = "expired"
)

// CreateRequest describes a video before upload.
type CreateRequest struct {
	Filename string
	MIME     string
	Size     int64
}

// Session identifies a resumable upload.
type Session struct {
	ID        string
	State     State
	PartSize  int64
	ExpiresAt *time.Time
}

// PublishRequest supplies content metadata after upload.
type PublishRequest struct {
	Title       string
	Description string
	CoverID     string
}

// Job represents asynchronous processing and publication.
type Job struct {
	ID        string
	PostID    string
	State     State
	Message   string
	UpdatedAt *time.Time
}

// Workflow uploads, processes, and publishes short video.
type Workflow interface {
	Create(context.Context, CreateRequest) (*Session, error)
	Upload(context.Context, string, io.Reader, int64) error
	Complete(context.Context, string) error
	Publish(context.Context, string, PublishRequest) (*Job, error)
	Status(context.Context, string) (*Job, error)
	Abort(context.Context, string) error
}
