package misskey

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type misskeyErrorEnvelope struct {
	Error struct {
		Message string          `json:"message"`
		Code    string          `json:"code"`
		ID      string          `json:"id"`
		Kind    string          `json:"kind"`
		Info    json.RawMessage `json:"info"`
	} `json:"error"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response misskeyErrorEnvelope
	_ = json.Unmarshal(body, &response)
	code, class := classifyError(status, response.Error.Code, response.Error.Kind)
	err := &socialhub.Error{
		Code: code, Class: class, Platform: "misskey", Product: productName,
		Op: "http", HTTPStatus: status, PlatformCode: boundedMessage(response.Error.Code, 128),
		PlatformMessage: boundedMessage(response.Error.Message, 512),
		RequestID:       requestID(header), RetryAfter: retryAfter(header.Get("Retry-After")),
	}
	if code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = docURL + "permission/"
	}
	return err
}

func classifyError(status int, platformCode, kind string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case "CREDENTIAL_REQUIRED", "AUTHENTICATION_FAILED":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "PERMISSION_DENIED":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "ACCESS_DENIED", "ROLE_PERMISSION_DENIED", "YOUR_ACCOUNT_SUSPENDED", "YOUR_ACCOUNT_MOVED", "YOU_HAVE_BEEN_BLOCKED":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "RATE_LIMIT_EXCEEDED":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "NO_SUCH_NOTE", "NO_SUCH_USER", "NO_SUCH_FILE", "NO_SUCH_CHANNEL":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "ALREADY_REACTED", "NOT_REACTED":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "INVALID_PARAM", "INVALID_FILE_NAME", "MAX_FILE_SIZE_EXCEEDED", "UNALLOWED_FILE_TYPE",
		"CONTENT_REQUIRED", "CANNOT_REACT_TO_RENOTE", "CANNOT_RENOTE_TO_A_PURE_RENOTE",
		"CANNOT_RENOTE_DUE_TO_VISIBILITY", "CANNOT_REPLY_TO_AN_INVISIBLE_NOTE":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "INTERNAL_ERROR":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	if kind == "server" {
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	if kind == "permission" {
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
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

func retryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds < 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func requestID(header http.Header) string {
	for _, name := range []string{"X-Request-Id", "X-Correlation-Id", "X-Trace-Id"} {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return boundedMessage(value, 512)
		}
	}
	return ""
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "misskey", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: "misskey", Product: productName, Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent,
		Platform: "misskey", Product: productName, Op: operation, PlatformMessage: message,
	}
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}
