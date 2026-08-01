package matrix

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) Sync(ctx context.Context, input SyncRequest, options ...socialhub.CallOption) (*SyncResponse, error) {
	if input.Timeout < 0 || input.Since != "" && !validOpaque(input.Since, maxOpaqueLength) {
		return nil, invalidArgument("sync", "since token or timeout is invalid")
	}
	query := url.Values{}
	if input.Since != "" {
		query.Set("since", input.Since)
	}
	if input.Timeout > 0 {
		query.Set("timeout", strconv.FormatInt(input.Timeout.Milliseconds(), 10))
	}
	if input.FullState {
		query.Set("full_state", "true")
	}
	var response SyncResponse
	if err := client.json(ctx, http.MethodGet, "/_matrix/client/v3/sync", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if !validOpaque(response.NextBatch, maxOpaqueLength) {
		return nil, platformError("sync", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	for roomID, room := range response.Rooms.Join {
		if !validRoomID(roomID) {
			return nil, platformError("sync", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		for index := range room.Timeline.Events {
			event := &room.Timeline.Events[index]
			if event.RoomID != "" && event.RoomID != roomID {
				return nil, platformError("sync", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
			}
			event.RoomID = roomID
		}
		response.Rooms.Join[roomID] = room
	}
	return &response, nil
}

var _ SyncWorkflow = (*Client)(nil)
