package lastfm

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

type apiCorrection struct {
	Value     string       `json:"#text"`
	Corrected flexibleBool `json:"corrected"`
}

func (c *Client) UpdateNowPlaying(ctx context.Context, input NowPlayingRequest, options ...socialhub.CallOption) (*NowPlayingResult, error) {
	if !validTrackRef(input.Artist, input.Track) || input.TrackNumber < 0 || input.Duration < 0 ||
		(input.Album != "" && !validText(input.Album, maxTextLength)) ||
		(input.AlbumArtist != "" && !validText(input.AlbumArtist, maxTextLength)) ||
		(input.MBID != "" && !validText(input.MBID, 128)) {
		return nil, invalidArgument("track.updateNowPlaying", "track metadata is invalid")
	}
	values := url.Values{"artist": {input.Artist}, "track": {input.Track}}
	setTrackMetadata(values, input.Album, input.AlbumArtist, input.TrackNumber, input.MBID, input.Duration)
	var response struct {
		NowPlaying struct {
			Track       apiCorrection `json:"track"`
			Artist      apiCorrection `json:"artist"`
			Album       apiCorrection `json:"album"`
			AlbumArtist apiCorrection `json:"albumArtist"`
		} `json:"nowplaying"`
	}
	if err := c.post(ctx, "track.updateNowPlaying", values, &response, options...); err != nil {
		return nil, err
	}
	return &NowPlayingResult{
		Track: mapCorrection(response.NowPlaying.Track), Artist: mapCorrection(response.NowPlaying.Artist),
		Album: mapCorrection(response.NowPlaying.Album), AlbumArtist: mapCorrection(response.NowPlaying.AlbumArtist),
	}, nil
}

func (c *Client) Scrobble(ctx context.Context, input []Scrobble, options ...socialhub.CallOption) (*ScrobbleResult, error) {
	if len(input) == 0 || len(input) > 50 {
		return nil, invalidArgument("track.scrobble", "between 1 and 50 scrobbles are required")
	}
	values := url.Values{}
	for index, item := range input {
		if !validTrackRef(item.Artist, item.Track) || item.StartedAt.IsZero() || item.StartedAt.Unix() <= 0 ||
			item.TrackNumber < 0 || item.Duration < 0 ||
			(item.Album != "" && !validText(item.Album, maxTextLength)) ||
			(item.AlbumArtist != "" && !validText(item.AlbumArtist, maxTextLength)) ||
			(item.MBID != "" && !validText(item.MBID, 128)) {
			return nil, invalidArgument("track.scrobble", "one or more scrobbles contain invalid metadata")
		}
		suffix := "[" + strconv.Itoa(index) + "]"
		values.Set("artist"+suffix, item.Artist)
		values.Set("track"+suffix, item.Track)
		values.Set("timestamp"+suffix, strconv.FormatInt(item.StartedAt.Unix(), 10))
		setIndexedMetadata(values, suffix, item)
	}
	var response struct {
		Scrobbles struct {
			Attr struct {
				Accepted flexibleInt64 `json:"accepted"`
				Ignored  flexibleInt64 `json:"ignored"`
			} `json:"@attr"`
			Scrobble []struct {
				Track       apiCorrection `json:"track"`
				Artist      apiCorrection `json:"artist"`
				Album       apiCorrection `json:"album"`
				AlbumArtist apiCorrection `json:"albumArtist"`
				Timestamp   flexibleInt64 `json:"timestamp"`
				Ignored     struct {
					Code    flexibleInt64 `json:"code"`
					Message string        `json:"#text"`
				} `json:"ignoredMessage"`
			} `json:"scrobble"`
		} `json:"scrobbles"`
	}
	if err := c.post(ctx, "track.scrobble", values, &response, options...); err != nil {
		return nil, err
	}
	result := &ScrobbleResult{Accepted: int(response.Scrobbles.Attr.Accepted), Ignored: int(response.Scrobbles.Attr.Ignored)}
	result.Items = make([]ScrobbleItemResult, 0, len(response.Scrobbles.Scrobble))
	for _, item := range response.Scrobbles.Scrobble {
		result.Items = append(result.Items, ScrobbleItemResult{
			Track: mapCorrection(item.Track), Artist: mapCorrection(item.Artist), Album: mapCorrection(item.Album),
			AlbumArtist: mapCorrection(item.AlbumArtist), Timestamp: time.Unix(int64(item.Timestamp), 0).UTC(),
			IgnoredCode: int(item.Ignored.Code), IgnoredMessage: item.Ignored.Message,
		})
	}
	return result, nil
}

func validTrackRef(artist, track string) bool {
	return validText(artist, maxTextLength) && validText(track, maxTextLength)
}

func setTrackMetadata(values url.Values, album, albumArtist string, trackNumber int, mbid string, duration time.Duration) {
	setIfPresent(values, "album", album)
	setIfPresent(values, "albumArtist", albumArtist)
	setIfPresent(values, "mbid", mbid)
	if trackNumber > 0 {
		values.Set("trackNumber", strconv.Itoa(trackNumber))
	}
	if duration > 0 {
		values.Set("duration", strconv.FormatInt(int64(duration/time.Second), 10))
	}
}

func setIndexedMetadata(values url.Values, suffix string, item Scrobble) {
	setIfPresent(values, "album"+suffix, item.Album)
	setIfPresent(values, "albumArtist"+suffix, item.AlbumArtist)
	setIfPresent(values, "mbid"+suffix, item.MBID)
	if item.TrackNumber > 0 {
		values.Set("trackNumber"+suffix, strconv.Itoa(item.TrackNumber))
	}
	if item.Duration > 0 {
		values.Set("duration"+suffix, strconv.FormatInt(int64(item.Duration/time.Second), 10))
	}
	if item.ChosenByUser != nil {
		if *item.ChosenByUser {
			values.Set("chosenByUser"+suffix, "1")
		} else {
			values.Set("chosenByUser"+suffix, "0")
		}
	}
}

func mapCorrection(input apiCorrection) Correction {
	return Correction{Value: input.Value, Corrected: bool(input.Corrected)}
}
