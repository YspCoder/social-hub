package linkedin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type createPostPayload struct {
	Author       string `json:"author"`
	Commentary   string `json:"commentary,omitempty"`
	Visibility   string `json:"visibility"`
	Distribution struct {
		FeedDistribution               string `json:"feedDistribution"`
		TargetEntities                 []any  `json:"targetEntities"`
		ThirdPartyDistributionChannels []any  `json:"thirdPartyDistributionChannels"`
	} `json:"distribution"`
	Content *struct {
		Media *struct {
			ID string `json:"id"`
		} `json:"media,omitempty"`
		MultiImage *struct {
			Images []struct {
				ID string `json:"id"`
			} `json:"images"`
		} `json:"multiImage,omitempty"`
	} `json:"content,omitempty"`
	LifecycleState            string `json:"lifecycleState"`
	IsReshareDisabledByAuthor bool   `json:"isReshareDisabledByAuthor"`
	ReshareContext            *struct {
		Parent string `json:"parent"`
	} `json:"reshareContext,omitempty"`
}

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if err := c.requireScopes("publish", c.socialScope(true)); err != nil {
		return nil, err
	}
	if input.ReplyToID != nil {
		return nil, unsupported("publish", "use Comment for LinkedIn post replies")
	}
	payload := createPostPayload{Author: c.authorURN, Visibility: "PUBLIC", LifecycleState: "PUBLISHED"}
	payload.Distribution.FeedDistribution = "MAIN_FEED"
	payload.Distribution.TargetEntities = []any{}
	payload.Distribution.ThirdPartyDistributionChannels = []any{}
	if input.Text != nil {
		payload.Commentary = *input.Text
	}
	if input.Visibility != nil {
		visibility := strings.ToUpper(*input.Visibility)
		if visibility != "PUBLIC" && visibility != "CONNECTIONS" {
			return nil, invalidArgument("publish", "visibility must be PUBLIC or CONNECTIONS")
		}
		if visibility == "CONNECTIONS" && c.organizationAuthor() {
			return nil, invalidArgument("publish", "organization posts cannot use CONNECTIONS visibility")
		}
		payload.Visibility = visibility
	}
	if input.QuotePostID != nil {
		if !validURN(*input.QuotePostID) || len(input.MediaIDs) > 0 {
			return nil, invalidArgument("publish", "reshares require one valid parent URN and cannot attach new media")
		}
		payload.ReshareContext = &struct {
			Parent string `json:"parent"`
		}{Parent: *input.QuotePostID}
	}
	if err := setPostMedia(&payload, input.MediaIDs); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := c.transport.NewRequest(ctx, http.MethodPost, "/rest/posts", nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	metadata, err := c.transport.DoWithMetadata(request, nil)
	if err != nil {
		return nil, err
	}
	id := metadata.Header.Get("X-RestLi-Id")
	if decoded, decodeErr := url.QueryUnescape(id); decodeErr == nil {
		id = decoded
	}
	if !validURN(id) {
		return nil, platformError("publish", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := c.clock.Now()
	post := &socialhub.Post{
		Platform: "linkedin", AccountID: c.accountID, ID: id, AuthorID: stringPointer(c.authorURN), Text: input.Text,
		CreatedAt: &now, URL: linkedInPostURL(id), Visibility: stringPointer(strings.ToLower(payload.Visibility)),
		Status: &socialhub.PublishStatus{ID: id, State: socialhub.PublishStatePublished, UpdatedAt: &now},
	}
	for _, mediaID := range input.MediaIDs {
		post.Media = append(post.Media, mediaFromURN(mediaID))
	}
	if input.QuotePostID != nil {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationRepost, PostID: *input.QuotePostID})
	}
	return post, nil
}

func setPostMedia(payload *createPostPayload, mediaIDs []string) error {
	if len(mediaIDs) == 0 {
		return nil
	}
	if len(mediaIDs) > 20 {
		return invalidArgument("publish", "LinkedIn posts accept at most 20 image URNs")
	}
	content := &struct {
		Media *struct {
			ID string `json:"id"`
		} `json:"media,omitempty"`
		MultiImage *struct {
			Images []struct {
				ID string `json:"id"`
			} `json:"images"`
		} `json:"multiImage,omitempty"`
	}{}
	if len(mediaIDs) == 1 {
		if !validMediaURN(mediaIDs[0]) {
			return invalidArgument("publish", "media ID must be an image, video, or document URN")
		}
		content.Media = &struct {
			ID string `json:"id"`
		}{ID: mediaIDs[0]}
	} else {
		content.MultiImage = &struct {
			Images []struct {
				ID string `json:"id"`
			} `json:"images"`
		}{}
		for _, mediaID := range mediaIDs {
			if !strings.HasPrefix(mediaID, "urn:li:image:") || !validURN(mediaID) {
				return invalidArgument("publish", "multi-media organic posts require image URNs")
			}
			content.MultiImage.Images = append(content.MultiImage.Images, struct {
				ID string `json:"id"`
			}{ID: mediaID})
		}
	}
	payload.Content = content
	return nil
}

func validMediaURN(value string) bool {
	return validURN(value) && (strings.HasPrefix(value, "urn:li:image:") || strings.HasPrefix(value, "urn:li:video:") || strings.HasPrefix(value, "urn:li:document:"))
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return post.Status, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if !validURN(postID) {
		return invalidArgument("delete_post", "post ID must be a LinkedIn URN")
	}
	if err := c.requireScopes("delete_post", c.socialScope(true)); err != nil {
		return err
	}
	return c.transport.JSON(ctx, http.MethodDelete, "/rest/posts/"+postID, nil, nil, nil, options...)
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if err := c.requireScopes("get_user", "openid", "profile"); err != nil {
		return nil, err
	}
	var response userInfo
	if err := c.transport.JSON(ctx, http.MethodGet, "/v2/userinfo", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Sub == "" {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	user := mapUser(c.accountID, response)
	if userID != "" && userID != "me" && userID != response.Sub && userID != user.ID {
		return nil, platformError("get_user", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return user, nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validURN(postID) {
		return nil, invalidArgument("get_post", "post ID must be a LinkedIn URN")
	}
	if err := c.requireScopes("get_post", c.socialScope(false)); err != nil {
		return nil, err
	}
	var response linkedInPost
	if err := c.transport.JSON(ctx, http.MethodGet, "/rest/posts/"+postID, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		response.ID = postID
	}
	return mapPost(c.accountID, response), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != c.authorURN {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must match the configured author URN")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "LinkedIn's author finder does not expose time-range filters")
	}
	if err := c.requireScopes("list_posts", c.socialScope(false)); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	start, err := parseCursor(input.Cursor)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"q": {"author"}, "author": {c.authorURN}, "start": {strconv.Itoa(start)}}
	if input.MaxResults > 0 {
		limit := input.MaxResults
		if limit > 100 {
			limit = 100
		}
		query.Set("count", strconv.Itoa(limit))
	}
	var response postPage
	if err := c.transport.JSON(ctx, http.MethodGet, "/rest/posts", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Elements))
	for _, post := range response.Elements {
		items = append(items, *mapPost(c.accountID, post))
	}
	next := nextCursor(response.Paging, len(items))
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func parseCursor(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	result, err := strconv.Atoi(value)
	if err != nil || result < 0 {
		return 0, invalidArgument("list", "cursor must be a non-negative integer offset")
	}
	return result, nil
}

func nextCursor(value paging, itemCount int) *string {
	hasNext := value.Total > value.Start+itemCount
	for _, link := range value.Links {
		if strings.EqualFold(link.Rel, "next") {
			hasNext = true
			break
		}
	}
	if !hasNext || itemCount == 0 {
		return nil
	}
	next := strconv.Itoa(value.Start + itemCount)
	return &next
}
