package googledatamanager

import (
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	decimalPattern = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	emailPattern   = regexp.MustCompile("^[a-z0-9!#$%&'*+/=?^_`{|}~.-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$")
	gcpKEKPattern  = regexp.MustCompile(`^(?:gcp-kms://)?projects/[^/]+/locations/[^/]+/keyRings/[^/]+/cryptoKeys/[^/]+$`)
	awsRolePattern = regexp.MustCompile(`^arn:[^:]+:iam::[0-9]{12}:role/.+$`)
	awsKEKPattern  = regexp.MustCompile(`^(?:aws-kms://)?arn:[^:]+:kms:[^:]+:[0-9]{12}:key/.+$`)
)

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	address := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || address != nil && address.IsLoopback()
}

func validOpaque(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && len(value) <= maximum &&
		utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n")
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validText(value string, maximum int, required bool) bool {
	if required && value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validDecimal(value Decimal) bool {
	return len(value) > 0 && len(value) <= 128 && decimalPattern.MatchString(string(value))
}

func validNonNegativeDecimal(value Decimal) bool {
	return validDecimal(value) && !strings.HasPrefix(string(value), "-")
}

func validNumericID(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	nonzero := false
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
		nonzero = nonzero || value[index] != '0'
	}
	return nonzero
}

func validServiceAccountEmail(value string) bool {
	if value == "" || len(value) > 254 || value != strings.TrimSpace(value) || strings.Count(value, "@") != 1 || strings.ContainsAny(value, "\x00\r\n ") {
		return false
	}
	parts := strings.Split(value, "@")
	return parts[0] != "" && strings.HasSuffix(strings.ToLower(parts[1]), ".gserviceaccount.com")
}

func validOAuthScopes(scopes []string) bool {
	return len(scopes) == 1 && scopes[0] == dataManagerScope
}

func validAccountType(value AccountType) bool {
	switch value {
	case AccountTypeGoogleAds, AccountTypeDisplayVideoPartner, AccountTypeDisplayVideoAdvertiser,
		AccountTypeDataPartner, AccountTypeGoogleAnalyticsProperty, AccountTypeGoogleAdManagerAudience,
		AccountTypeFloodlightConfig:
		return true
	default:
		return false
	}
}

func validConsent(value *Consent) bool {
	if value == nil {
		return true
	}
	return validConsentStatus(value.AdUserData) && validConsentStatus(value.AdPersonalization) &&
		(value.AdUserData != "" || value.AdPersonalization != "")
}

func validConsentStatus(value ConsentStatus) bool {
	return value == "" || value == ConsentGranted || value == ConsentDenied
}

func validEventSource(value EventSource) bool {
	switch value {
	case "", EventSourceWeb, EventSourceApp, EventSourceInStore, EventSourcePhone, EventSourceMessage, EventSourceOther:
		return true
	default:
		return false
	}
}

func validCustomerType(value CustomerType) bool {
	return value == "" || value == CustomerTypeNew || value == CustomerTypeReturning || value == CustomerTypeReengaged
}

func validCustomerValueBucket(value CustomerValueBucket) bool {
	return value == "" || value == CustomerValueLow || value == CustomerValueMedium || value == CustomerValueHigh
}

func validEncryptionEntityType(value EncryptionEntityType) bool {
	switch value {
	case EncryptionEntityCampaignManagerAccount, EncryptionEntityCampaignManagerAdvertiser,
		EncryptionEntityDisplayVideoPartner, EncryptionEntityDisplayVideoAdvertiser,
		EncryptionEntityGoogleAdsCustomer, EncryptionEntityGoogleAdManagerNetwork:
		return true
	default:
		return false
	}
}

func validEncryptionSource(value EncryptionSource) bool {
	return value == EncryptionSourceAdServing || value == EncryptionSourceDataTransfer
}

func validEncoding(value Encoding) bool { return value == EncodingHex || value == EncodingBase64 }

func validEncoded(value string, encoding Encoding, exactHash bool) bool {
	if value == "" {
		return false
	}
	var decoded []byte
	var err error
	switch encoding {
	case EncodingHex:
		decoded, err = hex.DecodeString(value)
	case EncodingBase64:
		decoded, err = base64.StdEncoding.DecodeString(value)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(value)
		}
	default:
		return false
	}
	return err == nil && len(decoded) > 0 && (!exactHash || len(decoded) == 32)
}

func validCurrency(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 3 {
		return false
	}
	for index := range value {
		if value[index] < 'A' || value[index] > 'Z' {
			return false
		}
	}
	return true
}

func validRegionCode(value string) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validLanguageCode(value string) bool {
	if value == "" {
		return true
	}
	return len(value) == 2 && unicode.IsLetter(rune(value[0])) && unicode.IsLetter(rune(value[1]))
}

func validIPAddress(value string) bool {
	return value == "" || value == strings.TrimSpace(value) && net.ParseIP(value) != nil
}

func validQuantity(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed >= 0 && strconv.FormatInt(parsed, 10) == value
}

func validEncryptionInfo(value *EncryptionInfo) bool {
	if value == nil || boolInt(value.GCPWrappedKeyInfo != nil)+boolInt(value.AWSWrappedKeyInfo != nil) != 1 {
		return false
	}
	if value.GCPWrappedKeyInfo != nil {
		key := value.GCPWrappedKeyInfo
		return key.KeyType == KeyTypeXChaCha20Poly1305 && validOpaque(key.WIPProvider, 4096) &&
			gcpKEKPattern.MatchString(key.KEKURI) && validBase64(key.EncryptedDEK)
	}
	key := value.AWSWrappedKeyInfo
	return key.KeyType == KeyTypeXChaCha20Poly1305 && awsRolePattern.MatchString(key.RoleARN) &&
		awsKEKPattern.MatchString(key.KEKURI) && validBase64(key.EncryptedDEK)
}

func validBase64(value string) bool {
	if value == "" {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(decoded) > 0
}
