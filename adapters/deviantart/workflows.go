package deviantart

import (
	"context"

	"social-hub/pkg/socialhub"
)

type UserWorkflow interface {
	WhoAmI(context.Context, ...socialhub.CallOption) (*User, error)
	Profile(context.Context, string, ...socialhub.CallOption) (*Profile, error)
}

type DeviationWorkflow interface {
	GetDeviation(context.Context, string, ...socialhub.CallOption) (*Deviation, error)
	ListProfilePosts(context.Context, ProfilePostsRequest, ...socialhub.CallOption) (*ProfilePostPage, error)
}

type GalleryWorkflow interface {
	ListGallery(context.Context, GalleryPageRequest, ...socialhub.CallOption) (*DeviationPage, error)
}

type StatusWorkflow interface {
	PostStatus(context.Context, StatusPostRequest, ...socialhub.CallOption) (*StatusPublishResponse, error)
}

type CommentWorkflow interface {
	ListDeviationComments(context.Context, DeviationCommentsRequest, ...socialhub.CallOption) (*CommentPage, error)
	PostDeviationComment(context.Context, DeviationCommentRequest, ...socialhub.CallOption) (*Comment, error)
}

type CollectionWorkflow interface {
	Favourite(context.Context, FavouriteRequest, ...socialhub.CallOption) (*FavouriteResponse, error)
	Unfavourite(context.Context, FavouriteRequest, ...socialhub.CallOption) (*FavouriteResponse, error)
}

var _ UserWorkflow = (*Client)(nil)
var _ DeviationWorkflow = (*Client)(nil)
var _ GalleryWorkflow = (*Client)(nil)
var _ StatusWorkflow = (*Client)(nil)
var _ CommentWorkflow = (*Client)(nil)
var _ CollectionWorkflow = (*Client)(nil)
