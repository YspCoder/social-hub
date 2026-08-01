package threads

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type graphErrorResponse struct {
	Error struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		TraceID      string `json:"fbtrace_id"`
		IsTransient  bool   `json:"is_transient"`
	} `json:"error"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response graphErrorResponse
	_ = json.Unmarshal(body, &response)
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
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
	switch response.Error.Code {
	case 190:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 10, 200, 299, 368:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 100:
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 4, 17, 32, 613:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 506:
		code, class = socialhub.CodeConflict, socialhub.ClassPermanent
	case 1, 2:
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	if response.Error.IsTransient {
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	platformCode := ""
	if response.Error.Code != 0 {
		platformCode = strconv.Itoa(response.Error.Code)
		if response.Error.ErrorSubcode != 0 {
			platformCode += "/" + strconv.Itoa(response.Error.ErrorSubcode)
		}
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "threads", Product: productName, HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(response.Error.Message, 512),
		RequestID: firstNonEmpty(response.Error.TraceID, header.Get("x-fb-trace-id")), RetryAfter: retryAfter(header.Get("Retry-After")),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "threads", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "threads", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "threads", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil || seconds < 0 {
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
		if value != "" {
			return value
		}
	}
	return ""
}
