package socialhub

import (
	"errors"
	"time"
)

// CreatePostRequest is the platform-neutral subset of a publication request.
type CreatePostRequest struct {
	Text        *string  `json:"text,omitempty"`
	MediaIDs    []string `json:"media_ids,omitempty"`
	ReplyToID   *string  `json:"reply_to_id,omitempty"`
	QuotePostID *string  `json:"quote_post_id,omitempty"`
	Visibility  *string  `json:"visibility,omitempty"`
}

// Validate rejects requests that cannot represent any publishable content.
func (r CreatePostRequest) Validate() error {
	if (r.Text == nil || *r.Text == "") && len(r.MediaIDs) == 0 {
		return errors.New("socialhub: post requires text or media")
	}
	return nil
}

// ListPostsRequest selects posts belonging to a user or the configured account.
type ListPostsRequest struct {
	UserID     string
	Cursor     string
	MaxResults int
	StartTime  *time.Time
	EndTime    *time.Time
}

// ListCommentsRequest selects comments for one post.
type ListCommentsRequest struct {
	PostID     string
	Cursor     string
	MaxResults int
}

// BeginUploadRequest describes media before any bytes are uploaded.
type BeginUploadRequest struct {
	Filename string
	Type     MediaType
	MIME     string
	Size     int64
	Category string
}

// UploadSession identifies a resumable platform upload.
type UploadSession struct {
	ID        string
	MediaID   string
	PartSize  int64
	ExpiresAt *time.Time
}

// UploadedPart records one accepted upload part.
type UploadedPart struct {
	Number int
	ETag   string
	Size   int64
}

// ReactionKind is a normalized interaction type.
type ReactionKind string

const (
	ReactionLike   ReactionKind = "like"
	ReactionRepost ReactionKind = "repost"
)

// ReactionRequest targets a post as a specific platform user.
type ReactionRequest struct {
	ActorID  string
	TargetID string
	Kind     ReactionKind
}

// CreateCommentRequest creates a comment or reply.
type CreateCommentRequest struct {
	PostID   string
	ParentID *string
	Text     string
}

// SendMessageRequest creates an outbound message.
type SendMessageRequest struct {
	ConversationID string
	RecipientIDs   []string
	Text           *string
	MediaIDs       []string
	ReplyToID      *string
}
