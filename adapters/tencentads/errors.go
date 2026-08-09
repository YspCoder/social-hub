package tencentads

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiEnvelope[T any] struct {
	Code      *int64          `json:"code"`
	Message   string          `json:"message"`
	MessageCN string          `json:"message_cn"`
	Errors    json.RawMessage `json:"errors"`
	Data      *T              `json:"data"`
}

func requireEnvelope[T any](operation string, envelope apiEnvelope[T], header http.Header) (*T, error) {
	if envelope.Code == nil {
		return nil, platformContractError(operation, "Tencent Ads response omitted code")
	}
	if *envelope.Code != 0 {
		return nil, businessError(operation, *envelope.Code, firstNonEmpty(envelope.MessageCN, envelope.Message), header)
	}
	if envelope.Data == nil {
		return nil, platformContractError(operation, "Tencent Ads success response omitted data")
	}
	return envelope.Data, nil
}

func businessError(operation string, providerCode int64, message string, header http.Header) error {
	code, class := classifyBusinessError(providerCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		PlatformCode: strconv.FormatInt(providerCode, 10), PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID: boundedMessage(header.Get("X-TSA-Trace-Id"), 256), RetryAfter: businessRetryAfter(providerCode),
	}
}

func classifyBusinessError(code int64) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch code {
	case 11000, 11002, 11004, 11005, 11007, 11010, 11012, 11020,
		30100, 30101, 30102, 30108, 30109, 30110, 34106, 50003, 50004:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 11008, 11016, 11017, 11018, 11019, 12006, 30104, 30105, 37622,
		39312, 54160, 55202, 60005, 72005, 72006:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 12203, 31033:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 12000, 12001, 12004, 12009, 12200, 12201:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	default:
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func businessRetryAfter(code int64) time.Duration {
	switch code {
	case 11017, 30104:
		return time.Minute
	case 11018:
		return 24 * time.Hour
	default:
		return 0
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		Code      *int64 `json:"code"`
		Message   string `json:"message"`
		MessageCN string `json:"message_cn"`
	}
	_ = json.Unmarshal(body, &envelope)
	code, class := classifyHTTPError(status)
	platformCode := ""
	if envelope.Code != nil {
		platformCode = strconv.FormatInt(*envelope.Code, 10)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode:    boundedMessage(platformCode, 128),
		PlatformMessage: boundedMessage(redactSensitive(firstNonEmpty(envelope.MessageCN, envelope.Message)), 512),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("X-TSA-Trace-Id"), header.Get("X-Request-ID")), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
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
	return 0
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

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "refresh_token", "client_secret", "authorization_code"} {
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
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&\"'", rune(value[valueEnd])) {
				valueEnd++
			}
			value = value[:valueStart] + "[REDACTED]" + value[valueEnd:]
			cursor = valueStart + len("[REDACTED]")
		}
	}
	return value
}
