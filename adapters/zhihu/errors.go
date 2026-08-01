package zhihu

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

type responseEnvelope[T any] struct {
	Code      int    `json:"Code"`
	Message   string `json:"Message"`
	RequestID string `json:"RequestID"`
	Data      T      `json:"Data"`
}

func (r responseEnvelope[T]) Err(operation string, status int, header http.Header) error {
	if r.Code == 0 {
		return nil
	}
	code, class := classifyError(r.Code, status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "zhihu", Product: "data-api", Op: operation,
		HTTPStatus: status, PlatformCode: strconv.Itoa(r.Code), PlatformMessage: r.Message,
		RequestID: firstNonEmpty(r.RequestID, header.Get("X-Request-ID")), RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response responseEnvelope[json.RawMessage]
	_ = json.Unmarshal(body, &response)
	if response.Code != 0 {
		return response.Err("http", status, header)
	}
	code, class := classifyError(0, status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "zhihu", Product: "data-api", Op: "http",
		HTTPStatus: status, RequestID: header.Get("X-Request-ID"), RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyError(platformCode, status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case 10001:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 20001:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 30001:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 30002:
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case 90001:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
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

func approvalError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "zhihu", Product: "data-api", Op: operation,
		ApprovalURL: approvalURL, PlatformMessage: message,
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
	return &socialhub.Error{Code: code, Class: class, Platform: "zhihu", Product: "data-api", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "zhihu", Product: "data-api", Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "zhihu", Product: "data-api", Op: operation, PlatformMessage: message}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
