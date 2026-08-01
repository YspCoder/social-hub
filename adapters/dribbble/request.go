package dribbble

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tomnomnom/linkheader"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const textMediaType = "application/vnd.dribbble.v2.text+json"

func (client *Client) requestJSON(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	var body *bytes.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return transport.ResponseMetadata{}, platformError(method+" "+path, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	} else {
		body = bytes.NewReader(nil)
	}
	request, err := client.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	request.Header.Set("Accept", textMediaType)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	metadata, err := client.api.DoWithMetadata(request, output)
	client.recordRateLimit(metadata.Header)
	client.applyResetRetryAfter(err, metadata.Header)
	return metadata, err
}

func (client *Client) recordRateLimit(header http.Header) {
	if header == nil {
		return
	}
	limit, limitErr := strconv.ParseInt(header.Get("X-RateLimit-Limit"), 10, 64)
	remaining, remainingErr := strconv.ParseInt(header.Get("X-RateLimit-Remaining"), 10, 64)
	reset, resetErr := strconv.ParseInt(header.Get("X-RateLimit-Reset"), 10, 64)
	if limitErr != nil && remainingErr != nil && resetErr != nil {
		return
	}
	snapshot := RateLimit{ObservedAt: client.clock.Now()}
	if limitErr == nil && limit >= 0 {
		snapshot.Limit = limit
	}
	if remainingErr == nil && remaining >= 0 {
		snapshot.Remaining = remaining
	}
	if resetErr == nil && reset > 0 {
		snapshot.ResetAt = time.Unix(reset, 0).UTC()
	}
	client.rateMu.Lock()
	client.rate = snapshot
	client.rateMu.Unlock()
}

func (client *Client) applyResetRetryAfter(err error, header http.Header) {
	if err == nil || header == nil {
		return
	}
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || platformErr.RetryAfter > 0 {
		return
	}
	reset, parseErr := strconv.ParseInt(header.Get("X-RateLimit-Reset"), 10, 64)
	if parseErr != nil || reset <= 0 {
		return
	}
	duration := time.Unix(reset, 0).Sub(client.clock.Now())
	if duration > 0 {
		platformErr.RetryAfter = duration
	}
}

func pageQuery(cursor string, maximum int) (url.Values, error) {
	if maximum < 0 {
		return nil, invalidArgument("pagination", "max_results must not be negative")
	}
	query := make(url.Values)
	if cursor != "" {
		if !validID(cursor) {
			return nil, invalidArgument("pagination", "cursor must be a positive decimal page number")
		}
		query.Set("page", cursor)
	}
	if maximum > 0 {
		if maximum > 100 {
			maximum = 100
		}
		query.Set("per_page", strconv.Itoa(maximum))
	}
	return query, nil
}

func (client *Client) pageCursors(header http.Header, expectedPath string) (*string, *string) {
	var next, previous *string
	for _, link := range linkheader.Parse(header.Get("Link")) {
		parsed, err := url.Parse(link.URL)
		if err != nil || parsed.Scheme != client.baseURL.Scheme || parsed.Host != client.baseURL.Host || parsed.Path != expectedPath {
			continue
		}
		page := parsed.Query().Get("page")
		if !validID(page) {
			continue
		}
		value := page
		switch link.Rel {
		case "next":
			next = &value
		case "prev":
			previous = &value
		}
	}
	return next, previous
}

func validID(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}
