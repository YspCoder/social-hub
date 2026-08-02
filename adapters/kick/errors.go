package kick

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
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	Message          string `json:"message"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response errorEnvelope
	_ = json.Unmarshal(body, &response)
	code, class := classifyHTTPError(status)
	platformCode := response.Error
	if platformCode == "" {
		platformCode = strconv.Itoa(status)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "kick", Product: productName, HTTPStatus: status,
		PlatformCode:    boundedMessage(platformCode, 128),
		PlatformMessage: boundedMessage(firstNonEmpty(response.Message, response.ErrorDescription, response.Error), 512),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("X-Request-Id"), header.Get("X-Correlation-Id"), header.Get("Cf-Ray")), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "kick", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "kick", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "kick", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func approvalRequired(operation string, scopes []string, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "kick", Product: productName,
		Op: operation, RequiredScopes: append([]string(nil), scopes...), ApprovalURL: documentationURL + "/getting-started/kick-apps-setup",
		PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	if deadline, err := http.ParseTime(value); err == nil {
		delay := time.Until(deadline)
		if delay > 0 && delay <= 24*time.Hour {
			return delay
		}
	}
	return 0
}

func sanitizeTransportError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
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
