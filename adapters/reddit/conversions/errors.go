package conversions

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type errorEnvelope struct {
	Error struct {
		Code int `json:"code"`
	} `json:"error"`
}

// Error messages are deliberately discarded because CAPI schema failures can
// echo user identifiers. Numeric codes and request IDs are sufficient for
// classification and support correlation.
func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var envelope errorEnvelope
		_ = json.Unmarshal(body, &envelope)
		platformCode := ""
		if envelope.Error.Code != 0 {
			platformCode = strconv.Itoa(envelope.Error.Code)
		}
		code, class := classifyError(status)
		requestID := firstNonEmpty(header.Get("x-request-id"), header.Get("x-correlation-id"))
		if !validOptionalOpaque(requestID, 256) {
			requestID = ""
		}
		return &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
			PlatformCode: platformCode, PlatformMessage: "Reddit rejected the Conversions API request",
			RequestID: requestID, RetryAfter: retryDelay(header, clock.Now()),
		}
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
	case http.StatusNotFound:
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

func retryDelay(header http.Header, now time.Time) time.Duration {
	value := boundedHeader(header.Get("Retry-After"), 128)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
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

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: cause,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedMessage(message, 512),
	}
}

func authenticationError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: documentationURL,
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedMessage(message, 512),
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

func withRetrySafety(err error, retrySafe bool) error {
	if err == nil || retrySafe {
		return err
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) && hub.Retryable() && hub.Code != socialhub.CodeRateLimited {
		hub.Class = socialhub.ClassPermanent
		hub.PlatformMessage = "Reddit conversion submission is not safe to retry without a stable conversion_id on every event"
	}
	return err
}

func boundedMessage(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if maximum <= 0 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func boundedHeader(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len(value) > maximum || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
