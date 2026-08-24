package openverse

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

const maxErrorBodyBytes = 64 << 10

type ErrorEnvelope struct {
	Error  string   `json:"error,omitempty"`
	Detail string   `json:"detail,omitempty"`
	Fields []string `json:"fields,omitempty"`
}

// APIError preserves a sanitized Openverse error envelope. Raw is always
// valid JSON, including when the provider returns HTML, text, or an oversized
// body.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: openverse: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, bearerToken string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		sanitized := sanitizeErrorJSON(body, bearerToken)
		meta := redactResponseMeta(responseMeta(header), bearerToken)
		var provider ErrorEnvelope
		_ = json.Unmarshal(sanitized, &provider)
		provider.Error = boundedMessage(provider.Error, 256)
		provider.Detail = boundedMessage(provider.Detail, 1024)
		for index := range provider.Fields {
			provider.Fields[index] = boundedMessage(provider.Fields[index], 256)
		}
		message := firstNonEmpty(provider.Detail, provider.Error, http.StatusText(status))
		code, class := classifyHTTPError(status)
		platformCode := provider.Error
		if platformCode == "" {
			platformCode = "http_" + strconv.Itoa(status)
		}
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus:      status,
			PlatformCode:    boundedMessage(platformCode, 256),
			PlatformMessage: boundedMessage(message, 1024),
			RequestID:       boundedMessage(redactExact(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), bearerToken), 256),
			RetryAfter:      parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = authenticationURL
		}
		return &APIError{Hub: hub, Provider: provider, Meta: meta, Raw: sanitized}
	}
}

func sanitizeErrorJSON(body []byte, bearerToken string) json.RawMessage {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return errorFallbackJSON("Openverse returned an empty error body")
	}
	if len(trimmed) > maxErrorBodyBytes {
		return errorFallbackJSON("Openverse error body exceeded 64 KiB")
	}
	if !json.Valid(trimmed) {
		return errorFallbackJSON("Openverse returned a non-JSON error body")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return errorFallbackJSON("Openverse returned an invalid JSON error body")
	}
	encoded, err := json.Marshal(redactJSONValue(value, bearerToken))
	if err != nil || len(encoded) > maxErrorBodyBytes || bearerToken != "" && bytes.Contains(encoded, []byte(bearerToken)) {
		return errorFallbackJSON("Openverse error body could not be safely preserved")
	}
	return json.RawMessage(encoded)
}

func redactResponseMeta(meta ResponseMeta, bearerToken string) ResponseMeta {
	meta.RequestID = redactExact(meta.RequestID, bearerToken)
	if bearerToken == "" || len(meta.RateLimits) == 0 {
		return meta
	}
	redacted := make(map[string]RateLimit, len(meta.RateLimits))
	for scope, value := range meta.RateLimits {
		value.Limit = redactExact(value.Limit, bearerToken)
		value.Available = redactExact(value.Available, bearerToken)
		redacted[redactExact(scope, bearerToken)] = value
	}
	meta.RateLimits = redacted
	return meta
}

func errorFallbackJSON(detail string) json.RawMessage {
	encoded, _ := json.Marshal(ErrorEnvelope{Detail: detail})
	return json.RawMessage(encoded)
}

func redactJSONValue(value any, bearerToken string) any {
	switch typed := value.(type) {
	case string:
		return redactExact(typed, bearerToken)
	case []any:
		for index := range typed {
			typed[index] = redactJSONValue(typed[index], bearerToken)
		}
		return typed
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, item := range typed {
			redacted[redactExact(key, bearerToken)] = redactJSONValue(item, bearerToken)
		}
		return redacted
	default:
		return value
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

func authenticationError(operation string, cause error) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		ApprovalURL: authenticationURL, Cause: sanitizeCause(cause),
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
