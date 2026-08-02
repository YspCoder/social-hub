package mixcloud

import (
	"context"
	"io"

	"social-hub/pkg/socialhub"
)

// UserWorkflow exposes authorized and public Mixcloud profiles.
type UserWorkflow interface {
	CurrentUser(context.Context, ...socialhub.CallOption) (*User, error)
	GetMixcloudUser(context.Context, string, ...socialhub.CallOption) (*User, error)
}

// CloudcastWorkflow exposes Mixcloud's audio-show resources and comments.
type CloudcastWorkflow interface {
	GetCloudcast(context.Context, string, ...socialhub.CallOption) (*Cloudcast, error)
	ListUserCloudcasts(context.Context, string, PageRequest, ...socialhub.CallOption) (*CloudcastPage, error)
	ListCloudcastComments(context.Context, string, PageRequest, ...socialhub.CallOption) (*CommentPage, error)
}

// DiscoveryWorkflow searches the three resource types documented by Mixcloud.
type DiscoveryWorkflow interface {
	SearchCloudcasts(context.Context, SearchRequest, ...socialhub.CallOption) (*CloudcastPage, error)
	SearchUsers(context.Context, SearchRequest, ...socialhub.CallOption) (*UserPage, error)
	SearchTags(context.Context, SearchRequest, ...socialhub.CallOption) (*TagPage, error)
}

// UploadWorkflow owns Mixcloud's single-request MP3 publication lifecycle.
type UploadWorkflow interface {
	Upload(context.Context, UploadRequest, io.Reader, io.Reader, ...socialhub.CallOption) (*ActionResponse, error)
	Edit(context.Context, string, EditRequest, io.Reader, ...socialhub.CallOption) (*ActionResponse, error)
}

// LibraryWorkflow exposes Mixcloud's platform-specific collection actions.
type LibraryWorkflow interface {
	Favourite(context.Context, string, ...socialhub.CallOption) (*ActionResponse, error)
	Unfavourite(context.Context, string, ...socialhub.CallOption) (*ActionResponse, error)
	Repost(context.Context, string, ...socialhub.CallOption) (*ActionResponse, error)
	Unrepost(context.Context, string, ...socialhub.CallOption) (*ActionResponse, error)
	ListenLater(context.Context, string, ...socialhub.CallOption) (*ActionResponse, error)
	RemoveListenLater(context.Context, string, ...socialhub.CallOption) (*ActionResponse, error)
	Follow(context.Context, string, ...socialhub.CallOption) (*ActionResponse, error)
	Unfollow(context.Context, string, ...socialhub.CallOption) (*ActionResponse, error)
}

var _ UserWorkflow = (*Client)(nil)
var _ CloudcastWorkflow = (*Client)(nil)
var _ DiscoveryWorkflow = (*Client)(nil)
var _ UploadWorkflow = (*Client)(nil)
var _ LibraryWorkflow = (*Client)(nil)
