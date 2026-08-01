package bilibili

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

// SubmissionRequest preserves metadata required by Bilibili archive submission.
type SubmissionRequest struct {
	UploadToken string
	CoverID     string
	Title       string
	Description string
	TID         int
	Tags        []string
	Copyright   int
	Source      string
	NoReprint   bool
}

// SubmissionWorkflow exposes Bilibili-specific archive publication.
type SubmissionWorkflow interface {
	Publish(context.Context, SubmissionRequest, ...socialhub.CallOption) (*socialhub.Post, error)
}

// SubmissionService implements the typed archive publication workflow.
type SubmissionService struct{ client *Client }

func (s *SubmissionService) Publish(ctx context.Context, input SubmissionRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := s.client.requireScope("submission_publish", "ARC_BASE"); err != nil {
		return nil, err
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.UploadToken == "" || input.Title == "" || len([]rune(input.Title)) >= 80 || input.TID <= 0 || len(input.Tags) == 0 {
		return nil, invalidArgument("submission_publish", "upload token, title shorter than 80 characters, tid, and tags are required")
	}
	if len([]rune(input.Description)) >= 250 {
		return nil, invalidArgument("submission_publish", "description must be shorter than 250 characters")
	}
	tags := make([]string, 0, len(input.Tags))
	for _, tag := range input.Tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	if len(tags) == 0 || len([]rune(strings.Join(tags, ","))) >= 200 {
		return nil, invalidArgument("submission_publish", "non-empty tags with combined length below 200 characters are required")
	}
	if input.Copyright != 1 && input.Copyright != 2 {
		return nil, invalidArgument("submission_publish", "copyright must be 1 (original) or 2 (repost)")
	}
	if input.Copyright == 2 && strings.TrimSpace(input.Source) == "" {
		return nil, invalidArgument("submission_publish", "source is required for reposted archives")
	}
	video, cover, err := s.client.submissionMedia(input.UploadToken, input.CoverID)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"title": input.Title, "tid": input.TID, "tag": strings.Join(tags, ","), "desc": input.Description,
		"copyright": input.Copyright, "no_reprint": boolInt(input.NoReprint),
	}
	if input.Source != "" {
		body["source"] = input.Source
	}
	if cover != nil {
		body["cover"] = cover.media.URL
	}
	query := url.Values{"upload_token": {video.media.ID}}
	var response responseEnvelope[struct {
		ResourceID string `json:"resource_id"`
	}]
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/arcopen/fn/archive/add-by-utoken", query, body, &response, options...); err != nil {
		return nil, err
	}
	if err := response.Err("submission_publish", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if response.Data.ResourceID == "" {
		return nil, wrapError("submission_publish", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	status := &socialhub.PublishStatus{ID: response.Data.ResourceID, State: socialhub.PublishStatePending, Message: "submitted for Bilibili review"}
	media := []socialhub.Media{{ID: video.media.ID, MIME: video.media.MIME, Type: socialhub.MediaTypeVideo, State: socialhub.MediaStateProcessing}}
	if cover != nil {
		media = append(media, *cover.media)
	}
	return &socialhub.Post{Platform: "bilibili", AccountID: s.client.accountID, ID: response.Data.ResourceID, AuthorID: stringPointer(s.client.openID), Text: &input.Title, Media: media, Status: status}, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

var _ SubmissionWorkflow = (*SubmissionService)(nil)
