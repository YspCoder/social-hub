package marketing

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type graphErrorEnvelope struct {
	Error struct {
		Message        string `json:"message"`
		Type           string `json:"type"`
		Code           int    `json:"code"`
		ErrorSubcode   int    `json:"error_subcode"`
		ErrorUserTitle string `json:"error_user_title"`
		ErrorUserMsg   string `json:"error_user_msg"`
		IsTransient    bool   `json:"is_transient"`
		TraceID        string `json:"fbtrace_id"`
	} `json:"error"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response graphErrorEnvelope
	_ = json.Unmarshal(body, &response)
	code, class := classifyGraphError(status, response.Error.Code, response.Error.IsTransient)
	platformCode := ""
	if response.Error.Code != 0 {
		platformCode = strconv.Itoa(response.Error.Code)
		if response.Error.ErrorSubcode != 0 {
			platformCode += "/" + strconv.Itoa(response.Error.ErrorSubcode)
		}
	}
	message := firstNonEmpty(response.Error.ErrorUserMsg, response.Error.Message, response.Error.ErrorUserTitle)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 128), PlatformMessage: boundedMessage(message, 512),
		RequestID:  boundedMessage(firstNonEmpty(response.Error.TraceID, header.Get("X-FB-Trace-ID")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyGraphError(status, graphCode int, transient bool) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if transient {
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch graphCode {
	case 190:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 4, 17, 32, 613:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 10, 200, 294:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 100:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
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

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	seconds, err := strconv.ParseFloat(value, 64)
	if err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	return 0
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
