package mercadobrandads

import (
	"mime"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var brandMetricsStart = time.Date(2023, time.February, 9, 0, 0, 0, 0, time.UTC)

func resolveCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, invalidArgument(operation, "call options are invalid")
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Mercado Libre assigns request IDs; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "read-only Brand Ads operations do not accept idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "metric field selection is expressed by the typed Brand Ads request")
	}
	if resolved.Timeout < 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "timeout must not be negative")
	}
	return resolved, nil
}

func resolvedCallOption(resolved socialhub.CallOptions) socialhub.CallOption {
	return func(target *socialhub.CallOptions) error {
		*target = resolved
		target.Fields = append([]string(nil), resolved.Fields...)
		return nil
	}
}

func validAuthorizationEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Port() != "" ||
		parsed.User != nil || parsed.Path != "/authorization" || parsed.RawPath != "" ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "auth.mercadolibre.com.ar", "auth.mercadolibre.com.bo", "auth.mercadolivre.com.br",
		"auth.mercadolibre.cl", "auth.mercadolibre.com.co", "auth.mercadolibre.co.cr",
		"auth.mercadolibre.com.do", "auth.mercadolibre.com.ec", "auth.mercadolibre.com.gt",
		"auth.mercadolibre.com.hn", "auth.mercadolibre.com.mx", "auth.mercadolibre.com.ni",
		"auth.mercadolibre.com.pa", "auth.mercadolibre.com.pe", "auth.mercadolibre.com.py",
		"auth.mercadolibre.com.sv", "auth.mercadolibre.com.uy", "auth.mercadolibre.com.ve":
		return true
	default:
		return false
	}
}

func validTokenEndpoint(value string) bool {
	return value == defaultTokenURL
}

func validCallbackURL(value string) bool {
	if !validOpaque(value, 4096) {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func validJSONMediaType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
}

func validPositiveDecimal(value string) bool {
	if !validOpaque(value, 128) || strings.HasPrefix(value, "+") {
		return false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validPKCEChallenge(challenge, method string) bool {
	if challenge == "" || method == "" {
		return challenge == "" && method == ""
	}
	return (method == "S256" || method == "plain") && validPKCEValue(challenge)
}

func validPKCEVerifier(value string) bool { return validPKCEValue(value) }

func validPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && !strings.ContainsRune("-._~", character) {
			return false
		}
	}
	return true
}

func validDate(value Date) bool {
	if value == "" {
		return false
	}
	parsed, err := time.Parse("2006-01-02", string(value))
	return err == nil && DateFromTime(parsed) == value
}

func validMetricDateRange(from, to Date) bool {
	if !validDate(from) || !validDate(to) {
		return false
	}
	start, _ := time.Parse("2006-01-02", string(from))
	end, _ := time.Parse("2006-01-02", string(to))
	return !start.Before(brandMetricsStart) && !end.Before(start)
}

func dateWithin(value, from, to Date) bool {
	if !validDate(value) {
		return false
	}
	date, _ := time.Parse("2006-01-02", string(value))
	start, _ := time.Parse("2006-01-02", string(from))
	end, _ := time.Parse("2006-01-02", string(to))
	return !date.Before(start) && !date.After(end)
}

func validMetricQuery(input MetricQuery, competitive bool) bool {
	if !validMetricDateRange(input.DateFrom, input.DateTo) || input.Limit < 0 || input.Offset < 0 ||
		(input.AggregationType != "" && input.AggregationType != AggregationDaily && input.AggregationType != AggregationTotal) {
		return false
	}
	seen := make(map[MetricField]struct{}, len(input.Fields))
	competitiveRequested := false
	for _, field := range input.Fields {
		switch field {
		case MetricFieldPrints, MetricFieldClicks, MetricFieldCTR, MetricFieldCVR,
			MetricFieldConsumedBudget, MetricFieldCPC, MetricFieldACOS,
			MetricFieldEventTime, MetricFieldTouchPoint:
		case MetricFieldCompetitive:
			if !competitive {
				return false
			}
			competitiveRequested = true
		default:
			return false
		}
		if _, duplicate := seen[field]; duplicate {
			return false
		}
		seen[field] = struct{}{}
	}
	return !competitiveRequested || input.AggregationType == AggregationTotal
}

func validAdvertiserMetricRequest(input AdvertiserMetricRequest) bool {
	return validMetricQuery(input.MetricQuery, false) && input.DestinationID >= 0 &&
		(input.Status == "" || input.Status == CampaignFilterActive || input.Status == CampaignFilterPaused)
}

func validCampaignMetricRequest(input CampaignMetricRequest) bool {
	return input.CampaignID > 0 && validMetricQuery(input.MetricQuery, true)
}

func validKeywordMetricRequest(input KeywordMetricRequest) bool {
	return input.CampaignID > 0 && validMetricQuery(input.MetricQuery, false)
}

func validTimestamp(value string) bool {
	if !validOpaque(value, 128) {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func formatInt64(value int64) string { return strconv.FormatInt(value, 10) }
