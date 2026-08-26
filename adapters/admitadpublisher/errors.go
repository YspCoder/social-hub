package admitadpublisher

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// ErrorPayload preserves Admitad's OAuth and API error envelope.
type ErrorPayload struct {
	ErrorDescription string     `json:"error_description"`
	ErrorCode        ExactValue `json:"error_code"`
	Error            string     `json:"error"`
	Message          string     `json:"message"`
	Detail           string     `json:"detail"`
}

// APIError augments socialhub.Error with Admitad's structured error payload.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorPayload
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: admitad: platform_error"
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
	return func(status int, header http.Header, body []byte) error {
		return decodeProviderError(status, header, body, clock.Now())
	}
}

func decodeProviderError(status int, header http.Header, body []byte, now time.Time) error {
	var provider ErrorPayload
	decoded := json.Unmarshal(body, &provider) == nil
	platformCode := provider.ErrorCode.String()
	code, class := classifyAdmitadError(status, platformCode, provider.Error)
	message := firstNonEmpty(provider.ErrorDescription, provider.Message, provider.Detail)
	if message == "" && !decoded {
		message = strings.TrimSpace(string(body))
	}
	if message == "" {
		message = http.StatusText(status)
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: "http",
		HTTPStatus:      status,
		PlatformCode:    boundedMessage(firstNonEmpty(platformCode, provider.Error, "http_"+strconv.Itoa(status)), 256),
		PlatformMessage: boundedMessage(redactSensitive(message), 1024),
		RequestID:       boundedMessage(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	if code == socialhub.CodeApprovalRequired || code == socialhub.CodePermissionDenied {
		hub.ApprovalURL = documentationURL
	}
	provider.ErrorDescription = boundedMessage(redactSensitive(provider.ErrorDescription), 1024)
	provider.Error = boundedMessage(redactSensitive(provider.Error), 256)
	provider.Message = boundedMessage(redactSensitive(provider.Message), 1024)
	provider.Detail = boundedMessage(redactSensitive(provider.Detail), 1024)
	return &APIError{Hub: hub, Provider: provider}
}

func classifyAdmitadError(status int, platformCode, platformErrorName string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.TrimSpace(platformCode) {
	case "0", "1", "5":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "2":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "3":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "4":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "6":
		return socialhub.CodeConflict, socialhub.ClassUserAction
	}
	switch status {
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusRequestTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	if status >= 500 {
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch strings.ToLower(strings.TrimSpace(platformErrorName)) {
	case "invalid_client", "invalid_grant", "invalid_token":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "invalid_scope", "insufficient_scope", "access_denied":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "invalid_request", "unsupported_grant_type":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "temporarily_unavailable", "server_error":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return classifyHTTPError(status)
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
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
	case http.StatusRequestTimeout:
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

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func withOperationAndScope(err error, operation, requiredScope string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
		if (hub.Code == socialhub.CodeApprovalRequired || hub.Code == socialhub.CodePermissionDenied) && requiredScope != "" {
			hub.RequiredScopes = strings.Fields(requiredScope)
			hub.ApprovalURL = documentationURL
		}
	}
	return err
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
	if delay < 0 || delay > 24*time.Hour {
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

func redactSensitive(value string) string {
	for _, key := range []string{"authorization", "access_token", "refresh_token", "client_secret", "password", "secret"} {
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
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&\"'", rune(value[valueEnd])) {
				valueEnd++
			}
			value = value[:valueStart] + "[REDACTED]" + value[valueEnd:]
			cursor = valueStart + len("[REDACTED]")
		}
	}
	return value
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
