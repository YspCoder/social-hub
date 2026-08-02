package musicbrainz

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"social-hub/pkg/socialhub"
)

const (
	maxUserAgentLength = 512
	maxQueryLength     = 2048
	maxPageSize        = 100
	defaultPageSize    = 25
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && parsed.RawPath == "" &&
		!strings.Contains(parsed.Path, "..")
}

func validUserAgent(value string) bool {
	return value != "" && len(value) <= maxUserAgentLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validQuery(value string) bool {
	return value != "" && len(value) <= maxQueryLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func validMBID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
				return false
			}
		}
	}
	return true
}

func validatePage(cursor string, limit int) (int, error) {
	if limit < 0 || limit > maxPageSize {
		return 0, invalidArgument("pagination", "limit must be between 1 and 100 when set")
	}
	if cursor == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(cursor)
	maxInt := int(^uint(0) >> 1)
	if err != nil || offset < 0 || offset > maxInt-maxPageSize || strconv.Itoa(offset) != cursor {
		return 0, invalidArgument("pagination", "cursor must be a canonical nonnegative integer offset")
	}
	return offset, nil
}

func publicAccountOnly(account socialhub.AccountConfig) bool {
	return account.ClientID == "" && account.AppID == "" && account.SecretRef == "" && account.AccessTokenRef == "" &&
		account.TokenStore == "" && account.Webhook.SecretRef == "" && account.Webhook.TokenRef == "" && account.Webhook.AESKeyRef == "" &&
		account.Approval.AccountType == "" && len(account.Approval.Scopes) == 0 && len(account.Settings) == 0
}
