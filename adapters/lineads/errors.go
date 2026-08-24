package lineads

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type ProviderError struct {
	Reason   string `json:"reason"`
	Property string `json:"property"`
}

type ErrorEnvelope struct {
	Errors []ProviderError `json:"errors"`
}

// APIError augments socialhub.Error with LINE Ads request-quota headers.
// Provider free-text errors are deliberately not exposed.
type APIError struct {
	Hub               *socialhub.Error
	Provider          ErrorEnvelope
	RequestQuotaLimit string
	RequestQuotaUsed  string
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: line: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, _ []byte) error {
		platformMessage := http.StatusText(status)
		if platformMessage == "" {
			platformMessage = "LINE Ads request failed"
		}
		code, class := classifyHTTPError(status)
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
			PlatformCode:    "http_" + strconv.Itoa(status),
			PlatformMessage: platformMessage,
			RetryAfter:      parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		if code == socialhub.CodeApprovalRequired || code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = documentationURL
		}
		return &APIError{
			Hub:               hub,
			RequestQuotaLimit: safeHeaderValue(header.Get("X-Request-Quota-Limit"), 64),
			RequestQuotaUsed:  safeHeaderValue(header.Get("X-Request-Quota-Used"), 64),
		}
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		Cause: sanitizeCause(cause),
	}
}

func authenticationError(operation, message string, cause error, credentials ...string) error {
	var sanitized error
	if clean := sanitizeCause(cause); clean != nil {
		causeMessage := clean.Error()
		for _, credential := range credentials {
			causeMessage = redactExact(causeMessage, credential)
		}
		causeMessage = boundedMessage(redactSensitive(causeMessage), 1024)
		if causeMessage != "" {
			sanitized = errors.New(causeMessage)
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024), Cause: sanitized,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func approvalRequired(operation string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Reporting (General) or Ad Tech (General) partner entitlement is required",
		ApprovalURL:     documentationURL,
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
	}
	return err
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = safeHeaderValue(value, 128)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay < 0 || delay > 24*time.Hour {
		return 0
	}
	return delay
}

func boundedMessage(value string, maximum int) string {
	if maximum <= 0 || !utf8.ValidString(value) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func safeHeaderValue(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if !validOpaque(value, maximum) {
		return ""
	}
	return value
}

func redactSensitive(value string) string {
	for _, key := range []string{"authorization", "access key", "access_key", "secret key", "secret_key", "token"} {
		cursor := 0
		for {
			start := strings.Index(strings.ToLower(value[cursor:]), key)
			if start < 0 {
				break
			}
			start += cursor
			valueStart := start + len(key)
			for valueStart < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[valueStart])) {
				valueStart++
			}
			if valueStart == start+len(key) {
				cursor = valueStart
				continue
			}
			valueEnd := valueStart
			for valueEnd < len(value) && !strings.ContainsRune("\r\n,;}&\"' \t", rune(value[valueEnd])) {
				valueEnd++
			}
			value = value[:valueStart] + "[REDACTED]" + value[valueEnd:]
			cursor = valueStart + len("[REDACTED]")
		}
	}
	return value
}

func redactExact(value, credential string) string {
	if credential == "" {
		return value
	}
	return strings.ReplaceAll(value, credential, "[REDACTED]")
}

func sanitizeCause(err error) error {
	if err == nil {
		return nil
	}
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
