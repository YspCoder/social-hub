package officialaccount

import (
	"encoding/json"
	"net/http"
	"strconv"

	"social-hub/pkg/socialhub"
)

// APIResponse is embedded by WeChat JSON responses, including HTTP 200 errors.
type APIResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// Err converts a WeChat business error to the common error model.
func (r APIResponse) Err(operation string) error {
	if r.ErrCode == 0 {
		return nil
	}
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch r.ErrCode {
	case 40001, 40014, 42001:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 40003, 40013, 44001, 44002, 47001:
		code = socialhub.CodeInvalidArgument
	case 48001, 48002, 50001:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 45009, 45011:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case -1:
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{Code: code, Class: class, Platform: "wechat", Product: "official-account", Op: operation, PlatformCode: strconv.Itoa(r.ErrCode), PlatformMessage: r.ErrMsg}
}

func decodeHTTPError(status int, _ http.Header, body []byte) error {
	var response APIResponse
	_ = json.Unmarshal(body, &response)
	if response.ErrCode != 0 {
		return response.Err("http")
	}
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	if status == http.StatusTooManyRequests {
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	} else if status >= 500 {
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	} else if status == http.StatusUnauthorized {
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	} else if status == http.StatusForbidden {
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	}
	return wrapError("http", code, class, nil)
}

func wrapError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "wechat", Product: "official-account", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "wechat", Product: "official-account", Op: operation, PlatformMessage: message}
}
