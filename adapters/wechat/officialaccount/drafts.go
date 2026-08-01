package officialaccount

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

// Article is one article in a WeChat Official Account draft.
type Article struct {
	Title              string `json:"title"`
	Author             string `json:"author,omitempty"`
	Digest             string `json:"digest,omitempty"`
	Content            string `json:"content"`
	ContentSourceURL   string `json:"content_source_url,omitempty"`
	ThumbMediaID       string `json:"thumb_media_id"`
	NeedOpenComment    int    `json:"need_open_comment,omitempty"`
	OnlyFansCanComment int    `json:"only_fans_can_comment,omitempty"`
}

// PublishResult describes an asynchronous free-publish job.
type PublishResult struct {
	PublishID string `json:"publish_id"`
	ArticleID string `json:"article_id,omitempty"`
	Status    int    `json:"publish_status,omitempty"`
}

// DraftService exposes WeChat's typed multi-article draft workflow.
type DraftService struct{ client *Client }

// Add creates a draft and returns its media ID.
func (s *DraftService) Add(ctx context.Context, articles []Article, options ...socialhub.CallOption) (string, error) {
	if len(articles) == 0 {
		return "", invalidArgument("draft_add", "at least one article is required")
	}
	for _, article := range articles {
		if article.Title == "" || article.Content == "" || article.ThumbMediaID == "" {
			return "", invalidArgument("draft_add", "title, content, and thumb_media_id are required")
		}
	}
	var response struct {
		APIResponse
		MediaID string `json:"media_id"`
	}
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/cgi-bin/draft/add", nil, map[string]any{"articles": articles}, &response, options...); err != nil {
		return "", err
	}
	if err := response.APIResponse.Err("draft_add"); err != nil {
		return "", err
	}
	return response.MediaID, nil
}

// Delete removes a draft by media ID.
func (s *DraftService) Delete(ctx context.Context, mediaID string, options ...socialhub.CallOption) error {
	if mediaID == "" {
		return invalidArgument("draft_delete", "media ID is required")
	}
	var response APIResponse
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/cgi-bin/draft/delete", nil, map[string]string{"media_id": mediaID}, &response, options...); err != nil {
		return err
	}
	return response.Err("draft_delete")
}

// Publish submits a draft for publication.
func (s *DraftService) Publish(ctx context.Context, mediaID string, options ...socialhub.CallOption) (*PublishResult, error) {
	if mediaID == "" {
		return nil, invalidArgument("draft_publish", "media ID is required")
	}
	var response struct {
		APIResponse
		PublishID string `json:"publish_id"`
	}
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/cgi-bin/freepublish/submit", nil, map[string]string{"media_id": mediaID}, &response, options...); err != nil {
		return nil, err
	}
	if err := response.APIResponse.Err("draft_publish"); err != nil {
		return nil, err
	}
	return &PublishResult{PublishID: response.PublishID}, nil
}

// Status retrieves a free-publish job.
func (s *DraftService) Status(ctx context.Context, publishID string, options ...socialhub.CallOption) (*PublishResult, error) {
	if publishID == "" {
		return nil, invalidArgument("publish_status", "publish ID is required")
	}
	var response struct {
		APIResponse
		PublishID     string `json:"publish_id"`
		PublishStatus int    `json:"publish_status"`
		ArticleID     string `json:"article_id"`
	}
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/cgi-bin/freepublish/get", nil, map[string]string{"publish_id": publishID}, &response, options...); err != nil {
		return nil, err
	}
	if err := response.APIResponse.Err("publish_status"); err != nil {
		return nil, err
	}
	return &PublishResult{PublishID: response.PublishID, Status: response.PublishStatus, ArticleID: response.ArticleID}, nil
}
