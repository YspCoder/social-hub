package twitch

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListStreams(ctx context.Context, input StreamRequest, options ...socialhub.CallOption) (socialhub.Page[Stream], error) {
	query := url.Values{}
	if err := appendValues(query, "user_id", input.UserIDs, 100); err != nil {
		return socialhub.Page[Stream]{}, err
	}
	if err := appendValues(query, "user_login", input.UserLogins, 100); err != nil {
		return socialhub.Page[Stream]{}, err
	}
	if err := appendValues(query, "game_id", input.GameIDs, 100); err != nil {
		return socialhub.Page[Stream]{}, err
	}
	if err := appendValues(query, "language", input.Languages, 100); err != nil {
		return socialhub.Page[Stream]{}, err
	}
	if err := setPaging(query, input.Cursor, input.MaxResults); err != nil {
		return socialhub.Page[Stream]{}, err
	}
	var response streamPage
	if err := c.get(ctx, "/streams", query, &response, options...); err != nil {
		return socialhub.Page[Stream]{}, err
	}
	for _, stream := range response.Data {
		if stream.ID == "" || stream.UserID == "" {
			return socialhub.Page[Stream]{}, platformError("list_streams", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	next := stringPointer(response.Pagination.Cursor)
	return socialhub.Page[Stream]{Items: response.Data, NextCursor: next, HasMore: next != nil}, nil
}

func (c *Client) GetChannel(ctx context.Context, broadcasterID string, options ...socialhub.CallOption) (*Channel, error) {
	if strings.TrimSpace(broadcasterID) == "" {
		return nil, invalidArgument("get_channel", "broadcaster ID is required")
	}
	var response channelPage
	if err := c.get(ctx, "/channels", url.Values{"broadcaster_id": {broadcasterID}}, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 || response.Data[0].BroadcasterID == "" {
		return nil, platformError("get_channel", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return &response.Data[0], nil
}

func (c *Client) GetSchedule(ctx context.Context, broadcasterID, cursor string, maximum int, options ...socialhub.CallOption) (*Schedule, error) {
	if strings.TrimSpace(broadcasterID) == "" {
		return nil, invalidArgument("get_schedule", "broadcaster ID is required")
	}
	query := url.Values{"broadcaster_id": {broadcasterID}}
	if err := setPaging(query, cursor, maximum); err != nil {
		return nil, err
	}
	var response scheduleResponse
	if err := c.get(ctx, "/schedule", query, &response, options...); err != nil {
		return nil, err
	}
	if response.Data.BroadcasterID == "" {
		return nil, platformError("get_schedule", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	result := &Schedule{
		BroadcasterID: response.Data.BroadcasterID, BroadcasterLogin: response.Data.BroadcasterLogin,
		BroadcasterName: response.Data.BroadcasterName, Segments: make([]ScheduleSegment, 0, len(response.Data.Segments)),
		NextCursor: stringPointer(response.Pagination.Cursor),
	}
	for _, segment := range response.Data.Segments {
		mapped := ScheduleSegment{
			ID: segment.ID, StartTime: segment.StartTime, EndTime: segment.EndTime, Title: segment.Title,
			CanceledAt: segment.CanceledAt, Recurring: segment.Recurring,
		}
		if segment.Category != nil {
			mapped.CategoryID, mapped.CategoryName = segment.Category.ID, segment.Category.Name
		}
		result.Segments = append(result.Segments, mapped)
	}
	if response.Data.Vacation != nil {
		result.Vacation = &ScheduleVacation{StartTime: response.Data.Vacation.StartTime, EndTime: response.Data.Vacation.EndTime}
	}
	return result, nil
}

func (c *Client) ListClips(ctx context.Context, input ClipRequest, options ...socialhub.CallOption) (socialhub.Page[Clip], error) {
	dimensions := 0
	if len(input.IDs) > 0 {
		dimensions++
	}
	if strings.TrimSpace(input.BroadcasterID) != "" {
		dimensions++
	}
	if strings.TrimSpace(input.GameID) != "" {
		dimensions++
	}
	if dimensions != 1 {
		return socialhub.Page[Clip]{}, invalidArgument("list_clips", "exactly one of IDs, broadcaster ID, or game ID is required")
	}
	query := url.Values{}
	if len(input.IDs) > 0 {
		if err := appendValues(query, "id", input.IDs, 100); err != nil {
			return socialhub.Page[Clip]{}, err
		}
		if input.StartedAt != nil || input.EndedAt != nil || input.Featured != nil || input.Cursor != "" || input.MaxResults != 0 {
			return socialhub.Page[Clip]{}, invalidArgument("list_clips", "ID lookup cannot be combined with filters or pagination")
		}
	} else {
		if input.BroadcasterID != "" {
			query.Set("broadcaster_id", input.BroadcasterID)
		} else {
			query.Set("game_id", input.GameID)
		}
		if input.EndedAt != nil && input.StartedAt == nil {
			return socialhub.Page[Clip]{}, invalidArgument("list_clips", "ended_at requires started_at")
		}
		if input.StartedAt != nil {
			query.Set("started_at", input.StartedAt.UTC().Format(time.RFC3339))
		}
		if input.EndedAt != nil {
			if !input.EndedAt.After(*input.StartedAt) {
				return socialhub.Page[Clip]{}, invalidArgument("list_clips", "ended_at must be after started_at")
			}
			query.Set("ended_at", input.EndedAt.UTC().Format(time.RFC3339))
		}
		if input.Featured != nil {
			query.Set("is_featured", strconv.FormatBool(*input.Featured))
		}
		if err := setPaging(query, input.Cursor, input.MaxResults); err != nil {
			return socialhub.Page[Clip]{}, err
		}
	}
	var response clipPage
	if err := c.get(ctx, "/clips", query, &response, options...); err != nil {
		return socialhub.Page[Clip]{}, err
	}
	items := make([]Clip, 0, len(response.Data))
	for _, item := range response.Data {
		if item.ID == "" {
			return socialhub.Page[Clip]{}, platformError("list_clips", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		items = append(items, Clip{
			ID: item.ID, URL: item.URL, EmbedURL: item.EmbedURL, BroadcasterID: item.BroadcasterID,
			BroadcasterName: item.BroadcasterName, CreatorID: item.CreatorID, CreatorName: item.CreatorName,
			VideoID: item.VideoID, GameID: item.GameID, Language: item.Language, Title: item.Title,
			ViewCount: item.ViewCount, CreatedAt: item.CreatedAt, ThumbnailURL: item.ThumbnailURL,
			Duration: time.Duration(item.Duration * float64(time.Second)), VODOffset: item.VODOffset, Featured: item.Featured,
		})
	}
	next := stringPointer(response.Pagination.Cursor)
	return socialhub.Page[Clip]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func (c *Client) CreateClip(ctx context.Context, broadcasterID string, hasDelay bool, options ...socialhub.CallOption) (*ClipCreation, error) {
	if strings.TrimSpace(broadcasterID) == "" {
		return nil, invalidArgument("create_clip", "broadcaster ID is required")
	}
	if err := c.requireScope("create_clip", "clips:edit"); err != nil {
		return nil, err
	}
	query := url.Values{"broadcaster_id": {broadcasterID}, "has_delay": {strconv.FormatBool(hasDelay)}}
	var response struct {
		Data []struct {
			ID      string `json:"id"`
			EditURL string `json:"edit_url"`
		} `json:"data"`
	}
	if err := c.request(ctx, http.MethodPost, "/clips", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 || response.Data[0].ID == "" || response.Data[0].EditURL == "" {
		return nil, platformError("create_clip", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &ClipCreation{ID: response.Data[0].ID, EditURL: response.Data[0].EditURL}, nil
}
