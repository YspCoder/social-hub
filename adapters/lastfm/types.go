package lastfm

import "time"

// Image is one size-labelled Last.fm artwork URL.
type Image struct {
	Size string `json:"size"`
	URL  string `json:"url"`
}

type Tag struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type TextBlock struct {
	Published string `json:"published,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Content   string `json:"content,omitempty"`
}

type Artist struct {
	Name          string    `json:"name"`
	MBID          string    `json:"mbid,omitempty"`
	URL           string    `json:"url,omitempty"`
	Streamable    bool      `json:"streamable"`
	OnTour        bool      `json:"on_tour"`
	Listeners     int64     `json:"listeners,omitempty"`
	PlayCount     int64     `json:"play_count,omitempty"`
	UserPlayCount int64     `json:"user_play_count,omitempty"`
	Images        []Image   `json:"images,omitempty"`
	Tags          []Tag     `json:"tags,omitempty"`
	Similar       []Artist  `json:"similar,omitempty"`
	Biography     TextBlock `json:"biography"`
}

type Album struct {
	Name          string    `json:"name"`
	Artist        string    `json:"artist"`
	MBID          string    `json:"mbid,omitempty"`
	URL           string    `json:"url,omitempty"`
	Listeners     int64     `json:"listeners,omitempty"`
	PlayCount     int64     `json:"play_count,omitempty"`
	UserPlayCount int64     `json:"user_play_count,omitempty"`
	Images        []Image   `json:"images,omitempty"`
	Tags          []Tag     `json:"tags,omitempty"`
	Tracks        []Track   `json:"tracks,omitempty"`
	Wiki          TextBlock `json:"wiki"`
}

type Track struct {
	Name          string        `json:"name"`
	Artist        Artist        `json:"artist"`
	MBID          string        `json:"mbid,omitempty"`
	URL           string        `json:"url,omitempty"`
	Duration      time.Duration `json:"duration"`
	Streamable    bool          `json:"streamable"`
	FullTrack     bool          `json:"full_track"`
	Listeners     int64         `json:"listeners,omitempty"`
	PlayCount     int64         `json:"play_count,omitempty"`
	UserPlayCount int64         `json:"user_play_count,omitempty"`
	UserLoved     bool          `json:"user_loved"`
	Rank          int           `json:"rank,omitempty"`
	Album         *Album        `json:"album,omitempty"`
	Images        []Image       `json:"images,omitempty"`
	Tags          []Tag         `json:"tags,omitempty"`
	Wiki          TextBlock     `json:"wiki"`
	PlayedAt      *time.Time    `json:"played_at,omitempty"`
	LovedAt       *time.Time    `json:"loved_at,omitempty"`
	NowPlaying    bool          `json:"now_playing"`
}

type User struct {
	Name          string     `json:"name"`
	RealName      string     `json:"real_name,omitempty"`
	URL           string     `json:"url,omitempty"`
	Country       string     `json:"country,omitempty"`
	Age           int        `json:"age,omitempty"`
	Gender        string     `json:"gender,omitempty"`
	Subscriber    bool       `json:"subscriber"`
	PlayCount     int64      `json:"play_count,omitempty"`
	ArtistCount   int64      `json:"artist_count,omitempty"`
	AlbumCount    int64      `json:"album_count,omitempty"`
	TrackCount    int64      `json:"track_count,omitempty"`
	PlaylistCount int64      `json:"playlist_count,omitempty"`
	RegisteredAt  *time.Time `json:"registered_at,omitempty"`
	Images        []Image    `json:"images,omitempty"`
}
