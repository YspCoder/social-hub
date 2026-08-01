package stackexchange

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type apiEnvelope[T any] struct {
	Items          []T    `json:"items"`
	HasMore        bool   `json:"has_more"`
	QuotaMax       *int   `json:"quota_max"`
	QuotaRemaining *int   `json:"quota_remaining"`
	Backoff        int    `json:"backoff"`
	ErrorID        int    `json:"error_id"`
	ErrorName      string `json:"error_name"`
	ErrorMessage   string `json:"error_message"`
}

type queryAuthenticator struct {
	key          string
	includeToken bool
}

func (authenticator queryAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	query := request.URL.Query()
	query.Set("key", authenticator.key)
	if authenticator.includeToken {
		query.Set("access_token", token.AccessToken)
	}
	request.URL.RawQuery = query.Encode()
	return nil
}

func call[T any](client *Client, ctx context.Context, methodKey, httpMethod, path string, query, form url.Values, options ...socialhub.CallOption) (apiEnvelope[T], error) {
	if err := client.checkBackoff(methodKey); err != nil {
		return apiEnvelope[T]{}, err
	}
	if query == nil {
		query = make(url.Values)
	}
	query.Set("site", client.site)
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request, err := client.api.NewRequest(ctx, httpMethod, path, query, body, options...)
	if err != nil {
		return apiEnvelope[T]{}, err
	}
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	request.Header.Set("User-Agent", client.userAgent)
	var response apiEnvelope[T]
	if err := client.api.Do(request, &response); err != nil {
		var platformErr *socialhub.Error
		if errors.As(err, &platformErr) {
			if platformErr.Op == "" {
				platformErr.Op = methodKey
			}
			if platformErr.RetryAfter > 0 {
				client.recordEnvelope(methodKey, nil, nil, int(platformErr.RetryAfter/time.Second))
			}
		}
		return apiEnvelope[T]{}, err
	}
	client.recordEnvelope(methodKey, response.QuotaMax, response.QuotaRemaining, response.Backoff)
	if response.ErrorID != 0 || response.ErrorName != "" || response.ErrorMessage != "" {
		return apiEnvelope[T]{}, wrapperError(methodKey, response.ErrorID, response.ErrorName, response.ErrorMessage, response.Backoff)
	}
	return response, nil
}

func (client *Client) checkBackoff(methodKey string) error {
	client.rateMu.Lock()
	defer client.rateMu.Unlock()
	deadline, exists := client.backoff[methodKey]
	if !exists {
		return nil
	}
	remaining := deadline.Sub(client.clock.Now())
	if remaining <= 0 {
		delete(client.backoff, methodKey)
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeRateLimited, Class: socialhub.ClassRetryable, Platform: "stackexchange", Product: productName,
		Op: methodKey, RetryAfter: remaining, PlatformMessage: "Stack Exchange method backoff is active",
	}
}

func (client *Client) recordEnvelope(methodKey string, maximum, remaining *int, backoffSeconds int) {
	if maximum == nil && remaining == nil && backoffSeconds <= 0 {
		return
	}
	now := client.clock.Now()
	client.rateMu.Lock()
	defer client.rateMu.Unlock()
	if maximum != nil {
		client.quota.Maximum = *maximum
	}
	if remaining != nil {
		client.quota.Remaining = *remaining
	}
	client.quota.Backoff = time.Duration(backoffSeconds) * time.Second
	client.quota.Method = methodKey
	client.quota.ObservedAt = now
	if backoffSeconds > 0 {
		deadline := now.Add(time.Duration(backoffSeconds) * time.Second)
		if current := client.backoff[methodKey]; deadline.After(current) {
			client.backoff[methodKey] = deadline
		}
	}
}

func pageQuery(cursor string, maximum int) (url.Values, int, error) {
	if maximum < 0 {
		return nil, 0, invalidArgument("pagination", "max_results must not be negative")
	}
	page := 1
	if cursor != "" {
		parsed, err := parsePositive(cursor)
		if err != nil || parsed > int64(^uint(0)>>1) {
			return nil, 0, invalidArgument("pagination", "cursor must be a positive decimal page number")
		}
		page = int(parsed)
	}
	query := make(url.Values)
	if cursor != "" {
		query.Set("page", strconv.Itoa(page))
	}
	if maximum > 0 {
		if maximum > 100 {
			maximum = 100
		}
		query.Set("pagesize", strconv.Itoa(maximum))
	}
	return query, page, nil
}

func pageFrom[T any](items []T, page int, hasMore bool) socialhub.Page[T] {
	result := socialhub.Page[T]{Items: items, HasMore: hasMore}
	if hasMore {
		next := strconv.Itoa(page + 1)
		result.NextCursor = &next
	}
	if page > 1 {
		previous := strconv.Itoa(page - 1)
		result.PrevCursor = &previous
	}
	return result
}

func validID(value string) bool {
	_, err := parsePositive(value)
	return err == nil
}

func parsePositive(value string) (int64, error) {
	if value == "" {
		return 0, strconv.ErrSyntax
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, strconv.ErrSyntax
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, strconv.ErrSyntax
	}
	return parsed, nil
}
