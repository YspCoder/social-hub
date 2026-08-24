package conversions

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type errorEnvelope struct {
	Code      *int64 `json:"code"`
	RequestID string `json:"request_id"`
}

// TikTok validation messages can echo an event index and the rejected field
// value, so free text never leaves this decoder.
func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var envelope errorEnvelope
		_ = json.Unmarshal(body, &envelope)
		return conversionError("", status, envelope.Code, envelope.RequestID, header, clock.Now())
	}
}

func conversionError(operation string, status int, providerCode *int64, requestID string, header http.Header, now time.Time) error {
	code, class := classifyError(status, providerCode)
	platformCode := ""
	if providerCode != nil {
		platformCode = strconv.FormatInt(*providerCode, 10)
	}
	requestID = firstNonEmpty(requestID, header.Get("x-request-id"), header.Get("x-tt-logid"))
	if !validOptionalOpaque(requestID, 256) {
		requestID = ""
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: platformCode,
		PlatformMessage: "TikTok rejected the conversion request",
		RequestID:       requestID, RetryAfter: retryDelay(header.Get("Retry-After"), now),
	}
	if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
		hub.ApprovalURL = approvalURL
	}
	return hub
}

func classifyError(status int, providerCode *int64) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if providerCode != nil {
		switch *providerCode {
		case 40001:
			return socialhub.CodePermissionDenied, socialhub.ClassUserAction
		case 40002:
			return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
		case 40100:
			return socialhub.CodeRateLimited, socialhub.ClassRetryable
		case 40104:
			return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
		}
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

func retryDelay(value string, now time.Time) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(strings.TrimSpace(value))
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
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: sanitizeCause(cause),
	}
}

func authenticationError(operation, message string, cause error, credential string) error {
	if cause != nil {
		clean := sanitizeCause(cause)
		cause = errors.New(boundedMessage(redactCredentialMessage(clean.Error(), credential), 1024))
	}
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 512), Cause: cause, ApprovalURL: approvalURL,
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

func boundedMessage(value string, maximum int) string {
	if !utf8.ValidString(value) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactCredentialMessage(value, credential string) string {
	if credential != "" {
		value = strings.ReplaceAll(value, credential, "[REDACTED]")
	}
	for _, key := range []string{"access_token", "access-token", "authorization", "bearer", "client_secret", "password"} {
		cursor := 0
		for {
			start := strings.Index(strings.ToLower(value[cursor:]), key)
			if start < 0 {
				break
			}
			start += cursor
			valueStart := start + len(key)
			for valueStart < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[valueStart])) {
				valueStart++
			}
			if valueStart == start+len(key) {
				cursor = valueStart
				continue
			}
			valueEnd := valueStart
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&}\"'<", rune(value[valueEnd])) {
				valueEnd++
			}
			value = value[:valueStart] + "[REDACTED]" + value[valueEnd:]
			cursor = valueStart + len("[REDACTED]")
		}
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

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
