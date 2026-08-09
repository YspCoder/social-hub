package toutiao

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maxOpaqueLength = 4096
	maxTextBytes    = 32 << 10
	defaultPageSize = 10
	maxPageSize     = 20
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && parsed.RawPath == "" &&
		!strings.Contains(parsed.Path, "..")
}

func validRedirectURI(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalText(value string) bool {
	return len(value) <= maxTextBytes && utf8.ValidString(value)
}

func validScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 64 {
		return false
	}
	for _, scope := range scopes {
		if scope == "" || len(scope) > 128 {
			return false
		}
		for _, character := range scope {
			if !(unicode.IsLetter(character) || unicode.IsDigit(character) || strings.ContainsRune("._:-", character)) {
				return false
			}
		}
	}
	return true
}

func pageSize(value int) (int, error) {
	if value == 0 {
		return defaultPageSize, nil
	}
	if value < 1 || value > maxPageSize {
		return 0, invalidArgument("pagination", "max results must be between 1 and 20")
	}
	return value, nil
}

func parseCursor(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 || strconv.FormatInt(parsed, 10) != value {
		return 0, invalidArgument("pagination", "cursor must be a canonical non-negative integer returned by Toutiao")
	}
	return parsed, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func intPointer(value int) *int {
	if value == 0 {
		return nil
	}
	copy := value
	return &copy
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}
