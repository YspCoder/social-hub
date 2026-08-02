package zalo

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
	Error   int    `json:"error"`
	Message string `json:"message"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response errorResponse
	_ = json.Unmarshal(body, &response)
	if response.Error != 0 {
		err := mapAPIError("http", response.Error, response.Message)
		if platformErr, ok := err.(*socialhub.Error); ok {
			platformErr.HTTPStatus = status
			platformErr.RequestID = firstNonEmpty(header.Get("X-Request-Id"), header.Get("X-Zalo-Request-Id"))
		}
		return err
	}
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		code = socialhub.CodeInvalidArgument
	case http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
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
		Code: code, Class: class, Platform: "zalo", Product: productName, Op: "http", HTTPStatus: status,
		PlatformMessage: boundedMessage(response.Message, 512),
		RequestID:       firstNonEmpty(header.Get("X-Request-Id"), header.Get("X-Zalo-Request-Id")),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func mapAPIError(operation string, platformCode int, message string) error {
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch platformCode {
	case -32:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case -100, -204, -205, -1340:
		code = socialhub.CodeNotFound
	case -201, -210, -233, -240, -242:
		code = socialhub.CodeInvalidArgument
	case -214:
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case -216, -220:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case -209, -212, -213, -217, -219, -221, -223, -224, -227, -230, -232, -234, -235, -244, -248, -320, -403, -1341:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case -211, -218, -237, -238, -241, -321:
		code, class = socialhub.CodeConflict, socialhub.ClassUserAction
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "zalo", Product: productName, Op: operation,
		PlatformCode: strconv.Itoa(platformCode), PlatformMessage: boundedMessage(message, 512),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "zalo", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "zalo", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "zalo", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func boundedMessage(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
