package peertube

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type problemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Error    string `json:"error"`
	Status   int    `json:"status"`
	Code     string `json:"code"`
	Docs     string `json:"docs"`
	Instance string `json:"instance"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var problem problemDetails
	_ = json.Unmarshal(body, &problem)
	platformCode := problem.Code
	if platformCode == "" && problem.Type != "" && problem.Type != "about:blank" {
		platformCode = problem.Type[strings.LastIndex(problem.Type, "/")+1:]
	}
	code, class := classifyError(status, platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "peertube", Product: productName, Op: "http",
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 128),
		PlatformMessage: boundedMessage(firstNonEmpty(problem.Detail, problem.Error, problem.Title), 512),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case "invalid_client", "invalid_grant", "invalid_token", "missing_two_factor":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "quota_reached":
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case "video_not_found", "account_not_found", "comment_not_found":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType:
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
	case http.StatusRequestTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "peertube", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "peertube", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "peertube", Product: productName, Op: operation, PlatformMessage: message}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((24*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
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
