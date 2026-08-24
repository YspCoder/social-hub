package unityadvertising

import (
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	mongoIDPattern            = regexp.MustCompile(`(?i)^[0-9a-f]{24}$`)
	uuidPattern               = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	sourceAppIDPattern        = regexp.MustCompile(`^[a-zA-Z0-9]{12}$`)
	bidPattern                = regexp.MustCompile(`^\d{1,3}(\.\d{1,3})?$`)
	responseMaxBidPattern     = regexp.MustCompile(`^\d{1,6}(\.\d{1,3})?$`)
	roasGoalPattern           = regexp.MustCompile(`^\d{1,4}(\.\d{1,2})?$`)
	moneyPattern              = regexp.MustCompile(`^\d{1,9}(\.\d{1,2})?$`)
	totalMoneyPattern         = regexp.MustCompile(`^[1-9]\d{0,8}(\.\d{1,2})?$|^0(\.0{1,2})?$`)
	androidStoreListingRegexp = regexp.MustCompile(`^[a-z0-9\-._~]{1,40}$`)
	countryGroupNamePattern   = regexp.MustCompile(`^[a-zA-Z0-9\s\-_]+$`)
)

func validOpaque(value string, maximum int) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n")
}

func validBasicKeyID(value string) bool {
	return validOpaque(value, 1024) && !strings.ContainsRune(value, ':')
}

func validOrganizationID(value string) bool {
	if value == "" || value[0] == '0' {
		return false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}

func validMongoID(value string) bool { return mongoIDPattern.MatchString(value) }

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= maximum && !strings.ContainsRune(value, '\x00')
}

func validNullableText(value *NullableString, minimum, maximum int) bool {
	if value == nil || value.Value == nil {
		return true
	}
	text := *value.Value
	return utf8.ValidString(text) && utf8.RuneCountInString(text) >= minimum && utf8.RuneCountInString(text) <= maximum &&
		!strings.ContainsRune(text, '\x00')
}

func validHTTPSURL(value string) bool {
	if len(value) < 1 || len(value) > 2082 {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func validNullableHTTPSURL(value *NullableString) bool {
	return value == nil || value.Value == nil || validHTTPSURL(*value.Value)
}

func validDate(value string) bool {
	_, err := time.Parse(time.DateOnly, value)
	return err == nil
}

func validOptionalDate(value *string) bool { return value == nil || validDate(*value) }

func validCountry(value CountryCode) bool {
	return strings.Contains(validCountryCodes, " "+string(value)+" ")
}

func validRegionalCountry(value CountryCode) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validBid(value BidAmount) bool { return bidPattern.MatchString(string(value)) }
func validResponseMaxBid(value BidAmount) bool {
	return responseMaxBidPattern.MatchString(string(value))
}
func validROASGoal(value ROASGoal) bool { return roasGoalPattern.MatchString(string(value)) }
func validMoney(value Money) bool       { return moneyPattern.MatchString(string(value)) }
func validPositiveMoney(value Money) bool {
	text := string(value)
	return moneyPattern.MatchString(text) && strings.ContainsAny(text, "123456789")
}
func validTotalMoney(value Money) bool { return totalMoneyPattern.MatchString(string(value)) }

func validSourceAppID(value string) bool { return sourceAppIDPattern.MatchString(value) }

func validStore(value Store) bool {
	return value == StoreApple || value == StoreGoogle || value == StoreStandaloneAndroid
}

func validCreativeLanguage(value CreativeLanguage) bool {
	return strings.Contains(validCreativeLanguages, " "+string(value)+" ")
}

func validUploadFilename(filename string, extensions ...string) bool {
	if !validText(filename, 255) || filepath.Base(filename) != filename || strings.ContainsAny(filename, "/\\\r\n") {
		return false
	}
	extension := strings.ToLower(filepath.Ext(filename))
	for _, allowed := range extensions {
		if extension == allowed {
			return true
		}
	}
	return false
}

func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

const validCountryCodes = " AD AE AF AG AI AL AM AO AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BM BN BO BR BS BT BV BW BY BZ CA CD CF CG CH CI CK CL CM CN CO CR CV CY CZ DE DJ DK DM DO DZ EC EE EG ER ES ET FI FJ FK FM FO FR GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IS IT JE JM JO JP KE KG KH KI KM KN KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PR PS PT PW PY QA RE RO RS RU RW SA SB SC SD SE SG SI SK SL SM SN SO SR ST SV SZ TC TD TF TG TH TJ TK TM TN TO TR TT TV TW TZ UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS XK YE YT ZA ZM ZW "

const validCreativeLanguages = " zxx und af ar as az be bn bg ca zh hr cs da nl en et fil fi fr de el he hi hu is id it ja kk ko ky lv lt mk ms mr mn ne no ps fa pl pt ro ru sa sr sk sl es sv ta th tr uk ur uz vi "
