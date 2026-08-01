package soundcloud

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

// Playlist is the SoundCloud-specific playlist subset retained in feed activities.
type Playlist struct {
	URN          string
	Title        string
	Description  string
	Sharing      string
	PermalinkURL string
	UserURN      string
	TrackURNs    []string
}

// Activity retains SoundCloud feed semantics that do not fit the common Post model.
type Activity struct {
	Type        string
	CreatedAt   *time.Time
	ReposterURN string
	Track       *socialhub.Post
	Playlist    *Playlist
	RawOrigin   json.RawMessage
}

// ActivityPage is a cursor page from /me/feed.
type ActivityPage struct {
	Items        []Activity
	NextCursor   *string
	FutureCursor *string
	HasMore      bool
}

// ActivityWorkflow exposes the authenticated SoundCloud feed.
type ActivityWorkflow interface {
	Feed(context.Context, string, int, ...socialhub.CallOption) (ActivityPage, error)
}

func (c *Client) Feed(ctx context.Context, cursor string, maximum int, options ...socialhub.CallOption) (ActivityPage, error) {
	query, err := pageQuery("feed", cursor, maximum)
	if err != nil {
		return ActivityPage{}, err
	}
	var response soundCloudActivities
	if err := c.requestJSON(ctx, http.MethodGet, "/me/feed", query, nil, &response, options...); err != nil {
		return ActivityPage{}, err
	}
	items := make([]Activity, 0, len(response.Collection))
	for _, raw := range response.Collection {
		activity := Activity{Type: raw.Type, CreatedAt: timePointer(raw.CreatedAt), ReposterURN: raw.Reposter, RawOrigin: append(json.RawMessage(nil), raw.Origin...)}
		switch {
		case strings.HasPrefix(raw.Type, "track"):
			var track soundCloudTrack
			if err := json.Unmarshal(raw.Origin, &track); err != nil {
				return ActivityPage{}, platformError("feed", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
			}
			activity.Track, err = c.mapTrack(track)
		case strings.HasPrefix(raw.Type, "playlist"):
			var playlist soundCloudPlaylist
			if err := json.Unmarshal(raw.Origin, &playlist); err != nil {
				return ActivityPage{}, platformError("feed", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
			}
			activity.Playlist, err = c.mapPlaylist(playlist)
		}
		if err != nil {
			return ActivityPage{}, err
		}
		items = append(items, activity)
	}
	next, err := paginationCursor(response.NextHref, c.apiBaseURL)
	if err != nil {
		return ActivityPage{}, platformError("feed", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	future, err := paginationCursor(response.FutureHref, c.apiBaseURL)
	if err != nil {
		return ActivityPage{}, platformError("feed", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return ActivityPage{Items: items, NextCursor: next, FutureCursor: future, HasMore: next != nil}, nil
}

var _ ActivityWorkflow = (*Client)(nil)
