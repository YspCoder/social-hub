package youtubereporting

import (
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.HasSuffix(value, "/")
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && parsed.User == nil && parsed.Fragment == "" &&
		(parsed.Scheme == "https" || parsed.Scheme == "http")
}

func validOpaque(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && len(value) <= maximum &&
		!strings.ContainsAny(value, "\x00\r\n")
}

func validText(value string, maximumCharacters int, required bool) bool {
	if required && strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) ||
		utf8.RuneCountInString(value) > maximumCharacters || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("_+-.", character) {
			continue
		}
		return false
	}
	return true
}

func validServiceAccountEmail(value string) bool {
	if value == "" || len(value) > 254 || value != strings.TrimSpace(value) || strings.Count(value, "@") != 1 || strings.ContainsAny(value, "\x00\r\n ") {
		return false
	}
	parts := strings.Split(value, "@")
	return parts[0] != "" && strings.HasSuffix(strings.ToLower(parts[1]), ".gserviceaccount.com")
}

func validOAuthScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > len(supportedScopes) {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope != analyticsReadScope && scope != analyticsRevenueScope {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func validListRequest(value ListRequest) bool {
	return value.PageSize >= 0 && (value.PageToken == "" || validOpaque(value.PageToken, 4096))
}

func validListReportsRequest(value ListReportsRequest) bool {
	if value.PageSize < 0 || value.PageToken != "" && !validOpaque(value.PageToken, 4096) {
		return false
	}
	for _, timestamp := range []time.Time{value.CreatedAfter, value.StartTimeAtOrAfter, value.StartTimeBefore} {
		if !timestamp.IsZero() && (timestamp.Year() < 1 || timestamp.Year() > 9999) {
			return false
		}
	}
	return value.StartTimeAtOrAfter.IsZero() || value.StartTimeBefore.IsZero() || value.StartTimeAtOrAfter.Before(value.StartTimeBefore)
}

func validReportTypeID(value string) bool { return validIdentifier(value, 100) }
func validJobID(value string) bool        { return validIdentifier(value, 40) }
func validReportID(value string) bool     { return validIdentifier(value, 1024) }

func validTimestamp(value string, required bool) bool {
	if value == "" {
		return !required
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && !parsed.IsZero()
}

func validReportType(value ReportType) bool {
	return len(value.Raw) > 0 && validReportTypeID(value.ID) && validText(value.Name, 100, true) && validTimestamp(value.DeprecateTime, false)
}

func validJob(value Job) bool {
	if len(value.Raw) == 0 || !validJobID(value.ID) || !validReportTypeID(value.ReportTypeID) || !validText(value.Name, 100, true) ||
		!validTimestamp(value.CreateTime, true) || !validTimestamp(value.ExpireTime, false) {
		return false
	}
	if value.ExpireTime == "" {
		return true
	}
	created, _ := time.Parse(time.RFC3339Nano, value.CreateTime)
	expires, _ := time.Parse(time.RFC3339Nano, value.ExpireTime)
	return created.Before(expires)
}

func validReport(value Report) bool {
	if len(value.Raw) == 0 || !validReportID(value.ID) || !validJobID(value.JobID) || !validTimestamp(value.StartTime, true) ||
		!validTimestamp(value.EndTime, true) || !validTimestamp(value.CreateTime, true) || !validTimestamp(value.JobExpireTime, false) ||
		!validDownloadMetadataURL(value.DownloadURL) {
		return false
	}
	start, _ := time.Parse(time.RFC3339Nano, value.StartTime)
	end, _ := time.Parse(time.RFC3339Nano, value.EndTime)
	return start.Before(end)
}

func validDownloadMetadataURL(value string) bool {
	if !validText(value, 1000, true) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validReportTypesResponse(value listReportTypesResponse) bool {
	if len(value.Raw) == 0 || value.NextPageToken != "" && !validOpaque(value.NextPageToken, 4096) {
		return false
	}
	seen := make(map[string]struct{}, len(value.ReportTypes))
	for _, reportType := range value.ReportTypes {
		if !validReportType(reportType) {
			return false
		}
		if _, exists := seen[reportType.ID]; exists {
			return false
		}
		seen[reportType.ID] = struct{}{}
	}
	return true
}

func validJobsResponse(value listJobsResponse) bool {
	if len(value.Raw) == 0 || value.NextPageToken != "" && !validOpaque(value.NextPageToken, 4096) {
		return false
	}
	seen := make(map[string]struct{}, len(value.Jobs))
	for _, job := range value.Jobs {
		if !validJob(job) {
			return false
		}
		if _, exists := seen[job.ID]; exists {
			return false
		}
		seen[job.ID] = struct{}{}
	}
	return true
}

func validReportsResponse(value listReportsResponse, jobID string) bool {
	if len(value.Raw) == 0 || value.NextPageToken != "" && !validOpaque(value.NextPageToken, 4096) {
		return false
	}
	seen := make(map[string]struct{}, len(value.Reports))
	for _, report := range value.Reports {
		if !validReport(report) || report.JobID != jobID {
			return false
		}
		if _, exists := seen[report.ID]; exists {
			return false
		}
		seen[report.ID] = struct{}{}
	}
	return true
}

func sanitizeCause(err error) error {
	if err == nil {
		return nil
	}
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
