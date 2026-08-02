package tvmaze

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"social-hub/pkg/socialhub"
)

const (
	maxUserAgentLength = 256
	maxQueryLength     = 512
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
	return value != "" && len(value) <= maxQueryLength && strings.TrimSpace(value) != "" &&
		strings.TrimSpace(value) == value && !containsControl(value)
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func validCountry(value string) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validDate(value time.Time) bool { return !value.IsZero() }

func validIMDBID(value string) bool {
	if len(value) < 3 || !strings.HasPrefix(value, "tt") {
		return false
	}
	for _, character := range value[2:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func lookupQuery(request LookupShowRequest) (url.Values, error) {
	configured := 0
	if request.IMDB != "" {
		configured++
	}
	if request.TVDB != 0 {
		configured++
	}
	if request.TVRage != 0 {
		configured++
	}
	if configured != 1 {
		return nil, invalidArgument("lookup_show", "exactly one external ID is required")
	}
	query := url.Values{}
	switch {
	case request.IMDB != "":
		if !validIMDBID(request.IMDB) {
			return nil, invalidArgument("lookup_show", "imdb must be a canonical tt-prefixed numeric ID")
		}
		query.Set("imdb", request.IMDB)
	case request.TVDB > 0:
		query.Set("thetvdb", strconv.FormatInt(request.TVDB, 10))
	case request.TVRage > 0:
		query.Set("tvrage", strconv.FormatInt(request.TVRage, 10))
	default:
		return nil, invalidArgument("lookup_show", "external numeric IDs must be positive")
	}
	return query, nil
}

func validUpdatePeriod(period UpdatePeriod) bool {
	return period == "" || period == UpdateDay || period == UpdateWeek || period == UpdateMonth
}

func publicAccountOnly(account socialhub.AccountConfig) bool {
	return account.ClientID == "" && account.AppID == "" && account.SecretRef == "" && account.AccessTokenRef == "" &&
		account.TokenStore == "" && account.Webhook.SecretRef == "" && account.Webhook.TokenRef == "" && account.Webhook.AESKeyRef == "" &&
		account.Approval.AccountType == "" && len(account.Approval.Scopes) == 0 && len(account.Settings) == 0
}
