package snapchat

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type subRequestState struct {
	Status string
	Reason string
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response responseMeta
	_ = json.Unmarshal(body, &response)
	platformCode := firstNonEmpty(response.ErrorCode, response.Error)
	code, class := mapError(status, platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "snapchat", Product: "snapchat-public-profile", HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(firstNonEmpty(response.DisplayMessage, response.DebugMessage, response.ErrorDescription), 512),
		RequestID: firstNonEmpty(response.RequestID, header.Get("x-request-id")), RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func checkResponse(operation string, response responseMeta, states []subRequestState) error {
	var failed *subRequestState
	for i := range states {
		if states[i].Status != "SUCCESS" {
			failed = &states[i]
			break
		}
	}
	if response.RequestStatus == "SUCCESS" && failed == nil {
		return nil
	}
	platformCode := response.ErrorCode
	message := firstNonEmpty(response.DisplayMessage, response.DebugMessage)
	if failed != nil {
		if platformCode == "" {
			platformCode = failed.Status
		}
		message = firstNonEmpty(failed.Reason, message)
	}
	if platformCode == "" {
		platformCode = firstNonEmpty(response.RequestStatus, "MALFORMED_RESPONSE")
	}
	code, class := mapError(0, platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "snapchat", Product: "snapchat-public-profile", Op: operation,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(message, 512), RequestID: response.RequestID,
	}
}

func mapError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	upper := strings.ToUpper(platformCode)
	if upper == "AUTHORIZATION_PERMISSION_DENIED" {
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	}
	if strings.Contains(upper, "RATE_LIMIT") || status == http.StatusTooManyRequests {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	if strings.Contains(upper, "AUTHENTICATION") || status == http.StatusUnauthorized {
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	if strings.Contains(upper, "NOT_FOUND") || status == http.StatusNotFound {
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "snapchat", Product: "snapchat-public-profile", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "snapchat", Product: "snapchat-public-profile", Op: operation, PlatformMessage: message}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
