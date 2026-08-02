package tvmaze

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListShowUpdates(ctx context.Context, period UpdatePeriod, options ...socialhub.CallOption) ([]Update, error) {
	return c.listUpdates(ctx, "list_show_updates", "/updates/shows", period, options...)
}

func (c *Client) ListPeopleUpdates(ctx context.Context, period UpdatePeriod, options ...socialhub.CallOption) ([]Update, error) {
	return c.listUpdates(ctx, "list_people_updates", "/updates/people", period, options...)
}

func (c *Client) listUpdates(ctx context.Context, operation, path string, period UpdatePeriod, options ...socialhub.CallOption) ([]Update, error) {
	if !validUpdatePeriod(period) {
		return nil, invalidArgument(operation, "period must be day, week, or month when supplied")
	}
	query := url.Values{}
	if period != "" {
		query.Set("since", string(period))
	}
	var raw map[string]int64
	if _, err := requestJSON(ctx, c.api, operation, path, query, &raw, options...); err != nil {
		return nil, err
	}
	updates := make([]Update, 0, len(raw))
	for identifier, timestamp := range raw {
		id, err := strconv.ParseInt(identifier, 10, 64)
		if err != nil || id <= 0 || strconv.FormatInt(id, 10) != identifier || timestamp <= 0 {
			return nil, invalidPlatformResponse(operation, "updates response contained a noncanonical identifier or timestamp")
		}
		updates = append(updates, Update{ID: id, UpdatedAt: time.Unix(timestamp, 0).UTC()})
	}
	sort.Slice(updates, func(i, j int) bool { return updates[i].ID < updates[j].ID })
	return updates, nil
}
