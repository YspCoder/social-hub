package tidal

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

// ExplicitFilter controls whether explicit content may appear in search results.
type ExplicitFilter string

const (
	ExplicitInclude ExplicitFilter = "INCLUDE"
	ExplicitExclude ExplicitFilter = "EXCLUDE"
)

// Sort selects a documented album or track collection order.
type Sort string

const (
	SortCreatedAtAscending  Sort = "createdAt"
	SortCreatedAtDescending Sort = "-createdAt"
	SortTitleAscending      Sort = "title"
	SortTitleDescending     Sort = "-title"
)

// SearchRequest describes a TIDAL catalog search.
type SearchRequest struct {
	Query          string
	ExplicitFilter ExplicitFilter
	CountryCode    string
	Include        []string
}

// ResourceRequest supplies common options for one resource lookup.
type ResourceRequest struct {
	CountryCode string
	Include     []string
}

// ListArtistsRequest filters the artist collection. Exactly one of IDs or
// Handles must be supplied.
type ListArtistsRequest struct {
	IDs         []string
	Handles     []string
	CountryCode string
	Include     []string
}

// ListAlbumsRequest filters and pages the album collection.
type ListAlbumsRequest struct {
	IDs         []string
	BarcodeIDs  []string
	CountryCode string
	Include     []string
	Sort        Sort
	Cursor      string
}

// ListTracksRequest filters and pages the track collection.
type ListTracksRequest struct {
	IDs         []string
	ISRCs       []string
	CountryCode string
	Include     []string
	Sort        Sort
	Cursor      string
}

// Links is the JSON:API document or relationship link set.
type Links struct {
	Self string     `json:"self"`
	Next string     `json:"next,omitempty"`
	Meta *LinksMeta `json:"meta,omitempty"`
}

// LinksMeta carries TIDAL's non-standard copy of the next cursor.
type LinksMeta struct {
	NextCursor string `json:"nextCursor"`
}

// Relationship preserves requested JSON:API linkage. A nil Data means the
// data member was absent; a non-nil RawMessage may contain an object, array, or null.
type Relationship struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Links Links           `json:"links"`
	Meta  json.RawMessage `json:"meta,omitempty"`
	Raw   json.RawMessage `json:"-"`
}

func (relationship *Relationship) UnmarshalJSON(data []byte) error {
	type wire Relationship
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*relationship = Relationship(decoded)
	relationship.Raw = cloneRaw(data)
	return nil
}

// IncludedResource preserves an included resource without guessing its type.
type IncludedResource struct {
	Type          string                  `json:"type"`
	ID            string                  `json:"id"`
	Attributes    json.RawMessage         `json:"attributes,omitempty"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
	Raw           json.RawMessage         `json:"-"`
}

func (resource *IncludedResource) UnmarshalJSON(data []byte) error {
	type wire IncludedResource
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*resource = IncludedResource(decoded)
	resource.Raw = cloneRaw(data)
	return nil
}

// ResponseMeta preserves dynamic response and lifecycle headers. TIDAL does
// not publish a fixed numeric quota or a guaranteed rate-limit header set.
type ResponseMeta struct {
	StatusCode         int
	RetryAfter         string
	RateLimitLimit     string
	RateLimitRemaining string
	RateLimitReset     string
	CloudFrontID       string
	RequestID          string
	ETag               string
	CacheControl       string
	Deprecation        string
	Sunset             string
	Warning            string
	LastModified       string
}

// Page is one JSON:API collection response.
type Page[T any] struct {
	Items      []T
	Included   []IncludedResource
	Links      Links
	NextCursor string
	Meta       json.RawMessage
	Response   ResponseMeta
	Raw        json.RawMessage
}

// Document is one normalized JSON:API resource response.
type Document[T any] struct {
	Item     T
	Included []IncludedResource
	Links    Links
	Meta     json.RawMessage
	Response ResponseMeta
	Raw      json.RawMessage
}

// ExternalLink is a provider link and its forward-compatible link type.
type ExternalLink struct {
	Href string           `json:"href"`
	Meta ExternalLinkMeta `json:"meta"`
}

type ExternalLinkMeta struct {
	Type string `json:"type"`
}

type Copyright struct {
	Text string `json:"text"`
}

// ArtistAttributes is the stable catalog subset. Pointer scalar fields retain
// the distinction between a redacted field and its zero value.
type ArtistAttributes struct {
	Name                    *string        `json:"name,omitempty"`
	Popularity              *float64       `json:"popularity,omitempty"`
	Handle                  *string        `json:"handle,omitempty"`
	ContributionsEnabled    *bool          `json:"contributionsEnabled,omitempty"`
	ContributionsSalesPitch *string        `json:"contributionsSalesPitch,omitempty"`
	ExternalLinks           []ExternalLink `json:"externalLinks,omitempty"`
	OwnerType               *string        `json:"ownerType,omitempty"`
	Spotlighted             *bool          `json:"spotlighted,omitempty"`
}

// AlbumAttributes is the stable catalog subset. String enums remain open so
// newly introduced provider values decode without failure.
type AlbumAttributes struct {
	AccessType      *string        `json:"accessType,omitempty"`
	AI              *bool          `json:"ai,omitempty"`
	AlbumType       *string        `json:"albumType,omitempty"`
	BarcodeID       *string        `json:"barcodeId,omitempty"`
	Copyright       *Copyright     `json:"copyright,omitempty"`
	CreatedAt       *string        `json:"createdAt,omitempty"`
	Duration        *string        `json:"duration,omitempty"`
	Explicit        *bool          `json:"explicit,omitempty"`
	ExternalLinks   []ExternalLink `json:"externalLinks,omitempty"`
	MediaTags       []string       `json:"mediaTags,omitempty"`
	NumberOfItems   *int           `json:"numberOfItems,omitempty"`
	NumberOfVolumes *int           `json:"numberOfVolumes,omitempty"`
	Popularity      *float64       `json:"popularity,omitempty"`
	ReleaseDate     *string        `json:"releaseDate,omitempty"`
	Title           *string        `json:"title,omitempty"`
	Version         *string        `json:"version,omitempty"`
}

// TrackAttributes is the stable catalog subset and deliberately omits
// playback, download, source-file, and usage-rule payloads.
type TrackAttributes struct {
	AccessType    *string        `json:"accessType,omitempty"`
	AI            *bool          `json:"ai,omitempty"`
	BPM           *float64       `json:"bpm,omitempty"`
	Copyright     *Copyright     `json:"copyright,omitempty"`
	CreatedAt     *string        `json:"createdAt,omitempty"`
	Duration      *string        `json:"duration,omitempty"`
	Explicit      *bool          `json:"explicit,omitempty"`
	ExternalLinks []ExternalLink `json:"externalLinks,omitempty"`
	ISRC          *string        `json:"isrc,omitempty"`
	Key           *string        `json:"key,omitempty"`
	KeyScale      *string        `json:"keyScale,omitempty"`
	MediaTags     []string       `json:"mediaTags,omitempty"`
	Popularity    *float64       `json:"popularity,omitempty"`
	Spotlighted   *bool          `json:"spotlighted,omitempty"`
	Title         *string        `json:"title,omitempty"`
	ToneTags      []string       `json:"toneTags,omitempty"`
	Version       *string        `json:"version,omitempty"`
}

type SearchResultAttributes struct {
	Query      *string `json:"query,omitempty"`
	TrackingID *string `json:"trackingId,omitempty"`
	DidYouMean *string `json:"didYouMean,omitempty"`
}

type Artist struct {
	Type          string                  `json:"type"`
	ID            string                  `json:"id"`
	Attributes    *ArtistAttributes       `json:"attributes,omitempty"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
	Raw           json.RawMessage         `json:"-"`
}

func (resource *Artist) UnmarshalJSON(data []byte) error {
	type wire Artist
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*resource = Artist(decoded)
	resource.Raw = cloneRaw(data)
	return nil
}

type Album struct {
	Type          string                  `json:"type"`
	ID            string                  `json:"id"`
	Attributes    *AlbumAttributes        `json:"attributes,omitempty"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
	Raw           json.RawMessage         `json:"-"`
}

func (resource *Album) UnmarshalJSON(data []byte) error {
	type wire Album
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*resource = Album(decoded)
	resource.Raw = cloneRaw(data)
	return nil
}

type Track struct {
	Type          string                  `json:"type"`
	ID            string                  `json:"id"`
	Attributes    *TrackAttributes        `json:"attributes,omitempty"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
	Raw           json.RawMessage         `json:"-"`
}

func (resource *Track) UnmarshalJSON(data []byte) error {
	type wire Track
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*resource = Track(decoded)
	resource.Raw = cloneRaw(data)
	return nil
}

type SearchResult struct {
	Type          string                  `json:"type"`
	ID            string                  `json:"id"`
	Attributes    *SearchResultAttributes `json:"attributes,omitempty"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
	Raw           json.RawMessage         `json:"-"`
}

func (resource *SearchResult) UnmarshalJSON(data []byte) error {
	type wire SearchResult
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*resource = SearchResult(decoded)
	resource.Raw = cloneRaw(data)
	return nil
}

// CatalogWorkflow is the complete minimal TIDAL catalog read surface.
type CatalogWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (*Page[SearchResult], error)
	ListArtists(context.Context, ListArtistsRequest, ...socialhub.CallOption) (*Page[Artist], error)
	GetArtist(context.Context, string, ResourceRequest, ...socialhub.CallOption) (*Document[Artist], error)
	ListAlbums(context.Context, ListAlbumsRequest, ...socialhub.CallOption) (*Page[Album], error)
	GetAlbum(context.Context, string, ResourceRequest, ...socialhub.CallOption) (*Document[Album], error)
	ListTracks(context.Context, ListTracksRequest, ...socialhub.CallOption) (*Page[Track], error)
	GetTrack(context.Context, string, ResourceRequest, ...socialhub.CallOption) (*Document[Track], error)
}

func cloneRaw(data []byte) json.RawMessage { return append(json.RawMessage(nil), data...) }

var _ CatalogWorkflow = (*Client)(nil)
