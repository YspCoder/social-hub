package marketing

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type responseMeta struct {
	RequestStatus    string `json:"request_status"`
	RequestID        string `json:"request_id"`
	ErrorCode        string `json:"error_code"`
	DisplayMessage   string `json:"display_message"`
	DebugMessage     string `json:"debug_message"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type apiError struct {
	ErrorCode      string `json:"error_code"`
	Message        string `json:"message"`
	DisplayMessage string `json:"display_message"`
	DebugMessage   string `json:"debug_message"`
}

type subRequestState struct {
	Status string
	Reason string
	Errors []apiError
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response responseMeta
	_ = json.Unmarshal(body, &response)
	platformCode := firstNonEmpty(response.ErrorCode, response.Error)
	message := firstNonEmpty(response.DisplayMessage, response.DebugMessage, response.ErrorDescription)
	code, class := classifyError(status, platformCode, message)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 256), PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID:  boundedMessage(firstNonEmpty(response.RequestID, header.Get("x-request-id")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func checkResponse(operation string, response responseMeta, states []subRequestState) error {
	var failed *subRequestState
	for index := range states {
		if !strings.EqualFold(states[index].Status, "SUCCESS") {
			failed = &states[index]
			break
		}
	}
	if strings.EqualFold(response.RequestStatus, "SUCCESS") && failed == nil {
		return nil
	}
	platformCode := firstNonEmpty(response.ErrorCode, response.Error)
	message := firstNonEmpty(response.DisplayMessage, response.DebugMessage, response.ErrorDescription)
	if failed != nil {
		platformCode = firstNonEmpty(platformCode, failed.Status)
		message = firstNonEmpty(failed.Reason, message)
		if len(failed.Errors) > 0 {
			platformCode = firstNonEmpty(failed.Errors[0].ErrorCode, platformCode)
			message = firstNonEmpty(failed.Errors[0].DisplayMessage, failed.Errors[0].Message, failed.Errors[0].DebugMessage, message)
		}
	}
	platformCode = firstNonEmpty(platformCode, response.RequestStatus, "MALFORMED_RESPONSE")
	code, class := classifyError(0, platformCode, message)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		PlatformCode: boundedMessage(platformCode, 256), PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID: boundedMessage(response.RequestID, 256),
	}
}

func classifyError(status int, platformCode, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	upper := strings.ToUpper(platformCode + " " + message)
	if strings.Contains(upper, "RATE_LIMIT") || strings.Contains(upper, "THROTTL") || strings.Contains(upper, "QUOTA") || status == http.StatusTooManyRequests {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	if strings.Contains(upper, "AUTHORIZATION_PERMISSION_DENIED") || strings.Contains(upper, "SCOPE") {
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	}
	if strings.Contains(upper, "AUTHENTICATION") || strings.Contains(upper, "INVALID_TOKEN") || status == http.StatusUnauthorized {
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	if strings.Contains(upper, "NOT_FOUND") || status == http.StatusNotFound {
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: cause}
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
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	if parsed, parseErr := http.ParseTime(strings.TrimSpace(value)); parseErr == nil {
		delay := time.Until(parsed)
		if delay > 0 && delay <= 24*time.Hour {
			return delay
		}
	}
	return 0
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactSensitive(value string) string {
	for _, marker := range []string{"access_token", "refresh_token", "client_secret", "authorization"} {
		cursor := 0
		for {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
