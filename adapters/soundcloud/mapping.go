package soundcloud

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) mapUser(input soundCloudUser) (*socialhub.User, error) {
	if !validURN(input.URN, "users") {
		return nil, platformError("map_user", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid SoundCloud user URN"))
	}
	extension, _ := json.Marshal(struct {
		City                 string `json:"city,omitempty"`
		Country              string `json:"country,omitempty"`
		Description          string `json:"description,omitempty"`
		Website              string `json:"website,omitempty"`
		FollowersCount       *int64 `json:"followers_count,omitempty"`
		FollowingsCount      *int64 `json:"followings_count,omitempty"`
		TrackCount           *int64 `json:"track_count,omitempty"`
		PlaylistCount        *int64 `json:"playlist_count,omitempty"`
		PublicFavoritesCount *int64 `json:"public_favorites_count,omitempty"`
		RepostsCount         *int64 `json:"reposts_count,omitempty"`
	}{
		input.City, input.Country, input.Description, input.Website, input.FollowersCount,
		input.FollowingsCount, input.TrackCount, input.PlaylistCount, input.PublicFavoritesCount, input.RepostsCount,
	})
	return &socialhub.User{
		Platform: "soundcloud", AccountID: c.accountID, ID: input.URN,
		Username: stringPointer(input.Username), DisplayName: stringPointer(firstNonEmpty(input.FullName, input.Username)),
		AvatarURL: stringPointer(input.AvatarURL), ProfileURL: stringPointer(input.PermalinkURL),
		AccountType: stringPointer(input.Plan), Extensions: map[string]json.RawMessage{"soundcloud.user": extension},
	}, nil
}

func (c *Client) mapTrack(input soundCloudTrack) (*socialhub.Post, error) {
	if !validURN(input.URN, "tracks") {
		return nil, platformError("map_track", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid SoundCloud track URN"))
	}
	authorURN := firstNonEmpty(input.UserURN, input.User.URN)
	if authorURN != "" && !validURN(authorURN, "users") {
		return nil, platformError("map_track", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid SoundCloud track owner URN"))
	}
	trackExtension, _ := json.Marshal(struct {
		Title        string `json:"title,omitempty"`
		Artist       string `json:"metadata_artist,omitempty"`
		ArtworkURL   string `json:"artwork_url,omitempty"`
		Access       string `json:"access,omitempty"`
		Genre        string `json:"genre,omitempty"`
		License      string `json:"license,omitempty"`
		TagList      string `json:"tag_list,omitempty"`
		Streamable   bool   `json:"streamable"`
		Downloadable bool   `json:"downloadable"`
		Commentable  bool   `json:"commentable"`
		APIURI       string `json:"uri,omitempty"`
	}{input.Title, input.MetadataArtist, input.ArtworkURL, input.Access, input.Genre, input.License, input.TagList, input.Streamable, input.Downloadable, input.Commentable, input.URI})
	duration := time.Duration(input.Duration) * time.Millisecond
	post := &socialhub.Post{
		Platform: "soundcloud", AccountID: c.accountID, ID: input.URN,
		AuthorID: stringPointer(authorURN), Text: stringPointer(firstNonEmpty(input.Description, input.Title)),
		CreatedAt: timePointer(input.CreatedAt), URL: stringPointer(input.PermalinkURL), Visibility: stringPointer(input.Sharing),
		Media: []socialhub.Media{{
			ID: input.URN, Type: socialhub.MediaTypeAudio, Duration: durationPointer(duration), State: socialhub.MediaStateReady,
			Extensions: map[string]json.RawMessage{"soundcloud.track": trackExtension},
		}},
		Extensions: map[string]json.RawMessage{"soundcloud.track": trackExtension},
	}
	observedAt := c.clock.Now()
	for name, value := range map[string]*int64{
		"comments": input.CommentCount, "likes": input.FavoritingsCount, "plays": input.PlaybackCount,
		"reposts": input.RepostsCount, "downloads": input.DownloadCount,
	} {
		if value != nil {
			post.Metrics = append(post.Metrics, socialhub.Metric{Name: name, Value: float64(*value), AsOf: observedAt, Definition: "SoundCloud track " + name + " total"})
		}
	}
	return post, nil
}

func (c *Client) mapComment(trackURN string, input soundCloudComment) (socialhub.Comment, error) {
	if !validURN(input.URN, "comments") {
		return socialhub.Comment{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid SoundCloud comment URN"))
	}
	responseTrackURN := firstNonEmpty(input.TrackURN, trackURN)
	if responseTrackURN != trackURN {
		return socialhub.Comment{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("comment track URN mismatch"))
	}
	authorURN := firstNonEmpty(input.UserURN, input.User.URN)
	if authorURN != "" && !validURN(authorURN, "users") {
		return socialhub.Comment{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid SoundCloud comment author URN"))
	}
	extension, _ := json.Marshal(struct {
		Timestamp json.RawMessage `json:"timestamp,omitempty"`
		URI       string          `json:"uri,omitempty"`
	}{input.Timestamp, input.URI})
	return socialhub.Comment{
		Platform: "soundcloud", AccountID: c.accountID, ID: input.URN, PostID: trackURN,
		AuthorID: stringPointer(authorURN), Text: input.Body, CreatedAt: timePointer(input.CreatedAt),
		Extensions: map[string]json.RawMessage{"soundcloud.comment": extension},
	}, nil
}

func (c *Client) mapPlaylist(input soundCloudPlaylist) (*Playlist, error) {
	if !validURN(input.URN, "playlists") {
		return nil, platformError("map_playlist", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid SoundCloud playlist URN"))
	}
	trackURNs := make([]string, 0, len(input.Tracks))
	for _, track := range input.Tracks {
		if !validURN(track.URN, "tracks") {
			return nil, platformError("map_playlist", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid SoundCloud playlist track URN"))
		}
		trackURNs = append(trackURNs, track.URN)
	}
	userURN := firstNonEmpty(input.UserURN, input.User.URN)
	if userURN != "" && !validURN(userURN, "users") {
		return nil, platformError("map_playlist", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid SoundCloud playlist owner URN"))
	}
	return &Playlist{
		URN: input.URN, Title: input.Title, Description: input.Description, Sharing: input.Sharing,
		PermalinkURL: input.PermalinkURL, UserURN: userURN, TrackURNs: trackURNs,
	}, nil
}

func paginationCursor(value string, base *url.URL) (*string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid SoundCloud pagination URL")
	}
	if (parsed.Scheme != "" || parsed.Host != "") &&
		(base == nil || !strings.EqualFold(parsed.Scheme, base.Scheme) || !strings.EqualFold(parsed.Host, base.Host)) {
		return nil, fmt.Errorf("SoundCloud pagination URL changed origin")
	}
	cursor := parsed.Query().Get("cursor")
	if !validCursor(cursor) {
		return nil, fmt.Errorf("invalid SoundCloud pagination cursor")
	}
	return &cursor, nil
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func durationPointer(value time.Duration) *time.Duration {
	if value <= 0 {
		return nil
	}
	copy := value
	return &copy
}

func timePointer(value soundCloudTime) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value.Time
	return &copy
}
