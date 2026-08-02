package lastfm

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) LoveTrack(ctx context.Context, input TrackRef, options ...socialhub.CallOption) error {
	return c.changeLove(ctx, "track.love", input, options...)
}

func (c *Client) UnloveTrack(ctx context.Context, input TrackRef, options ...socialhub.CallOption) error {
	return c.changeLove(ctx, "track.unlove", input, options...)
}

func (c *Client) changeLove(ctx context.Context, method string, input TrackRef, options ...socialhub.CallOption) error {
	if !validTrackRef(input.Artist, input.Track) {
		return invalidArgument(method, "artist and track are required")
	}
	return c.post(ctx, method, url.Values{"artist": {input.Artist}, "track": {input.Track}}, nil, options...)
}
