package googlebusinessprofile

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const maxReviewPageSize = 50

func (client *Client) GetReview(ctx context.Context, reviewID string, options ...socialhub.CallOption) (*Review, error) {
	if !validResourceSegment(reviewID) {
		return nil, invalidArgument("review_get", "review ID must be a bounded resource ID segment")
	}
	if err := client.requireScope("review_get"); err != nil {
		return nil, err
	}
	var response Review
	if err := client.api.JSON(ctx, http.MethodGet, "/"+client.reviewResource(reviewID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateReview("review_get", &response, reviewID); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) ListReviews(ctx context.Context, input ReviewListRequest, options ...socialhub.CallOption) (ReviewPage, error) {
	pageSize, err := reviewPageSize(input.MaxResults)
	if err != nil {
		return ReviewPage{}, err
	}
	if input.Cursor != "" && !validOpaque(input.Cursor, 4096) {
		return ReviewPage{}, invalidArgument("review_list", "page token is invalid")
	}
	if err := client.requireScope("review_list"); err != nil {
		return ReviewPage{}, err
	}
	query := url.Values{"pageSize": {strconv.Itoa(pageSize)}}
	if input.Cursor != "" {
		query.Set("pageToken", input.Cursor)
	}
	var response reviewListResponse
	path := "/" + client.locationResource() + "/reviews"
	if err := client.api.JSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return ReviewPage{}, err
	}
	for index := range response.Reviews {
		if err := client.validateReview("review_list", &response.Reviews[index], ""); err != nil {
			return ReviewPage{}, err
		}
	}
	page := ReviewPage{
		Items: response.Reviews, AverageRating: response.AverageRating, TotalReviewCount: response.TotalReviewCount,
	}
	if response.NextPageToken != "" {
		if !validOpaque(response.NextPageToken, 4096) {
			return ReviewPage{}, platformError("review_list", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		page.NextCursor, page.HasMore = stringPointer(response.NextPageToken), true
	}
	return page, nil
}

func (client *Client) UpdateReviewReply(ctx context.Context, reviewID, comment string, options ...socialhub.CallOption) (*ReviewReply, error) {
	if !validResourceSegment(reviewID) || !validRequiredText(comment, 4096) {
		return nil, invalidArgument("review_reply_update", "review ID and a reply of at most 4096 bytes are required")
	}
	if err := client.requireScope("review_reply_update"); err != nil {
		return nil, err
	}
	input := ReviewReply{Comment: comment}
	var response ReviewReply
	path := "/" + client.reviewResource(reviewID) + "/reply"
	if err := client.api.JSON(ctx, http.MethodPut, path, nil, input, &response, options...); err != nil {
		return nil, err
	}
	if !validRequiredText(response.Comment, 4096) {
		return nil, platformError("review_reply_update", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) DeleteReviewReply(ctx context.Context, reviewID string, options ...socialhub.CallOption) error {
	if !validResourceSegment(reviewID) {
		return invalidArgument("review_reply_delete", "review ID must be a bounded resource ID segment")
	}
	if err := client.requireScope("review_reply_delete"); err != nil {
		return err
	}
	path := "/" + client.reviewResource(reviewID) + "/reply"
	return client.api.JSON(ctx, http.MethodDelete, path, nil, nil, nil, options...)
}

func reviewPageSize(value int) (int, error) {
	if value < 0 || value > maxReviewPageSize {
		return 0, invalidArgument("review_pagination", "max_results must be between 0 and 50")
	}
	if value == 0 {
		value = maxReviewPageSize
	}
	return value, nil
}

func validStarRating(value StarRating) bool {
	return value == StarOne || value == StarTwo || value == StarThree || value == StarFour || value == StarFive
}
