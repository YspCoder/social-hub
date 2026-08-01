package imgur

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type errorPayload struct {
	Error   string `json:"error"`
	Request string `json:"request"`
	Method  string `json:"method"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope apiEnvelope
	_ = json.Unmarshal(body, &envelope)
	if envelope.Status == 0 {
		envelope.Status = status
	}
	return imgurError(status, header, envelope.Data)
}

func imgurError(status int, header http.Header, data json.RawMessage) error {
	code, class := classifyError(status)
	var payload errorPayload
	_ = json.Unmarshal(data, &payload)
	message := payload.Error
	if message == "" {
		_ = json.Unmarshal(data, &message)
	}
	requestID, retryDelay := "", time.Duration(0)
	if header != nil {
		requestID = firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID"))
		retryDelay = parseRetryAfter(header.Get("Retry-After"))
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "imgur", Product: productName, Op: firstNonEmpty(payload.Method+" "+payload.Request, "http"),
		HTTPStatus: status, PlatformMessage: boundedMessage(message, 512), RequestID: boundedMessage(requestID, 512), RetryAfter: retryDelay,
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

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "imgur", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "imgur", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "imgur", Product: productName,
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
