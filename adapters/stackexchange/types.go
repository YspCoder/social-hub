package stackexchange

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

// BadgeCounts preserves Stack Exchange badge totals.
type BadgeCounts struct {
	Bronze int64 `json:"bronze"`
	Silver int64 `json:"silver"`
	Gold   int64 `json:"gold"`
}

// UserDetails contains Stack Exchange fields preserved in common model extensions.
type UserDetails struct {
	UserID       int64       `json:"user_id"`
	AccountID    int64       `json:"account_id,omitempty"`
	DisplayName  string      `json:"display_name,omitempty"`
	ProfileImage string      `json:"profile_image,omitempty"`
	Link         string      `json:"link,omitempty"`
	WebsiteURL   string      `json:"website_url,omitempty"`
	Location     string      `json:"location,omitempty"`
	UserType     string      `json:"user_type,omitempty"`
	Reputation   int64       `json:"reputation,omitempty"`
	CreationDate int64       `json:"creation_date,omitempty"`
	LastAccess   int64       `json:"last_access_date,omitempty"`
	Badges       BadgeCounts `json:"badge_counts,omitempty"`
}

// PostDetails contains question and answer fields preserved in common extensions.
type PostDetails struct {
	PostID           int64       `json:"post_id,omitempty"`
	PostType         string      `json:"post_type,omitempty"`
	QuestionID       int64       `json:"question_id,omitempty"`
	AnswerID         int64       `json:"answer_id,omitempty"`
	AcceptedAnswerID int64       `json:"accepted_answer_id,omitempty"`
	Owner            UserDetails `json:"owner,omitempty"`
	Title            string      `json:"title,omitempty"`
	Body             string      `json:"body,omitempty"`
	BodyMarkdown     string      `json:"body_markdown,omitempty"`
	Tags             []string    `json:"tags,omitempty"`
	Link             string      `json:"link,omitempty"`
	ContentLicense   string      `json:"content_license,omitempty"`
	CreationDate     int64       `json:"creation_date,omitempty"`
	LastActivityDate int64       `json:"last_activity_date,omitempty"`
	Score            int64       `json:"score,omitempty"`
	ViewCount        int64       `json:"view_count,omitempty"`
	AnswerCount      int64       `json:"answer_count,omitempty"`
	CommentCount     int64       `json:"comment_count,omitempty"`
	FavoriteCount    int64       `json:"favorite_count,omitempty"`
	IsAccepted       bool        `json:"is_accepted,omitempty"`
	ClosedDate       int64       `json:"closed_date,omitempty"`
	ClosedReason     string      `json:"closed_reason,omitempty"`
}

// CommentDetails contains Stack Exchange comment fields preserved in extensions.
type CommentDetails struct {
	CommentID    int64       `json:"comment_id"`
	PostID       int64       `json:"post_id"`
	Owner        UserDetails `json:"owner,omitempty"`
	ReplyToUser  UserDetails `json:"reply_to_user,omitempty"`
	Body         string      `json:"body,omitempty"`
	BodyMarkdown string      `json:"body_markdown,omitempty"`
	CreationDate int64       `json:"creation_date,omitempty"`
	Score        int64       `json:"score,omitempty"`
	Edited       bool        `json:"edited,omitempty"`
}

// Quota is the latest wrapper-provided daily quota and backoff snapshot.
type Quota struct {
	Maximum    int
	Remaining  int
	Backoff    time.Duration
	Method     string
	ObservedAt time.Time
}

// CreateQuestionRequest contains fields required by /questions/add.
type CreateQuestionRequest struct {
	Title string
	Body  string
	Tags  []string
}

// CreateAnswerRequest contains fields required by /questions/{id}/answers/add.
type CreateAnswerRequest struct {
	QuestionID string
	Body       string
}

// SearchQuestionsRequest selects questions through /search/advanced.
type SearchQuestionsRequest struct {
	Query      string
	Tagged     []string
	Cursor     string
	MaxResults int
	Sort       string
}

// QnAWorkflow exposes operations whose required fields do not fit the common Publisher.
type QnAWorkflow interface {
	CreateQuestion(context.Context, CreateQuestionRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	CreateAnswer(context.Context, CreateAnswerRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	SearchQuestions(context.Context, SearchQuestionsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
}

var _ QnAWorkflow = (*Client)(nil)
