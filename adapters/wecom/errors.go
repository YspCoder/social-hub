package wecom

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// APIResponse is embedded by WeCom JSON responses, including HTTP 200 errors.
type APIResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// Err converts a WeCom business error into the common error model.
func (r APIResponse) Err(operation string) error {
	if r.ErrCode == 0 {
		return nil
	}
	code, class := classifyError(0, r.ErrCode)
	err := &socialhub.Error{
		Code: code, Class: class, Platform: "wecom", Product: productName, Op: operation,
		PlatformCode: strconv.Itoa(r.ErrCode), PlatformMessage: boundedMessage(r.ErrMsg, 512),
	}
	if code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = "https://developer.work.weixin.qq.com/document/path/90664"
	}
	return err
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response APIResponse
	if json.Unmarshal(body, &response) == nil && response.ErrCode != 0 {
		err := response.Err("http").(*socialhub.Error)
		err.HTTPStatus = status
		err.RequestID = requestID(header)
		err.RetryAfter = retryAfter(header.Get("Retry-After"))
		return err
	}
	code, class := classifyError(status, 0)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "wecom", Product: productName, Op: "http",
		HTTPStatus: status, RequestID: requestID(header), RetryAfter: retryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status, platformCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case -1:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 40001, 40013, 40014, 42001:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 40003, 40004, 40058, 44001, 44002, 45002, 81013:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 48002, 50001, 50002:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 60011, 60111:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 45009, 45011:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
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

func isTokenError(code int) bool {
	return code == 40014 || code == 42001
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "wecom", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "wecom", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "wecom", Product: productName, Op: operation, PlatformMessage: message}
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func requestID(header http.Header) string {
	for _, name := range []string{"X-Request-ID", "X-Logid", "X-Correlation-ID"} {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return boundedMessage(value, 512)
		}
	}
	return ""
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}
