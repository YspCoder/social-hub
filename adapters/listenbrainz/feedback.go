package listenbrainz

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) SubmitFeedback(ctx context.Context, input FeedbackSubmission, options ...socialhub.CallOption) error {
	const operation = "submit_feedback"
	if (input.RecordingMBID == "" && input.RecordingMSID == "") || !validOptionalMBID(input.RecordingMBID) ||
		!validOptionalMBID(input.RecordingMSID) || !validFeedbackScore(input.Score) {
		return invalidArgument(operation, "a canonical recording MBID or MSID and score -1, 0, or 1 are required")
	}
	if err := c.requireToken(operation); err != nil {
		return err
	}
	return c.requestJSON(ctx, operation, http.MethodPost, "/1/feedback/recording-feedback", nil, input, nil, options...)
}

func (c *Client) ListFeedback(ctx context.Context, request FeedbackListRequest, options ...socialhub.CallOption) (socialhub.Page[Feedback], error) {
	const operation = "list_feedback"
	username, err := c.resolveUsername(operation, request.Username)
	if err != nil {
		return socialhub.Page[Feedback]{}, err
	}
	if request.Score != nil && (*request.Score != FeedbackHate && *request.Score != FeedbackLove) {
		return socialhub.Page[Feedback]{}, invalidArgument(operation, "score filter must be -1 or 1")
	}
	if err := validatePage(request.MaxResults, maxListenPageSize); err != nil {
		return socialhub.Page[Feedback]{}, err
	}
	offset, err := validateOffset(request.Cursor, maxListenPageSize)
	if err != nil {
		return socialhub.Page[Feedback]{}, err
	}
	query := make(url.Values)
	if request.Score != nil {
		query.Set("score", strconv.Itoa(int(*request.Score)))
	}
	if request.MaxResults > 0 {
		query.Set("count", strconv.Itoa(request.MaxResults))
	}
	if request.Cursor != "" {
		query.Set("offset", request.Cursor)
	}
	if request.Metadata {
		query.Set("metadata", "true")
	}
	var envelope struct {
		Count      int        `json:"count"`
		Feedback   []Feedback `json:"feedback"`
		Offset     int        `json:"offset"`
		TotalCount int        `json:"total_count"`
	}
	path := "/1/feedback/user/" + url.PathEscape(username) + "/get-feedback"
	if err := getOnly(ctx, c, operation, path, query, &envelope, options...); err != nil {
		return socialhub.Page[Feedback]{}, err
	}
	if envelope.Count > maxListenPageSize {
		return socialhub.Page[Feedback]{}, invalidPlatformResponse(operation, "response exceeded the feedback page limit")
	}
	return offsetPage(operation, envelope.Feedback, envelope.Count, envelope.Offset, envelope.TotalCount, offset, request.MaxResults)
}

func validFeedbackScore(score FeedbackScore) bool {
	return score == FeedbackHate || score == FeedbackRemove || score == FeedbackLove
}

func offsetPage[T any](operation string, items []T, count, offset, total, expectedOffset, requestedLimit int) (socialhub.Page[T], error) {
	if count != len(items) || count < 0 || offset != expectedOffset || total < offset+len(items) {
		return socialhub.Page[T]{}, invalidPlatformResponse(operation, "response contained invalid pagination metadata")
	}
	page := socialhub.Page[T]{Items: items}
	nextOffset := offset + len(items)
	page.HasMore = nextOffset < total
	if page.HasMore {
		next := strconv.Itoa(nextOffset)
		page.NextCursor = &next
	}
	if offset > 0 {
		limit := requestedLimit
		if limit <= 0 {
			limit = 25
		}
		previousOffset := offset - limit
		if previousOffset < 0 {
			previousOffset = 0
		}
		previous := strconv.Itoa(previousOffset)
		page.PrevCursor = &previous
	}
	return page, nil
}
