package vk

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiError struct {
	Code             int    `json:"error_code"`
	Subcode          int    `json:"error_subcode"`
	Message          string `json:"error_msg"`
	Text             string `json:"error_text"`
	CaptchaImage     string `json:"captcha_img"`
	ConfirmationText string `json:"confirmation_text"`
	RedirectURI      string `json:"redirect_uri"`
}

func (e *apiError) err(operation string) error {
	if e == nil || e.Code == 0 {
		return nil
	}
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch e.Code {
	case 5, 27, 28:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 6, 9, 29:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 1, 10, 32, 33, 36, 43:
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 14, 17, 24, 25:
		code, class = socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 7, 15, 20, 21, 23, 30, 42, 200, 203, 204, 210, 211, 212, 214:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 18, 19, 39, 40, 104:
		code = socialhub.CodeNotFound
	case 2, 3, 4, 8, 11, 12, 13, 16, 22, 34, 35, 100, 101, 113, 118, 121, 122, 125, 129:
		code = socialhub.CodeInvalidArgument
	}
	message := strings.TrimSpace(strings.Join(nonEmpty(e.Message, e.Text, e.ConfirmationText), ": "))
	approvalURL := ""
	if code == socialhub.CodeApprovalRequired {
		approvalURL = firstNonEmpty(e.RedirectURI, e.CaptchaImage)
	}
	platformCode := strconv.Itoa(e.Code)
	if e.Subcode != 0 {
		platformCode += "." + strconv.Itoa(e.Subcode)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "vk", Product: productName, Op: operation,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(message, 512), ApprovalURL: approvalURL,
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope apiEnvelope
	_ = json.Unmarshal(body, &envelope)
	if envelope.Error != nil {
		err := envelope.Error.err("http")
		if platformErr, ok := err.(*socialhub.Error); ok {
			platformErr.HTTPStatus = status
			platformErr.RequestID = firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID"))
		}
		return err
	}
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
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
	return &socialhub.Error{
		Code: code, Class: class, Platform: "vk", Product: productName, HTTPStatus: status,
		RequestID: firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "vk", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "vk", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "vk", Product: productName, Op: operation, PlatformMessage: message}
}

func tokenPermission(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodePermissionDenied, Class: socialhub.ClassUserAction, Platform: "vk", Product: productName, Op: operation, PlatformMessage: message}
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
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}
