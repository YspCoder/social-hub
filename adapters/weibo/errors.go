package weibo

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

// APIError is returned by Weibo for both OAuth and Open API failures.
type APIError struct {
	Message string `json:"error"`
	Code    int    `json:"error_code"`
	Request string `json:"request"`
}

// Err maps a Weibo business failure to the common error model.
func (e APIError) Err(operation string, status int, header http.Header) error {
	if e.Code == 0 && e.Message == "" {
		return nil
	}
	code, class := classifyError(status, e.Code)
	return &socialhub.Error{
		Code:            code,
		Class:           class,
		Platform:        "weibo",
		Product:         "open-api",
		Op:              operation,
		HTTPStatus:      status,
		PlatformCode:    strconv.Itoa(e.Code),
		PlatformMessage: e.Message,
		RequestID:       firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Log-Id")),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response APIError
	_ = json.Unmarshal(body, &response)
	if response.Code == 0 && response.Message == "" {
		code, class := classifyError(status, 0)
		return &socialhub.Error{Code: code, Class: class, Platform: "weibo", Product: "open-api", Op: "http", HTTPStatus: status, RequestID: firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Log-Id")), RetryAfter: parseRetryAfter(header.Get("Retry-After"))}
	}
	return response.Err("http", status, header)
}

func classifyError(status, platformCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if platformCode >= 21301 && platformCode <= 21332 {
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	switch platformCode {
	case 10006, 10008, 10016, 20017, 20019:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 10014, 10030, 20006, 20016, 20101:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 10022, 10023, 10024, 20014:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 20003, 20112:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func parseRetryAfter(value string) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func wrapError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "weibo", Product: "open-api", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "weibo", Product: "open-api", Op: operation, PlatformMessage: message}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
