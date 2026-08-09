package unitystatistics

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
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

func prepareAcquisitionsRequest(input AcquisitionsReportRequest) (url.Values, ReportFormat, error) {
	const operation = "acquisitions_report"
	query, format, err := prepareReportBase(operation, input.Start, input.End, input.Scale, input.Format, input.EOFMarker)
	if err != nil {
		return nil, "", err
	}
	if value, ok := joinValues(input.Metrics, validAcquisitionMetric); !ok || value == "" {
		return nil, "", invalidArgument(operation, "metrics must contain unique supported Acquisition metrics")
	} else {
		query.Set("metrics", value)
	}
	if !setValues(query, "breakdowns", input.Breakdowns, validAcquisitionBreakdown) ||
		!setValues(query, "appIds", input.AppIDs, validFilterValue) ||
		!setValues(query, "campaignIds", input.CampaignIDs, validFilterValue) ||
		!setValues(query, "gameIds", input.GameIDs, validFilterValue) ||
		!setValues(query, "creativePackIds", input.CreativePackIDs, validFilterValue) ||
		!setValues(query, "creativePackTypes", input.CreativePackTypes, validCreativePackType) ||
		!setValues(query, "countries", input.Countries, validCountry) ||
		!setValues(query, "platforms", input.Platforms, validPlatform) ||
		!setValues(query, "eventTypes", input.EventTypes, validFilterValue) ||
		!setValues(query, "eventNames", input.EventNames, validFilterValue) {
		return nil, "", invalidArgument(operation, "breakdowns or report filters contain an invalid or duplicate value")
	}
	return query, format, nil
}

func prepareSKANRequest(input SKANReportRequest) (url.Values, ReportFormat, error) {
	const operation = "skan_report"
	query, format, err := prepareReportBase(operation, input.Start, input.End, input.Scale, input.Format, input.EOFMarker)
	if err != nil {
		return nil, "", err
	}
	if value, ok := joinValues(input.Metrics, validSKANMetric); !ok || value == "" {
		return nil, "", invalidArgument(operation, "metrics must contain unique supported SKAN metrics")
	} else {
		query.Set("metrics", value)
	}
	if !setValues(query, "breakdowns", input.Breakdowns, validSKANBreakdown) ||
		!setValues(query, "appIds", input.AppIDs, validFilterValue) ||
		!setValues(query, "campaignIds", input.CampaignIDs, validFilterValue) ||
		!setValues(query, "gameIds", input.GameIDs, validFilterValue) {
		return nil, "", invalidArgument(operation, "breakdowns or report filters contain an invalid or duplicate value")
	}
	return query, format, nil
}

func prepareReportBase(operation string, start, end time.Time, scale Scale, format ReportFormat, eofMarker bool) (url.Values, ReportFormat, error) {
	if start.IsZero() || end.IsZero() || !start.Before(end) || !validScale(scale) {
		return nil, "", invalidArgument(operation, "start, end, and scale are required; start must precede end")
	}
	if format == "" {
		format = FormatCSV
	}
	if format != FormatCSV && format != FormatJSON {
		return nil, "", invalidArgument(operation, "format must be csv or json")
	}
	if eofMarker && format != FormatCSV {
		return nil, "", invalidArgument(operation, "eofMarker is only valid for CSV reports")
	}
	query := make(url.Values)
	query.Set("start", start.UTC().Format(time.RFC3339Nano))
	query.Set("end", end.UTC().Format(time.RFC3339Nano))
	query.Set("scale", string(scale))
	query.Set("format", string(format))
	if eofMarker {
		query.Set("eofMarker", "true")
	}
	return query, format, nil
}

func validScale(value Scale) bool {
	switch value {
	case ScaleSummary, ScaleHour, ScaleDay, ScaleWeek, ScaleMonth:
		return true
	default:
		return false
	}
}

func validAcquisitionMetric(value AcquisitionMetric) bool {
	switch value {
	case MetricStarts, MetricViews, MetricClicks, MetricInstalls, MetricSpend, MetricCPI, MetricCTR, MetricCVR, MetricECPM:
		return true
	}
	text := string(value)
	if len(text) < 3 || text[0] != 'd' {
		return false
	}
	index := 1
	for index < len(text) && text[index] >= '0' && text[index] <= '9' {
		index++
	}
	day, err := strconv.Atoi(text[1:index])
	if err != nil || !validPostInstallDay(day) {
		return false
	}
	switch text[index:] {
	case "AdRevenue", "AdRevenueRoas", "IapRevenue", "IapRoas", "Purchases", "UniquePurchasers", "Retained",
		"RetentionRate", "TotalRoas", "LevelComplete", "CostPerLevelComplete", "LevelCompleteRate", "Payer", "PayerRate", "CostPerPayer":
		return true
	default:
		return false
	}
}

func validPostInstallDay(value int) bool {
	switch value {
	case 0, 1, 3, 7, 14, 21, 28:
		return true
	default:
		return false
	}
}

func validSKANMetric(value SKANMetric) bool {
	switch value {
	case SKANMetricStarts, SKANMetricViews, SKANMetricClicks, SKANMetricInstalls, SKANMetricSpend, SKANMetricCPI, SKANMetricCVR:
		return true
	default:
		return false
	}
}

func validAcquisitionBreakdown(value AcquisitionBreakdown) bool {
	switch value {
	case BreakdownApp, BreakdownCampaign, BreakdownCountry, BreakdownCreativePack, BreakdownCreativePackType,
		BreakdownOSVersion, BreakdownPlatform, BreakdownSourceAppID, BreakdownStore, BreakdownTargetGame,
		BreakdownEventType, BreakdownEventName:
		return true
	default:
		return false
	}
}

func validSKANBreakdown(value SKANBreakdown) bool {
	switch value {
	case SKANBreakdownApp, SKANBreakdownCampaign, SKANBreakdownConversionValue, SKANBreakdownTargetGame:
		return true
	default:
		return false
	}
}

func validCreativePackType(value CreativePackType) bool {
	return value == CreativePackVideo || value == CreativePackPlayable || value == CreativePackVideoPlayable
}

func validPlatform(value Platform) bool { return value == PlatformIOS || value == PlatformAndroid }

func validCountry(value CountryCode) bool {
	return strings.Contains(validCountryCodes, " "+string(value)+" ")
}

func validFilterValue[T ~string](value T) bool {
	text := string(value)
	return text != "" && text == strings.TrimSpace(text) && utf8.ValidString(text) && len(text) <= 256 &&
		!strings.ContainsAny(text, ",\x00\r\n")
}

func setValues[T ~string](query url.Values, name string, values []T, valid func(T) bool) bool {
	joined, ok := joinValues(values, valid)
	if !ok {
		return false
	}
	if joined != "" {
		query.Set(name, joined)
	}
	return true
}

func joinValues[T ~string](values []T, valid func(T) bool) (string, bool) {
	seen := make(map[T]struct{}, len(values))
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if !valid(value) {
			return "", false
		}
		if _, exists := seen[value]; exists {
			return "", false
		}
		seen[value] = struct{}{}
		parts = append(parts, string(value))
	}
	return strings.Join(parts, ","), true
}

func normalizedCompression(value Compression) (Compression, bool) {
	if value == "" {
		return CompressionIdentity, true
	}
	switch value {
	case CompressionIdentity, CompressionGzip, CompressionDeflate:
		return value, true
	default:
		return "", false
	}
}

const validCountryCodes = " AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS XK YE YT ZA ZM ZW "
