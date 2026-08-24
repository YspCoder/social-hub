package unsplash

import (
	"bytes"
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

const (
	maxErrorMessages = 32
	maxErrorRawBytes = 64 << 10
)

type ErrorEnvelope struct {
	Errors []string `json:"errors"`
}

// APIError preserves Unsplash's bounded error envelope and rate metadata.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: unsplash: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, accessKey string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		meta := responseMeta(status, header, 0, 0, accessKey)
		raw, provider, decoded := sanitizeErrorBody(body, accessKey)
		message := strings.Join(provider.Errors, "; ")
		if message == "" && !decoded {
			_ = json.Unmarshal(raw, &message)
		}
		if message == "" {
			message = http.StatusText(status)
		}
		code, class := classifyHTTPError(status, message, meta)
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: "http_" + strconv.Itoa(status),
			PlatformMessage: boundedMessage(redactSensitive(redactExact(message, accessKey)), 1024),
			RequestID:       meta.RequestID,
			RetryAfter:      parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = manageAppURL
		}
		return &APIError{Hub: hub, Provider: provider, Meta: meta, Raw: raw}
	}
}

func sanitizeErrorBody(body []byte, accessKey string) (json.RawMessage, ErrorEnvelope, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, ErrorEnvelope{}, false
	}
	var provider ErrorEnvelope
	if json.Unmarshal(trimmed, &provider) == nil && provider.Errors != nil {
		if len(provider.Errors) > maxErrorMessages {
			provider.Errors = provider.Errors[:maxErrorMessages]
		}
		for index := range provider.Errors {
			provider.Errors[index] = boundedMessage(redactSensitive(redactExact(provider.Errors[index], accessKey)), 1024)
		}
		encoded, err := json.Marshal(provider)
		if err == nil && len(encoded) <= maxErrorRawBytes {
			return append(json.RawMessage(nil), encoded...), provider, true
		}
		return json.RawMessage(`{"truncated":true}`), provider, true
	}
	message := strings.ToValidUTF8(string(trimmed), "")
	message = boundedMessage(redactSensitive(redactExact(message, accessKey)), 4096)
	encoded, err := json.Marshal(message)
	if err != nil || len(encoded) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`), ErrorEnvelope{}, false
	}
	return append(json.RawMessage(nil), encoded...), ErrorEnvelope{}, false
}

func classifyHTTPError(status int, message string, meta ResponseMeta) (socialhub.ErrorCode, socialhub.ErrorClass) {
	lower := strings.ToLower(message)
	if status == http.StatusForbidden && (strings.TrimSpace(meta.RateLimitRemaining) == "0" || strings.Contains(lower, "rate limit")) {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusGone,
		http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
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
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		Cause: sanitizeCause(cause),
	}
}

func authenticationError(operation, message string, cause error, accessKey string) error {
	if cause != nil {
		clean := sanitizeCause(cause)
		cause = errors.New(boundedMessage(redactSensitive(redactExact(clean.Error(), accessKey)), 1024))
	}
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024), Cause: cause, ApprovalURL: manageAppURL,
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

func boundedMessage(value string, maximum int) string {
	if !utf8.ValidString(value) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactExact(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
}

func redactSensitive(value string) string {
	lower := strings.ToLower(value)
	for _, marker := range []string{"access_key", "access token", "access_token", "authorization", "bearer", "client-id", "client_id", "secret"} {
		if strings.Contains(lower, marker) {
			return "Unsplash rejected the request; provider message was redacted"
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
