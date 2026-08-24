package taboola

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) accountPath(resource string) string {
	return url.PathEscape(client.advertiserID) + "/" + strings.TrimLeft(resource, "/")
}

func (client *Client) getJSON(ctx context.Context, operation string, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodGet, path, query, nil, output, options...), operation)
}

func (client *Client) postJSON(ctx context.Context, operation string, path string, input, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodPost, path, nil, input, output, options...), operation)
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
	}
	return err
}

type pageEnvelope[T any] struct {
	Results  []T `json:"results"`
	Metadata struct {
		Total int `json:"total"`
		Count int `json:"count"`
	} `json:"metadata"`
}

func paginationQuery(page, pageSize int) url.Values {
	query := make(url.Values)
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
		query.Set("page_size", strconv.Itoa(pageSize))
	}
	return query
}
