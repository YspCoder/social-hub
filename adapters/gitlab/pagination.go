package gitlab

import (
	"encoding/json"
	"strconv"
	"strings"
)

func buildPage[T any](operation string, items []T, meta ResponseMeta, raw json.RawMessage) (*Page[T], error) {
	page, ok := parsePageHeader(meta.Page, 1, 0)
	if !ok {
		return nil, platformContractError(operation, "GitLab returned an invalid X-Page header")
	}
	perPage, ok := parsePageHeader(meta.PerPage, 1, MaximumPageSize)
	if !ok {
		return nil, platformContractError(operation, "GitLab returned an invalid X-Per-Page header")
	}
	next, ok := parsePageHeader(meta.NextPage, 1, 0)
	if !ok {
		return nil, platformContractError(operation, "GitLab returned an invalid X-Next-Page header")
	}
	previous, ok := parsePageHeader(meta.PreviousPage, 1, 0)
	if !ok {
		return nil, platformContractError(operation, "GitLab returned an invalid X-Prev-Page header")
	}
	total, ok := parsePageHeader(meta.Total, 0, 0)
	if !ok {
		return nil, platformContractError(operation, "GitLab returned an invalid X-Total header")
	}
	totalPages, ok := parsePageHeader(meta.TotalPages, 0, 0)
	if !ok {
		return nil, platformContractError(operation, "GitLab returned an invalid X-Total-Pages header")
	}
	return &Page[T]{
		Items: items,
		Pagination: Pagination{
			Page: page, PerPage: perPage, NextPage: next, PreviousPage: previous,
			Total: total, TotalPages: totalPages, Link: meta.Link,
		},
		Meta: meta, Raw: raw,
	}, nil
}

func parsePageHeader(value string, minimum, maximum int64) (*int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < minimum || maximum > 0 && parsed > maximum {
		return nil, false
	}
	return &parsed, true
}
