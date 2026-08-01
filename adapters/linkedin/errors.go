package linkedin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type errorResponse struct {
	Status           int    `json:"status"`
	ServiceErrorCode int    `json:"serviceErrorCode"`
	Code             string `json:"code"`
	Message          string `json:"message"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response errorResponse
	_ = json.Unmarshal(body, &response)
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
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
	platformCode := response.Code
	if platformCode == "" && response.ServiceErrorCode != 0 {
		platformCode = strconv.Itoa(response.ServiceErrorCode)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "linkedin", Product: "linkedin-rest", HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(response.Message, 512),
		RequestID: firstNonEmpty(header.Get("x-li-uuid"), header.Get("x-restli-id")), RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "linkedin", Product: "linkedin-rest", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "linkedin", Product: "linkedin-rest", Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "linkedin", Product: "linkedin-rest", Op: operation, PlatformMessage: message}
}

func parseRetryAfter(value string) time.Duration {
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
