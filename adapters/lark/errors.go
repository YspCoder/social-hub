package lark

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func apiResponseError(operation string, status int, header http.Header, response apiEnvelope) error {
	platformCode := 0
	if response.Code != nil {
		platformCode = *response.Code
	}
	code, class := classifyError(status, platformCode)
	err := &socialhub.Error{
		Code: code, Class: class, Platform: "lark", Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: strconv.Itoa(platformCode),
		PlatformMessage: boundedMessage(response.Msg, 512), RequestID: larkRequestID(header),
		RetryAfter: retryAfter(header.Get("Retry-After")),
	}
	if code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = docURL + "application-scope/scope-list"
	}
	return err
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope apiEnvelope
	if json.Unmarshal(body, &envelope) == nil && envelope.Code != nil {
		return apiResponseError("http", status, header, envelope)
	}
	code, class := classifyError(status, 0)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "lark", Product: productName, Op: "http",
		HTTPStatus: status, RequestID: larkRequestID(header), RetryAfter: retryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status, platformCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case 99991400, 99991401, 230020, 11232, 11233:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 99991663, 99991664, 99991668, 99991671, 20005:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 99991672:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 230002, 230006, 230013, 41050, 99991679:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 230011, 230040, 232006, 20008:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 230001, 234001, 234006, 234010, 234011, 234039, 20001:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 234096, 20050:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
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

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "lark", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "lark", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "lark", Product: productName, Op: operation, PlatformMessage: message}
}

func larkRequestID(header http.Header) string {
	for _, name := range []string{"X-Tt-Logid", "X-Request-ID", "X-Tt-Trace-Id"} {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}
