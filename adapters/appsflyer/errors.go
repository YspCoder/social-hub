package appsflyer

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

// newHTTPErrorDecoder uses provider text only for classification. It never
// retains the response body because validation errors can identify an app or
// device.
func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		code, class := classifyError(status, body)
		result := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformMessage: "AppsFlyer rejected the Mobile S2S event",
			RequestID:  boundedHeader(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
			RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
			result.ApprovalURL = tokenManagementURL
		}
		return result
	}
}

func classifyError(status int, body []byte) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if status == http.StatusBadRequest {
		message := normalizedProviderError(body)
		if strings.Contains(message, "authenticate") || strings.Contains(message, "authentication") ||
			message == "app_id" || (strings.Contains(message, "app_id") && strings.Contains(message, "token")) {
			return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
		}
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusRequestTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func normalizedProviderError(body []byte) string {
	const maximumProviderErrorBytes = 1024
	if len(body) > maximumProviderErrorBytes {
		body = body[:maximumProviderErrorBytes]
	}
	var message string
	if err := json.Unmarshal(body, &message); err != nil {
		message = string(body)
	}
	return strings.ToLower(strings.TrimSpace(message))
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: cause,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func authenticationError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: tokenManagementURL,
	}
}

func platformContractError(operation, message string, status int) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformMessage: message,
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

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
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

func boundedHeader(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if len(value) > maximum || strings.ContainsFunc(value, func(character rune) bool {
		return character < 0x20 || character == 0x7f
	}) {
		return ""
	}
	return value
}
