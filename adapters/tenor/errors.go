package tenor

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

var errInvalidCategoryPath = errors.New("tenor: invalid category path")

const maxErrorRawBytes = 64 << 10

// ProviderError is the standard Google API error object used by Tenor.
type ProviderError struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Status  string            `json:"status"`
	Details []json.RawMessage `json:"details"`
}

type errorEnvelope struct {
	Error ProviderError `json:"error"`
}

// APIError augments socialhub.Error with Tenor's Google API error object. Raw
// is independently bounded, valid JSON with the configured API key redacted.
type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderError
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: tenor: platform_error"
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
		sanitizedBody := sanitizeProviderJSON(body, apiKey)
		var envelope errorEnvelope
		decoded := json.Unmarshal(sanitizedBody, &envelope) == nil
		provider := envelope.Error
		reason := firstErrorReason(provider.Details)
		platformCode := firstNonEmpty(reason, provider.Status, "http_"+strconv.Itoa(status))
		message := provider.Message
		if message == "" && !decoded {
			message = string(bytes.TrimSpace(sanitizedBody))
		}
		message = firstNonEmpty(message, http.StatusText(status), "Tenor rejected the request")
		code, class := classifyHTTPError(status, provider.Status, reason)
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
			PlatformMessage: boundedMessage(redactExact(message, apiKey), 1024),
			RequestID:       boundedMessage(firstNonEmpty(header.Get("X-Google-Request-ID"), header.Get("X-Request-ID")), 256),
			RetryAfter:      parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = quickstartURL
		}
		provider.Message = boundedMessage(provider.Message, 1024)
		provider.Status = boundedMessage(provider.Status, 256)
		return &APIError{
			Hub: hub, Provider: provider,
			Raw: sanitizedBody,
		}
	}
}

func sanitizeProviderJSON(body []byte, secret string) json.RawMessage {
	var value any
	if json.Unmarshal(body, &value) != nil {
		return boundedProviderRaw([]byte(redactExact(string(body), secret)))
	}
	encoded, err := json.Marshal(redactJSONValue(value, secret))
	if err != nil {
		return boundedProviderRaw([]byte(redactExact(string(body), secret)))
	}
	return boundedProviderRaw(encoded)
}

func boundedProviderRaw(value []byte) json.RawMessage {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return json.RawMessage("null")
	}
	if len(trimmed) <= maxErrorRawBytes && json.Valid(trimmed) {
		return append(json.RawMessage(nil), trimmed...)
	}
	if len(trimmed) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`)
	}
	encoded, _ := json.Marshal(string(bytes.ToValidUTF8(trimmed, []byte("?"))))
	if len(encoded) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`)
	}
	return encoded
}

func redactJSONValue(value any, secret string) any {
	switch typed := value.(type) {
	case string:
		return redactExact(typed, secret)
	case []any:
		for index := range typed {
			typed[index] = redactJSONValue(typed[index], secret)
		}
		return typed
	case map[string]any:
		for key := range typed {
			typed[key] = redactJSONValue(typed[key], secret)
		}
		return typed
	default:
		return value
	}
}

func firstErrorReason(details []json.RawMessage) string {
	for _, detail := range details {
		var value struct {
			Reason string `json:"reason"`
		}
		if json.Unmarshal(detail, &value) == nil && value.Reason != "" {
			return value.Reason
		}
	}
	return ""
}

func classifyHTTPError(status int, providerStatus, reason string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	providerStatus = strings.ToUpper(strings.TrimSpace(providerStatus))
	reason = strings.ToUpper(strings.TrimSpace(reason))
	if providerStatus == "RESOURCE_EXHAUSTED" || strings.Contains(reason, "RATE_LIMIT") || strings.Contains(reason, "QUOTA") {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	if providerStatus == "UNAUTHENTICATED" || strings.Contains(reason, "API_KEY_INVALID") {
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
		ApprovalURL: quickstartURL, Cause: sanitizeCause(cause),
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

func redactExact(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
