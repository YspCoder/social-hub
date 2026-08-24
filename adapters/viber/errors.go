package viber

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type errorResponse struct {
	Status        int    `json:"status"`
	StatusMessage string `json:"status_message"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response errorResponse
	_ = json.Unmarshal(body, &response)
	if response.Status != 0 {
		return mapStatus("http", status, response.Status, response.StatusMessage)
	}
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		code = socialhub.CodeInvalidArgument
	case http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		code = socialhub.CodeNotFound
	case http.StatusConflict:
		code = socialhub.CodeConflict
	case http.StatusTooManyRequests:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "viber", Product: productName, Op: "http", HTTPStatus: status,
		RequestID: header.Get("X-Request-ID"), RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func mapStatus(operation string, httpStatus, status int, message string) error {
	if status == 0 {
		return nil
	}
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case 1, 3, 4, 13, 14:
		code = socialhub.CodeInvalidArgument
	case 2:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 5, 8:
		code = socialhub.CodeNotFound
	case 6, 7, 9, 15, 20, 21, 22, 23, 24:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 10, 11, 16, 17, 18, 19:
		code, class = socialhub.CodeConflict, socialhub.ClassUserAction
	case 12:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "viber", Product: productName, Op: operation,
		HTTPStatus: httpStatus, PlatformCode: strconv.Itoa(status), PlatformMessage: boundedMessage(message, 512),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "viber", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "viber", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "viber", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func boundedMessage(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}
