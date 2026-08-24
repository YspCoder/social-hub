package bilibililive

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type responseEnvelope[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      T      `json:"data"`
}

func (response responseEnvelope[T]) Err(operation string, status int, header http.Header) error {
	if response.Code == 0 {
		return nil
	}
	code, class, retryAfter := classifyError(response.Code, status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "bilibili", Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: strconv.Itoa(response.Code), PlatformMessage: boundedMessage(response.Message, 512),
		RequestID:  boundedMessage(firstNonEmpty(response.RequestID, header.Get("X-Request-ID")), 256),
		RetryAfter: retryAfter,
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response responseEnvelope[json.RawMessage]
	_ = json.Unmarshal(body, &response)
	if response.Code != 0 {
		return response.Err("http", status, header)
	}
	code, class, retryAfter := classifyError(0, status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "bilibili", Product: productName, Op: "http",
		HTTPStatus: status, RequestID: boundedMessage(firstNonEmpty(response.RequestID, header.Get("X-Request-ID")), 256),
		RetryAfter: retryAfter,
	}
}

func classifyError(platformCode, status int) (socialhub.ErrorCode, socialhub.ErrorClass, time.Duration) {
	switch platformCode {
	case 4000, 4005, 4006, 4011, 4012, 4013, 6000, 6001, 6002, 6010, 7004, 7005, 7007:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent, 0
	case 4001, 4002, 4003:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, 0
	case 4007, 4008, 5005, 8002:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction, 0
	case 5004, 5011, 7009:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction, 0
	case 4010, 6011:
		return socialhub.CodeNotFound, socialhub.ClassPermanent, 0
	case 4004, 6012, 7000, 7002, 7003, 7008, 7010:
		return socialhub.CodeConflict, socialhub.ClassPermanent, 0
	case 4009, 6003:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable, 0
	case 7001:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, 10 * time.Second
	case 5000, 5001, 5002, 5003, 6013, 6014, 6015:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, 0
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent, 0
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, 0
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction, 0
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent, 0
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent, 0
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable, 0
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, 0
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent, 0
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "bilibili", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "bilibili", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unavailable(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeTemporarilyUnavailable, Class: socialhub.ClassRetryable, Platform: "bilibili", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
