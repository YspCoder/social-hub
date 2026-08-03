package messenger

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type graphErrorResponse struct {
	Error struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		IsTransient  bool   `json:"is_transient"`
		TraceID      string `json:"fbtrace_id"`
	} `json:"error"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response graphErrorResponse
	_ = json.Unmarshal(body, &response)
	code, class := classifyHTTPStatus(status)
	switch response.Error.Code {
	case 1, 2:
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 4, 17, 32, 613:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 10, 200:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 100:
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 190:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 551:
		code, class = socialhub.CodeConflict, socialhub.ClassUserAction
	}
	if response.Error.ErrorSubcode == 1545041 {
		code, class = socialhub.CodeConflict, socialhub.ClassUserAction
	}
	if response.Error.IsTransient {
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	platformCode := ""
	if response.Error.Code != 0 {
		platformCode = strconv.Itoa(response.Error.Code)
		if response.Error.ErrorSubcode != 0 {
			platformCode += "/" + strconv.Itoa(response.Error.ErrorSubcode)
		}
	}
	error := &socialhub.Error{
		Code: code, Class: class, Platform: "facebook", Product: productName, Op: "http", HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(response.Error.Message, 512),
		RequestID:  firstNonEmpty(response.Error.TraceID, header.Get("x-fb-trace-id"), header.Get("x-fb-request-id")),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
	if code == socialhub.CodePermissionDenied {
		error.RequiredScopes = []string{"pages_messaging"}
		error.ApprovalURL = docURL + "/overview"
	}
	return error
}

func classifyHTTPStatus(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		code = socialhub.CodeInvalidArgument
	case http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		code = socialhub.CodeNotFound
	case http.StatusConflict:
		code = socialhub.CodeConflict
	case http.StatusTooManyRequests:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
	}
	return code, class
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "facebook", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "facebook", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "facebook", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func approvalRequired(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "facebook", Product: productName,
		Op: operation, PlatformMessage: message, RequiredScopes: []string{"business_asset_user_profile_access"},
		ApprovalURL: docURL + "/identity/user-profile",
	}
}

func boundedMessage(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
