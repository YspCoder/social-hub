package socialhub

import (
	"context"
	"io"
	"net/http"
)

// Client is the common entry point for one configured platform account.
type Client interface {
	CapabilityProvider
	Platform() Platform
	Account() AccountID
	Publisher() (Publisher, bool)
	Fetcher() (Fetcher, bool)
	MediaUploader() (MediaUploader, bool)
	Reactor() (Reactor, bool)
	Messenger() (Messenger, bool)
	WebhookHandler() (WebhookHandler, bool)
	Close() error
}

// Publisher creates and manages posts.
type Publisher interface {
	Publish(context.Context, CreatePostRequest, ...CallOption) (*Post, error)
	PublishStatus(context.Context, string, ...CallOption) (*PublishStatus, error)
	DeletePost(context.Context, string, ...CallOption) error
}

// Fetcher retrieves users, posts, and comments.
type Fetcher interface {
	GetUser(context.Context, string, ...CallOption) (*User, error)
	GetPost(context.Context, string, ...CallOption) (*Post, error)
	ListPosts(context.Context, ListPostsRequest, ...CallOption) (Page[Post], error)
	ListComments(context.Context, ListCommentsRequest, ...CallOption) (Page[Comment], error)
}

// MediaUploader handles simple and resumable media uploads.
type MediaUploader interface {
	BeginUpload(context.Context, BeginUploadRequest, ...CallOption) (*UploadSession, error)
	UploadPart(context.Context, string, int, io.Reader, ...CallOption) (*UploadedPart, error)
	CompleteUpload(context.Context, string, []UploadedPart, ...CallOption) (*Media, error)
	MediaStatus(context.Context, string, ...CallOption) (*Media, error)
}

// Reactor creates and removes interactions with posts.
type Reactor interface {
	React(context.Context, ReactionRequest, ...CallOption) error
	RemoveReaction(context.Context, ReactionRequest, ...CallOption) error
	Comment(context.Context, CreateCommentRequest, ...CallOption) (*Comment, error)
	DeleteComment(context.Context, string, ...CallOption) error
}

// Messenger sends and retrieves messages.
type Messenger interface {
	SendMessage(context.Context, SendMessageRequest, ...CallOption) (*Message, error)
	GetMessage(context.Context, string, ...CallOption) (*Message, error)
}

// WebhookHandler verifies and decodes callbacks for one adapter account.
type WebhookHandler interface {
	Verify(context.Context, *http.Request, []byte) error
	Decode(context.Context, *http.Request, []byte) ([]Event, error)
}

// Event is a normalized webhook event with a type-specific payload.
type Event struct {
	ID        string
	Type      string
	Platform  Platform
	AccountID AccountID
	Payload   any
}
