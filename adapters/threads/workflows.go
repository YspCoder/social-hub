package threads

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) PostInsights(ctx context.Context, postID string, metrics []string, options ...socialhub.CallOption) ([]Insight, error) {
	if strings.TrimSpace(postID) == "" || !validMetricNames(metrics) {
		return nil, invalidArgument("post_insights", "post ID and valid metric names are required")
	}
	if err := c.requireScope("post_insights", "threads_manage_insights"); err != nil {
		return nil, err
	}
	var response graphInsightPage
	if err := c.get(ctx, "/"+url.PathEscape(postID)+"/insights", url.Values{"metric": {strings.Join(metrics, ",")}}, &response, options...); err != nil {
		return nil, err
	}
	return mapInsights(response), nil
}

func (c *Client) AccountInsights(ctx context.Context, metrics []string, options ...socialhub.CallOption) ([]Insight, error) {
	if !validMetricNames(metrics) {
		return nil, invalidArgument("account_insights", "valid metric names are required")
	}
	if err := c.requireScope("account_insights", "threads_manage_insights"); err != nil {
		return nil, err
	}
	var response graphInsightPage
	if err := c.get(ctx, "/me/threads_insights", url.Values{"metric": {strings.Join(metrics, ",")}}, &response, options...); err != nil {
		return nil, err
	}
	return mapInsights(response), nil
}

func (c *Client) LookupProfile(ctx context.Context, username string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if strings.TrimSpace(username) == "" {
		return nil, invalidArgument("profile_lookup", "username is required")
	}
	if err := c.requireScope("profile_lookup", "threads_profile_discovery"); err != nil {
		return nil, err
	}
	var response graphProfile
	if err := c.get(ctx, "/profile_lookup", url.Values{"username": {username}, "fields": {profileFields}}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" || response.Username == "" {
		return nil, platformError("profile_lookup", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapProfile(c.accountID, response), nil
}

func (c *Client) ProfilePosts(ctx context.Context, username string, page PageRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if strings.TrimSpace(username) == "" {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("profile_posts", "username is required")
	}
	if err := c.requireScope("profile_posts", "threads_profile_discovery"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"username": {username}, "fields": {postFields}}
	if err := setPaging(query, page.Cursor, page.MaxResults); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	var response graphPostPage
	if err := c.get(ctx, "/profile_posts", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapPostPage(c.accountID, "", response)
}

func (c *Client) KeywordSearch(ctx context.Context, input KeywordSearchRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if strings.TrimSpace(input.Query) == "" {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("keyword_search", "query is required")
	}
	searchType := input.Type
	if searchType == "" {
		searchType = KeywordSearchTop
	}
	if searchType != KeywordSearchTop && searchType != KeywordSearchRecent {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("keyword_search", "search type must be TOP or RECENT")
	}
	if err := c.requireScope("keyword_search", "threads_keyword_search"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"q": {input.Query}, "search_type": {string(searchType)}, "fields": {postFields}}
	if err := setPaging(query, input.Cursor, input.MaxResults); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	var response graphPostPage
	if err := c.get(ctx, "/keyword_search", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapPostPage(c.accountID, "", response)
}

func (c *Client) Mentions(ctx context.Context, page PageRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if err := c.requireScope("mentions", "threads_manage_mentions"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"fields": {postFields}}
	if err := setPaging(query, page.Cursor, page.MaxResults); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	var response graphPostPage
	if err := c.get(ctx, "/me/mentions", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapPostPage(c.accountID, "", response)
}

func (c *Client) SetReplyHidden(ctx context.Context, replyID string, hidden bool, options ...socialhub.CallOption) error {
	if strings.TrimSpace(replyID) == "" {
		return invalidArgument("manage_reply", "reply ID is required")
	}
	if err := c.requireScope("manage_reply", "threads_manage_replies"); err != nil {
		return err
	}
	var response successResponse
	if err := c.form(ctx, http.MethodPost, "/"+url.PathEscape(replyID)+"/manage_reply", url.Values{"hide": {strconv.FormatBool(hidden)}}, &response, options...); err != nil {
		return err
	}
	if !response.Success {
		return platformError("manage_reply", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) ReviewPendingReply(ctx context.Context, replyID string, approve bool, options ...socialhub.CallOption) error {
	if strings.TrimSpace(replyID) == "" {
		return invalidArgument("manage_pending_reply", "reply ID is required")
	}
	if err := c.requireScope("manage_pending_reply", "threads_manage_replies"); err != nil {
		return err
	}
	var response successResponse
	if err := c.form(ctx, http.MethodPost, "/"+url.PathEscape(replyID)+"/manage_pending_reply", url.Values{"approve": {strconv.FormatBool(approve)}}, &response, options...); err != nil {
		return err
	}
	if !response.Success {
		return platformError("manage_pending_reply", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Repost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if strings.TrimSpace(postID) == "" {
		return nil, invalidArgument("repost", "post ID is required")
	}
	if err := c.requireScope("repost", "threads_content_publish"); err != nil {
		return nil, err
	}
	var response idResponse
	if err := c.form(ctx, http.MethodPost, "/"+url.PathEscape(postID)+"/repost", url.Values{}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, platformError("repost", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := c.clock.Now()
	return &socialhub.Post{
		Platform: "threads", AccountID: c.accountID, ID: response.ID, AuthorID: stringPointer(c.userID),
		Relations: []socialhub.PostRelation{{Type: socialhub.RelationRepost, PostID: postID}}, CreatedAt: &now,
		Visibility: stringPointer("public"), Status: &socialhub.PublishStatus{ID: response.ID, State: socialhub.PublishStatePublished, UpdatedAt: &now},
	}, nil
}

func (c *Client) PublishingQuota(ctx context.Context, options ...socialhub.CallOption) (*PublishingQuota, error) {
	if err := c.requireScope("publishing_quota", "threads_content_publish"); err != nil {
		return nil, err
	}
	fields := "quota_usage,config,reply_quota_usage,reply_config,delete_quota_usage,delete_config,location_search_quota_usage,location_search_config"
	var response graphQuotaPage
	if err := c.get(ctx, "/me/threads_publishing_limit", url.Values{"fields": {fields}}, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 {
		return nil, platformError("publishing_quota", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Data[0], nil
}
