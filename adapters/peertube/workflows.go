package peertube

import (
	"context"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

// VideoListRequest selects a page of global, account, or channel videos.
type VideoListRequest struct {
	AccountName   string
	ChannelHandle string
	Cursor        string
	MaxResults    int
	Sort          string
	Search        string
}

// ChannelListRequest selects a page of video channels.
type ChannelListRequest struct {
	Cursor     string
	MaxResults int
	Sort       string
}

// UploadVideoRequest contains PeerTube's required publication context and a
// conservative subset of optional metadata supported by the legacy uploader.
type UploadVideoRequest struct {
	Filename              string
	MIME                  string
	ChannelID             int64
	Name                  string
	Privacy               *int
	Category              *int
	Licence               *int
	Language              *string
	Description           *string
	WaitTranscoding       *bool
	GenerateTranscription *bool
	Support               *string
	NSFW                  *bool
	Tags                  []string
	CommentsPolicy        *int
	DownloadEnabled       *bool
	OriginallyPublishedAt *time.Time
}

// UpdateVideoRequest changes only non-nil metadata fields.
type UpdateVideoRequest struct {
	ChannelID             *int64
	Name                  *string
	Privacy               *int
	Category              *int
	Licence               *int
	Language              *string
	Description           *string
	WaitTranscoding       *bool
	Support               *string
	NSFW                  *bool
	Tags                  *[]string
	CommentsPolicy        *int
	DownloadEnabled       *bool
	OriginallyPublishedAt *time.Time
}

// VideoUploadResult is the minimal object returned by PeerTube after upload.
type VideoUploadResult struct {
	ID        int64  `json:"id"`
	UUID      string `json:"uuid"`
	ShortUUID string `json:"shortUUID,omitempty"`
}

// VideoWorkflow exposes typed video reads and the PeerTube publication lifecycle.
type VideoWorkflow interface {
	GetVideo(context.Context, string, ...socialhub.CallOption) (*Video, error)
	ListVideos(context.Context, VideoListRequest, ...socialhub.CallOption) (socialhub.Page[Video], error)
	UploadVideo(context.Context, UploadVideoRequest, io.Reader, ...socialhub.CallOption) (*VideoUploadResult, error)
	UpdateVideo(context.Context, string, UpdateVideoRequest, ...socialhub.CallOption) error
	DeleteVideo(context.Context, string, ...socialhub.CallOption) error
}

// ChannelWorkflow exposes public video-channel discovery.
type ChannelWorkflow interface {
	GetChannel(context.Context, string, ...socialhub.CallOption) (*VideoChannel, error)
	ListChannels(context.Context, ChannelListRequest, ...socialhub.CallOption) (socialhub.Page[VideoChannel], error)
}

// CommentWorkflow exposes operations whose PeerTube path requires both video
// and comment identifiers and therefore cannot be represented by Reactor alone.
type CommentWorkflow interface {
	GetCommentThread(context.Context, string, string, ...socialhub.CallOption) (*VideoCommentThread, error)
	DeleteVideoComment(context.Context, string, string, ...socialhub.CallOption) error
}
