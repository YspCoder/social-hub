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

type graphErrorEnvelope struct {
	Error struct {
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		IsTransient  bool   `json:"is_transient"`
		TraceID      string `json:"fbtrace_id"`
	} `json:"error"`
}

// decodeHTTPError intentionally does not retain Meta's free-form error text.
// CAPI validation errors can echo customer data; numeric codes and trace IDs
// are sufficient for classification and support correlation.
func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response graphErrorEnvelope
	_ = json.Unmarshal(body, &response)
	code, class := classifyGraphError(status, response.Error.Code, response.Error.IsTransient)
	platformCode := ""
	if response.Error.Code != 0 {
		platformCode = strconv.Itoa(response.Error.Code)
		if response.Error.ErrorSubcode != 0 {
			platformCode += "/" + strconv.Itoa(response.Error.ErrorSubcode)
		}
	}
	requestID := firstHeader(header, response.Error.TraceID, "X-FB-Trace-ID")
	if !validOptionalOpaque(requestID, 256) {
		requestID = ""
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 128),
		PlatformMessage: "Meta rejected the Conversions API request",
		RequestID:       requestID,
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyGraphError(status, graphCode int, transient bool) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if transient {
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch graphCode {
	case 190:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 4, 17, 32, 613:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 10, 200, 294:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 100:
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
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 512),
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 512),
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

func firstHeader(header http.Header, fallback string, names ...string) string {
	if fallback != "" {
		return fallback
	}
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
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
	if !utf8.ValidString(value) {
		return ""
	}
	if len([]rune(value)) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactCredentialMessage(value, credential string) string {
	if credential != "" {
		value = strings.ReplaceAll(value, credential, "[REDACTED]")
	}
	for _, key := range []string{"access_token", "appsecret_proof", "authorization", "bearer", "client_secret", "password"} {
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

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
