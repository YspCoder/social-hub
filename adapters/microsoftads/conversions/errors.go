package conversions

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var safeErrorCode = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

type errorEnvelope struct {
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var envelope errorEnvelope
		_ = json.Unmarshal(body, &envelope)
		platformCode := ""
		if envelope.Error != nil && safeErrorCode.MatchString(envelope.Error.Code) {
			platformCode = envelope.Error.Code
		}
		code, class := classifyError(status)
		requestID := firstNonEmpty(header.Get("x-ms-request-id"), header.Get("request-id"), header.Get("x-request-id"))
		if !validOptionalOpaque(requestID, 256) {
			requestID = ""
		}
		return &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
			PlatformCode: platformCode, PlatformMessage: "Microsoft rejected the conversion request",
			RequestID: requestID, RetryAfter: retryDelay(header.Get("Retry-After"), clock.Now()),
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

func retryDelay(value string, now time.Time) time.Duration {
	value = boundedHeader(value, 128)
	if value == "" {
		return 0
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
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
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: sanitizeCause(cause)}
}

func authenticationError(operation, message string, cause error, secrets ...string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: documentationURL,
		Cause: sanitizeCredentialCause(cause, secrets...),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedMessage(message, 512),
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
	if errors.As(err, &hub) && hub.Class == socialhub.ClassRetryable {
		hub.Class = socialhub.ClassPermanent
		hub.PlatformMessage = "Microsoft conversion submission is not safe to retry without a stable eventId on every event"
	}
	return err
}

func boundedMessage(value string, maximum int) string {
	if !utf8.ValidString(value) {
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

func sanitizeCause(err error) error {
	if err == nil {
		return nil
	}
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func sanitizeCredentialCause(cause error, secrets ...string) error {
	if cause == nil {
		return nil
	}
	message := cause.Error()
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	if !utf8.ValidString(message) || len(message) > 1024 || strings.ContainsFunc(message, unicode.IsControl) {
		message = "credential resolution failed"
	}
	return errors.New(message)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
