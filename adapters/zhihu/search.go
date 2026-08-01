package zhihu

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

// SearchWorkflow exposes official Zhihu site-search and hot-list APIs.
type SearchWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (*SearchResult, error)
	HotList(context.Context, int, ...socialhub.CallOption) (*HotListResult, error)
}

// SearchService implements SearchWorkflow.
type SearchService struct{ client *Client }

func (s *SearchService) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (*SearchResult, error) {
	if err := s.client.requireApproval("search"); err != nil {
		return nil, err
	}
	queryText := strings.TrimSpace(input.Query)
	if queryText == "" {
		return nil, invalidArgument("search", "query is required")
	}
	count := input.Count
	if count <= 0 {
		count = 10
	}
	if count > 10 {
		count = 10
	}
	query := url.Values{"Query": {queryText}, "Count": {strconv.Itoa(count)}}
	var response responseEnvelope[searchData]
	if err := s.client.transport.JSON(ctx, http.MethodGet, "/api/v1/content/zhihu_search", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := response.Err("search", http.StatusOK, nil); err != nil {
		return nil, err
	}
	items := make([]SearchItem, 0, len(response.Data.Items))
	for _, item := range response.Data.Items {
		comments := make([]string, 0, len(item.Comments))
		for _, comment := range item.Comments {
			comments = append(comments, comment.Content)
		}
		items = append(items, SearchItem{
			Title: item.Title, ContentType: item.ContentType, ContentID: item.ContentID, ContentText: item.ContentText,
			URL: item.URL, CommentCount: item.CommentCount, VoteUpCount: item.VoteUpCount, AuthorName: item.AuthorName,
			AuthorAvatar: item.AuthorAvatar, AuthorBadge: item.AuthorBadge, AuthorBadgeText: item.AuthorBadgeText,
			EditedAt: unixTimePointer(item.EditTime), Comments: comments, AuthorityLevel: item.AuthorityLevel, RankingScore: item.RankingScore,
		})
	}
	return &SearchResult{HasMore: response.Data.HasMore, SearchHashID: response.Data.SearchHashID, EmptyReason: response.Data.EmptyReason, Items: items}, nil
}

func (s *SearchService) HotList(ctx context.Context, limit int, options ...socialhub.CallOption) (*HotListResult, error) {
	if err := s.client.requireApproval("hot_list"); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 30 {
		limit = 30
	}
	query := url.Values{"Limit": {strconv.Itoa(limit)}}
	var response responseEnvelope[hotListData]
	if err := s.client.transport.JSON(ctx, http.MethodGet, "/api/v1/content/hot_list", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := response.Err("hot_list", http.StatusOK, nil); err != nil {
		return nil, err
	}
	items := make([]HotListItem, 0, len(response.Data.Items))
	for _, item := range response.Data.Items {
		items = append(items, HotListItem{Title: item.Title, URL: item.URL, ThumbnailURL: item.ThumbnailURL, Summary: item.Summary})
	}
	return &HotListResult{Total: response.Data.Total, Items: items}, nil
}

func unixTimePointer(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	value := time.Unix(seconds, 0).UTC()
	return &value
}

var _ SearchWorkflow = (*SearchService)(nil)
