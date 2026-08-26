package googleplaces

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// ProviderError is Google's canonical google.rpc.Status JSON representation.
type ProviderError struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Status  string            `json:"status"`
	Details []json.RawMessage `json:"details"`
}

type ProviderErrorEnvelope struct {
	Error *ProviderError `json:"error"`
}

// APIError augments socialhub.Error with Google's sanitized provider envelope
// and dynamic quota metadata.
type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: google-places: platform_error"
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
		meta := responseMeta(header, clock.Now(), apiKey)
		sanitized := sanitizeProviderBody(body, apiKey)
		var envelope ProviderErrorEnvelope
		decoded := json.Unmarshal(sanitized, &envelope) == nil && envelope.Error != nil
		if !decoded {
			var message string
			if err := json.Unmarshal(sanitized, &message); err != nil {
				message = "Google returned an invalid error response"
			}
			envelope.Error = &ProviderError{Code: status, Message: message}
		}
		return newProviderAPIError("", status, meta, envelope, sanitized)
	}
}

func newProviderAPIError(operation string, status int, meta ResponseMeta, envelope ProviderErrorEnvelope, raw json.RawMessage) error {
	provider := envelope.Error
	if provider == nil {
		provider = &ProviderError{Code: status}
		envelope.Error = provider
	}
	provider.Message = boundedMessage(redactSensitive(provider.Message), 1024)
	provider.Status = boundedMessage(redactSensitive(provider.Status), 256)
	code, class := classifyProviderError(status, provider.Status, provider.Code)
	platformCode := firstNonEmpty(provider.Status, strconv.Itoa(provider.Code), "http_"+strconv.Itoa(status))
	message := firstNonEmpty(provider.Message, http.StatusText(status), "Google rejected the request")
	retryAfter := meta.RetryAfter
	if retryAfter == 0 {
		retryAfter = retryDelayFromDetails(provider.Details)
	}
	meta.RetryAfter = retryAfter
	return &APIError{
		Hub: &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
			HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
			PlatformMessage: boundedMessage(message, 1024), RequestID: meta.RequestID, RetryAfter: retryAfter,
		},
		Provider: envelope,
		Meta:     meta,
		Raw:      append(json.RawMessage(nil), raw...),
	}
}

func classifyProviderError(status int, providerStatus string, providerCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.ToUpper(strings.TrimSpace(providerStatus)) {
	case "INVALID_ARGUMENT", "OUT_OF_RANGE":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "UNAUTHENTICATED":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "PERMISSION_DENIED":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "NOT_FOUND":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "ALREADY_EXISTS", "FAILED_PRECONDITION":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "ABORTED":
		return socialhub.CodeConflict, socialhub.ClassRetryable
	case "RESOURCE_EXHAUSTED":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "DEADLINE_EXCEEDED", "INTERNAL", "UNAVAILABLE", "UNKNOWN", "CANCELLED":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	if providerCode >= 300 && providerCode <= 599 {
		return classifyHTTPError(providerCode)
	}
	if status >= 200 && status < 300 {
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
	return classifyHTTPError(status)
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable,
		http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict, http.StatusPreconditionFailed:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status >= 300 && status < 400 {
			return socialhub.CodeConflict, socialhub.ClassPermanent
		}
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func retryDelayFromDetails(details []json.RawMessage) time.Duration {
	for _, raw := range details {
		var detail struct {
			Type       string `json:"@type"`
			RetryDelay string `json:"retryDelay"`
		}
		if json.Unmarshal(raw, &detail) != nil || !strings.HasSuffix(detail.Type, "google.rpc.RetryInfo") {
			continue
		}
		delay, err := time.ParseDuration(detail.RetryDelay)
		if err == nil && delay >= 0 && delay <= 7*24*time.Hour {
			return delay
		}
	}
	return 0
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
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((7*24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay < 0 || delay > 7*24*time.Hour {
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

func redactSensitive(value string) string {
	for _, key := range []string{"x-goog-api-key", "access_token", "api_key", "apikey", "authorization", "client_secret", "password", "token", "key"} {
		cursor := 0
		for cursor < len(value) {
			lower := strings.ToLower(value)
			start := strings.Index(lower[cursor:], key)
			if start < 0 {
				break
			}
			start += cursor
			if start > 0 && (unicode.IsLetter(rune(value[start-1])) || unicode.IsDigit(rune(value[start-1])) || value[start-1] == '_') {
				cursor = start + len(key)
				continue
			}
			valueStart := start + len(key)
			for valueStart < len(value) && strings.ContainsRune(" \t:=\"'?&", rune(value[valueStart])) {
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

func sanitizeProviderBody(body []byte, secret string) json.RawMessage {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	if json.Valid(trimmed) {
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		decoder.UseNumber()
		var value any
		if decoder.Decode(&value) == nil {
			value = sanitizeProviderValue(value, secret)
			if encoded, err := json.Marshal(value); err == nil {
				return encoded
			}
		}
	}
	encoded, _ := json.Marshal(redactSensitive(redactExact(string(trimmed), secret)))
	return encoded
}

func sanitizeProviderValue(value any, secret string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			switch strings.ToLower(key) {
			case "x-goog-api-key", "key", "api_key", "apikey", "authorization", "access_token", "client_secret", "token", "password":
				typed[key] = "[REDACTED]"
			default:
				typed[key] = sanitizeProviderValue(child, secret)
			}
		}
		return typed
	case []any:
		for index := range typed {
			typed[index] = sanitizeProviderValue(typed[index], secret)
		}
		return typed
	case string:
		return redactExact(typed, secret)
	default:
		return value
	}
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
