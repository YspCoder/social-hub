package trakt

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

const (
	maxCredentialLength = 4096
	maxIdentifierLength = 512
	maxTextLength       = 2048
	maxCommentLength    = 10000
	maxPageSize         = 100
	maxPageNumber       = 1_000_000
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validRedirectURI(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validCredential(value string) bool {
	return value != "" && len(value) <= maxCredentialLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validIdentifier(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "/\\?#") && !containsControl(value)
}

func validText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value && !containsControl(value)
}

func validComment(value string) bool {
	if value == "" || len(value) > maxCommentLength || strings.TrimSpace(value) == "" {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return false
		}
	}
	return true
}

func validUserAgent(value string) bool {
	return validText(value, 256)
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func validatePage(cursor string, maxResults int) (int, error) {
	if maxResults < 0 || maxResults > maxPageSize {
		return 0, invalidArgument("pagination", "max_results must be between 1 and 100 when set")
	}
	if cursor == "" {
		return 0, nil
	}
	page, err := strconv.Atoi(cursor)
	if err != nil || page < 1 || page > maxPageNumber {
		return 0, invalidArgument("pagination", "cursor must be a positive page number")
	}
	return page, nil
}
