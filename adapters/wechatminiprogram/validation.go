package wechatminiprogram

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maxAppIDLength           = 256
	maxSecretReferenceLength = 4_096
	maxCredentialLength      = 16_384
	maxCodeLength            = 4_096
	maxOpenIDLength          = 512
	maxRequestIDLength       = 256
	maxRequestBodyBytes      = 64 << 10
)

func validSensitive(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalSensitive(value string, maximum int) bool {
	return value == "" || validSensitive(value, maximum)
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "idempotency keys are not supported by this Mini Program operation")
	}
	if len(resolved.Fields) != 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "field selection is not supported by this Mini Program operation")
	}
	if !validOptionalSensitive(resolved.RequestID, maxRequestIDLength) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

func normalizeSubscription(input SubscriptionMessage) (SubscriptionMessage, error) {
	const operation = "send_subscription_message"
	if !validSensitive(input.ToUser, maxOpenIDLength) || !validSensitive(input.TemplateID, 512) {
		return SubscriptionMessage{}, invalidArgument(operation, "recipient openid or template id is invalid")
	}
	if len(input.Data) == 0 {
		return SubscriptionMessage{}, invalidArgument(operation, "template data is required")
	}
	if input.Page != "" && !validMiniProgramPage(input.Page) {
		return SubscriptionMessage{}, invalidArgument(operation, "page must be a relative path within the current Mini Program")
	}
	if input.MiniProgramState == "" {
		input.MiniProgramState = StateFormal
	}
	if input.Language == "" {
		input.Language = LanguageSimplifiedChinese
	}
	if !validMiniProgramState(input.MiniProgramState) || !validLanguage(input.Language) {
		return SubscriptionMessage{}, invalidArgument(operation, "mini-program state or language is invalid")
	}
	data := make(map[string]TemplateValue, len(input.Data))
	for key, value := range input.Data {
		if !validTemplateKey(key) || !validTemplateValue(value.Value) {
			return SubscriptionMessage{}, invalidArgument(operation, "template data contains an invalid key or value")
		}
		data[key] = value
	}
	input.Data = data
	return input, nil
}

func validMiniProgramState(value MiniProgramState) bool {
	return value == StateDeveloper || value == StateTrial || value == StateFormal
}

func validLanguage(value Language) bool {
	return value == LanguageSimplifiedChinese || value == LanguageEnglish ||
		value == LanguageTraditionalHongKong || value == LanguageTraditionalTaiwan
}

func validMiniProgramPage(value string) bool {
	if !validSensitive(value, 1_024) || strings.HasPrefix(value, "//") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "" && parsed.Host == "" && parsed.User == nil && parsed.Fragment == "" && parsed.Path != ""
}

func validTemplateKey(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validTemplateValue(value string) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 256 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validatePhoneRequest(input PhoneNumberRequest) error {
	if !validSensitive(input.Code, maxCodeLength) || !validOptionalSensitive(input.OpenID, maxOpenIDLength) {
		return invalidArgument("exchange_phone_number", "phone code or optional openid is invalid")
	}
	return nil
}

func validatePhoneInfo(info PhoneInfo, appID string) error {
	if !validSensitive(info.PhoneNumber, 64) || !validSensitive(info.PurePhoneNumber, 64) ||
		!validSensitive(info.CountryCode, 16) || info.Watermark.Timestamp <= 0 || info.Watermark.AppID != appID {
		return platformContractError("exchange_phone_number", "WeChat returned incomplete or mismatched phone information")
	}
	return nil
}
