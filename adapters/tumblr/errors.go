package tumblr

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type tumblrMeta struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

type tumblrAPIError struct {
	Title  string `json:"title"`
	Code   int    `json:"code"`
	Detail string `json:"detail"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope tumblrEnvelope
	_ = json.Unmarshal(body, &envelope)
	return tumblrError(status, header, envelope.Meta, envelope.Errors)
}

func tumblrError(status int, header http.Header, meta tumblrMeta, errors []tumblrAPIError) error {
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		code = socialhub.CodeInvalidArgument
	case http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
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
	platformCode, message := "", meta.Msg
	if len(errors) > 0 {
		if errors[0].Code != 0 {
			platformCode = strconv.Itoa(errors[0].Code)
		}
		message = firstNonEmpty(errors[0].Detail, errors[0].Title, message)
	}
	requestID, retryDelay := "", time.Duration(0)
	if header != nil {
		requestID = firstNonEmpty(header.Get("X-Request-Id"), header.Get("X-Tumblr-Request-Id"))
		retryDelay = retryAfter(header.Get("Retry-After"))
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "tumblr", Product: productName, HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(message, 512), RequestID: requestID, RetryAfter: retryDelay,
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "tumblr", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "tumblr", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "tumblr", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func retryAfter(value string) time.Duration {
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
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
