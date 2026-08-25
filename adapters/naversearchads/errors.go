package naversearchads

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var ErrOutcomeUnknown = errors.New("naversearchads: mutation outcome unknown")
var ErrPartialMutation = errors.New("naversearchads: partial mutation")

type apiErrorEnvelope struct {
	Code  json.RawMessage `json:"code"`
	Error struct {
		Code json.RawMessage `json:"code"`
	} `json:"error"`
}

func newHTTPErrorDecoder(clock socialhub.Clock, redactions ...string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now(), redactions)
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, redactions []string) error {
	var response apiErrorEnvelope
	_ = json.Unmarshal(body, &response)
	platformCode := firstNonEmpty(scalarCode(response.Code), scalarCode(response.Error.Code))
	code, class := classifyError(status, platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: "NAVER Search AD API request failed",
		RequestID:  safeRequestID(header, redactions),
		RetryAfter: retryDelay(header, now),
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case "429", "1016":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "503":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "1001", "3001", "3501", "3601", "3701", "3801", "3901", "4001", "4101", "4403", "4601", "4702":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "1006", "1018":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "1012", "2023", "3506", "3710", "3822", "3905", "3907", "3908", "4102":
		return socialhub.CodeConflict, socialhub.ClassPermanent
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
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: sanitizeCause(cause),
	}
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
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedText(message, 512),
	}
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
	}
	return err
}

func outcomeUnknownError(operation string, cause error) error {
	requestID := ""
	var hub *socialhub.Error
	if errors.As(cause, &hub) {
		requestID = hub.RequestID
	}
	return &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "NAVER mutation outcome is unknown; reconcile provider state before retrying",
		RequestID:       requestID, Cause: errors.Join(ErrOutcomeUnknown, sanitizeCause(cause)),
	}
}

func partialMutationError(operation string, cause error) error {
	return &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "NAVER applied only part of the Keyword batch; reconcile provider state before retrying",
		Cause:           errors.Join(ErrPartialMutation, sanitizeCause(cause)),
	}
}

func ambiguousMutationError(err error) bool {
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		return false
	}
	return hub.HTTPStatus == 0 || hub.HTTPStatus == http.StatusRequestTimeout || hub.HTTPStatus >= 500 ||
		hub.HTTPStatus >= 200 && hub.HTTPStatus < 300
}

func scalarCode(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	if strings.HasPrefix(trimmed, "\"") {
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return ""
		}
		trimmed = value
	}
	if len(trimmed) == 0 || len(trimmed) > 20 {
		return ""
	}
	for _, character := range trimmed {
		if character < '0' || character > '9' {
			return ""
		}
	}
	return trimmed
}

func retryDelay(header http.Header, now time.Time) time.Duration {
	value := header.Get("Retry-After")
	if value == "" || len(value) > 128 || !utf8.ValidString(value) {
		return 0
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return 0
		}
	}
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil && seconds <= uint64((24*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay < 0 || delay > 24*time.Hour {
		return 0
	}
	return delay
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func boundedText(value string, maximum int) string {
	if !utf8.ValidString(value) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func boundedOpaque(value string, maximum int) string {
	value = boundedText(strings.TrimSpace(value), maximum)
	if !validOpaque(value, maximum) {
		return ""
	}
	return value
}

func safeRequestID(header http.Header, redactions []string) string {
	value := strings.TrimSpace(header.Get("X-Transaction-ID"))
	if value == "" {
		return ""
	}
	for _, redaction := range redactions {
		if value == redaction {
			return ""
		}
	}
	return boundedOpaque(value, 256)
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
