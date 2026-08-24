package admob

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestMalformedReportStreamsAreRejected(t *testing.T) {
	valid := reportResponse([]Dimension{DimensionDate}, []Metric{MetricClicks})
	encode := func(value any) string {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	mutatedMetric := reportResponse([]Dimension{DimensionDate}, []Metric{MetricClicks})
	mutatedMetric[1] = map[string]any{"row": map[string]any{
		"dimensionValues": map[string]any{"DATE": map[string]any{"value": "20260809"}},
		"metricValues":    map[string]any{"CLICKS": map[string]any{"integerValue": "1", "doubleValue": 0.5}},
	}}
	wrongKind := reportResponse([]Dimension{DimensionDate}, []Metric{MetricClicks})
	wrongKind[1] = map[string]any{"row": map[string]any{
		"dimensionValues": map[string]any{"DATE": map[string]any{"value": "20260809"}},
		"metricValues":    map[string]any{"CLICKS": map[string]any{"doubleValue": 0.5}},
	}}
	badHeader := reportResponse([]Dimension{DimensionDate}, []Metric{MetricClicks})
	badHeader[0] = map[string]any{"header": map[string]any{
		"dateRange":            DateRange{StartDate: Date{Year: 2026, Month: 2, Day: 30}, EndDate: validDateFixture()},
		"localizationSettings": LocalizationSettings{CurrencyCode: "USD", LanguageCode: "en-US"}, "reportingTimeZone": "America/Los_Angeles",
	}}
	badFooter := reportResponse([]Dimension{DimensionDate}, []Metric{MetricClicks})
	badFooter[2] = map[string]any{"footer": map[string]any{"matchingRowCount": "bad"}}
	extraRow := reportResponse([]Dimension{DimensionDate}, []Metric{MetricClicks})
	extraRow = append(extraRow[:2], extraRow[1], extraRow[2])
	tests := []string{
		`{"header":{}}`,
		`[]`,
		encode(valid[:2]),
		encode([]any{valid[2], valid[0]}),
		encode([]any{map[string]any{"header": map[string]any{}, "row": map[string]any{}}}),
		encode(mutatedMetric),
		encode(wrongKind),
		encode(badHeader),
		encode(badFooter),
		encode(extraRow),
		encode(valid) + `{}`,
	}
	for index, responseBody := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(responseBody))
		}))
		_, client := newStaticClient(t, server)
		spec := validNetworkSpec()
		spec.Metrics = []Metric{MetricClicks}
		spec.MaxReportRows = 1
		_, err := client.GenerateNetworkReport(context.Background(), spec)
		server.Close()
		if requireHubError(t, err).Code != socialhub.CodePlatformError {
			t.Errorf("case %d error=%v", index, err)
		}
	}
}

func TestMetricAndFooterUnmarshalValidation(t *testing.T) {
	for _, input := range []string{
		`{}`, `{"unknown":"1"}`, `{"integerValue":"not-int"}`, `{"microsValue":"9223372036854775808"}`,
		`{"doubleValue":1e999}`, `{"integerValue":null}`,
	} {
		var value MetricValue
		if json.Unmarshal([]byte(input), &value) == nil {
			t.Errorf("metric %s accepted", input)
		}
	}
	for _, input := range []string{`{}`, `{"matchingRowCount":"-1"}`, `{"matchingRowCount":"bad"}`} {
		var value ReportFooter
		if json.Unmarshal([]byte(input), &value) == nil {
			t.Errorf("footer %s accepted", input)
		}
	}
	var metric MetricValue
	if err := json.Unmarshal([]byte(`{"integerValue":"5"}`), &metric); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(metric)
	if err != nil || string(encoded) != `{"integerValue":"5"}` {
		t.Fatalf("metric JSON=%s err=%v", encoded, err)
	}
	footer := ReportFooter{MatchingRowCount: 7}
	encoded, err = json.Marshal(footer)
	if err != nil || string(encoded) != `{"matchingRowCount":"7"}` {
		t.Fatalf("footer JSON=%s err=%v", encoded, err)
	}
}

func TestReportHTTPAndTransportErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-Goog-Request-Id", "request-9")
		writer.Header().Set("Retry-After", "3")
		writeJSON(t, writer, http.StatusTooManyRequests, map[string]any{"error": map[string]any{
			"code": 429, "status": "RESOURCE_EXHAUSTED", "message": "quota", "details": []any{map[string]any{"reason": "QUOTA_EXCEEDED"}},
		}})
	}))
	_, client := newStaticClient(t, server)
	_, err := client.GenerateNetworkReport(context.Background(), validNetworkSpec())
	var api *APIError
	if !errors.As(err, &api) || !api.Retryable() || api.Hub.Code != socialhub.CodeRateLimited || api.Hub.RequestID != "request-9" {
		t.Fatalf("API error=%#v", api)
	}
	server.Close()

	oversized := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(strings.Repeat("x", int(maxErrorResponseBytes)+1)))
	}))
	defer oversized.Close()
	_, client = newStaticClient(t, oversized)
	if _, err := client.GenerateNetworkReport(context.Background(), validNetworkSpec()); requireHubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("oversized error=%v", err)
	}
}
