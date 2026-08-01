package spotify

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type spotifyErrorEnvelope struct {
	Error struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Reason  string `json:"reason"`
	} `json:"error"`
	Reason string `json:"reason"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response spotifyErrorEnvelope
	_ = json.Unmarshal(body, &response)
	code, class := classifyError(status)
	reason := firstNonEmpty(response.Reason, response.Error.Reason)
	if reason == "" {
		reason = strconv.Itoa(status)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "spotify", Product: productName, Op: "http",
		HTTPStatus: status, PlatformCode: boundedMessage(reason, 128), PlatformMessage: boundedMessage(response.Error.Message, 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("X-Request-Id"), header.Get("Spotify-Request-Id")), 512),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
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

func platformErrorWithCause(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "spotify", Product: productName, Op: operation, Cause: cause}
}

func platformError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: "spotify", Product: productName, Op: operation, PlatformMessage: message,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: "spotify", Product: productName, Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds < 0 || seconds > int64((24*time.Hour)/time.Second) {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
