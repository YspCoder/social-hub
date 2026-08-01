package dribbble

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type dribbbleError struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Errors  []struct {
		Attribute string `json:"attribute"`
		Message   string `json:"message"`
	} `json:"errors"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response dribbbleError
	_ = json.Unmarshal(body, &response)
	code, class := classifyError(status)
	message := firstNonEmpty(response.Message, response.Error)
	if message == "" && len(response.Errors) > 0 {
		message = strings.TrimSpace(response.Errors[0].Attribute + ": " + response.Errors[0].Message)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "dribbble", Product: productName, HTTPStatus: status,
		PlatformMessage: boundedMessage(message, 512), RequestID: boundedMessage(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
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
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "dribbble", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "dribbble", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "dribbble", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds < 0 || seconds > float64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
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
