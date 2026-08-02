package dingtalk

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiError struct {
	Code               flexibleCode       `json:"code"`
	LegacyCode         flexibleCode       `json:"errcode"`
	Message            string             `json:"message"`
	LegacyMessage      string             `json:"errmsg"`
	RequestID          string             `json:"requestid"`
	RequestIDAlt       string             `json:"requestId"`
	AccessDeniedDetail accessDeniedDetail `json:"accessdenieddetail"`
}

type accessDeniedDetail struct {
	RequiredScopes []string `json:"requiredScopes"`
}

type flexibleCode string

func (code *flexibleCode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	}
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
	} else {
		text = string(data)
	}
	*code = flexibleCode(strings.TrimSpace(text))
	return nil
}

func (e apiError) effectiveCode() string {
	if e.Code != "" {
		return string(e.Code)
	}
	return string(e.LegacyCode)
}

func (e apiError) Err(operation string, status int, header http.Header) error {
	platformCode := e.effectiveCode()
	if platformCode == "" || platformCode == "0" {
		return nil
	}
	message := e.Message
	if message == "" {
		message = e.LegacyMessage
	}
	code, class := classifyError(status, platformCode)
	requestID := firstNonEmpty(e.RequestID, e.RequestIDAlt, requestID(header))
	err := &socialhub.Error{
		Code: code, Class: class, Platform: "dingtalk", Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: boundedMessage(message, 512), RequestID: boundedMessage(requestID, 512),
		RequiredScopes: boundedStrings(e.AccessDeniedDetail.RequiredScopes, 64, 256),
	}
	if header != nil {
		err.RetryAfter = retryAfter(header.Get("Retry-After"))
	}
	if code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = "https://open.dingtalk.com/document/orgapp-server/add-api-permission"
	}
	return err
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response apiError
	if json.Unmarshal(body, &response) == nil {
		if platformCode := response.effectiveCode(); platformCode != "" && platformCode != "0" {
			return response.Err("http", status, header)
		}
	}
	code, class := classifyError(status, "")
	return &socialhub.Error{
		Code: code, Class: class, Platform: "dingtalk", Product: productName, Op: "http",
		HTTPStatus: status, RequestID: requestID(header), RetryAfter: retryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.ToLower(strings.TrimSpace(platformCode))
	switch {
	case normalized == "forbidden.accessdenied.accesstokenpermissiondenied":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case normalized == "90002" || normalized == "90006" || normalized == "90018" || strings.Contains(normalized, "toofast"):
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case strings.Contains(normalized, "invalidauthentication") ||
		(strings.Contains(normalized, "accesstoken") && (strings.Contains(normalized, "invalid") || strings.Contains(normalized, "expired"))):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case strings.Contains(normalized, "permissiondenied") || strings.HasPrefix(normalized, "forbidden"):
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case strings.Contains(normalized, "notfound"):
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case strings.Contains(normalized, "invalidparameter") || strings.Contains(normalized, "missingparameter"):
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
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
	case http.StatusGatewayTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func isTokenError(err error) bool {
	var platformErr *socialhub.Error
	return errors.As(err, &platformErr) && platformErr.Code == socialhub.CodeUnauthenticated
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "dingtalk", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "dingtalk", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "dingtalk", Product: productName, Op: operation, PlatformMessage: message}
}

func requestID(header http.Header) string {
	if header == nil {
		return ""
	}
	for _, name := range []string{"x-acs-request-id", "X-Request-ID", "X-Correlation-ID"} {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return boundedMessage(value, 512)
		}
	}
	return ""
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 || seconds > int64((24*time.Hour)/time.Second) {
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

func boundedStrings(values []string, maximumItems, maximumLength int) []string {
	if len(values) > maximumItems {
		values = values[:maximumItems]
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, boundedMessage(value, maximumLength))
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
