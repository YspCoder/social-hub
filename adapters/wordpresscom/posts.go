package wordpresscom

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (client *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if input.ReplyToID != nil || input.QuotePostID != nil {
		return nil, unsupported("publish", "WordPress Posts do not use reply or quote publication fields; use Reactor.Comment for discussion replies")
	}
	if len(input.MediaIDs) > 1 {
		return nil, unsupported("publish", "common publishing can map one media ID as featured_image; use PostWorkflow for richer layouts")
	}
	request := PostWriteRequest{Content: input.Text}
	if len(input.MediaIDs) == 1 {
		mediaID := input.MediaIDs[0]
		request.FeaturedImageID = &mediaID
	}
	status := PostPublished
	if input.Visibility != nil {
		switch *input.Visibility {
		case "", "public", "publish":
			status = PostPublished
		case "private":
			status = PostPrivate
		case "draft":
			status = PostDraft
		case "pending":
			status = PostPending
		default:
			return nil, unsupported("publish", "visibility must be public, private, draft, or pending")
		}
	}
	request.Status = &status
	return client.CreatePost(ctx, request, options...)
}

func (client *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := client.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return post.Status, nil
}

func (client *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if !validID(postID) {
		return invalidArgument("delete_post", "Post ID must be a positive integer")
	}
	api, err := client.requireUser("delete_post")
	if err != nil {
		return err
	}
	if err := client.requireScopes("delete_post", "posts"); err != nil {
		return err
	}
	var response wpPost
	if err := client.form(ctx, api, client.sitePath("posts", postID, "delete"), nil, &response, options...); err != nil {
		return err
	}
	if response.ID != mustID(postID) || response.SiteID > 0 && !client.matchesSite(response.SiteID) {
		return platformError("delete_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (client *Client) CreatePost(ctx context.Context, input PostWriteRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	values, err := postForm(input, true)
	if err != nil {
		return nil, err
	}
	api, err := client.requireUser("create_post")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("create_post", "posts"); err != nil {
		return nil, err
	}
	var response wpPost
	if err := client.form(ctx, api, client.sitePath("posts", "new"), values, &response, options...); err != nil {
		return nil, err
	}
	if response.ID <= 0 || response.SiteID <= 0 || !client.matchesSite(response.SiteID) {
		return nil, platformError("create_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPost(client.accountID, response, client.clock.Now()), nil
}

func (client *Client) UpdatePost(ctx context.Context, postID string, input PostWriteRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validID(postID) {
		return nil, invalidArgument("update_post", "Post ID must be a positive integer")
	}
	values, err := postForm(input, false)
	if err != nil {
		return nil, err
	}
	api, err := client.requireUser("update_post")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("update_post", "posts"); err != nil {
		return nil, err
	}
	var response wpPost
	if err := client.form(ctx, api, client.sitePath("posts", postID), values, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != mustID(postID) || response.SiteID <= 0 || !client.matchesSite(response.SiteID) {
		return nil, platformError("update_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPost(client.accountID, response, client.clock.Now()), nil
}

func (client *Client) RestorePost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validID(postID) {
		return nil, invalidArgument("restore_post", "Post ID must be a positive integer")
	}
	api, err := client.requireUser("restore_post")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("restore_post", "posts"); err != nil {
		return nil, err
	}
	var response wpPost
	if err := client.form(ctx, api, client.sitePath("posts", postID, "restore"), nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != mustID(postID) || response.SiteID <= 0 || !client.matchesSite(response.SiteID) {
		return nil, platformError("restore_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPost(client.accountID, response, client.clock.Now()), nil
}

func postForm(input PostWriteRequest, create bool) (url.Values, error) {
	if !hasPostFields(input) {
		return nil, invalidArgument(postOperation(create), "at least one Post field is required")
	}
	if create && !hasPostContent(input) {
		return nil, invalidArgument("create_post", "title, content, or featured image is required")
	}
	for _, value := range []*string{input.Title, input.Content, input.Excerpt, input.Slug, input.PublicizeMessage} {
		if value != nil && !validText(*value) {
			return nil, invalidArgument(postOperation(create), "Post text fields must be valid UTF-8 without NUL or carriage return")
		}
	}
	if input.Status != nil && !validPostStatus(*input.Status) {
		return nil, invalidArgument(postOperation(create), "Post status is invalid")
	}
	if input.Status != nil && *input.Status == PostFuture && input.Date == nil {
		return nil, invalidArgument(postOperation(create), "future status requires a publication date")
	}
	if input.Date != nil && input.Date.IsZero() {
		return nil, invalidArgument(postOperation(create), "publication date is invalid")
	}
	if input.FeaturedImageID != nil && *input.FeaturedImageID != "" && !validID(*input.FeaturedImageID) {
		return nil, invalidArgument(postOperation(create), "featured image ID must be empty or a positive integer")
	}
	if !validTerms(input.Categories) || !validTerms(input.Tags) {
		return nil, invalidArgument(postOperation(create), "categories and tags must contain non-empty single-line values")
	}
	values := url.Values{}
	setOptional(values, "title", input.Title)
	setOptional(values, "content", input.Content)
	setOptional(values, "excerpt", input.Excerpt)
	setOptional(values, "slug", input.Slug)
	setOptional(values, "featured_image", input.FeaturedImageID)
	setOptional(values, "publicize_message", input.PublicizeMessage)
	if input.Status != nil {
		values.Set("status", string(*input.Status))
	}
	if input.Date != nil {
		values.Set("date", input.Date.UTC().Format(time.RFC3339))
	}
	for _, category := range input.Categories {
		values.Add("categories[]", strings.TrimSpace(category))
	}
	for _, tag := range input.Tags {
		values.Add("tags[]", strings.TrimSpace(tag))
	}
	if input.Categories != nil && len(input.Categories) == 0 {
		values.Set("categories", "")
	}
	if input.Tags != nil && len(input.Tags) == 0 {
		values.Set("tags", "")
	}
	if input.Publicize != nil {
		values.Set("publicize", strconv.FormatBool(*input.Publicize))
	}
	if input.CommentsOpen != nil {
		values.Set("discussion[comments_open]", strconv.FormatBool(*input.CommentsOpen))
	}
	if input.LikesEnabled != nil {
		values.Set("likes_enabled", strconv.FormatBool(*input.LikesEnabled))
	}
	return values, nil
}

func postOperation(create bool) string {
	if create {
		return "create_post"
	}
	return "update_post"
}

func hasPostFields(input PostWriteRequest) bool {
	return input.Title != nil || input.Content != nil || input.Excerpt != nil || input.Slug != nil || input.Status != nil || input.Date != nil ||
		input.Categories != nil || input.Tags != nil || input.FeaturedImageID != nil || input.Publicize != nil || input.PublicizeMessage != nil ||
		input.CommentsOpen != nil || input.LikesEnabled != nil
}

func hasPostContent(input PostWriteRequest) bool {
	return input.Title != nil && strings.TrimSpace(*input.Title) != "" || input.Content != nil && strings.TrimSpace(*input.Content) != "" ||
		input.FeaturedImageID != nil && validID(*input.FeaturedImageID)
}

func validPostStatus(value PostStatus) bool {
	switch value {
	case PostPublished, PostPrivate, PostDraft, PostPending, PostFuture:
		return true
	default:
		return false
	}
}

func validText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r")
}

func validTerms(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\x00\r\n") {
			return false
		}
	}
	return true
}

func setOptional(values url.Values, key string, value *string) {
	if value != nil {
		values.Set(key, *value)
	}
}
