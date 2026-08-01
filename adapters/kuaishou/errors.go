package kuaishou

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

type resultEnvelope struct {
	Result       int    `json:"result"`
	ErrorMessage string `json:"error_msg"`
}

func resultError(result int, message, operation string, status int, header http.Header) error {
	if result == 1 {
		return nil
	}
	code, class := classifyError(result, status)
	return &socialhub.Error{
		Code:            code,
		Class:           class,
		Platform:        "kuaishou",
		Product:         "openapi",
		Op:              operation,
		HTTPStatus:      status,
		PlatformCode:    strconv.Itoa(result),
		PlatformMessage: message,
		RequestID:       firstNonEmpty(header.Get("X-Request-Id"), header.Get("X-Kwai-Log-Id")),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response resultEnvelope
	_ = json.Unmarshal(body, &response)
	if response.Result != 0 && response.Result != 1 {
		return resultError(response.Result, response.ErrorMessage, "http", status, header)
	}
	code, class := classifyError(0, status)
	return &socialhub.Error{Code: code, Class: class, Platform: "kuaishou", Product: "openapi", Op: "http", HTTPStatus: status, RequestID: firstNonEmpty(header.Get("X-Request-Id"), header.Get("X-Kwai-Log-Id")), RetryAfter: parseRetryAfter(header.Get("Retry-After"))}
}

func classifyError(platformCode, status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case 100100400:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 100120001:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 100120002:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case 100100402:
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case 400002:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 100200107:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
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
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func wrapError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "kuaishou", Product: "openapi", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "kuaishou", Product: "openapi", Op: operation, PlatformMessage: message}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
