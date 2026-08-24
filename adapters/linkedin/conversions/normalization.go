package conversions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"
)

type wireBatch struct {
	Elements []wireEvent `json:"elements"`
}

type wireEvent struct {
	Conversion           string     `json:"conversion"`
	ConversionHappenedAt int64      `json:"conversionHappenedAt"`
	ConversionValue      *wireMoney `json:"conversionValue,omitempty"`
	User                 wireUser   `json:"user"`
	EventID              string     `json:"eventId,omitempty"`
}

type wireMoney struct {
	CurrencyCode string  `json:"currencyCode"`
	Amount       Decimal `json:"amount"`
}

type wireUser struct {
	UserIDs     []wireUserID  `json:"userIds"`
	UserInfo    *wireUserInfo `json:"userInfo,omitempty"`
	Lead        string        `json:"lead,omitempty"`
	ExternalIDs []string      `json:"externalIds,omitempty"`
}

type wireUserID struct {
	IDType  string `json:"idType"`
	IDValue string `json:"idValue"`
}

type wireUserInfo struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	CompanyName string `json:"companyName,omitempty"`
	Title       string `json:"title,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
}

func normalizeRequest(conversionURN string, now time.Time, input SubmitEventsRequest) ([]wireEvent, error) {
	if len(input.Events) == 0 || len(input.Events) > MaximumBatchSize {
		return nil, fmt.Errorf("events must contain between 1 and %d entries", MaximumBatchSize)
	}
	output := make([]wireEvent, len(input.Events))
	for index := range input.Events {
		event, err := normalizeEvent(conversionURN, now, input.Events[index])
		if err != nil {
			return nil, fmt.Errorf("events[%d]: %w", index, err)
		}
		output[index] = event
	}
	return output, nil
}

func normalizeEvent(conversionURN string, now time.Time, input ConversionEvent) (wireEvent, error) {
	minimum := now.Add(-MaximumEventAgeDays * 24 * time.Hour).UnixMilli()
	maximum := now.UnixMilli()
	if input.ConversionHappenedAt < minimum || input.ConversionHappenedAt > maximum ||
		!validOptionalOpaque(input.EventID, 4096) {
		return wireEvent{}, fmt.Errorf("conversionHappenedAt must be within the past 90 days and eventId must be valid")
	}
	user, err := normalizeUser(input.User)
	if err != nil {
		return wireEvent{}, err
	}
	var money *wireMoney
	if input.ConversionValue != nil {
		currency := strings.ToUpper(strings.TrimSpace(input.ConversionValue.CurrencyCode))
		if !validCurrency(currency) || !validDecimal(input.ConversionValue.Amount) {
			return wireEvent{}, fmt.Errorf("conversionValue requires an ISO currency and a non-negative decimal-string amount")
		}
		money = &wireMoney{CurrencyCode: currency, Amount: input.ConversionValue.Amount}
	}
	return wireEvent{
		Conversion: conversionURN, ConversionHappenedAt: input.ConversionHappenedAt,
		ConversionValue: money, User: user, EventID: input.EventID,
	}, nil
}

func normalizeUser(input User) (wireUser, error) {
	identifierCount := len(input.Emails) + len(input.LinkedInFirstPartyTrackingUUIDs) + len(input.AcxiomIDs) +
		len(input.PlaintextIPAddresses) + len(input.SHA256IPAddresses) + len(input.GoogleAdvertisingIDs)
	if identifierCount > MaximumBatchSize {
		return wireUser{}, fmt.Errorf("user contains too many matching identifiers")
	}
	ids := make([]wireUserID, 0, identifierCount)
	seen := make(map[string]struct{}, cap(ids))
	appendID := func(idType, value string) {
		key := idType + "\x00" + value
		if _, found := seen[key]; found {
			return
		}
		seen[key] = struct{}{}
		ids = append(ids, wireUserID{IDType: idType, IDValue: value})
	}
	for index, value := range input.Emails {
		hashed, err := normalizeEmail(value)
		if err != nil {
			return wireUser{}, fmt.Errorf("user.emails[%d] is invalid", index)
		}
		appendID("SHA256_EMAIL", hashed)
	}
	for _, field := range []struct {
		path   string
		kind   string
		values []string
	}{
		{"linkedin_first_party_tracking_uuids", "LINKEDIN_FIRST_PARTY_ADS_TRACKING_UUID", input.LinkedInFirstPartyTrackingUUIDs},
		{"acxiom_ids", "ACXIOM_ID", input.AcxiomIDs},
	} {
		for index, value := range field.values {
			if !validOpaque(value, 4096) {
				return wireUser{}, fmt.Errorf("user.%s[%d] is invalid", field.path, index)
			}
			appendID(field.kind, value)
		}
	}
	for index, value := range input.PlaintextIPAddresses {
		normalized, valid := normalizeIPv4(value)
		if !valid {
			return wireUser{}, fmt.Errorf("user.plaintext_ip_addresses[%d] is invalid", index)
		}
		appendID("PLAINTEXT_IP_ADDRESS", normalized)
	}
	for index, value := range input.SHA256IPAddresses {
		trimmed := strings.TrimSpace(value)
		if !lowerSHA256.MatchString(trimmed) {
			return wireUser{}, fmt.Errorf("user.sha256_ip_addresses[%d] is invalid", index)
		}
		appendID("SHA256_IP_ADDRESS", trimmed)
	}
	for index, value := range input.GoogleAdvertisingIDs {
		trimmed := strings.TrimSpace(value)
		if !validUUID(trimmed) {
			return wireUser{}, fmt.Errorf("user.google_advertising_ids[%d] is invalid", index)
		}
		appendID("GOOGLE_AID", trimmed)
	}
	info, err := normalizeUserInfo(input.Info)
	if err != nil {
		return wireUser{}, err
	}
	lead := strings.TrimSpace(input.LeadURN)
	if lead != "" && (!strings.HasPrefix(lead, leadURNPrefix) || !validOpaque(strings.TrimPrefix(lead, leadURNPrefix), 4096)) {
		return wireUser{}, fmt.Errorf("user.lead_urn is invalid")
	}
	if len(input.ExternalIDs) > 1 || len(input.ExternalIDs) == 1 && !validOpaque(input.ExternalIDs[0], 4096) {
		return wireUser{}, fmt.Errorf("user.external_ids accepts at most one non-empty value")
	}
	if len(ids) == 0 && info == nil && lead == "" && len(input.ExternalIDs) == 0 {
		return wireUser{}, fmt.Errorf("user requires a matching identifier")
	}
	return wireUser{UserIDs: ids, UserInfo: info, Lead: lead, ExternalIDs: append([]string(nil), input.ExternalIDs...)}, nil
}

func normalizeEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if !validOpaque(trimmed, 4096) {
		return "", fmt.Errorf("invalid email")
	}
	if lowerSHA256.MatchString(trimmed) {
		return trimmed, nil
	}
	if anySHA256.MatchString(trimmed) || legacyMD5.MatchString(trimmed) {
		return "", fmt.Errorf("unsupported digest")
	}
	normalized := strings.Map(func(character rune) rune {
		if unicode.IsSpace(character) {
			return -1
		}
		return unicode.ToLower(character)
	}, trimmed)
	if !emailPattern.MatchString(normalized) {
		return "", fmt.Errorf("invalid email")
	}
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:]), nil
}

func normalizeUserInfo(input *UserInfo) (*wireUserInfo, error) {
	if input == nil {
		return nil, nil
	}
	if !validText(input.FirstName, 256) || !validText(input.LastName, 256) ||
		!validOptionalText(input.CompanyName, 512) || !validOptionalText(input.Title, 512) {
		return nil, fmt.Errorf("user.info requires valid first_name and last_name")
	}
	country := strings.ToUpper(strings.TrimSpace(input.CountryCode))
	if !validCountryCode(country) {
		return nil, fmt.Errorf("user.info.country_code is invalid")
	}
	return &wireUserInfo{
		FirstName: input.FirstName, LastName: input.LastName, CompanyName: input.CompanyName,
		Title: input.Title, CountryCode: country,
	}, nil
}
