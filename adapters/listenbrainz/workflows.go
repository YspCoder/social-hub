package listenbrainz

import (
	"context"

	"social-hub/pkg/socialhub"
)

// ListensRequest selects a descending page of one user's listens.
// MinTimestamp and MaxTimestamp are exclusive and cannot be combined.
type ListensRequest struct {
	Username     string
	MinTimestamp int64
	MaxTimestamp int64
	Count        int
}

// FeedbackListRequest selects a page of one user's recording feedback.
type FeedbackListRequest struct {
	Username   string
	Score      *FeedbackScore
	Metadata   bool
	Cursor     string
	MaxResults int
}

// PlaylistPageRequest selects an offset-based playlist page.
type PlaylistPageRequest struct {
	Cursor     string
	MaxResults int
}

// PlaylistSearchRequest searches public playlist titles and descriptions.
type PlaylistSearchRequest struct {
	Query      string
	Cursor     string
	MaxResults int
}

type AuthWorkflow interface {
	ValidateToken(context.Context, ...socialhub.CallOption) (*TokenValidation, error)
}

type ListeningWorkflow interface {
	SearchUsers(context.Context, string, ...socialhub.CallOption) ([]User, error)
	ListListens(context.Context, ListensRequest, ...socialhub.CallOption) (*ListenPage, error)
	GetPlayingNow(context.Context, string, ...socialhub.CallOption) (*Listen, error)
	GetListenCount(context.Context, string, ...socialhub.CallOption) (int64, error)
	SubmitSingle(context.Context, ListenSubmission, ...socialhub.CallOption) (*SubmissionResult, error)
	SubmitImport(context.Context, []ListenSubmission, ...socialhub.CallOption) (*SubmissionResult, error)
	SubmitPlayingNow(context.Context, PlayingNowSubmission, bool, ...socialhub.CallOption) (*SubmissionResult, error)
	DeleteListen(context.Context, DeleteListenRequest, ...socialhub.CallOption) error
}

type FeedbackWorkflow interface {
	SubmitFeedback(context.Context, FeedbackSubmission, ...socialhub.CallOption) error
	ListFeedback(context.Context, FeedbackListRequest, ...socialhub.CallOption) (socialhub.Page[Feedback], error)
}

type PlaylistWorkflow interface {
	SearchPlaylists(context.Context, PlaylistSearchRequest, ...socialhub.CallOption) (socialhub.Page[Playlist], error)
	ListUserPlaylists(context.Context, string, PlaylistPageRequest, ...socialhub.CallOption) (socialhub.Page[Playlist], error)
	GetPlaylist(context.Context, string, bool, ...socialhub.CallOption) (*Playlist, error)
}

var _ AuthWorkflow = (*Client)(nil)
var _ ListeningWorkflow = (*Client)(nil)
var _ FeedbackWorkflow = (*Client)(nil)
var _ PlaylistWorkflow = (*Client)(nil)
