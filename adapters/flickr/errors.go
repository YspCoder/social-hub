package flickr

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type errorEnvelope struct {
	Stat    string `json:"stat"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func decodeAPIError(operation string, status int, header http.Header, body []byte) error {
	var envelope errorEnvelope
	_ = json.Unmarshal(body, &envelope)
	code, class := classifyError(status, envelope.Code)
	if envelope.Code == 9 && operation == "flickr.photos.comments.addComment" {
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	err := &socialhub.Error{
		Code: code, Class: class, Platform: "flickr", Product: productName, Op: operation,
		HTTPStatus: status, PlatformMessage: boundedMessage(envelope.Message, 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), 512),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
	if envelope.Code != 0 {
		err.PlatformCode = strconv.Itoa(envelope.Code)
	}
	if code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = "https://www.flickr.com/services/apps/create/"
	}
	return err
}

func classifyError(status, platformCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case 1, 2:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 3, 9:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case 6:
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case 14, 105, 106:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 95:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 96, 97, 98, 100:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 99:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 116:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "flickr", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "flickr", Product: productName, Op: operation, PlatformMessage: message}
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
