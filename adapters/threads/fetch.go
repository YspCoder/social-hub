package threads

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	profileFields = "id,username,name,is_verified,threads_profile_picture_url,threads_biography,recently_searched_keywords,is_eligible_for_geo_gating"
	postFields    = "id,media_product_type,media_type,media_url,gif_url,permalink,owner,username,text,timestamp,shortcode,thumbnail_url,children{id,media_type,media_url,gif_url,thumbnail_url,alt_text,is_spoiler_media},is_quote_post,quoted_post{id},reposted_post{id},has_replies,alt_text,link_attachment_url,poll_attachment,text_entities,location_id,topic_tag,is_verified,profile_picture_url,is_reply,root_post{id},replied_to{id},reply_audience,is_spoiler_media,ghost_post_status"
	replyFields   = "id,text,timestamp,media_product_type,media_type,media_url,gif_url,permalink,owner,username,shortcode,thumbnail_url,children{id,media_type,media_url,gif_url,thumbnail_url,alt_text,is_spoiler_media},is_quote_post,quoted_post{id},reposted_post{id},alt_text,link_attachment_url,has_replies,is_reply,is_reply_owned_by_me,root_post{id},replied_to{id},hide_status,reply_audience,poll_attachment,text_entities,location_id,topic_tag,is_verified,profile_picture_url,reply_approval_status,is_spoiler_media,ghost_post_status"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID != "" && userID != "me" && userID != c.userID {
		return nil, invalidArgument("get_user", "common profile reads are limited to the configured Threads user")
	}
	if err := c.requireScope("get_user", "threads_basic"); err != nil {
		return nil, err
	}
	var response graphProfile
	if err := c.get(ctx, "/me", url.Values{"fields": {profileFields}}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" || response.Username == "" {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapProfile(c.accountID, response), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if strings.TrimSpace(postID) == "" {
		return nil, invalidArgument("get_post", "post ID is required")
	}
	if err := c.requireScope("get_post", "threads_basic"); err != nil {
		return nil, err
	}
	var response graphPost
	if err := c.get(ctx, "/"+url.PathEscape(postID), url.Values{"fields": {postFields}}, &response, options...); err != nil {
		return nil, err
	}
	return mapPost(c.accountID, c.userID, response)
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != "me" && input.UserID != c.userID {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "common post listing is limited to the configured Threads user")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Threads own-post listing does not expose exact time-range filters")
	}
	if err := c.requireScope("list_posts", "threads_basic"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"fields": {postFields}}
	if err := setPaging(query, input.Cursor, input.MaxResults); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	var response graphPostPage
	if err := c.get(ctx, "/me/threads", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapPostPage(c.accountID, c.userID, response)
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if strings.TrimSpace(input.PostID) == "" {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID is required")
	}
	if err := c.requireScope("list_comments", "threads_read_replies"); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query := url.Values{"fields": {replyFields}, "reverse": {"false"}}
	if err := setPaging(query, input.Cursor, input.MaxResults); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	var response graphPostPage
	if err := c.get(ctx, "/"+url.PathEscape(input.PostID)+"/replies", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	return mapCommentPage(c.accountID, input.PostID, response)
}
