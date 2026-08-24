package conversions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strings"
	"unicode"
)

type hashKind uint8

const (
	hashEmail hashKind = iota
	hashPhone
	hashName
	hashGender
	hashLocation
	hashZip
	hashCountry
	hashExternalID
)

var (
	lowerSHA256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
	anySHA256Pattern   = regexp.MustCompile(`(?i)^[a-f0-9]{64}$`)
	legacyMD5Pattern   = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)
	emailPattern       = regexp.MustCompile("^[a-z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$")
)

type wireUserData struct {
	Emails      []string `json:"em,omitempty"`
	Phones      []string `json:"ph,omitempty"`
	FirstNames  []string `json:"fn,omitempty"`
	LastNames   []string `json:"ln,omitempty"`
	Genders     []string `json:"ge,omitempty"`
	Cities      []string `json:"ct,omitempty"`
	States      []string `json:"st,omitempty"`
	Zips        []string `json:"zp,omitempty"`
	Countries   []string `json:"country,omitempty"`
	ExternalIDs []string `json:"external_id,omitempty"`

	ClientIPAddress string `json:"client_ip_address,omitempty"`
	ClientUserAgent string `json:"client_user_agent,omitempty"`
	SubscriptionID  string `json:"subscription_id,omitempty"`
	LeadID          string `json:"lead_id,omitempty"`
	AnonymousID     string `json:"anon_id,omitempty"`
	MobileAdID      string `json:"madid,omitempty"`
	DownloadID      string `json:"download_id,omitempty"`
	SnapClickID     string `json:"sc_click_id,omitempty"`
	SnapCookie1     string `json:"sc_cookie1,omitempty"`
	IDFV            string `json:"idfv,omitempty"`
	PartnerID       string `json:"partner_id,omitempty"`
}

func normalizeUserData(input UserData, action ActionSource) (wireUserData, error) {
	var output wireUserData
	var err error
	fields := []struct {
		path   string
		values []string
		kind   hashKind
		target *[]string
	}{
		{"user_data.emails", input.Emails, hashEmail, &output.Emails},
		{"user_data.phones", input.Phones, hashPhone, &output.Phones},
		{"user_data.first_names", input.FirstNames, hashName, &output.FirstNames},
		{"user_data.last_names", input.LastNames, hashName, &output.LastNames},
		{"user_data.genders", input.Genders, hashGender, &output.Genders},
		{"user_data.cities", input.Cities, hashLocation, &output.Cities},
		{"user_data.states", input.States, hashLocation, &output.States},
		{"user_data.zips", input.Zips, hashZip, &output.Zips},
		{"user_data.countries", input.Countries, hashCountry, &output.Countries},
		{"user_data.external_ids", input.ExternalIDs, hashExternalID, &output.ExternalIDs},
	}
	for _, field := range fields {
		*field.target, err = normalizeMulti(field.values, field.kind, field.path)
		if err != nil {
			return wireUserData{}, err
		}
	}
	if input.ClientIPAddress != "" && net.ParseIP(input.ClientIPAddress) == nil {
		return wireUserData{}, fmt.Errorf("user_data.client_ip_address is invalid")
	}
	for _, field := range []struct {
		path    string
		value   string
		maximum int
	}{
		{"user_data.client_user_agent", input.ClientUserAgent, 8192},
		{"user_data.subscription_id", input.SubscriptionID, 4096},
		{"user_data.lead_id", input.LeadID, 4096},
		{"user_data.anonymous_id", input.AnonymousID, 4096},
		{"user_data.mobile_ad_id", input.MobileAdID, 4096},
		{"user_data.download_id", input.DownloadID, 4096},
		{"user_data.sc_click_id", input.SnapClickID, 4096},
		{"user_data.sc_cookie1", input.SnapCookie1, 4096},
		{"user_data.idfv", input.IDFV, 4096},
		{"user_data.partner_id", input.PartnerID, 4096},
	} {
		if !validOptionalOpaque(field.value, field.maximum) {
			return wireUserData{}, fmt.Errorf("%s is invalid", field.path)
		}
	}
	if action != ActionSourceMobileApp && (input.AnonymousID != "" || input.MobileAdID != "" || input.DownloadID != "" || input.IDFV != "") {
		return wireUserData{}, fmt.Errorf("app-only user_data fields require action_source MOBILE_APP")
	}
	output.ClientIPAddress = input.ClientIPAddress
	output.ClientUserAgent = input.ClientUserAgent
	output.SubscriptionID = input.SubscriptionID
	output.LeadID = input.LeadID
	output.AnonymousID = input.AnonymousID
	output.MobileAdID = strings.ToLower(input.MobileAdID)
	output.DownloadID = input.DownloadID
	output.SnapClickID = input.SnapClickID
	output.SnapCookie1 = input.SnapCookie1
	output.IDFV = input.IDFV
	output.PartnerID = input.PartnerID

	hasPrimaryHash := len(output.Emails)+len(output.Phones) > 0
	hasIPPair := output.ClientIPAddress != "" && output.ClientUserAgent != ""
	hasMobileID := output.MobileAdID != ""
	if !hasPrimaryHash && !hasIPPair && !hasMobileID {
		return wireUserData{}, fmt.Errorf("user_data requires hashed email, hashed phone, client IP and user agent, or mobile ad ID")
	}
	return output, nil
}

func normalizeMulti(values []string, kind hashKind, path string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > maximumValuesPerField {
		return nil, fmt.Errorf("%s has too many values", path)
	}
	seen := make(map[string]struct{}, len(values))
	output := make([]string, 0, len(values))
	for index, value := range values {
		hashed, err := normalizeAndHash(value, kind)
		if err != nil {
			return nil, fmt.Errorf("%s[%d] is invalid", path, index)
		}
		if _, found := seen[hashed]; found {
			continue
		}
		seen[hashed] = struct{}{}
		output = append(output, hashed)
	}
	return output, nil
}

func normalizeAndHash(value string, kind hashKind) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len(trimmed) > 4096 || strings.ContainsFunc(trimmed, unicode.IsControl) {
		return "", fmt.Errorf("invalid value")
	}
	if lowerSHA256Pattern.MatchString(trimmed) {
		return trimmed, nil
	}
	if anySHA256Pattern.MatchString(trimmed) || legacyMD5Pattern.MatchString(trimmed) {
		return "", fmt.Errorf("unsupported digest")
	}
	normalized := strings.ToLower(trimmed)
	var err error
	switch kind {
	case hashEmail:
		if !emailPattern.MatchString(normalized) {
			err = fmt.Errorf("invalid email")
		}
	case hashPhone:
		normalized, err = normalizePhone(normalized)
	case hashName, hashLocation:
		normalized = keepLetters(normalized)
		if normalized == "" {
			err = fmt.Errorf("invalid text")
		}
	case hashGender:
		switch normalized {
		case "female", "f":
			normalized = "f"
		case "male", "m":
			normalized = "m"
		default:
			err = fmt.Errorf("invalid gender")
		}
	case hashZip:
		normalized = strings.Map(func(character rune) rune {
			if unicode.IsLetter(character) || unicode.IsDigit(character) {
				return character
			}
			return -1
		}, normalized)
		if len(normalized) < 2 {
			err = fmt.Errorf("invalid postal code")
		}
	case hashCountry:
		if len(normalized) != 2 || normalized[0] < 'a' || normalized[0] > 'z' || normalized[1] < 'a' || normalized[1] > 'z' {
			err = fmt.Errorf("invalid country")
		}
	case hashExternalID:
		// trim and lowercase are the complete normalization.
	default:
		err = fmt.Errorf("unsupported hash kind")
	}
	if err != nil || normalized == "" {
		return "", err
	}
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:]), nil
}

func normalizePhone(value string) (string, error) {
	var digits strings.Builder
	for _, character := range value {
		switch {
		case character >= '0' && character <= '9':
			digits.WriteRune(character)
		case strings.ContainsRune("+-() .", character):
		default:
			return "", fmt.Errorf("invalid phone")
		}
	}
	normalized := digits.String()
	normalized = strings.TrimPrefix(normalized, "00")
	normalized = strings.TrimPrefix(normalized, "0")
	if len(normalized) < 7 || len(normalized) > 16 {
		return "", fmt.Errorf("invalid phone")
	}
	return normalized, nil
}

func keepLetters(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) {
			return character
		}
		return -1
	}, value)
}
