package tmdb

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) requestJSON(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	body := bytes.NewReader(nil)
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(method+" "+path, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := c.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	err = c.api.Do(request, output)
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = method + " " + path
	}
	return err
}

type pageEnvelope[T any] struct {
	Page         int `json:"page"`
	Results      []T `json:"results"`
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
}

func pageFromEnvelope[T any](envelope pageEnvelope[T]) (socialhub.Page[T], error) {
	if envelope.Page < 1 || envelope.Page > maxPageNumber || envelope.TotalPages < 0 || envelope.TotalPages > maxPageNumber ||
		envelope.TotalResults < 0 || (envelope.TotalPages > 0 && envelope.Page > envelope.TotalPages) {
		return socialhub.Page[T]{}, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	result := socialhub.Page[T]{Items: envelope.Results, HasMore: envelope.Page < envelope.TotalPages}
	if result.HasMore {
		next := strconv.Itoa(envelope.Page + 1)
		result.NextCursor = &next
	}
	if envelope.Page > 1 {
		previous := strconv.Itoa(envelope.Page - 1)
		result.PrevCursor = &previous
	}
	return result, nil
}

func setPageAndLanguage(query url.Values, page int, language string) {
	query.Set("page", strconv.Itoa(page))
	if language != "" {
		query.Set("language", language)
	}
}

func (c *Client) accountQuery(operation string) (url.Values, error) {
	if err := c.requireAccount(operation); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("session_id", c.sessionID)
	return query, nil
}

func (c *Client) ratingQuery(operation string) (url.Values, error) {
	if err := c.requireRating(operation); err != nil {
		return nil, err
	}
	query := url.Values{}
	if c.sessionID != "" {
		query.Set("session_id", c.sessionID)
	} else {
		query.Set("guest_session_id", c.guestSessionID)
	}
	return query, nil
}

func validateStatus(operation string, response *StatusResponse) error {
	if response == nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	switch response.StatusCode {
	case 1, 12, 13, 40:
		return nil
	case 21:
		return &socialhub.Error{
			Code: socialhub.CodeNotFound, Class: socialhub.ClassPermanent,
			Platform: "tmdb", Product: productName, Op: operation,
			PlatformCode: "21", PlatformMessage: bounded(response.StatusMessage, 512),
		}
	default:
		platformCode := ""
		if response.StatusCode != 0 {
			platformCode = strconv.Itoa(response.StatusCode)
		}
		return &socialhub.Error{
			Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
			Platform: "tmdb", Product: productName, Op: operation,
			PlatformCode: platformCode, PlatformMessage: bounded(response.StatusMessage, 512),
		}
	}
}
