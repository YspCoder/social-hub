package slack

import (
	"context"
	"encoding/json"
	"io"

	"social-hub/pkg/socialhub"
)

// PostMessageRequest exposes an explicit Slack conversation and thread.
type PostMessageRequest struct {
	ChannelID    string
	Text         string
	ThreadPostID string
}

// UpdateMessageRequest updates a message identified by channel_id:ts.
type UpdateMessageRequest struct {
	PostID string
	Text   string
}

// ChatWorkflow exposes Slack chat controls not carried by the common
// Publisher's configured default channel.
type ChatWorkflow interface {
	PostMessage(context.Context, PostMessageRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	UpdateMessage(context.Context, UpdateMessageRequest, ...socialhub.CallOption) (*socialhub.Post, error)
}

// FileUploadRequest starts Slack's external file upload workflow. ChannelID
// empty leaves the completed file private.
type FileUploadRequest struct {
	Filename       string
	Size           int64
	MIME           string
	Title          string
	AltText        string
	SnippetType    string
	ChannelID      string
	ThreadPostID   string
	InitialComment string
}

// FileWorkflow exposes channel- and thread-aware external uploads.
type FileWorkflow interface {
	BeginFileUpload(context.Context, FileUploadRequest, ...socialhub.CallOption) (*socialhub.UploadSession, error)
	UploadFilePart(context.Context, string, int, io.Reader, ...socialhub.CallOption) (*socialhub.UploadedPart, error)
	CompleteFileUpload(context.Context, string, []socialhub.UploadedPart, ...socialhub.CallOption) (*socialhub.Media, error)
	GetFile(context.Context, string, ...socialhub.CallOption) (*socialhub.Media, error)
}

// ReactionEvent preserves the emoji and Slack item actor details.
type ReactionEvent struct {
	Reaction   string
	UserID     string
	ItemUserID string
	TargetID   string
	Added      bool
}

// EventsPayload is the typed payload returned by Events API decoding.
type EventsPayload struct {
	ID           string
	Type         string
	TeamID       string
	APIAppID     string
	EventContext string
	RetryNumber  int
	RetryReason  string
	Challenge    string
	Post         *socialhub.Post
	Message      *socialhub.Message
	Reaction     *ReactionEvent
	File         *socialhub.Media
	Raw          json.RawMessage
}

var _ ChatWorkflow = (*Client)(nil)
var _ FileWorkflow = (*Client)(nil)
