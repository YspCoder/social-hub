package youtubeanalytics

import (
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"strconv"
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

func validText(value string, maximum int, required bool) bool {
	if required && strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) ||
		len([]byte(value)) > maximum || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validAccountBinding(value AccountSettings) bool {
	channel := validIdentifier(value.ChannelID, 256)
	owner := validIdentifier(value.ContentOwnerID, 256)
	return channel != owner
}

func validIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("_+-", character) {
			continue
		}
		return false
	}
	return true
}

func validOpaqueID(value string) bool {
	return validText(value, 1024, true) && !strings.ContainsRune(value, ',')
}

func validFieldName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

func validOAuthScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > len(supportedScopes) {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		valid := false
		for _, supported := range supportedScopes {
			valid = valid || scope == supported
		}
		if !valid {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func validDate(value string) (time.Time, bool) {
	if len(value) != len("2006-01-02") {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", value)
	return parsed, err == nil && parsed.Format("2006-01-02") == value
}

func validCurrency(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validReportQuery(value ReportQuery, contentOwner bool) bool {
	start, startOK := validDate(value.StartDate)
	end, endOK := validDate(value.EndDate)
	if !startOK || !endOK || start.After(end) || len(value.Metrics) == 0 || len(value.Metrics) > 100 ||
		len(value.Dimensions) > 100 || len(value.Filters) > 100 || len(value.Sort) > 100 || !validCurrency(value.Currency) ||
		value.MaxResults < 0 || value.StartIndex < 0 || value.IncludeHistoricalChannelData && !contentOwner {
		return false
	}
	requested := make(map[string]struct{}, len(value.Metrics)+len(value.Dimensions))
	for _, metric := range value.Metrics {
		name := string(metric)
		if !validFieldName(name) {
			return false
		}
		if _, found := requested[name]; found {
			return false
		}
		requested[name] = struct{}{}
	}
	for _, dimension := range value.Dimensions {
		name := string(dimension)
		if !validFieldName(name) {
			return false
		}
		if _, found := requested[name]; found {
			return false
		}
		requested[name] = struct{}{}
	}
	filterDimensions := make(map[string]struct{}, len(value.Filters))
	videoCount := 0
	for _, filter := range value.Filters {
		name := string(filter.Dimension)
		if !validFieldName(name) || len(filter.Values) == 0 || len(filter.Values) > DefaultQuotaPolicy().MaximumFilterIDs {
			return false
		}
		if _, found := filterDimensions[name]; found {
			return false
		}
		filterDimensions[name] = struct{}{}
		if len(filter.Values) > 1 && name != string(DimensionVideo) && name != string(DimensionPlaylist) && name != string(DimensionChannel) {
			return false
		}
		seenValues := make(map[string]struct{}, len(filter.Values))
		for _, filterValue := range filter.Values {
			if !validFilterValue(filterValue) {
				return false
			}
			if _, found := seenValues[filterValue]; found {
				return false
			}
			seenValues[filterValue] = struct{}{}
		}
		if name == string(DimensionVideo) {
			videoCount = len(filter.Values)
		}
	}
	sortNames := make(map[string]struct{}, len(value.Sort))
	for _, sort := range value.Sort {
		if _, found := requested[sort.Name]; !found {
			return false
		}
		if _, found := sortNames[sort.Name]; found {
			return false
		}
		sortNames[sort.Name] = struct{}{}
	}
	if value.StartIndex != 0 && value.StartIndex < 1 {
		return false
	}
	if videoCount > 0 && isTrafficSourceReport(value) {
		days := int(end.Sub(start)/(24*time.Hour)) + 1
		if videoCount*days > DefaultQuotaPolicy().MaximumTrafficSourceCost {
			return false
		}
	}
	return true
}

func validFilterValue(value string) bool {
	return validText(value, 1024, true) && !strings.ContainsAny(value, ";,") && !strings.Contains(value, "==")
}

func isTrafficSourceReport(value ReportQuery) bool {
	for _, dimension := range value.Dimensions {
		if dimension == DimensionTrafficSourceType || dimension == DimensionTrafficSourceDetail {
			return true
		}
	}
	return false
}

func requiresMonetaryScope(value ReportQuery) bool {
	if value.Monetary {
		return true
	}
	for _, metric := range value.Metrics {
		if _, found := monetaryMetrics[metric]; found {
			return true
		}
	}
	return false
}

var monetaryMetrics = map[Metric]struct{}{
	"estimatedRevenue": {}, "estimatedAdRevenue": {}, "estimatedRedPartnerRevenue": {},
	"grossRevenue": {}, "cpm": {}, "playbackBasedCpm": {}, "adImpressions": {}, "monetizedPlaybacks": {},
}

func validReportResponse(value *Report, request ReportQuery) bool {
	if value == nil || value.Kind != "youtubeAnalytics#resultTable" || value.Errors != nil || len(value.Raw) == 0 ||
		request.MaxResults > 0 && len(value.Rows) > int(request.MaxResults) {
		return false
	}
	if len(value.ColumnHeaders) != len(request.Dimensions)+len(request.Metrics) {
		return false
	}
	index := 0
	for _, dimension := range request.Dimensions {
		header := value.ColumnHeaders[index]
		if header.Name != string(dimension) || header.ColumnType != ColumnDimension || !validDataType(header.DataType) {
			return false
		}
		index++
	}
	for _, metric := range request.Metrics {
		header := value.ColumnHeaders[index]
		if header.Name != string(metric) || header.ColumnType != ColumnMetric || !validDataType(header.DataType) {
			return false
		}
		index++
	}
	for _, row := range value.Rows {
		if len(row) != len(value.ColumnHeaders) {
			return false
		}
		for column, cell := range row {
			if !validCell(value.ColumnHeaders[column].DataType, cell) {
				return false
			}
		}
	}
	return true
}

func validDataType(value DataType) bool {
	return value == DataString || value == DataInteger || value == DataFloat || value == DataBoolean
}

func validCell(dataType DataType, value any) bool {
	switch dataType {
	case DataString:
		_, ok := value.(string)
		return ok
	case DataInteger:
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		parsed, err := strconv.ParseFloat(number.String(), 64)
		return err == nil && !math.IsInf(parsed, 0) && !math.IsNaN(parsed) && math.Trunc(parsed) == parsed
	case DataFloat:
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		parsed, err := strconv.ParseFloat(number.String(), 64)
		return err == nil && !math.IsInf(parsed, 0) && !math.IsNaN(parsed)
	case DataBoolean:
		_, ok := value.(bool)
		return ok
	default:
		return false
	}
}

func validResourceKind(value ResourceKind, contentOwner bool) bool {
	if contentOwner {
		return value == ResourceChannel || value == ResourcePlaylist || value == ResourceVideo || value == ResourceAsset
	}
	return value == ResourcePlaylist || value == ResourceVideo
}

func validListGroupsRequest(value ListGroupsRequest) bool {
	if value.Mine == (len(value.IDs) > 0) || len(value.IDs) > DefaultQuotaPolicy().MaximumFilterIDs ||
		value.PageToken != "" && !validOpaque(value.PageToken, 4096) {
		return false
	}
	seen := make(map[string]struct{}, len(value.IDs))
	for _, id := range value.IDs {
		if !validOpaqueID(id) {
			return false
		}
		if _, found := seen[id]; found {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}

func validGroup(value *Group, contentOwner bool) bool {
	if value == nil || value.Kind != "youtube#group" || value.Errors != nil || len(value.Raw) == 0 ||
		!validOpaqueID(value.ID) || value.Snippet == nil || !validText(value.Snippet.Title, 1024, true) ||
		value.ContentDetails == nil || !validResourceKind(value.ContentDetails.ItemType, contentOwner) ||
		value.ContentDetails.ItemCount > uint64(DefaultQuotaPolicy().MaximumGroupItems) {
		return false
	}
	if value.Snippet.PublishedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, value.Snippet.PublishedAt); err != nil {
			return false
		}
	}
	return true
}

func validGroupsResponse(value *ListGroupsResponse, contentOwner bool) bool {
	if value == nil || value.Kind != "youtube#groupListResponse" || value.Errors != nil || len(value.Raw) == 0 ||
		value.NextPageToken != "" && !validOpaque(value.NextPageToken, 4096) {
		return false
	}
	seen := make(map[string]struct{}, len(value.Items))
	for index := range value.Items {
		if !validGroup(&value.Items[index], contentOwner) {
			return false
		}
		if _, found := seen[value.Items[index].ID]; found {
			return false
		}
		seen[value.Items[index].ID] = struct{}{}
	}
	return true
}

func validGroupItem(value *GroupItem, contentOwner bool) bool {
	return value != nil && value.Kind == "youtube#groupItem" && value.Errors == nil && len(value.Raw) > 0 &&
		validOpaqueID(value.ID) && validOpaqueID(value.GroupID) && value.Resource != nil &&
		validOpaqueID(value.Resource.ID) && validResourceKind(value.Resource.Kind, contentOwner)
}

func validGroupItemsResponse(value *ListGroupItemsResponse, contentOwner bool) bool {
	if value == nil || value.Kind != "youtube#groupItemListResponse" || value.Errors != nil || len(value.Raw) == 0 ||
		len(value.Items) > DefaultQuotaPolicy().MaximumGroupItems {
		return false
	}
	seen := make(map[string]struct{}, len(value.Items))
	for index := range value.Items {
		if !validGroupItem(&value.Items[index], contentOwner) {
			return false
		}
		if _, found := seen[value.Items[index].ID]; found {
			return false
		}
		seen[value.Items[index].ID] = struct{}{}
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
