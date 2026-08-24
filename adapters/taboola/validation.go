package taboola

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func validOpaque(value string, maximum int) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= maximum &&
		!strings.ContainsAny(value, "\x00\r\n")
}

func validPathID(value string, numeric bool) bool {
	if !validOpaque(value, 256) || strings.ContainsAny(value, "/?#%") {
		return false
	}
	if numeric {
		_, err := strconv.ParseUint(value, 10, 64)
		return err == nil
	}
	return true
}

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= maximum && !strings.ContainsRune(value, '\x00')
}

func validOptionalText(value *string, maximum int) bool {
	return value == nil || validText(*value, maximum)
}

func validPositive(value *float64) bool {
	return value == nil || (*value > 0 && *value <= 1_000_000_000)
}

func validDate(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func validDateTime(value string) bool {
	if _, err := time.Parse("2006-01-02T15:04:05", value); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func validDestinationURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && len(value) <= 4096
}

func validDimension(value string) bool {
	if !validOpaque(value, 128) {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && character != '_' {
			return false
		}
	}
	return true
}

func validCampaignList(input ListCampaignsRequest) bool {
	if (input.Page == 0) != (input.PageSize == 0) || input.Page < 0 || input.PageSize < 0 || input.PageSize > 1000 {
		return false
	}
	if input.FetchLevel != "" && input.FetchLevel != FetchRecent && input.FetchLevel != FetchRecentAndPaused {
		return false
	}
	return input.Sort == "" || validOpaque(input.Sort, 256)
}

func validReportWindow(start, end string, realtime bool) bool {
	if realtime {
		if !validDateTime(start) || !validDateTime(end) {
			return false
		}
		startTime, first := parseReportTime(start)
		endTime, second := parseReportTime(end)
		return first && second && !endTime.Before(startTime)
	}
	if !validDate(start) || !validDate(end) || start == "" || end == "" {
		return false
	}
	startTime, _ := time.Parse("2006-01-02", start)
	endTime, _ := time.Parse("2006-01-02", end)
	return !endTime.Before(startTime)
}

func parseReportTime(value string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02T15:04:05", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
