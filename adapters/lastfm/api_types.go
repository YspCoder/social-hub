package lastfm

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type flexibleInt64 int64

func (value *flexibleInt64) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) || bytes.Equal(data, []byte(`""`)) {
		*value = 0
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		parsed, err := strconv.ParseInt(number.String(), 10, 64)
		if err == nil {
			*value = flexibleInt64(parsed)
			return nil
		}
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*value = flexibleInt64(parsed)
	return nil
}

type flexibleBool bool

func (value *flexibleBool) UnmarshalJSON(data []byte) error {
	var boolean bool
	if err := json.Unmarshal(data, &boolean); err == nil {
		*value = flexibleBool(boolean)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		if parsed, err := strconv.ParseBool(strings.ToLower(text)); err == nil {
			*value = flexibleBool(parsed)
			return nil
		}
	}
	var number flexibleInt64
	if err := json.Unmarshal(data, &number); err == nil {
		*value = number != 0
		return nil
	}
	return &json.UnmarshalTypeError{Value: string(data), Type: nil}
}

type apiStreamable struct {
	Value     bool
	FullTrack bool
}

func (value *apiStreamable) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '{' {
		var payload struct {
			Value     flexibleBool `json:"#text"`
			FullTrack flexibleBool `json:"fulltrack"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return err
		}
		value.Value, value.FullTrack = bool(payload.Value), bool(payload.FullTrack)
		return nil
	}
	var boolean flexibleBool
	if err := json.Unmarshal(data, &boolean); err != nil {
		return err
	}
	value.Value = bool(boolean)
	return nil
}

type apiImage struct {
	Size string `json:"size"`
	URL  string `json:"#text"`
}

type apiTag struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type apiTextBlock struct {
	Published string `json:"published"`
	Summary   string `json:"summary"`
	Content   string `json:"content"`
}

type apiArtist struct {
	Name       string       `json:"name"`
	Text       string       `json:"#text"`
	MBID       string       `json:"mbid"`
	URL        string       `json:"url"`
	Streamable flexibleBool `json:"streamable"`
	OnTour     flexibleBool `json:"ontour"`
	Image      []apiImage   `json:"image"`
	Stats      struct {
		Listeners     flexibleInt64 `json:"listeners"`
		PlayCount     flexibleInt64 `json:"playcount"`
		UserPlayCount flexibleInt64 `json:"userplaycount"`
	} `json:"stats"`
	Similar struct {
		Artist []apiArtist `json:"artist"`
	} `json:"similar"`
	Tags struct {
		Tag []apiTag `json:"tag"`
	} `json:"tags"`
	Bio apiTextBlock `json:"bio"`
}

func (artist *apiArtist) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &artist.Name)
	}
	type alias apiArtist
	return json.Unmarshal(data, (*alias)(artist))
}

type apiAlbum struct {
	Name          string        `json:"name"`
	Text          string        `json:"#text"`
	Title         string        `json:"title"`
	Artist        apiArtist     `json:"artist"`
	MBID          string        `json:"mbid"`
	URL           string        `json:"url"`
	Image         []apiImage    `json:"image"`
	Listeners     flexibleInt64 `json:"listeners"`
	PlayCount     flexibleInt64 `json:"playcount"`
	UserPlayCount flexibleInt64 `json:"userplaycount"`
	Tracks        struct {
		Track []apiTrack `json:"track"`
	} `json:"tracks"`
	Tags struct {
		Tag []apiTag `json:"tag"`
	} `json:"toptags"`
	Wiki apiTextBlock `json:"wiki"`
}

func (album *apiAlbum) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &album.Name)
	}
	type alias apiAlbum
	return json.Unmarshal(data, (*alias)(album))
}

type apiTrack struct {
	Name          string        `json:"name"`
	Text          string        `json:"#text"`
	Artist        apiArtist     `json:"artist"`
	Album         *apiAlbum     `json:"album"`
	MBID          string        `json:"mbid"`
	URL           string        `json:"url"`
	Duration      flexibleInt64 `json:"duration"`
	Streamable    apiStreamable `json:"streamable"`
	Listeners     flexibleInt64 `json:"listeners"`
	PlayCount     flexibleInt64 `json:"playcount"`
	UserPlayCount flexibleInt64 `json:"userplaycount"`
	UserLoved     flexibleBool  `json:"userloved"`
	Image         []apiImage    `json:"image"`
	Tags          struct {
		Tag []apiTag `json:"tag"`
	} `json:"toptags"`
	Wiki apiTextBlock `json:"wiki"`
	Attr struct {
		NowPlaying flexibleBool  `json:"nowplaying"`
		Rank       flexibleInt64 `json:"rank"`
	} `json:"@attr"`
	Date struct {
		Unix flexibleInt64 `json:"uts"`
	} `json:"date"`
}

type apiPageAttr struct {
	Page       flexibleInt64 `json:"page"`
	PerPage    flexibleInt64 `json:"perPage"`
	TotalPages flexibleInt64 `json:"totalPages"`
	Total      flexibleInt64 `json:"total"`
}

func mapImages(input []apiImage) []Image {
	output := make([]Image, 0, len(input))
	for _, image := range input {
		if image.URL != "" {
			output = append(output, Image{Size: image.Size, URL: image.URL})
		}
	}
	return output
}

func mapTags(input []apiTag) []Tag {
	output := make([]Tag, 0, len(input))
	for _, tag := range input {
		output = append(output, Tag{Name: tag.Name, URL: tag.URL})
	}
	return output
}

func mapTextBlock(input apiTextBlock) TextBlock {
	return TextBlock{Published: input.Published, Summary: input.Summary, Content: input.Content}
}

func mapArtist(input apiArtist) Artist {
	artist := Artist{
		Name: firstNonEmpty(input.Name, input.Text), MBID: input.MBID, URL: input.URL,
		Streamable: bool(input.Streamable), OnTour: bool(input.OnTour), Images: mapImages(input.Image),
		Listeners: int64(input.Stats.Listeners), PlayCount: int64(input.Stats.PlayCount),
		UserPlayCount: int64(input.Stats.UserPlayCount), Tags: mapTags(input.Tags.Tag), Biography: mapTextBlock(input.Bio),
	}
	artist.Similar = make([]Artist, 0, len(input.Similar.Artist))
	for _, similar := range input.Similar.Artist {
		artist.Similar = append(artist.Similar, mapArtist(similar))
	}
	return artist
}

func mapAlbum(input apiAlbum, includeTracks bool) Album {
	album := Album{
		Name: firstNonEmpty(input.Name, input.Title, input.Text), Artist: firstNonEmpty(input.Artist.Name, input.Artist.Text),
		MBID: input.MBID, URL: input.URL, Listeners: int64(input.Listeners), PlayCount: int64(input.PlayCount),
		UserPlayCount: int64(input.UserPlayCount), Images: mapImages(input.Image), Tags: mapTags(input.Tags.Tag), Wiki: mapTextBlock(input.Wiki),
	}
	if includeTracks {
		album.Tracks = make([]Track, 0, len(input.Tracks.Track))
		for _, track := range input.Tracks.Track {
			album.Tracks = append(album.Tracks, mapTrack(track, time.Second, false))
		}
	}
	return album
}

func mapTrack(input apiTrack, durationUnit time.Duration, includeAlbum bool) Track {
	track := Track{
		Name: firstNonEmpty(input.Name, input.Text), Artist: mapArtist(input.Artist), MBID: input.MBID, URL: input.URL,
		Duration: time.Duration(input.Duration) * durationUnit, Streamable: input.Streamable.Value, FullTrack: input.Streamable.FullTrack,
		Listeners: int64(input.Listeners), PlayCount: int64(input.PlayCount), UserPlayCount: int64(input.UserPlayCount),
		UserLoved: bool(input.UserLoved), Rank: int(input.Attr.Rank), Images: mapImages(input.Image),
		Tags: mapTags(input.Tags.Tag), Wiki: mapTextBlock(input.Wiki), NowPlaying: bool(input.Attr.NowPlaying),
	}
	if includeAlbum && input.Album != nil {
		album := mapAlbum(*input.Album, false)
		track.Album = &album
	}
	if input.Date.Unix > 0 {
		played := time.Unix(int64(input.Date.Unix), 0).UTC()
		track.PlayedAt = &played
	}
	return track
}

func unixTime(value flexibleInt64) *time.Time {
	if value <= 0 {
		return nil
	}
	parsed := time.Unix(int64(value), 0).UTC()
	return &parsed
}
