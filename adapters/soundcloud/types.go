package soundcloud

import (
	"encoding/json"
	"time"
)

type soundCloudPage[T any] struct {
	Collection []T    `json:"collection"`
	NextHref   string `json:"next_href"`
}

type soundCloudUser struct {
	URN                  string `json:"urn"`
	Username             string `json:"username"`
	FullName             string `json:"full_name"`
	AvatarURL            string `json:"avatar_url"`
	PermalinkURL         string `json:"permalink_url"`
	Plan                 string `json:"plan"`
	City                 string `json:"city"`
	Country              string `json:"country"`
	Description          string `json:"description"`
	Website              string `json:"website"`
	FollowersCount       *int64 `json:"followers_count"`
	FollowingsCount      *int64 `json:"followings_count"`
	TrackCount           *int64 `json:"track_count"`
	PlaylistCount        *int64 `json:"playlist_count"`
	PublicFavoritesCount *int64 `json:"public_favorites_count"`
	RepostsCount         *int64 `json:"reposts_count"`
}

type soundCloudTrack struct {
	URN              string         `json:"urn"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	ArtworkURL       string         `json:"artwork_url"`
	Duration         int64          `json:"duration"`
	CreatedAt        soundCloudTime `json:"created_at"`
	Sharing          string         `json:"sharing"`
	Access           string         `json:"access"`
	MetadataArtist   string         `json:"metadata_artist"`
	Genre            string         `json:"genre"`
	License          string         `json:"license"`
	TagList          string         `json:"tag_list"`
	User             soundCloudUser `json:"user"`
	UserURN          string         `json:"user_urn"`
	CommentCount     *int64         `json:"comment_count"`
	FavoritingsCount *int64         `json:"favoritings_count"`
	PlaybackCount    *int64         `json:"playback_count"`
	RepostsCount     *int64         `json:"reposts_count"`
	DownloadCount    *int64         `json:"download_count"`
	Streamable       bool           `json:"streamable"`
	Downloadable     bool           `json:"downloadable"`
	Commentable      bool           `json:"commentable"`
	URI              string         `json:"uri"`
	PermalinkURL     string         `json:"permalink_url"`
}

type soundCloudComment struct {
	URN       string          `json:"urn"`
	TrackURN  string          `json:"track_urn"`
	UserURN   string          `json:"user_urn"`
	Body      string          `json:"body"`
	Timestamp json.RawMessage `json:"timestamp"`
	CreatedAt soundCloudTime  `json:"created_at"`
	User      soundCloudUser  `json:"user"`
	URI       string          `json:"uri"`
}

type soundCloudPlaylist struct {
	URN          string            `json:"urn"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Sharing      string            `json:"sharing"`
	PermalinkURL string            `json:"permalink_url"`
	UserURN      string            `json:"user_urn"`
	User         soundCloudUser    `json:"user"`
	Tracks       []soundCloudTrack `json:"tracks"`
}

type soundCloudActivities struct {
	Collection []soundCloudActivity `json:"collection"`
	NextHref   string               `json:"next_href"`
	FutureHref string               `json:"future_href"`
}

type soundCloudActivity struct {
	Type      string          `json:"type"`
	CreatedAt soundCloudTime  `json:"created_at"`
	Reposter  string          `json:"reposter"`
	Origin    json.RawMessage `json:"origin"`
}

type soundCloudTime struct{ time.Time }

func (t *soundCloudTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	for _, layout := range []string{time.RFC3339Nano, "2006/01/02 15:04:05 -0700", "2006-01-02 15:04:05 -0700"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			t.Time = parsed
			return nil
		}
	}
	return &time.ParseError{Layout: time.RFC3339, Value: value}
}
