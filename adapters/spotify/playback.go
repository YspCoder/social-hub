package spotify

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetPlaybackState(ctx context.Context, market string, options ...socialhub.CallOption) (*PlaybackState, error) {
	if err := c.requireScopes("get_playback_state", ScopeUserReadPlaybackState); err != nil {
		return nil, err
	}
	if !validMarket(market) {
		return nil, invalidArgument("get_playback_state", "market must be an uppercase ISO 3166-1 alpha-2 code")
	}
	query := url.Values{"additional_types": {"track,episode"}}
	if market != "" {
		query.Set("market", market)
	}
	var response spotifyPlaybackState
	metadata, err := c.requestJSON(ctx, http.MethodGet, "/me/player", query, nil, &response, options...)
	if err != nil {
		return nil, err
	}
	if metadata.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	return mapPlaybackState(response)
}

func (c *Client) ListDevices(ctx context.Context, options ...socialhub.CallOption) ([]Device, error) {
	if err := c.requireScopes("list_devices", ScopeUserReadPlaybackState); err != nil {
		return nil, err
	}
	var response spotifyDevicesResponse
	if _, err := c.requestJSON(ctx, http.MethodGet, "/me/player/devices", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	devices := make([]Device, 0, len(response.Devices))
	for _, item := range response.Devices {
		if !validDeviceID(item.ID) {
			return nil, mappingError("list_devices", "Spotify returned an invalid device ID")
		}
		devices = append(devices, mapDevice(item))
	}
	return devices, nil
}

func (c *Client) GetQueue(ctx context.Context, options ...socialhub.CallOption) (*Queue, error) {
	if err := c.requireAnyScope("get_queue", ScopeUserReadCurrentlyPlaying, ScopeUserReadPlaybackState); err != nil {
		return nil, err
	}
	var response spotifyQueue
	if _, err := c.requestJSON(ctx, http.MethodGet, "/me/player/queue", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	current, err := mapPlayable(response.CurrentlyPlaying)
	if err != nil {
		return nil, err
	}
	items := make([]Playable, 0, len(response.Queue))
	for _, raw := range response.Queue {
		item, err := mapPlayable(raw)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return &Queue{CurrentlyPlaying: current, Items: items}, nil
}

func (c *Client) TransferPlayback(ctx context.Context, input TransferPlaybackRequest, options ...socialhub.CallOption) error {
	if input.DeviceID == "" || !validDeviceID(input.DeviceID) {
		return invalidArgument("transfer_playback", "one valid Spotify device ID is required")
	}
	if err := c.requirePlaybackControl("transfer_playback"); err != nil {
		return err
	}
	payload := struct {
		DeviceIDs []string `json:"device_ids"`
		Play      bool     `json:"play"`
	}{[]string{input.DeviceID}, input.Play}
	_, err := c.requestJSON(ctx, http.MethodPut, "/me/player", nil, payload, nil, options...)
	return err
}

func (c *Client) StartPlayback(ctx context.Context, input StartPlaybackRequest, options ...socialhub.CallOption) error {
	if !validDeviceID(input.DeviceID) {
		return invalidArgument("start_playback", "device ID is invalid")
	}
	if input.ContextURI != "" && !validContextURI(input.ContextURI) {
		return invalidArgument("start_playback", "context URI must identify an album, artist, or playlist")
	}
	if input.ContextURI != "" && len(input.URIs) > 0 {
		return invalidArgument("start_playback", "context URI and explicit playable URIs are mutually exclusive")
	}
	if len(input.URIs) > 0 && !validPlaybackURIs(input.URIs) {
		return invalidArgument("start_playback", "at most 100 valid track URIs are allowed")
	}
	if input.Offset != nil {
		contextType, _ := spotifyURIType(input.ContextURI)
		if (contextType != "album" && contextType != "playlist") || (input.Offset.Position == nil) == (input.Offset.URI == "") {
			return invalidArgument("start_playback", "offset requires an album or playlist context and exactly one position or URI")
		}
		if input.Offset.Position != nil && *input.Offset.Position < 0 {
			return invalidArgument("start_playback", "offset position must not be negative")
		}
		if input.Offset.URI != "" && !validPlayableURI(input.Offset.URI) {
			return invalidArgument("start_playback", "offset URI must identify a track or episode")
		}
	}
	var positionMS *int64
	if input.Position != nil {
		if *input.Position < 0 {
			return invalidArgument("start_playback", "position must not be negative")
		}
		value := input.Position.Milliseconds()
		positionMS = &value
	}
	if err := c.requirePlaybackControl("start_playback"); err != nil {
		return err
	}
	query := deviceQuery(input.DeviceID)
	var body any
	if input.ContextURI != "" || len(input.URIs) > 0 || input.Offset != nil || positionMS != nil {
		payload := struct {
			ContextURI string          `json:"context_uri,omitempty"`
			URIs       []string        `json:"uris,omitempty"`
			Offset     *playbackOffset `json:"offset,omitempty"`
			PositionMS *int64          `json:"position_ms,omitempty"`
		}{ContextURI: input.ContextURI, URIs: input.URIs, PositionMS: positionMS}
		if input.Offset != nil {
			payload.Offset = &playbackOffset{Position: input.Offset.Position, URI: input.Offset.URI}
		}
		body = payload
	}
	_, err := c.requestJSON(ctx, http.MethodPut, "/me/player/play", query, body, nil, options...)
	return err
}

func (c *Client) PausePlayback(ctx context.Context, deviceID string, options ...socialhub.CallOption) error {
	return c.playbackCommand(ctx, "pause_playback", http.MethodPut, "/me/player/pause", deviceID, options...)
}

func (c *Client) SkipNext(ctx context.Context, deviceID string, options ...socialhub.CallOption) error {
	return c.playbackCommand(ctx, "skip_next", http.MethodPost, "/me/player/next", deviceID, options...)
}

func (c *Client) SkipPrevious(ctx context.Context, deviceID string, options ...socialhub.CallOption) error {
	return c.playbackCommand(ctx, "skip_previous", http.MethodPost, "/me/player/previous", deviceID, options...)
}

func (c *Client) Seek(ctx context.Context, position time.Duration, deviceID string, options ...socialhub.CallOption) error {
	if position < 0 {
		return invalidArgument("seek", "position must not be negative")
	}
	query := deviceQuery(deviceID)
	query.Set("position_ms", strconv.FormatInt(position.Milliseconds(), 10))
	return c.playbackCommandQuery(ctx, "seek", http.MethodPut, "/me/player/seek", deviceID, query, options...)
}

func (c *Client) SetRepeat(ctx context.Context, state, deviceID string, options ...socialhub.CallOption) error {
	if state != "track" && state != "context" && state != "off" {
		return invalidArgument("set_repeat", "repeat state must be track, context, or off")
	}
	query := deviceQuery(deviceID)
	query.Set("state", state)
	return c.playbackCommandQuery(ctx, "set_repeat", http.MethodPut, "/me/player/repeat", deviceID, query, options...)
}

func (c *Client) SetVolume(ctx context.Context, percent int, deviceID string, options ...socialhub.CallOption) error {
	if percent < 0 || percent > 100 {
		return invalidArgument("set_volume", "volume percent must be between 0 and 100")
	}
	query := deviceQuery(deviceID)
	query.Set("volume_percent", strconv.Itoa(percent))
	return c.playbackCommandQuery(ctx, "set_volume", http.MethodPut, "/me/player/volume", deviceID, query, options...)
}

func (c *Client) SetShuffle(ctx context.Context, enabled bool, deviceID string, options ...socialhub.CallOption) error {
	query := deviceQuery(deviceID)
	query.Set("state", strconv.FormatBool(enabled))
	return c.playbackCommandQuery(ctx, "set_shuffle", http.MethodPut, "/me/player/shuffle", deviceID, query, options...)
}

func (c *Client) AddToQueue(ctx context.Context, uri, deviceID string, options ...socialhub.CallOption) error {
	if !validPlayableURI(uri) {
		return invalidArgument("add_to_queue", "a track or episode Spotify URI is required")
	}
	query := deviceQuery(deviceID)
	query.Set("uri", uri)
	return c.playbackCommandQuery(ctx, "add_to_queue", http.MethodPost, "/me/player/queue", deviceID, query, options...)
}

func (c *Client) playbackCommand(ctx context.Context, operation, method, path, deviceID string, options ...socialhub.CallOption) error {
	return c.playbackCommandQuery(ctx, operation, method, path, deviceID, deviceQuery(deviceID), options...)
}

func (c *Client) playbackCommandQuery(ctx context.Context, operation, method, path, deviceID string, query url.Values, options ...socialhub.CallOption) error {
	if !validDeviceID(deviceID) {
		return invalidArgument(operation, "device ID is invalid")
	}
	if err := c.requirePlaybackControl(operation); err != nil {
		return err
	}
	_, err := c.requestJSON(ctx, method, path, query, nil, nil, options...)
	return err
}

func (c *Client) requirePlaybackControl(operation string) error {
	if err := c.requireScopes(operation, ScopeUserModifyPlaybackState); err != nil {
		return err
	}
	return c.requirePremium(operation)
}

func deviceQuery(deviceID string) url.Values {
	query := url.Values{}
	if deviceID != "" {
		query.Set("device_id", deviceID)
	}
	return query
}

func validPlaybackURIs(uris []string) bool {
	if len(uris) > 100 {
		return false
	}
	for _, uri := range uris {
		typeName, ok := spotifyURIType(uri)
		if !ok || typeName != "track" {
			return false
		}
	}
	return true
}

type playbackOffset struct {
	Position *int   `json:"position,omitempty"`
	URI      string `json:"uri,omitempty"`
}

var _ PlaybackWorkflow = (*Client)(nil)
