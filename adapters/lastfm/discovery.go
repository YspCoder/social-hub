package lastfm

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetTrack(ctx context.Context, input TrackInfoRequest, options ...socialhub.CallOption) (*Track, error) {
	if !validInfoIdentity(input.MBID, input.Artist, input.Track) || (input.Username != "" && !validText(input.Username, 255)) {
		return nil, invalidArgument("track.getInfo", "mbid or artist and track, plus a valid username when set, are required")
	}
	values := url.Values{}
	setIfPresent(values, "mbid", input.MBID)
	setIfPresent(values, "artist", input.Artist)
	setIfPresent(values, "track", input.Track)
	setIfPresent(values, "username", firstNonEmpty(input.Username, c.username))
	setAutocorrect(values, input.Autocorrect)
	var response struct {
		Track apiTrack `json:"track"`
	}
	if err := c.get(ctx, "track.getInfo", values, false, &response, options...); err != nil {
		return nil, err
	}
	track := mapTrack(response.Track, time.Millisecond, true)
	return &track, nil
}

func (c *Client) SearchTracks(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Track], error) {
	page, err := validateSearch(input)
	if err != nil {
		return socialhub.Page[Track]{}, err
	}
	values := searchValues("track", input, page)
	setIfPresent(values, "artist", input.Artist)
	var response struct {
		Results struct {
			Total   flexibleInt64 `json:"opensearch:totalResults"`
			PerPage flexibleInt64 `json:"opensearch:itemsPerPage"`
			Matches struct {
				Track []apiTrack `json:"track"`
			} `json:"trackmatches"`
		} `json:"results"`
	}
	if err := c.get(ctx, "track.search", values, false, &response, options...); err != nil {
		return socialhub.Page[Track]{}, err
	}
	items := make([]Track, 0, len(response.Results.Matches.Track))
	for _, item := range response.Results.Matches.Track {
		items = append(items, mapTrack(item, time.Millisecond, false))
	}
	return makePage(items, effectivePage(page), searchTotalPages(int64(response.Results.Total), int64(response.Results.PerPage))), nil
}

func (c *Client) GetArtist(ctx context.Context, input ArtistInfoRequest, options ...socialhub.CallOption) (*Artist, error) {
	if !validSingleIdentity(input.MBID, input.Artist) || (input.Language != "" && !validLanguage(input.Language)) ||
		(input.Username != "" && !validText(input.Username, 255)) {
		return nil, invalidArgument("artist.getInfo", "mbid or artist, ISO 639-1 language, and username are invalid")
	}
	values := url.Values{}
	setIfPresent(values, "mbid", input.MBID)
	setIfPresent(values, "artist", input.Artist)
	setIfPresent(values, "lang", input.Language)
	setIfPresent(values, "username", firstNonEmpty(input.Username, c.username))
	setAutocorrect(values, input.Autocorrect)
	var response struct {
		Artist apiArtist `json:"artist"`
	}
	if err := c.get(ctx, "artist.getInfo", values, false, &response, options...); err != nil {
		return nil, err
	}
	artist := mapArtist(response.Artist)
	return &artist, nil
}

func (c *Client) SearchArtists(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Artist], error) {
	page, err := validateSearch(input)
	if err != nil {
		return socialhub.Page[Artist]{}, err
	}
	var response struct {
		Results struct {
			Total   flexibleInt64 `json:"opensearch:totalResults"`
			PerPage flexibleInt64 `json:"opensearch:itemsPerPage"`
			Matches struct {
				Artist []apiArtist `json:"artist"`
			} `json:"artistmatches"`
		} `json:"results"`
	}
	if err := c.get(ctx, "artist.search", searchValues("artist", input, page), false, &response, options...); err != nil {
		return socialhub.Page[Artist]{}, err
	}
	items := make([]Artist, 0, len(response.Results.Matches.Artist))
	for _, item := range response.Results.Matches.Artist {
		items = append(items, mapArtist(item))
	}
	return makePage(items, effectivePage(page), searchTotalPages(int64(response.Results.Total), int64(response.Results.PerPage))), nil
}

func (c *Client) GetAlbum(ctx context.Context, input AlbumInfoRequest, options ...socialhub.CallOption) (*Album, error) {
	if !validInfoIdentity(input.MBID, input.Artist, input.Album) || (input.Language != "" && !validLanguage(input.Language)) ||
		(input.Username != "" && !validText(input.Username, 255)) {
		return nil, invalidArgument("album.getInfo", "mbid or artist and album, ISO 639-1 language, and username are invalid")
	}
	values := url.Values{}
	setIfPresent(values, "mbid", input.MBID)
	setIfPresent(values, "artist", input.Artist)
	setIfPresent(values, "album", input.Album)
	setIfPresent(values, "lang", input.Language)
	setIfPresent(values, "username", firstNonEmpty(input.Username, c.username))
	setAutocorrect(values, input.Autocorrect)
	var response struct {
		Album apiAlbum `json:"album"`
	}
	if err := c.get(ctx, "album.getInfo", values, false, &response, options...); err != nil {
		return nil, err
	}
	album := mapAlbum(response.Album, true)
	return &album, nil
}

func (c *Client) SearchAlbums(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Album], error) {
	page, err := validateSearch(input)
	if err != nil {
		return socialhub.Page[Album]{}, err
	}
	var response struct {
		Results struct {
			Total   flexibleInt64 `json:"opensearch:totalResults"`
			PerPage flexibleInt64 `json:"opensearch:itemsPerPage"`
			Matches struct {
				Album []apiAlbum `json:"album"`
			} `json:"albummatches"`
		} `json:"results"`
	}
	if err := c.get(ctx, "album.search", searchValues("album", input, page), false, &response, options...); err != nil {
		return socialhub.Page[Album]{}, err
	}
	items := make([]Album, 0, len(response.Results.Matches.Album))
	for _, item := range response.Results.Matches.Album {
		items = append(items, mapAlbum(item, false))
	}
	return makePage(items, effectivePage(page), searchTotalPages(int64(response.Results.Total), int64(response.Results.PerPage))), nil
}

func validateSearch(input SearchRequest) (int, error) {
	if !validText(input.Query, maxTextLength) || (input.Artist != "" && !validText(input.Artist, maxTextLength)) {
		return 0, invalidArgument("search", "query or artist filter is invalid")
	}
	return validatePage(input.Cursor, input.MaxResults)
}

func searchValues(parameter string, input SearchRequest, page int) url.Values {
	values := url.Values{parameter: {input.Query}}
	setPage(values, page, input.MaxResults)
	return values
}

func validSingleIdentity(mbid, name string) bool {
	return (mbid != "" && validText(mbid, 128)) || (name != "" && validText(name, maxTextLength))
}

func validInfoIdentity(mbid, first, second string) bool {
	return (mbid != "" && validText(mbid, 128)) || (validText(first, maxTextLength) && validText(second, maxTextLength))
}

func validLanguage(value string) bool {
	return len(value) == 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		((value[1] >= 'A' && value[1] <= 'Z') || (value[1] >= 'a' && value[1] <= 'z'))
}

func setIfPresent(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func setAutocorrect(values url.Values, enabled bool) {
	if enabled {
		values.Set("autocorrect", "1")
	}
}

func setPage(values url.Values, page, maxResults int) {
	if page > 0 {
		values.Set("page", strconv.Itoa(page))
	}
	if maxResults > 0 {
		values.Set("limit", strconv.Itoa(maxResults))
	}
}

func effectivePage(page int) int {
	if page == 0 {
		return 1
	}
	return page
}
