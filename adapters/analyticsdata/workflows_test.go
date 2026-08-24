package analyticsdata

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

func TestMinuteRangeExplicitZeroWireValue(t *testing.T) {
	encoded, err := json.Marshal(MinuteRange{StartMinutesAgo: 0, EndMinutesAgo: 0})
	if err != nil || !strings.Contains(string(encoded), `"startMinutesAgo":0`) {
		t.Fatalf("minute range JSON=%s err=%v", encoded, err)
	}
}

func TestAnalyticsDataWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		base := "/v1beta/" + propertyName()
		switch request.URL.Path {
		case base + "/metadata":
			assertAPIRequest(t, request, http.MethodGet, base+"/metadata")
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"name":        propertyName() + "/metadata",
				"dimensions":  []any{map[string]any{"apiName": "country", "uiName": "Country"}},
				"metrics":     []any{map[string]any{"apiName": "eventCount", "type": MetricTypeInteger, "uiName": "Event count"}},
				"comparisons": []any{map[string]any{"apiName": "comparisons/42", "uiName": "All users"}},
			})
		case base + ":checkCompatibility":
			assertAPIRequest(t, request, http.MethodPost, base+":checkCompatibility")
			var input CheckCompatibilityRequest
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.Dimensions[0].Name != "country" {
				t.Fatalf("compatibility body=%#v err=%v", input, err)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"dimensionCompatibilities": []any{map[string]any{"dimensionMetadata": map[string]any{"apiName": "city"}, "compatibility": Compatible}},
				"metricCompatibilities":    []any{map[string]any{"metricMetadata": map[string]any{"apiName": "sessions", "type": MetricTypeInteger}, "compatibility": Compatible}},
			})
		case base + ":runReport":
			assertAPIRequest(t, request, http.MethodPost, base+":runReport")
			var input RunReportRequest
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.Limit != 100 {
				t.Fatalf("report body=%#v err=%v", input, err)
			}
			writeJSON(t, writer, http.StatusOK, reportFixture("analyticsData#runReport", "country"))
		case base + ":batchRunReports":
			assertAPIRequest(t, request, http.MethodPost, base+":batchRunReports")
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"reports": []any{reportFixture("analyticsData#runReport", "country")}, "kind": "analyticsData#batchRunReports",
			})
		case base + ":runRealtimeReport":
			assertAPIRequest(t, request, http.MethodPost, base+":runRealtimeReport")
			writeJSON(t, writer, http.StatusOK, reportFixture("analyticsData#runRealtimeReport", "country", "dateRange"))
		case base + ":runPivotReport":
			assertAPIRequest(t, request, http.MethodPost, base+":runPivotReport")
			writeJSON(t, writer, http.StatusOK, pivotFixture())
		case base + ":batchRunPivotReports":
			assertAPIRequest(t, request, http.MethodPost, base+":batchRunPivotReports")
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"pivotReports": []any{pivotFixture()}, "kind": "analyticsData#batchRunPivotReports",
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)

	metadata, err := client.GetMetadata(context.Background())
	if err != nil || metadata.Name != propertyName()+"/metadata" || len(metadata.Raw) == 0 {
		t.Fatalf("metadata=%#v err=%v", metadata, err)
	}
	compatibility, err := client.CheckCompatibility(context.Background(), CheckCompatibilityRequest{
		Dimensions: []Dimension{{Name: "country"}}, Metrics: []Metric{{Name: "eventCount"}}, CompatibilityFilter: Compatible,
	})
	if err != nil || len(compatibility.DimensionCompatibilities) != 1 || len(compatibility.Raw) == 0 {
		t.Fatalf("compatibility=%#v err=%v", compatibility, err)
	}
	report, err := client.RunReport(context.Background(), reportRequest(), socialhub.WithRequestID("request-1"))
	if err != nil || report.RowCount != 1 || len(report.Raw) == 0 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	batch, err := client.BatchRunReports(context.Background(), BatchRunReportsRequest{Requests: []RunReportRequest{reportRequest()}})
	if err != nil || len(batch.Reports) != 1 || len(batch.Raw) == 0 {
		t.Fatalf("batch=%#v err=%v", batch, err)
	}
	realtime, err := client.RunRealtimeReport(context.Background(), RunRealtimeReportRequest{
		Dimensions: []Dimension{{Name: "country"}}, Metrics: []Metric{{Name: "eventCount"}}, Limit: 100,
		MinuteRanges: []MinuteRange{{StartMinutesAgo: 29, EndMinutesAgo: 15}, {StartMinutesAgo: 14, EndMinutesAgo: 0}},
	})
	if err != nil || realtime.RowCount != 1 {
		t.Fatalf("realtime=%#v err=%v", realtime, err)
	}
	pivot, err := client.RunPivotReport(context.Background(), pivotRequest())
	if err != nil || len(pivot.PivotHeaders) != 2 || len(pivot.Raw) == 0 {
		t.Fatalf("pivot=%#v err=%v", pivot, err)
	}
	pivotBatch, err := client.BatchRunPivotReports(context.Background(), BatchRunPivotReportsRequest{Requests: []RunPivotReportRequest{pivotRequest()}})
	if err != nil || len(pivotBatch.PivotReports) != 1 || len(pivotBatch.Raw) == 0 {
		t.Fatalf("pivot batch=%#v err=%v", pivotBatch, err)
	}
}

func TestWorkflowRequestValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newStaticClient(t, server)
	tests := []func() error{
		func() error {
			_, err := client.CheckCompatibility(context.Background(), CheckCompatibilityRequest{})
			return err
		},
		func() error { _, err := client.RunReport(context.Background(), RunReportRequest{}); return err },
		func() error {
			_, err := client.BatchRunReports(context.Background(), BatchRunReportsRequest{})
			return err
		},
		func() error {
			_, err := client.RunRealtimeReport(context.Background(), RunRealtimeReportRequest{})
			return err
		},
		func() error {
			_, err := client.RunPivotReport(context.Background(), RunPivotReportRequest{})
			return err
		},
		func() error {
			_, err := client.BatchRunPivotReports(context.Background(), BatchRunPivotReportsRequest{})
			return err
		},
	}
	for index, invoke := range tests {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("case %d error=%v", index, err)
		}
	}
}

func TestWorkflowResponseContractErrors(t *testing.T) {
	report := reportRequest()
	pivot := pivotRequest()
	tests := []struct {
		path   string
		body   any
		invoke func(*Client) error
	}{
		{"/v1beta/" + propertyName() + "/metadata", map[string]any{"name": "properties/999/metadata"}, func(client *Client) error { _, err := client.GetMetadata(context.Background()); return err }},
		{"/v1beta/" + propertyName() + ":checkCompatibility", map[string]any{"dimensionCompatibilities": []any{map[string]any{"dimensionMetadata": map[string]any{"apiName": "country"}, "compatibility": "BROKEN"}}}, func(client *Client) error {
			_, err := client.CheckCompatibility(context.Background(), CheckCompatibilityRequest{Dimensions: []Dimension{{Name: "country"}}})
			return err
		}},
		{"/v1beta/" + propertyName() + ":runReport", reportFixture("wrong-kind", "country"), func(client *Client) error { _, err := client.RunReport(context.Background(), report); return err }},
		{"/v1beta/" + propertyName() + ":batchRunReports", map[string]any{"reports": []any{}, "kind": "analyticsData#batchRunReports"}, func(client *Client) error {
			_, err := client.BatchRunReports(context.Background(), BatchRunReportsRequest{Requests: []RunReportRequest{report}})
			return err
		}},
		{"/v1beta/" + propertyName() + ":runRealtimeReport", reportFixture("analyticsData#runRealtimeReport", "wrong"), func(client *Client) error {
			_, err := client.RunRealtimeReport(context.Background(), RunRealtimeReportRequest{Dimensions: []Dimension{{Name: "country"}}, Metrics: []Metric{{Name: "eventCount"}}})
			return err
		}},
		{"/v1beta/" + propertyName() + ":runPivotReport", reportFixture("analyticsData#runPivotReport", "country"), func(client *Client) error { _, err := client.RunPivotReport(context.Background(), pivot); return err }},
		{"/v1beta/" + propertyName() + ":batchRunPivotReports", map[string]any{"pivotReports": []any{}, "kind": "analyticsData#batchRunPivotReports"}, func(client *Client) error {
			_, err := client.BatchRunPivotReports(context.Background(), BatchRunPivotReportsRequest{Requests: []RunPivotReportRequest{pivot}})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.path {
					t.Fatalf("path=%s", request.URL.Path)
				}
				writeJSON(t, writer, http.StatusOK, test.body)
			}))
			defer server.Close()
			_, client := newStaticClient(t, server)
			err := test.invoke(client)
			hub := requireHubError(t, err)
			if hub.Code != socialhub.CodePlatformError || hub.Class != socialhub.ClassPermanent {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestAPIErrorMappingAndRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "1.5")
		writer.Header().Set("x-goog-request-id", "google-request")
		writeJSON(t, writer, http.StatusTooManyRequests, map[string]any{"error": map[string]any{
			"code": 429, "status": "RESOURCE_EXHAUSTED", "message": "access_token: should-not-leak",
			"details": []any{map[string]string{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "RATE_LIMIT_EXCEEDED", "domain": "analyticsdata.googleapis.com"}},
		}})
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	_, err := client.GetMetadata(context.Background())
	var api *APIError
	if !errors.As(err, &api) || !api.Retryable() || api.Hub.Code != socialhub.CodeRateLimited || api.Hub.RequestID != "google-request" ||
		api.Hub.RetryAfter <= 0 || api.Google.Message == "access_token: should-not-leak" {
		t.Fatalf("api error=%#v err=%v", api, err)
	}
}
