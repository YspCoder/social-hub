package outbrain

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

// APIError augments the common error with Outbrain's structured fields.
type APIError struct {
	Hub          *socialhub.Error
	MoreInfo     string
	ErrorMessage string
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: outbrain: platform_error"
	}
	return err.Hub.Error()
}

func (err *APIError) Unwrap() error {
	if err == nil || err.Hub == nil {
		return nil
	}
	return err.Hub
}

func (err *APIError) Retryable() bool { return err != nil && err.Hub != nil && err.Hub.Retryable() }

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		MoreInfo     string `json:"moreInfo"`
		ErrorMessage string `json:"errorMessage"`
	}
	_ = json.Unmarshal(body, &envelope)
	code, class := classifyError(status)
	platformCode := boundedMessage(redactSensitive(envelope.MoreInfo), 256)
	message := boundedMessage(redactSensitive(envelope.ErrorMessage), 512)
	if platformCode == "" {
		platformCode = "http_" + strconv.Itoa(status)
	}
	if message == "" {
		message = boundedMessage(redactSensitive(strings.TrimSpace(string(body))), 512)
	}
	if message == "" {
		message = "Outbrain request failed"
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: message,
		RequestID:  boundedMessage(headerValue(header, "AMPLIFY-REQUEST-ID"), 256),
		RetryAfter: parseRateLimitDelay(header),
	}
	return &APIError{Hub: hub, MoreInfo: platformCode, ErrorMessage: message}
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
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: sanitizeCause(cause)}
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

func parseRateLimitDelay(header http.Header) time.Duration {
	if value := strings.TrimSpace(headerValue(header, "rate-limit-msec-left")); value != "" {
		milliseconds, err := strconv.ParseInt(value, 10, 64)
		if err == nil && milliseconds >= 0 && milliseconds <= int64((24*time.Hour)/time.Millisecond) {
			return time.Duration(milliseconds) * time.Millisecond
		}
	}
	return parseRetryAfter(headerValue(header, "Retry-After"))
}

func headerValue(header http.Header, name string) string {
	for key, values := range header {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := time.Until(when)
	if delay < 0 || delay > 24*time.Hour {
		return 0
	}
	return delay
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func redactSensitive(value string) string {
	markers := []string{"ob-token-v1", "authorization", "password", "access_token", "accesstoken", "token"}
	for _, marker := range markers {
		for cursor := 0; cursor < len(value); {
			index := strings.Index(strings.ToLower(value[cursor:]), marker)
			if index < 0 {
				break
			}
			index += cursor
			start := index + len(marker)
			for start < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[start])) {
				start++
			}
			if start == index+len(marker) {
				cursor = start
				continue
			}
			end := start
			for end < len(value) && !strings.ContainsRune(" \t\r\n,;}&\"'", rune(value[end])) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return value
}
