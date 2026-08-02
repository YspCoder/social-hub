package lastfm

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, username string, options ...socialhub.CallOption) (*User, error) {
	username, err := c.resolveUsername(username, "user.getInfo")
	if err != nil {
		return nil, err
	}
	var response struct {
		User struct {
			Name          string        `json:"name"`
			RealName      string        `json:"realname"`
			URL           string        `json:"url"`
			Country       string        `json:"country"`
			Age           flexibleInt64 `json:"age"`
			Gender        string        `json:"gender"`
			Subscriber    flexibleBool  `json:"subscriber"`
			PlayCount     flexibleInt64 `json:"playcount"`
			ArtistCount   flexibleInt64 `json:"artist_count"`
			AlbumCount    flexibleInt64 `json:"album_count"`
			TrackCount    flexibleInt64 `json:"track_count"`
			PlaylistCount flexibleInt64 `json:"playlists"`
			Registered    struct {
				Unix flexibleInt64 `json:"unixtime"`
			} `json:"registered"`
			Image []apiImage `json:"image"`
		} `json:"user"`
	}
	if err := c.get(ctx, "user.getInfo", url.Values{"user": {username}}, false, &response, options...); err != nil {
		return nil, err
	}
	user := User{
		Name: response.User.Name, RealName: response.User.RealName, URL: response.User.URL, Country: response.User.Country,
		Age: int(response.User.Age), Gender: response.User.Gender, Subscriber: bool(response.User.Subscriber),
		PlayCount: int64(response.User.PlayCount), ArtistCount: int64(response.User.ArtistCount),
		AlbumCount: int64(response.User.AlbumCount), TrackCount: int64(response.User.TrackCount),
		PlaylistCount: int64(response.User.PlaylistCount), RegisteredAt: unixTime(response.User.Registered.Unix),
		Images: mapImages(response.User.Image),
	}
	return &user, nil
}

func (c *Client) RecentTracks(ctx context.Context, input RecentTracksRequest, options ...socialhub.CallOption) (socialhub.Page[Track], error) {
	username, err := c.resolveUsername(input.Username, "user.getRecentTracks")
	if err != nil {
		return socialhub.Page[Track]{}, err
	}
	page, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil || (!input.From.IsZero() && !input.To.IsZero() && input.From.After(input.To)) {
		if err != nil {
			return socialhub.Page[Track]{}, err
		}
		return socialhub.Page[Track]{}, invalidArgument("user.getRecentTracks", "from must not be after to")
	}
	values := url.Values{"user": {username}}
	setPage(values, page, input.MaxResults)
	if !input.From.IsZero() {
		values.Set("from", strconv.FormatInt(input.From.Unix(), 10))
	}
	if !input.To.IsZero() {
		values.Set("to", strconv.FormatInt(input.To.Unix(), 10))
	}
	if input.Extended {
		values.Set("extended", "1")
	}
	var response struct {
		Recent struct {
			Track []apiTrack  `json:"track"`
			Attr  apiPageAttr `json:"@attr"`
		} `json:"recenttracks"`
	}
	if err := c.get(ctx, "user.getRecentTracks", values, false, &response, options...); err != nil {
		return socialhub.Page[Track]{}, err
	}
	items := make([]Track, 0, len(response.Recent.Track))
	for _, item := range response.Recent.Track {
		items = append(items, mapTrack(item, time.Millisecond, true))
	}
	return makePage(items, responsePage(response.Recent.Attr, page), int(response.Recent.Attr.TotalPages)), nil
}

func (c *Client) TopTracks(ctx context.Context, input TopTracksRequest, options ...socialhub.CallOption) (socialhub.Page[Track], error) {
	username, err := c.resolveUsername(input.Username, "user.getTopTracks")
	if err != nil {
		return socialhub.Page[Track]{}, err
	}
	page, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil || !validPeriod(input.Period) {
		if err != nil {
			return socialhub.Page[Track]{}, err
		}
		return socialhub.Page[Track]{}, invalidArgument("user.getTopTracks", "period is invalid")
	}
	values := url.Values{"user": {username}}
	setPage(values, page, input.MaxResults)
	setIfPresent(values, "period", string(input.Period))
	var response struct {
		Top struct {
			Track []apiTrack  `json:"track"`
			Attr  apiPageAttr `json:"@attr"`
		} `json:"toptracks"`
	}
	if err := c.get(ctx, "user.getTopTracks", values, false, &response, options...); err != nil {
		return socialhub.Page[Track]{}, err
	}
	items := make([]Track, 0, len(response.Top.Track))
	for _, item := range response.Top.Track {
		items = append(items, mapTrack(item, time.Millisecond, false))
	}
	return makePage(items, responsePage(response.Top.Attr, page), int(response.Top.Attr.TotalPages)), nil
}

func (c *Client) LovedTracks(ctx context.Context, input UserTracksRequest, options ...socialhub.CallOption) (socialhub.Page[Track], error) {
	username, err := c.resolveUsername(input.Username, "user.getLovedTracks")
	if err != nil {
		return socialhub.Page[Track]{}, err
	}
	page, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[Track]{}, err
	}
	values := url.Values{"user": {username}}
	setPage(values, page, input.MaxResults)
	var response struct {
		Loved struct {
			Track []apiTrack  `json:"track"`
			Attr  apiPageAttr `json:"@attr"`
		} `json:"lovedtracks"`
	}
	if err := c.get(ctx, "user.getLovedTracks", values, false, &response, options...); err != nil {
		return socialhub.Page[Track]{}, err
	}
	items := make([]Track, 0, len(response.Loved.Track))
	for _, item := range response.Loved.Track {
		track := mapTrack(item, time.Millisecond, false)
		track.LovedAt, track.PlayedAt = track.PlayedAt, nil
		items = append(items, track)
	}
	return makePage(items, responsePage(response.Loved.Attr, page), int(response.Loved.Attr.TotalPages)), nil
}

func (c *Client) resolveUsername(value, operation string) (string, error) {
	value = firstNonEmpty(value, c.username)
	if !validText(value, 255) {
		return "", invalidArgument(operation, "username is required")
	}
	return value, nil
}

func validPeriod(value TopTracksPeriod) bool {
	switch value {
	case "", PeriodOverall, Period7Day, Period1Month, Period3Month, Period6Month, Period12Month:
		return true
	default:
		return false
	}
}

func responsePage(attr apiPageAttr, requested int) int {
	if attr.Page > 0 {
		return int(attr.Page)
	}
	return effectivePage(requested)
}
