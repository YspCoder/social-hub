package xiaohongshu

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response struct {
		Code         int    `json:"code"`
		ErrorCode    int    `json:"error_code"`
		Message      string `json:"message"`
		ErrorMessage string `json:"error_msg"`
	}
	_ = json.Unmarshal(body, &response)
	code := response.Code
	if code == 0 {
		code = response.ErrorCode
	}
	return platformError("http", code, firstNonEmpty(response.Message, response.ErrorMessage), status, header)
}

func platformError(operation string, platformCode int, message string, status int, header http.Header) error {
	code, class := classifyError(status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "xiaohongshu", Product: "share-js", Op: operation,
		HTTPStatus: status, PlatformCode: strconv.Itoa(platformCode), PlatformMessage: message,
		RequestID: firstNonEmpty(header.Get("X-Request-ID"), header.Get("Trace-Id")), RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func approvalError(operation string) error {
	return &socialhub.Error{Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "xiaohongshu", Product: "share-js", Op: operation, ApprovalURL: "https://agora.xiaohongshu.com/", PlatformMessage: "Share Open Platform onboarding is paused; configure approved=true only for an existing approved application"}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func wrapError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "xiaohongshu", Product: "share-js", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "xiaohongshu", Product: "share-js", Op: operation, PlatformMessage: message}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
