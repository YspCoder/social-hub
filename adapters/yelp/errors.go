package yelp

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

const maxErrorRawBytes = 64 << 10

type ErrorPayload struct {
	Code        string          `json:"code"`
	Description string          `json:"description"`
	Field       string          `json:"field"`
	Instance    json.RawMessage `json:"instance"`
}

type ErrorEnvelope struct {
	Error ErrorPayload `json:"error"`
}

// APIError augments socialhub.Error with Yelp's provider envelope and
// plan-dependent rate-limit headers. Raw is sanitized before exposure.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: yelp: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, apiKey string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		meta := responseMeta(header)
		raw, provider, decoded := sanitizeErrorBody(body, apiKey)
		platformCode := provider.Error.Code
		if platformCode == "" {
			platformCode = "http_" + strconv.Itoa(status)
		}
		message := provider.Error.Description
		if message == "" && !decoded {
			message = string(bytes.TrimSpace(body))
		}
		if message == "" {
			message = http.StatusText(status)
		}
		code, class := classifyHTTPError(status, platformCode)
		retryAfter := parseRetryAfter(header.Get("Retry-After"), clock.Now())
		if retryAfter == 0 && code == socialhub.CodeRateLimited {
			retryAfter = parseResetTime(meta.RateLimitResetTime, clock.Now())
		}
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
			PlatformMessage: boundedMessage(redactSensitive(redactExact(message, apiKey)), 1024),
			RequestID:       meta.RequestID, RetryAfter: retryAfter,
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = manageAppURL
		}
		return &APIError{Hub: hub, Provider: provider, Meta: meta, Raw: raw}
	}
}

func classifyHTTPError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(strings.TrimSpace(platformCode)))
	switch normalized {
	case "UNAUTHORIZED_API_KEY", "UNAUTHORIZED_ACCESS_TOKEN", "TOKEN_INVALID", "UNAUTHENTICATED":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "AUTHORIZATION_ERROR", "FORBIDDEN":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "VALIDATION_ERROR", "INVALID_REQUEST", "AREA_TOO_LARGE", "LOCATION_NOT_FOUND", "PAYLOAD_TOO_LARGE":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "NOT_FOUND", "BUSINESS_UNAVAILABLE":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "TOO_MANY_REQUESTS_PER_SECOND", "ACCESS_LIMIT_REACHED", "RATE_LIMITED":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "INTERNAL_ERROR", "SERVICE_UNAVAILABLE":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
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

func authenticationError(operation, message string, cause error, apiKey string) error {
	if cause != nil {
		clean := sanitizeCause(cause)
		cause = errors.New(boundedMessage(redactSensitive(redactExact(clean.Error(), apiKey)), 1024))
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
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((48*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	return boundedDelay(when.Sub(now))
}

func parseResetTime(value string, now time.Time) time.Duration {
	when, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return boundedDelay(when.Sub(now))
}

func boundedDelay(delay time.Duration) time.Duration {
	if delay < 0 || delay > 48*time.Hour {
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
	for _, key := range []string{"access_token", "api_key", "authorization", "bearer", "client_secret", "password"} {
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
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&}\"'<", rune(value[valueEnd])) {
				valueEnd++
			}
			value = value[:valueStart] + "[REDACTED]" + value[valueEnd:]
			cursor = valueStart + len("[REDACTED]")
		}
	}
	return value
}

func sanitizeErrorBody(body []byte, apiKey string) (json.RawMessage, ErrorEnvelope, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, ErrorEnvelope{}, false
	}
	var provider ErrorEnvelope
	if json.Unmarshal(trimmed, &provider) == nil {
		provider.Error.Code = boundedMessage(redactSensitive(redactExact(provider.Error.Code, apiKey)), 256)
		provider.Error.Description = boundedMessage(redactSensitive(redactExact(provider.Error.Description, apiKey)), 1024)
		provider.Error.Field = boundedMessage(redactSensitive(redactExact(provider.Error.Field, apiKey)), 256)
		if instance := bytes.TrimSpace(provider.Error.Instance); len(instance) > 0 && !bytes.Equal(instance, []byte("null")) {
			provider.Error.Instance = json.RawMessage(`"[REDACTED]"`)
		}
		encoded, err := json.Marshal(provider)
		if err == nil && len(encoded) <= maxErrorRawBytes {
			return append(json.RawMessage(nil), encoded...), provider, true
		}
		return json.RawMessage(`{"truncated":true}`), provider, true
	}
	message := strings.ToValidUTF8(string(trimmed), "")
	message = boundedMessage(redactSensitive(redactExact(message, apiKey)), 4096)
	encoded, err := json.Marshal(message)
	if err != nil || len(encoded) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`), ErrorEnvelope{}, false
	}
	return append(json.RawMessage(nil), encoded...), ErrorEnvelope{}, false
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
