package pixabay

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

type ProviderError struct {
	Message string `json:"message"`
}

// APIError preserves Pixabay's normalized plain-text error. Raw is always
// independently bounded valid JSON with query credentials redacted.
type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderError
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: pixabay: platform_error"
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
		meta := responseMeta(header, apiKey)
		message, raw := sanitizeErrorBody(body, apiKey)
		code, class := classifyHTTPError(status, message)
		retryAfter := parseRetryAfter(header.Get("Retry-After"), clock.Now())
		if retryAfter == 0 && code == socialhub.CodeRateLimited {
			retryAfter = meta.RateLimitResetAfter
		}
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: "http_" + strconv.Itoa(status),
			PlatformMessage: boundedMessage(message, 1024), RequestID: meta.RequestID, RetryAfter: retryAfter,
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = documentationURL
		}
		return &APIError{
			Hub: hub, Provider: ProviderError{Message: boundedMessage(message, 1024)},
			Meta: meta, Raw: raw,
		}
	}
}

func sanitizeErrorBody(body []byte, apiKey string) (string, json.RawMessage) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		message := "Pixabay returned an empty error body"
		return message, errorFallbackJSON(message)
	}
	if len(trimmed) > maxErrorRawBytes {
		message := "Pixabay error body exceeded 64 KiB"
		return message, errorFallbackJSON(message)
	}
	message := redactErrorText(string(bytes.ToValidUTF8(trimmed, []byte("?"))), apiKey)
	encoded, err := json.Marshal(message)
	if err != nil || len(encoded) > maxErrorRawBytes || containsCredential(encoded, apiKey) {
		message = "Pixabay error body could not be safely preserved"
		return message, errorFallbackJSON(message)
	}
	return message, json.RawMessage(encoded)
}

func errorFallbackJSON(message string) json.RawMessage {
	encoded, _ := json.Marshal(ProviderError{Message: message})
	return json.RawMessage(encoded)
}

func redactErrorText(value, apiKey string) string {
	if apiKey != "" {
		value = strings.ReplaceAll(value, apiKey, "[REDACTED]")
		value = strings.ReplaceAll(value, url.QueryEscape(apiKey), "[REDACTED]")
	}
	return redactQueryKey(value)
}

func redactQueryKey(value string) string {
	for cursor := 0; cursor < len(value); {
		lower := strings.ToLower(value)
		index := strings.Index(lower[cursor:], "key=")
		if index < 0 {
			break
		}
		index += cursor
		if index > 0 && !strings.ContainsRune("?& \t\r\n", rune(value[index-1])) {
			cursor = index + len("key=")
			continue
		}
		end := index + len("key=")
		for end < len(value) && !strings.ContainsRune("& \t\r\n\"'<>)]}", rune(value[end])) {
			end++
		}
		value = value[:index+len("key=")] + "[REDACTED]" + value[end:]
		cursor = index + len("key=[REDACTED]")
	}
	return value
}

func classifyHTTPError(status int, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	lower := strings.ToLower(message)
	if status == http.StatusBadRequest && strings.Contains(lower, "api key") {
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
		if status >= 300 && status < 400 {
			return socialhub.CodeConflict, socialhub.ClassPermanent
		}
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
		ApprovalURL: documentationURL, Cause: sanitizeCause(cause),
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

func parseResetSeconds(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds < 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
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

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
