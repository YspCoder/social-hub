package giphy

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		Meta Meta `json:"meta"`
	}
	_ = json.Unmarshal(body, &envelope)
	if envelope.Meta.Status == 0 {
		envelope.Meta.Status = status
	}
	return giphyError("http", status, envelope.Meta, header)
}

func giphyError(operation string, status int, meta Meta, header http.Header) error {
	code, class := classifyError(status)
	requestID, retryAfter := meta.ResponseID, time.Duration(0)
	if header != nil {
		requestID = firstNonEmpty(requestID, header.Get("X-Request-ID"), header.Get("X-Correlation-ID"))
		retryAfter = parseRetryAfter(header.Get("Retry-After"))
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "giphy", Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: strconv.Itoa(status), PlatformMessage: boundedMessage(meta.Message, 512),
		RequestID: boundedMessage(requestID, 512), RetryAfter: retryAfter,
	}
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusRequestURITooLong, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "giphy", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "giphy", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds <= 0 || seconds > (24*time.Hour).Seconds() {
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
