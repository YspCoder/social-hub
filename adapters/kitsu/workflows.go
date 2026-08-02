package kitsu

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

type PageRequest struct {
	Cursor string
	Limit  int
}

type SearchRequest struct {
	Query  string
	Cursor string
	Limit  int
}

type LibraryEntriesRequest struct {
	UserID    string
	MediaID   string
	MediaKind MediaKind
	Status    LibraryStatus
	Since     *time.Time
	Cursor    string
	Limit     int
}

type CreateLibraryEntryRequest struct {
	MediaID        string
	MediaKind      MediaKind
	Status         LibraryStatus
	Progress       *int
	VolumesOwned   *int
	Reconsuming    *bool
	ReconsumeCount *int
	Notes          *string
	Private        *bool
	RatingTwenty   *int
	StartedAt      *time.Time
	FinishedAt     *time.Time
}

type UpdateLibraryEntryRequest struct {
	ID             string
	Status         *LibraryStatus
	Progress       *int
	VolumesOwned   *int
	Reconsuming    *bool
	ReconsumeCount *int
	Notes          *string
	Private        *bool
	RatingTwenty   *int
	StartedAt      *time.Time
	FinishedAt     *time.Time
}

type PostsRequest struct {
	Cursor string
	Limit  int
}

type CreatePostRequest struct {
	Content       string
	Spoiler       bool
	NSFW          bool
	TargetGroupID string
	TargetUserID  string
	MediaID       string
	MediaKind     MediaKind
}

type UpdatePostRequest struct {
	ID      string
	Content *string
	Spoiler *bool
	NSFW    *bool
}

type CommentsRequest struct {
	PostID string
	Cursor string
	Limit  int
}

type CreateCommentRequest struct {
	PostID   string
	ParentID string
	Content  string
}

type UpdateCommentRequest struct {
	ID      string
	Content string
}

type TokenWorkflow interface {
	Refresh(context.Context, string) (socialhub.Token, error)
}

type AnimeWorkflow interface {
	SearchAnime(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Media], error)
	GetAnime(context.Context, string, ...socialhub.CallOption) (*Media, error)
}

type MangaWorkflow interface {
	SearchManga(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Media], error)
	GetManga(context.Context, string, ...socialhub.CallOption) (*Media, error)
}

type UserWorkflow interface {
	GetUser(context.Context, string, ...socialhub.CallOption) (*User, error)
	FindUserBySlug(context.Context, string, ...socialhub.CallOption) (*User, error)
	GetCurrentUser(context.Context, ...socialhub.CallOption) (*User, error)
}

type LibraryWorkflow interface {
	GetLibraryEntry(context.Context, string, ...socialhub.CallOption) (*LibraryEntry, error)
	ListLibraryEntries(context.Context, LibraryEntriesRequest, ...socialhub.CallOption) (socialhub.Page[LibraryEntry], error)
	CreateLibraryEntry(context.Context, CreateLibraryEntryRequest, ...socialhub.CallOption) (*LibraryEntry, error)
	UpdateLibraryEntry(context.Context, UpdateLibraryEntryRequest, ...socialhub.CallOption) (*LibraryEntry, error)
	DeleteLibraryEntry(context.Context, string, ...socialhub.CallOption) error
}

type PostWorkflow interface {
	ListPosts(context.Context, PostsRequest, ...socialhub.CallOption) (socialhub.Page[Post], error)
	GetPost(context.Context, string, ...socialhub.CallOption) (*Post, error)
	CreatePost(context.Context, CreatePostRequest, ...socialhub.CallOption) (*Post, error)
	UpdatePost(context.Context, UpdatePostRequest, ...socialhub.CallOption) (*Post, error)
	DeletePost(context.Context, string, ...socialhub.CallOption) error
}

type CommentWorkflow interface {
	ListComments(context.Context, CommentsRequest, ...socialhub.CallOption) (socialhub.Page[Comment], error)
	GetComment(context.Context, string, ...socialhub.CallOption) (*Comment, error)
	CreateComment(context.Context, CreateCommentRequest, ...socialhub.CallOption) (*Comment, error)
	UpdateComment(context.Context, UpdateCommentRequest, ...socialhub.CallOption) (*Comment, error)
	DeleteComment(context.Context, string, ...socialhub.CallOption) error
}

var _ TokenWorkflow = (*Client)(nil)
var _ AnimeWorkflow = (*Client)(nil)
var _ MangaWorkflow = (*Client)(nil)
var _ UserWorkflow = (*Client)(nil)
var _ LibraryWorkflow = (*Client)(nil)
var _ PostWorkflow = (*Client)(nil)
var _ CommentWorkflow = (*Client)(nil)
