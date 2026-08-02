package musicbrainz

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) SearchArtists(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Artist], error) {
	query, err := pageQuery(input.Query, input.Limit, input.Cursor)
	if err != nil {
		return socialhub.Page[Artist]{}, err
	}
	var response artistSearchEnvelope
	if err := c.requestJSON(ctx, "search_artists", "/artist", query, &response, options...); err != nil {
		return socialhub.Page[Artist]{}, err
	}
	return pageFromEnvelope("search_artists", response.Artists, response.Count, response.Offset, input.Limit)
}

func (c *Client) GetArtist(ctx context.Context, mbid string, options ...socialhub.CallOption) (*Artist, error) {
	if !validMBID(mbid) {
		return nil, invalidArgument("get_artist", "mbid must be a canonical lowercase UUID")
	}
	query := url.Values{"inc": {"aliases+genres"}}
	var artist Artist
	if err := c.requestJSON(ctx, "get_artist", "/artist/"+mbid, query, &artist, options...); err != nil {
		return nil, err
	}
	return &artist, nil
}

func (c *Client) SearchReleaseGroups(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[ReleaseGroup], error) {
	query, err := pageQuery(input.Query, input.Limit, input.Cursor)
	if err != nil {
		return socialhub.Page[ReleaseGroup]{}, err
	}
	var response releaseGroupSearchEnvelope
	if err := c.requestJSON(ctx, "search_release_groups", "/release-group", query, &response, options...); err != nil {
		return socialhub.Page[ReleaseGroup]{}, err
	}
	return pageFromEnvelope("search_release_groups", response.ReleaseGroups, response.Count, response.Offset, input.Limit)
}

func (c *Client) GetReleaseGroup(ctx context.Context, mbid string, options ...socialhub.CallOption) (*ReleaseGroup, error) {
	if !validMBID(mbid) {
		return nil, invalidArgument("get_release_group", "mbid must be a canonical lowercase UUID")
	}
	query := url.Values{"inc": {"artist-credits+genres+releases"}}
	var releaseGroup ReleaseGroup
	if err := c.requestJSON(ctx, "get_release_group", "/release-group/"+mbid, query, &releaseGroup, options...); err != nil {
		return nil, err
	}
	return &releaseGroup, nil
}

func (c *Client) SearchReleases(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Release], error) {
	query, err := pageQuery(input.Query, input.Limit, input.Cursor)
	if err != nil {
		return socialhub.Page[Release]{}, err
	}
	var response releaseSearchEnvelope
	if err := c.requestJSON(ctx, "search_releases", "/release", query, &response, options...); err != nil {
		return socialhub.Page[Release]{}, err
	}
	return pageFromEnvelope("search_releases", response.Releases, response.Count, response.Offset, input.Limit)
}

func (c *Client) GetRelease(ctx context.Context, mbid string, options ...socialhub.CallOption) (*Release, error) {
	if !validMBID(mbid) {
		return nil, invalidArgument("get_release", "mbid must be a canonical lowercase UUID")
	}
	query := url.Values{"inc": {"artist-credits+labels+recordings+release-groups+media+genres"}}
	var release Release
	if err := c.requestJSON(ctx, "get_release", "/release/"+mbid, query, &release, options...); err != nil {
		return nil, err
	}
	return &release, nil
}

func (c *Client) SearchRecordings(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Recording], error) {
	query, err := pageQuery(input.Query, input.Limit, input.Cursor)
	if err != nil {
		return socialhub.Page[Recording]{}, err
	}
	var response recordingSearchEnvelope
	if err := c.requestJSON(ctx, "search_recordings", "/recording", query, &response, options...); err != nil {
		return socialhub.Page[Recording]{}, err
	}
	return pageFromEnvelope("search_recordings", response.Recordings, response.Count, response.Offset, input.Limit)
}

func (c *Client) GetRecording(ctx context.Context, mbid string, options ...socialhub.CallOption) (*Recording, error) {
	if !validMBID(mbid) {
		return nil, invalidArgument("get_recording", "mbid must be a canonical lowercase UUID")
	}
	query := url.Values{"inc": {"artist-credits+isrcs+genres+releases"}}
	var recording Recording
	if err := c.requestJSON(ctx, "get_recording", "/recording/"+mbid, query, &recording, options...); err != nil {
		return nil, err
	}
	return &recording, nil
}

func (c *Client) SearchWorks(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Work], error) {
	query, err := pageQuery(input.Query, input.Limit, input.Cursor)
	if err != nil {
		return socialhub.Page[Work]{}, err
	}
	var response workSearchEnvelope
	if err := c.requestJSON(ctx, "search_works", "/work", query, &response, options...); err != nil {
		return socialhub.Page[Work]{}, err
	}
	return pageFromEnvelope("search_works", response.Works, response.Count, response.Offset, input.Limit)
}

func (c *Client) GetWork(ctx context.Context, mbid string, options ...socialhub.CallOption) (*Work, error) {
	if !validMBID(mbid) {
		return nil, invalidArgument("get_work", "mbid must be a canonical lowercase UUID")
	}
	query := url.Values{"inc": {"aliases+genres+artist-rels+recording-rels"}}
	var work Work
	if err := c.requestJSON(ctx, "get_work", "/work/"+mbid, query, &work, options...); err != nil {
		return nil, err
	}
	return &work, nil
}

func (c *Client) ListArtistReleaseGroups(ctx context.Context, artistMBID string, input BrowseRequest, options ...socialhub.CallOption) (socialhub.Page[ReleaseGroup], error) {
	query, err := browseQuery(artistMBID, input.Limit, input.Cursor)
	if err != nil {
		return socialhub.Page[ReleaseGroup]{}, err
	}
	query.Set("inc", "artist-credits")
	var response artistReleaseGroupsEnvelope
	if err := c.requestJSON(ctx, "list_artist_release_groups", "/release-group", query, &response, options...); err != nil {
		return socialhub.Page[ReleaseGroup]{}, err
	}
	return pageFromEnvelope("list_artist_release_groups", response.ReleaseGroups, response.Count, response.Offset, input.Limit)
}

func (c *Client) ListArtistRecordings(ctx context.Context, artistMBID string, input BrowseRequest, options ...socialhub.CallOption) (socialhub.Page[Recording], error) {
	query, err := browseQuery(artistMBID, input.Limit, input.Cursor)
	if err != nil {
		return socialhub.Page[Recording]{}, err
	}
	query.Set("inc", "artist-credits+isrcs")
	var response artistRecordingsEnvelope
	if err := c.requestJSON(ctx, "list_artist_recordings", "/recording", query, &response, options...); err != nil {
		return socialhub.Page[Recording]{}, err
	}
	return pageFromEnvelope("list_artist_recordings", response.Recordings, response.Count, response.Offset, input.Limit)
}
