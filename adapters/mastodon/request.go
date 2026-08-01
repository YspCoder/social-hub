package mastodon

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tomnomnom/linkheader"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	request, err := c.transport.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	return c.transport.DoWithMetadata(request, output)
}

func (c *Client) form(ctx context.Context, method, path string, values url.Values, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	request, err := c.transport.NewRequest(ctx, method, path, nil, strings.NewReader(values.Encode()), options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.transport.DoWithMetadata(request, output)
}

func setPageQuery(query url.Values, cursor string, maximum int) {
	if cursor != "" {
		query.Set("max_id", cursor)
	}
	if maximum > 0 {
		if maximum > 40 {
			maximum = 40
		}
		query.Set("limit", strconv.Itoa(maximum))
	}
}

func nextCursor(header http.Header) *string {
	for _, link := range linkheader.Parse(header.Get("Link")) {
		if link.Rel != "next" {
			continue
		}
		parsed, err := url.Parse(link.URL)
		if err != nil {
			return nil
		}
		value := parsed.Query().Get("max_id")
		if value == "" {
			return nil
		}
		return &value
	}
	return nil
}

func mapStatusPage(accountID socialhub.AccountID, statuses []mastodonStatus, observedAt time.Time, header http.Header) socialhub.Page[socialhub.Post] {
	items := make([]socialhub.Post, 0, len(statuses))
	for _, status := range statuses {
		items = append(items, *mapStatus(accountID, status, observedAt))
	}
	next := nextCursor(header)
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}
}
