package youtube

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type googleErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Errors  []struct {
			Reason string `json:"reason"`
		} `json:"errors"`
	} `json:"error"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response googleErrorResponse
	_ = json.Unmarshal(body, &response)
	reason := response.Error.Status
	if len(response.Error.Errors) > 0 && response.Error.Errors[0].Reason != "" {
		reason = response.Error.Errors[0].Reason
	}
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch reason {
	case "quotaExceeded", "rateLimitExceeded", "userRateLimitExceeded", "dailyLimitExceeded":
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "authError", "invalidCredentials":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "forbidden", "insufficientPermissions", "commentsDisabled":
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "videoNotFound", "channelNotFound", "commentNotFound":
		code = socialhub.CodeNotFound
	case "backendError":
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		switch status {
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			code = socialhub.CodeInvalidArgument
		case http.StatusUnauthorized:
			code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
		case http.StatusForbidden:
			code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
		case http.StatusNotFound:
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
	}
	platformCode := reason
	if platformCode == "" && response.Error.Code != 0 {
		platformCode = strconv.Itoa(response.Error.Code)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "youtube", Product: "youtube-data", HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(response.Error.Message, 512),
		RequestID: firstNonEmpty(header.Get("x-guploader-uploadid"), header.Get("x-request-id")), RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "youtube", Product: "youtube-data", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "youtube", Product: "youtube-data", Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "youtube", Product: "youtube-data", Op: operation, PlatformMessage: message}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
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
