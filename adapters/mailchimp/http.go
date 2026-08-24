package mailchimp

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	if ctx == nil {
		return ResponseMeta{}, nil, invalidArgument(operation, "context must not be nil")
	}
	if err := prepareCallOptions(operation, options); err != nil {
		return ResponseMeta{}, nil, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	meta := responseMeta(metadata.StatusCode, metadata.Header, client.clock, client.apiKey, client.authorization)
	if err != nil {
		return meta, nil, withOperation(err, operation)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return meta, nil, platformContractError(operation, "Mailchimp returned an empty, oversized, invalid, or non-object JSON success response")
	}
	if looksLikeProblem(trimmed) {
		decode := newHTTPErrorDecoder(client.clock, client.apiKey, client.authorization)
		return meta, nil, withOperation(decode(metadata.StatusCode, metadata.Header, trimmed), operation)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, nil, platformContractError(operation, "Mailchimp returned a non-JSON success response")
	}
	sanitized, ok := sanitizeProviderJSON(trimmed, client.apiKey, client.authorization, maxProviderObjectBytes)
	if !ok {
		return meta, nil, platformContractError(operation, "Mailchimp success JSON could not be safely retained")
	}
	if containsDisallowedPII(sanitized) {
		return meta, nil, platformContractError(operation, "Mailchimp returned fields outside the fixed non-PII projection")
	}
	if err := json.Unmarshal(sanitized, output); err != nil {
		return meta, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, sanitized, nil
}

func looksLikeProblem(body []byte) bool {
	var value struct {
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
	}
	return json.Unmarshal(body, &value) == nil && value.Status >= 400 && (value.Title != "" || value.Detail != "")
}

func responseMeta(status int, header http.Header, clock socialhub.Clock, apiKey, authorization string) ResponseMeta {
	retryAfterHeader := boundedMessage(redactText(header.Get("Retry-After"), apiKey, authorization), 128)
	return ResponseMeta{
		RequestID: boundedMessage(redactText(firstHeader(header,
			"X-Request-ID", "X-Request-Id", "X-Correlation-ID", "X-Correlation-Id"), apiKey, authorization), 512),
		RetryAfterHeader:   retryAfterHeader,
		RetryAfter:         parseRetryAfter(retryAfterHeader, clock.Now()),
		ConcurrencyLimit:   DocumentedConcurrencyLimit,
		ConcurrencyLimited: status == http.StatusTooManyRequests,
		LimitHeaders:       dynamicLimitHeaders(header, apiKey, authorization),
	}
}

func dynamicLimitHeaders(header http.Header, apiKey, authorization string) map[string]string {
	names := make([]string, 0, len(header))
	for name := range header {
		normalized := strings.ToLower(name)
		if strings.Contains(normalized, "ratelimit") || strings.Contains(normalized, "rate-limit") ||
			strings.Contains(normalized, "concurr") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	if len(names) > 64 {
		names = names[:64]
	}
	result := make(map[string]string, len(names))
	for _, name := range names {
		value := strings.Join(header.Values(name), ", ")
		result[boundedMessage(name, 256)] = boundedMessage(redactText(value, apiKey, authorization), 4096)
	}
	return result
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay <= 0 || delay > 24*time.Hour {
		return 0
	}
	return delay
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}

func containsDisallowedPII(body []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return true
	}
	return hasDisallowedPIIValue(value)
}

func hasDisallowedPIIValue(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(key))
			switch normalized {
			case "contact", "campaigndefaults", "permissionreminder", "notifyonsubscribe", "notifyonunsubscribe",
				"beameraddress", "emailaddress", "fromemail", "replyto", "replytoaddresses", "sharepassword":
				return true
			}
			if hasDisallowedPIIValue(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if hasDisallowedPIIValue(child) {
				return true
			}
		}
	}
	return false
}
