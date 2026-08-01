package kakao

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// APIError is Kakao REST API's documented error envelope.
type APIError struct {
	Code           int      `json:"code"`
	Message        string   `json:"msg"`
	APIType        string   `json:"api_type"`
	RequiredScopes []string `json:"required_scopes"`
	AllowedScopes  []string `json:"allowed_scopes"`
}

func (e APIError) Err(operation string, status int) error {
	if e.Code == 0 {
		return nil
	}
	code, class := classifyError(status, e.Code)
	err := &socialhub.Error{
		Code: code, Class: class, Platform: "kakao", Product: productName, Op: operation, HTTPStatus: status,
		PlatformCode: strconv.Itoa(e.Code), PlatformMessage: boundedMessage(e.Message, 512),
		RequiredScopes: append([]string(nil), e.RequiredScopes...),
	}
	if code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = approvalURL
	}
	return err
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response APIError
	if json.Unmarshal(body, &response) == nil && response.Code != 0 {
		err := response.Err("http", status).(*socialhub.Error)
		err.RequestID = requestID(header)
		err.RetryAfter = retryAfter(header.Get("Retry-After"))
		return err
	}
	code, class := classifyError(status, 0)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "kakao", Product: productName, Op: "http", HTTPStatus: status,
		RequestID: requestID(header), RetryAfter: retryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status, platformCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case -1, -7, -603, -815, -9798:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case -2, -8, -201:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case -3, -5, -402:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case -4, -6, -12, -13, -406, -501, -502, -530:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case -9:
		return socialhub.CodeUnsupported, socialhub.ClassPermanent
	case -10, -11, -532, -533, -536:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case -401, -903, -101, -103:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "kakao", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "kakao", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "kakao", Product: productName, Op: operation, PlatformMessage: message}
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func requestID(header http.Header) string {
	for _, name := range []string{"X-Kakao-Request-Id", "X-Request-Id", "X-Correlation-Id"} {
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
