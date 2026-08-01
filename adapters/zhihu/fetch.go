package zhihu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(context.Context, string, ...socialhub.CallOption) (*socialhub.User, error) {
	return nil, unsupported("get_user", "the documented Data Open Platform does not expose a user detail endpoint")
}

func (c *Client) GetPost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error) {
	return nil, unsupported("get_post", "the documented Data Open Platform does not expose a single-content detail endpoint")
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if err := c.requireApproval("list_posts"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if input.UserID != "" {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "the API selects the current or OAuth-authorized user from credentials, not a user ID")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "the user contents API does not support time-range filters")
	}
	offset := int64(0)
	if input.Cursor != "" {
		parsed, err := strconv.ParseInt(input.Cursor, 10, 64)
		if err != nil || parsed < 0 {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "cursor must be a non-negative offset returned by Zhihu")
		}
		offset = parsed
	}
	limit := input.MaxResults
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "max_results must not exceed 50")
	}
	query := url.Values{
		"Offset":      {strconv.FormatInt(offset, 10)},
		"Limit":       {strconv.Itoa(limit)},
		"ContentType": {"all"},
		"SortField":   {"ts"},
		"SortOrder":   {"desc"},
	}
	request, err := c.transport.NewRequest(ctx, http.MethodGet, "/api/v1/user/contents", query, nil, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if c.oauthToken != "" {
		request.Header.Set("X-OAuth-Token", c.oauthToken)
	}
	var response responseEnvelope[userContentsData]
	if err := c.transport.Do(request, &response); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if err := response.Err("list_posts", http.StatusOK, nil); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	observedAt := c.clock.Now()
	items := make([]socialhub.Post, 0, len(response.Data.Items))
	for _, item := range response.Data.Items {
		if item.URL == "" {
			return socialhub.Page[socialhub.Post]{}, wrapError("list_posts", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		extension, err := json.Marshal(struct {
			ContentType   string `json:"content_type"`
			Title         string `json:"title"`
			FavoriteCount int64  `json:"favorite_count"`
		}{ContentType: item.ContentType, Title: item.Title, FavoriteCount: item.FavoriteCount})
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, wrapError("list_posts", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
		var text *string
		if strings.TrimSpace(item.Summary) != "" {
			value := item.Summary
			text = &value
		}
		contentURL := item.URL
		items = append(items, socialhub.Post{
			Platform: "zhihu", AccountID: c.accountID, ID: item.URL, Text: text, CreatedAt: unixTimePointer(item.CreatedAt), URL: &contentURL,
			Metrics: []socialhub.Metric{
				{Name: "like_count", Value: float64(item.LikeCount), AsOf: observedAt, Definition: "likes reported by Zhihu Data Open Platform"},
				{Name: "comment_count", Value: float64(item.CommentCount), AsOf: observedAt, Definition: "comments reported by Zhihu Data Open Platform"},
				{Name: "favorite_count", Value: float64(item.FavoriteCount), AsOf: observedAt, Definition: "favorites reported by Zhihu Data Open Platform"},
			},
			Extensions: map[string]json.RawMessage{"zhihu.content": extension},
		})
	}
	var next *string
	if !response.Data.Paging.IsEnd && response.Data.Paging.NextOffset != "" {
		value := response.Data.Paging.NextOffset
		next = &value
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "the documented Data Open Platform does not expose comments as a pageable resource")
}
