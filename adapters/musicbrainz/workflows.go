package musicbrainz

import (
	"context"

	"social-hub/pkg/socialhub"
)

type SearchRequest struct {
	Query  string
	Limit  int
	Cursor string
}

type BrowseRequest struct {
	Limit  int
	Cursor string
}

// CatalogWorkflow exposes the primary MusicBrainz music metadata entities.
type CatalogWorkflow interface {
	SearchArtists(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Artist], error)
	GetArtist(context.Context, string, ...socialhub.CallOption) (*Artist, error)
	SearchReleaseGroups(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[ReleaseGroup], error)
	GetReleaseGroup(context.Context, string, ...socialhub.CallOption) (*ReleaseGroup, error)
	SearchReleases(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Release], error)
	GetRelease(context.Context, string, ...socialhub.CallOption) (*Release, error)
	SearchRecordings(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Recording], error)
	GetRecording(context.Context, string, ...socialhub.CallOption) (*Recording, error)
	SearchWorks(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Work], error)
	GetWork(context.Context, string, ...socialhub.CallOption) (*Work, error)
	ListArtistReleaseGroups(context.Context, string, BrowseRequest, ...socialhub.CallOption) (socialhub.Page[ReleaseGroup], error)
	ListArtistRecordings(context.Context, string, BrowseRequest, ...socialhub.CallOption) (socialhub.Page[Recording], error)
}

var _ CatalogWorkflow = (*Client)(nil)
