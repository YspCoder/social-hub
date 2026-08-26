package wikimedia

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func publicAccountOnly(account socialhub.AccountConfig) bool {
	return account.ClientID == "" && account.AppID == "" && account.SecretRef == "" &&
		account.AccessTokenRef == "" && account.TokenStore == "" &&
		account.Webhook == (socialhub.WebhookConfig{}) && account.Approval.AccountType == "" &&
		len(account.Approval.Scopes) == 0
}

func validUserAgent(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 512 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character > 0x7e {
			return false
		}
	}
	open := strings.IndexByte(value, '(')
	close := strings.LastIndexByte(value, ')')
	if !strings.Contains(value[:max(open, 0)], "/") || open < 1 || close <= open+1 {
		return false
	}
	contact := value[open+1 : close]
	return strings.Contains(contact, "https://") || strings.Contains(contact, "http://") ||
		strings.Contains(contact, "@") || strings.Contains(contact, "User:")
}

func validAccountSettings(settings AccountSettings) bool {
	switch settings.Project {
	case ProjectWikipedia:
		return validLanguage(settings.Language)
	case ProjectCommons:
		return settings.Language == ""
	default:
		return false
	}
}

func validLanguage(value string) bool {
	if value == "" || value != strings.ToLower(value) || len(value) > 32 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	previousDash := false
	for _, character := range value {
		if character == '-' {
			if previousDash {
				return false
			}
			previousDash = true
			continue
		}
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
		previousDash = false
	}
	return true
}

func siteBaseURL(settings AccountSettings) string {
	if settings.Project == ProjectCommons {
		return "https://commons.wikimedia.org/w/rest.php"
	}
	return "https://" + settings.Language + ".wikipedia.org/w/rest.php"
}

func validBoundedText(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validTitle(value string) bool { return validBoundedText(value, 4096) }

func validProviderURL(value string) bool { return validBoundedText(value, 8192) }

func validOptionalDimension(value *int64) bool { return value == nil || *value > 0 }

func validFileTitle(value string) bool {
	return validTitle(value) && len(value) > len("File:") && strings.EqualFold(value[:len("File:")], "File:")
}

func escapedTitle(value string) string {
	return url.PathEscape(strings.ReplaceAll(value, " ", "_"))
}

func validSearch(input SearchPagesRequest) bool {
	return validBoundedText(input.Query, 4096) && (input.Limit == 0 || input.Limit >= 1 && input.Limit <= 100)
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "anonymous MediaWiki reads do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "these MediaWiki endpoints do not define field selection")
	}
	if resolved.RequestID != "" && !validBoundedText(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}
