package taboola

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

// APIError augments the common error with Taboola's structured error fields.
type APIError struct {
	Hub                        *socialhub.Error
	OffendingField             string
	MessageCode                string
	MessageCodeEnglishTemplate string
	TemplateParameters         []string
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: taboola: platform_error"
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

type errorEnvelope struct {
	HTTPStatus                 int      `json:"http_status"`
	Message                    string   `json:"message"`
	OffendingField             string   `json:"offending_field"`
	MessageCode                string   `json:"message_code"`
	MessageCodeEnglishTemplate string   `json:"message_code_english_template"`
	TemplateParameters         []string `json:"template_parameters"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope errorEnvelope
	_ = json.Unmarshal(body, &envelope)
	code, class := classifyError(status)
	messageCode := boundedMessage(redactSensitive(envelope.MessageCode), 256)
	message := boundedMessage(redactSensitive(envelope.Message), 512)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: messageCode, PlatformMessage: message,
		RequestID:  boundedMessage(firstNonEmpty(header.Get("x-request-id"), header.Get("x-correlation-id"), header.Get("request-id")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
	parameters := make([]string, 0, len(envelope.TemplateParameters))
	for _, parameter := range envelope.TemplateParameters {
		parameters = append(parameters, boundedMessage(redactSensitive(parameter), 256))
	}
	return &APIError{
		Hub: hub, OffendingField: boundedMessage(envelope.OffendingField, 256), MessageCode: messageCode,
		MessageCodeEnglishTemplate: boundedMessage(redactSensitive(envelope.MessageCodeEnglishTemplate), 512),
		TemplateParameters:         parameters,
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

func redactSensitive(value string) string {
	for _, marker := range []string{"client_secret", "clientsecret", "access_token", "accesstoken", "authorization", "bearer"} {
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
			stops := " \t\r\n,;}&\"'"
			if marker == "authorization" || marker == "bearer" {
				stops = "\r\n,;}&"
			}
			for end < len(value) && !strings.ContainsRune(stops, rune(value[end])) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return value
}
