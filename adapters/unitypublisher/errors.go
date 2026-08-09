package unitypublisher

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

// UnityError is Unity's documented problem response.
type UnityError struct {
	Title     string      `json:"title,omitempty"`
	Detail    string      `json:"detail,omitempty"`
	Code      json.Number `json:"code,omitempty"`
	Type      string      `json:"type,omitempty"`
	Status    int         `json:"status,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
}

// APIError augments the common error with Unity's problem and quota metadata.
type APIError struct {
	Hub             *socialhub.Error
	Unity           UnityError
	RateLimitPolicy string
	RateLimit       string
	UnityRateLimit  string
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: unity: platform_error"
	}
	return err.Hub.Error()
}

func (err *APIError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Hub
}

func (err *APIError) Retryable() bool { return err != nil && err.Hub != nil && err.Hub.Retryable() }

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var problem UnityError
	decodeErr := json.Unmarshal(body, &problem)
	code, class := classifyError(status, problem.Code.String())
	message := firstNonEmpty(problem.Detail, problem.Title)
	if decodeErr != nil || message == "" {
		message = strings.TrimSpace(string(body))
	}
	if message == "" {
		message = http.StatusText(status)
	}
	platformCode := problem.Code.String()
	if platformCode == "" {
		platformCode = "http_" + strconv.Itoa(status)
	}
	requestID := firstNonEmpty(problem.RequestID, headerValue(header, "x-request-id"), headerValue(header, "x-correlation-id"))
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 128), PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID: boundedMessage(requestID, 256), RetryAfter: parseRetryAfter(headerValue(header, "Retry-After")),
	}
	problem.Title = boundedMessage(redactSensitive(problem.Title), 256)
	problem.Detail = boundedMessage(redactSensitive(problem.Detail), 512)
	problem.Type = boundedMessage(problem.Type, 512)
	problem.RequestID = boundedMessage(problem.RequestID, 256)
	return &APIError{
		Hub: hub, Unity: problem,
		RateLimitPolicy: boundedMessage(headerValue(header, "RateLimit-Policy"), 512),
		RateLimit:       boundedMessage(headerValue(header, "RateLimit"), 512),
		UnityRateLimit:  boundedMessage(headerValue(header, "Unity-RateLimit"), 1024),
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case "63", "64":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "65":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusPaymentRequired:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case http.StatusFailedDependency:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: sanitizeCause(cause)}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func headerValue(header http.Header, name string) string {
	for key, values := range header {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func redactSensitive(value string) string {
	markers := []string{"authorization", "secret_key", "secret", "access_token", "bearer", "token"}
	for _, marker := range markers {
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
