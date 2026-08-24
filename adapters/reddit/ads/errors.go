package ads

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type errorField struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error struct {
		Code    int          `json:"code"`
		Message string       `json:"message"`
		Fields  []errorField `json:"fields"`
	} `json:"error"`
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now())
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time) error {
	var envelope errorEnvelope
	_ = json.Unmarshal(body, &envelope)
	platformCode := ""
	if envelope.Error.Code != 0 {
		platformCode = strconv.Itoa(envelope.Error.Code)
	}
	message := envelope.Error.Message
	if len(envelope.Error.Fields) > 0 {
		message = firstNonEmpty(envelope.Error.Fields[0].Message, message)
	}
	code, class := classifyError(status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 256), PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("x-request-id"), header.Get("x-correlation-id")), 256),
		RetryAfter: retryDelay(header, now),
	}
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
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

func retryDelay(header http.Header, now time.Time) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	var retry time.Duration
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		retry = time.Duration(seconds * float64(time.Second))
	} else if parsed, err := http.ParseTime(value); err == nil {
		delay := parsed.Sub(now)
		if delay > 0 && delay <= 24*time.Hour {
			retry = delay
		}
	}
	if reset := rateLimitReset(header); reset > retry {
		retry = reset
	}
	return retry
}

func parseRateLimit(header map[string][]string) (RateLimit, bool) {
	policies := make(map[string]RateLimit)
	for _, item := range splitRateHeader(headerValue(header, "RateLimit-Policy")) {
		name, values, ok := parseRateItem(item)
		if !ok {
			continue
		}
		quota, quotaOK := parseBoundedInt(values["q"])
		window, windowOK := parseBoundedInt(values["w"])
		if !quotaOK || !windowOK || quota <= 0 || window <= 0 {
			continue
		}
		policies[name] = RateLimit{Policy: name, Quota: quota, Window: time.Duration(window) * time.Second}
	}
	var selected RateLimit
	found := false
	for _, item := range splitRateHeader(headerValue(header, "RateLimit")) {
		name, values, ok := parseRateItem(item)
		if !ok {
			continue
		}
		remaining, remainingOK := parseBoundedInt(values["r"])
		reset, resetOK := parseBoundedInt(values["t"])
		if !remainingOK || !resetOK || remaining < 0 || reset < 0 {
			continue
		}
		candidate := policies[name]
		candidate.Policy, candidate.Remaining, candidate.Reset = name, remaining, time.Duration(reset)*time.Second
		if !found || moreRestrictive(candidate, selected) {
			selected, found = candidate, true
		}
	}
	return selected, found
}

func moreRestrictive(candidate, current RateLimit) bool {
	if candidate.Quota > 0 && current.Quota > 0 {
		left := int64(candidate.Remaining) * int64(current.Quota)
		right := int64(current.Remaining) * int64(candidate.Quota)
		if left != right {
			return left < right
		}
	} else if candidate.Remaining != current.Remaining {
		return candidate.Remaining < current.Remaining
	}
	return candidate.Reset > current.Reset
}

func rateLimitReset(header map[string][]string) time.Duration {
	var longest time.Duration
	for _, item := range splitRateHeader(headerValue(header, "RateLimit")) {
		_, values, ok := parseRateItem(item)
		if !ok {
			continue
		}
		seconds, valid := parseBoundedInt(values["t"])
		if valid && seconds > 0 && time.Duration(seconds)*time.Second > longest {
			longest = time.Duration(seconds) * time.Second
		}
	}
	return longest
}

func splitRateHeader(value string) []string {
	return strings.Split(value, ",")
}

func parseRateItem(value string) (string, map[string]string, bool) {
	parts := strings.Split(value, ";")
	if len(parts) < 2 {
		return "", nil, false
	}
	name, err := strconv.Unquote(strings.TrimSpace(parts[0]))
	if err != nil || !validOpaque(name, 128) {
		return "", nil, false
	}
	values := make(map[string]string, len(parts)-1)
	for _, part := range parts[1:] {
		key, raw, found := strings.Cut(strings.TrimSpace(part), "=")
		if found {
			values[strings.ToLower(key)] = strings.TrimSpace(raw)
		}
	}
	return name, values, true
}

func parseBoundedInt(value string) (int, bool) {
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed >= 0 && parsed <= 1_000_000_000
}

func headerValue(header map[string][]string, name string) string {
	for key, values := range header {
		if strings.EqualFold(key, name) {
			return strings.Join(values, ",")
		}
	}
	return ""
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

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactSensitive(value string) string {
	for _, marker := range []string{"client_secret", "access_token", "refresh_token", "authorization", "bearer"} {
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
