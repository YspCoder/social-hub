package appleads

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

// ErrorItem contains Apple Ads' structured business error details.
type ErrorItem struct {
	MessageCode string `json:"messageCode"`
	Message     string `json:"message"`
	Field       string `json:"field"`
}

type errorBody struct {
	Errors []ErrorItem `json:"errors"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// APIError augments socialhub.Error with Apple Ads response details.
type APIError struct {
	Hub    *socialhub.Error
	Errors []ErrorItem
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: appleads: platform_error"
	}
	return err.Hub.Error()
}

func (err *APIError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Hub
}

func (err *APIError) Retryable() bool {
	return err != nil && err.Hub != nil && err.Hub.Retryable()
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope errorEnvelope
	_ = json.Unmarshal(body, &envelope)
	return newAPIError(status, header, envelope.Error.Errors)
}

func newAPIError(status int, header http.Header, items []ErrorItem) error {
	code, class := classifyError(status)
	sanitized := sanitizeErrorItems(items)
	var platformCode, message string
	if len(sanitized) > 0 {
		platformCode, message = sanitized[0].MessageCode, sanitized[0].Message
	}
	return &APIError{
		Hub: &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
			PlatformCode: platformCode, PlatformMessage: message,
			RequestID:  boundedMessage(firstNonEmpty(header.Get("x-request-id"), header.Get("request-id"), header.Get("x-apple-request-uuid")), 256),
			RetryAfter: parseRetryAfter(header.Get("Retry-After")),
		},
		Errors: sanitized,
	}
}

func businessError(operation string, items []ErrorItem) error {
	err := newAPIError(http.StatusBadRequest, nil, items)
	var api *APIError
	if errors.As(err, &api) && api.Hub != nil {
		api.Hub.Op = operation
	}
	return err
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
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
		PlatformMessage: message,
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := time.Until(when)
	if delay < 0 || delay > 24*time.Hour {
		return 0
	}
	return delay
}

func sanitizeErrorItems(items []ErrorItem) []ErrorItem {
	result := make([]ErrorItem, 0, len(items))
	for _, item := range items {
		result = append(result, ErrorItem{
			MessageCode: boundedMessage(redactSensitive(item.MessageCode), 256),
			Message:     boundedMessage(redactSensitive(item.Message), 512),
			Field:       boundedMessage(item.Field, 256),
		})
	}
	return result
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
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

func redactSensitive(value string) string {
	for _, marker := range []string{"client_secret", "clientsecret", "private_key", "access_token", "accesstoken", "authorization", "bearer"} {
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
			stops := " \t\r\n,;}&\"'"
			if marker == "authorization" || marker == "bearer" || marker == "private_key" {
				stops = "\r\n,;}&"
			}
			for end < len(value) && !strings.ContainsRune(stops, rune(value[end])) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return value
}
