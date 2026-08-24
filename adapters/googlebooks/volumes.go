package googlebooks

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

type volumesResponse struct {
	Kind       string   `json:"kind"`
	Items      []Volume `json:"items"`
	TotalItems int64    `json:"totalItems"`
}

// Search performs a public full-text Volume search. The query may use the
// documented intitle, inauthor, inpublisher, subject, isbn, lccn, and oclc prefixes.
func (client *Client) Search(ctx context.Context, input SearchVolumesRequest, options ...socialhub.CallOption) (*VolumePage, error) {
	const operation = "search_volumes"
	normalized, err := normalizeSearchRequest(input)
	if err != nil {
		return nil, err
	}
	query := url.Values{
		"q":          {normalized.Query},
		"startIndex": {strconv.Itoa(normalized.StartIndex)},
		"maxResults": {strconv.Itoa(normalized.MaxResults)},
	}
	if normalized.Filter != "" {
		query.Set("filter", string(normalized.Filter))
	}
	if normalized.OrderBy != "" {
		query.Set("orderBy", string(normalized.OrderBy))
	}
	if normalized.PrintType != "" {
		query.Set("printType", string(normalized.PrintType))
	}
	if normalized.Projection != "" {
		query.Set("projection", string(normalized.Projection))
	}
	if normalized.Language != "" {
		query.Set("langRestrict", normalized.Language)
	}
	var response volumesResponse
	meta, err := client.getJSON(ctx, operation, "/books/v1/volumes", query, &response, options...)
	if err != nil {
		return nil, err
	}
	if err := validateVolumePage(response, normalized); err != nil {
		return nil, err
	}
	return &VolumePage{
		Items: response.Items, TotalItems: response.TotalItems,
		StartIndex: normalized.StartIndex, MaxResults: normalized.MaxResults, Meta: meta,
	}, nil
}

// Get retrieves public metadata for one Volume ID.
func (client *Client) Get(ctx context.Context, input GetVolumeRequest, options ...socialhub.CallOption) (*VolumeResult, error) {
	const operation = "get_volume"
	normalized, err := normalizeGetRequest(input)
	if err != nil {
		return nil, err
	}
	query := make(url.Values)
	if normalized.Projection != "" {
		query.Set("projection", string(normalized.Projection))
	}
	var volume Volume
	meta, err := client.getJSON(ctx, operation, "/books/v1/volumes/"+url.PathEscape(normalized.VolumeID), query, &volume, options...)
	if err != nil {
		return nil, err
	}
	if err := validateVolume(operation, volume, normalized.VolumeID); err != nil {
		return nil, err
	}
	return &VolumeResult{Volume: volume, Meta: meta}, nil
}
