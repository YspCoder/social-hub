package moloco

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type ErrorDetail struct {
	Type       string            `json:"@type"`
	ErrorLogID string            `json:"error_log_id"`
	Reason     string            `json:"reason"`
	Context    map[string]string `json:"context"`
}

type ProviderError struct {
	Code    int32         `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details"`
}

type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderError
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: moloco: platform_error"
	}
	return value.Hub.Error()
}

func (value *APIError) Unwrap() error {
	if value == nil {
		return nil
	}
	return value.Hub
}

func (value *APIError) Retryable() bool {
	return value != nil && value.Hub != nil && value.Hub.Retryable()
}

func newHTTPErrorDecoder(clock socialhub.Clock, secrets func() []string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var current []string
		if secrets != nil {
			current = secrets()
		}
		return decodeHTTPError(status, header, body, clock.Now(), current...)
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, secrets ...string) error {
	var provider ProviderError
	_ = json.Unmarshal(body, &provider)
	reason, errorLogID := "", ""
	for index := range provider.Details {
		detail := &provider.Details[index]
		detail.Type = boundedMessage(redactErrorValue(detail.Type, secrets...), 256)
		detail.ErrorLogID = boundedMessage(redactErrorValue(detail.ErrorLogID, secrets...), 256)
		detail.Reason = boundedMessage(redactErrorValue(detail.Reason, secrets...), 256)
		sanitized := make(map[string]string, len(detail.Context))
		for key, value := range detail.Context {
			boundedKey := boundedMessage(redactErrorValue(key, secrets...), 128)
			if boundedKey != "" {
				if sensitiveContextKey(key) {
					sanitized[boundedKey] = "[REDACTED]"
				} else {
					sanitized[boundedKey] = boundedMessage(redactErrorValue(value, secrets...), 512)
				}
			}
		}
		detail.Context = sanitized
		reason = firstNonEmpty(reason, detail.Reason)
		errorLogID = firstNonEmpty(errorLogID, detail.ErrorLogID)
	}
	code, class := classifyError(status, provider.Code)
	platformCode := "http_" + strconv.Itoa(status)
	if provider.Code != 0 {
		platformCode = strconv.FormatInt(int64(provider.Code), 10)
	}
	if reason != "" {
		platformCode += ":" + reason
	}
	retryAfter := parseRetryAfter(header.Get("Retry-After"), now)
	if retryAfter == 0 && (provider.Code == 8 || status == http.StatusTooManyRequests) {
		retryAfter = parseRateLimitReset(header.Get("X-Rate-Limit-Reset"), now)
	}
	provider.Message = boundedMessage(redactErrorValue(provider.Message, secrets...), 1024)
	requestID := redactErrorValue(
		firstNonEmpty(errorLogID, firstHeader(header, "X-Request-ID", "X-Correlation-ID")), secrets...,
	)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: firstNonEmpty(provider.Message, http.StatusText(status)),
		RequestID:       boundedMessage(requestID, 256),
		RetryAfter:      retryAfter,
	}
	if code == socialhub.CodePermissionDenied || code == socialhub.CodeUnauthenticated {
		hub.ApprovalURL = gettingStartedURL
	}
	return &APIError{Hub: hub, Provider: provider}
}

func classifyError(status int, providerCode int32) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch providerCode {
	case 1, 2, 4, 13, 14:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 3, 11:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 5:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 6:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case 7:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 8:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 9:
		return socialhub.CodeConflict, socialhub.ClassUserAction
	case 10:
		return socialhub.CodeConflict, socialhub.ClassRetryable
	case 16:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case http.StatusRequestTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: sanitizeCause(cause),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func platformContractError(operation, message string, statuses ...int) error {
	result := &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
	if len(statuses) != 0 {
		result.HTTPStatus = statuses[0]
	}
	return result
}

func withHTTPStatus(err error, status int) error {
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.HTTPStatus = status
	}
	return err
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((24*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) || when.Sub(now) > 24*time.Hour {
		return 0
	}
	return when.Sub(now)
}

func parseRateLimitReset(value string, now time.Time) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	delay := time.Unix(seconds, 0).Sub(now)
	if delay <= 0 || delay > 24*time.Hour {
		return 0
	}
	return delay
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactErrorValue(value string, secrets ...string) string {
	ordered := append([]string(nil), secrets...)
	sort.SliceStable(ordered, func(left, right int) bool {
		return len(ordered[left]) > len(ordered[right])
	})
	for _, secret := range ordered {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return redactSensitive(value)
}

func redactSensitive(value string) string {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") {
		return "[REDACTED]"
	}
	for _, marker := range []string{
		"authorization", "api_key", "api key", "access_token", "bearer", "token",
		"password", "secret", "credential", "signature", "policy",
	} {
		for cursor := 0; cursor < len(value); {
			index := strings.Index(strings.ToLower(value[cursor:]), marker)
			if index < 0 {
				break
			}
			index += cursor
			start := index + len(marker)
			for start < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[start])) {
				start++
			}
			if start == index+len(marker) {
				cursor = start
				continue
			}
			end := start
			for end < len(value) && !strings.ContainsRune(" \t\r\n,;}&\"'", rune(value[end])) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return value
}

func sensitiveContextKey(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"authorization", "api_key", "api key", "access_token", "bearer", "token",
		"password", "secret", "credential", "signature", "policy", "url", "location",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
