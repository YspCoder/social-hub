package conversions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type hashKind uint8

const (
	hashEmail hashKind = iota
	hashPhone
	hashGender
	hashDateOfBirth
	hashFirstName
	hashLastName
	hashCity
	hashState
	hashZip
	hashCountry
	hashExternalID
	hashF5Name
	hashInitial
	hashBirthDay
	hashBirthMonth
	hashBirthYear
	hashAppUserID
)

var (
	lowerSHA256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
	anySHA256Pattern   = regexp.MustCompile(`(?i)^[a-f0-9]{64}$`)
	legacyMD5Pattern   = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)
	emailPattern       = regexp.MustCompile("^[a-z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$")
	digitsPattern      = regexp.MustCompile(`^[0-9]+$`)
)

type wireUserData struct {
	Emails       []string `json:"em,omitempty"`
	Phones       []string `json:"ph,omitempty"`
	Genders      []string `json:"ge,omitempty"`
	DatesOfBirth []string `json:"db,omitempty"`
	FirstNames   []string `json:"fn,omitempty"`
	LastNames    []string `json:"ln,omitempty"`
	Cities       []string `json:"ct,omitempty"`
	States       []string `json:"st,omitempty"`
	Zips         []string `json:"zp,omitempty"`
	Countries    []string `json:"country,omitempty"`
	ExternalIDs  []string `json:"external_id,omitempty"`

	ClientIPAddress string `json:"client_ip_address,omitempty"`
	ClientUserAgent string `json:"client_user_agent,omitempty"`
	FBC             string `json:"fbc,omitempty"`
	FBP             string `json:"fbp,omitempty"`
	SubscriptionID  string `json:"subscription_id,omitempty"`
	FBLoginID       string `json:"fb_login_id,omitempty"`
	LeadID          string `json:"lead_id,omitempty"`
	F5First         string `json:"f5first,omitempty"`
	F5Last          string `json:"f5last,omitempty"`
	FirstInitial    string `json:"fi,omitempty"`
	BirthDay        string `json:"dobd,omitempty"`
	BirthMonth      string `json:"dobm,omitempty"`
	BirthYear       string `json:"doby,omitempty"`
	MobileAdID      string `json:"madid,omitempty"`
	AnonymousID     string `json:"anon_id,omitempty"`
	AppUserID       string `json:"app_user_id,omitempty"`
	CTWAClID        string `json:"ctwa_clid,omitempty"`
	PageID          string `json:"page_id,omitempty"`
}

func normalizeUserData(input UserData) (wireUserData, error) {
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
		{"user_data.genders", input.Genders, hashGender, &output.Genders},
		{"user_data.dates_of_birth", input.DatesOfBirth, hashDateOfBirth, &output.DatesOfBirth},
		{"user_data.first_names", input.FirstNames, hashFirstName, &output.FirstNames},
		{"user_data.last_names", input.LastNames, hashLastName, &output.LastNames},
		{"user_data.cities", input.Cities, hashCity, &output.Cities},
		{"user_data.states", input.States, hashState, &output.States},
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

	singular := []struct {
		path   string
		value  string
		kind   hashKind
		target *string
	}{
		{"user_data.f5first", input.F5First, hashF5Name, &output.F5First},
		{"user_data.f5last", input.F5Last, hashF5Name, &output.F5Last},
		{"user_data.first_initial", input.FirstInitial, hashInitial, &output.FirstInitial},
		{"user_data.birth_day", input.BirthDay, hashBirthDay, &output.BirthDay},
		{"user_data.birth_month", input.BirthMonth, hashBirthMonth, &output.BirthMonth},
		{"user_data.birth_year", input.BirthYear, hashBirthYear, &output.BirthYear},
		{"user_data.app_user_id", input.AppUserID, hashAppUserID, &output.AppUserID},
	}
	for _, field := range singular {
		if field.value == "" {
			continue
		}
		*field.target, err = normalizeAndHash(field.value, field.kind)
		if err != nil {
			return wireUserData{}, fmt.Errorf("%s is invalid", field.path)
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
		{"user_data.fbc", input.FBC, 4096}, {"user_data.fbp", input.FBP, 4096},
		{"user_data.subscription_id", input.SubscriptionID, 4096},
		{"user_data.fb_login_id", input.FBLoginID, 4096},
		{"user_data.lead_id", input.LeadID, 4096},
		{"user_data.mobile_ad_id", input.MobileAdID, 4096},
		{"user_data.anonymous_id", input.AnonymousID, 4096},
		{"user_data.ctwa_clid", input.CTWAClID, 4096},
	} {
		if !validOptionalOpaque(field.value, field.maximum) {
			return wireUserData{}, fmt.Errorf("%s is invalid", field.path)
		}
	}
	if input.PageID != "" && !validNumericID(input.PageID) {
		return wireUserData{}, fmt.Errorf("user_data.page_id is invalid")
	}
	output.ClientIPAddress = input.ClientIPAddress
	output.ClientUserAgent = input.ClientUserAgent
	output.FBC = input.FBC
	output.FBP = input.FBP
	output.SubscriptionID = input.SubscriptionID
	output.FBLoginID = input.FBLoginID
	output.LeadID = input.LeadID
	output.MobileAdID = input.MobileAdID
	output.AnonymousID = input.AnonymousID
	output.CTWAClID = input.CTWAClID
	output.PageID = input.PageID

	hasHashedID := len(output.Emails)+len(output.Phones)+len(output.Genders)+len(output.DatesOfBirth)+
		len(output.FirstNames)+len(output.LastNames)+len(output.Cities)+len(output.States)+len(output.Zips)+
		len(output.Countries)+len(output.ExternalIDs) > 0 || output.F5First != "" || output.F5Last != "" ||
		output.FirstInitial != "" || output.BirthDay != "" || output.BirthMonth != "" || output.BirthYear != "" || output.AppUserID != ""
	hasRawID := output.FBC != "" || output.FBP != "" || output.SubscriptionID != "" || output.FBLoginID != "" ||
		output.LeadID != "" || output.MobileAdID != "" || output.AnonymousID != "" || output.CTWAClID != "" ||
		output.PageID != "" || output.ClientIPAddress != "" && output.ClientUserAgent != ""
	if !hasHashedID && !hasRawID {
		return wireUserData{}, fmt.Errorf("user_data requires at least one supported matching identifier")
	}
	return output, nil
}

func normalizeMulti(values []string, kind hashKind, path string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
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
	if trimmed == "" || len(trimmed) > 4096 || !utf8.ValidString(trimmed) || strings.ContainsFunc(trimmed, unicode.IsControl) {
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
	case hashGender:
		switch normalized {
		case "female", "f":
			normalized = "f"
		case "male", "m":
			normalized = "m"
		default:
			err = fmt.Errorf("invalid gender")
		}
	case hashDateOfBirth:
		if len(normalized) != 8 || !digitsPattern.MatchString(normalized) {
			err = fmt.Errorf("invalid date")
		} else if parsed, parseErr := time.Parse("20060102", normalized); parseErr != nil || parsed.Format("20060102") != normalized {
			err = fmt.Errorf("invalid date")
		}
	case hashCity, hashState:
		normalized = keepLetters(normalized)
		if normalized == "" {
			err = fmt.Errorf("invalid location")
		}
	case hashZip:
		normalized = strings.ReplaceAll(normalized, " ", "")
		normalized = strings.SplitN(normalized, "-", 2)[0]
		if len(normalized) < 2 {
			err = fmt.Errorf("invalid postal code")
		}
	case hashCountry:
		if len(normalized) != 2 || normalized[0] < 'a' || normalized[0] > 'z' || normalized[1] < 'a' || normalized[1] > 'z' {
			err = fmt.Errorf("invalid country")
		}
	case hashF5Name:
		runes := []rune(normalized)
		if len(runes) > 5 {
			normalized = string(runes[:5])
		}
	case hashInitial:
		normalized = string([]rune(normalized)[:1])
	case hashBirthDay:
		normalized, err = normalizeDatePart(normalized, 1, 31)
	case hashBirthMonth:
		normalized, err = normalizeDatePart(normalized, 1, 12)
	case hashBirthYear:
		if len(normalized) != 4 || !digitsPattern.MatchString(normalized) {
			err = fmt.Errorf("invalid year")
		}
	case hashFirstName, hashLastName, hashExternalID, hashAppUserID:
		// Trim and lowercase are the complete stable v26 normalization.
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
		case strings.ContainsRune("+-() @#<>'\",;", character):
		default:
			return "", fmt.Errorf("invalid phone")
		}
	}
	normalized := strings.TrimLeft(digits.String(), "0")
	if len(normalized) < 7 || len(normalized) > 16 {
		return "", fmt.Errorf("invalid phone")
	}
	return normalized, nil
}

func normalizeDatePart(value string, minimum, maximum int) (string, error) {
	if len(value) < 1 || len(value) > 2 || !digitsPattern.MatchString(value) {
		return "", fmt.Errorf("invalid date part")
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < minimum || number > maximum {
		return "", fmt.Errorf("invalid date part")
	}
	return fmt.Sprintf("%02d", number), nil
}

func keepLetters(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) {
			return character
		}
		return -1
	}, value)
}
