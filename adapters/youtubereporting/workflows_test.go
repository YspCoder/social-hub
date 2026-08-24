package youtubereporting

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestCompleteContentOwnerWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("onBehalfOfContentOwner") != testOwnerID {
			t.Errorf("owner query=%v", request.URL.Query())
		}
		assertAPIRequest(t, request, request.Method, request.URL.Path)
		switch request.Method + " " + request.URL.Path {
		case "GET /v1/reportTypes":
			if request.URL.Query().Get("pageSize") != "25" || request.URL.Query().Get("pageToken") != "types-next" ||
				request.URL.Query().Get("includeSystemManaged") != "true" {
				t.Errorf("report type query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"reportTypes": []any{reportTypeFixture()}, "nextPageToken": "types-more"})
		case "POST /v1/jobs":
			var body map[string]any
			if json.NewDecoder(request.Body).Decode(&body) != nil || len(body) != 2 || body["reportTypeId"] != testReportTypeID || body["name"] != "Daily channel report" {
				t.Errorf("create body=%v", body)
			}
			writeJSON(t, writer, http.StatusOK, jobFixture())
		case "GET /v1/jobs/" + testJobID:
			writeJSON(t, writer, http.StatusOK, jobFixture())
		case "GET /v1/jobs":
			if request.URL.Query().Get("pageSize") != "10" || request.URL.Query().Get("pageToken") != "jobs-next" ||
				request.URL.Query().Get("includeSystemManaged") != "true" {
				t.Errorf("jobs query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"jobs": []any{jobFixture()}, "nextPageToken": "jobs-more"})
		case "DELETE /v1/jobs/" + testJobID:
			writer.WriteHeader(http.StatusNoContent)
		case "GET /v1/jobs/" + testJobID + "/reports":
			query := request.URL.Query()
			if query.Get("pageSize") != "50" || query.Get("pageToken") != "reports-next" ||
				query.Get("createdAfter") != "2026-08-01T01:02:03.123456Z" ||
				query.Get("startTimeAtOrAfter") != "2026-08-02T00:00:00Z" || query.Get("startTimeBefore") != "2026-08-03T00:00:00Z" {
				t.Errorf("reports query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"reports": []any{reportFixture("https://youtubereporting.googleapis.com/v1/media/report?alt=media")}, "nextPageToken": "reports-more"})
		case "GET /v1/jobs/" + testJobID + "/reports/" + testReportID:
			writeJSON(t, writer, http.StatusOK, reportFixture("https://youtubereporting.googleapis.com/v1/media/report?alt=media"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, ownerConfig(server.URL))

	types, err := client.ListReportTypes(context.Background(), ListRequest{PageSize: 25, PageToken: "types-next", IncludeSystemManaged: true})
	if err != nil || len(types.Items) != 1 || types.Items[0].ID != testReportTypeID || types.NextPageToken != "types-more" || len(types.Items[0].Raw) == 0 {
		t.Fatalf("types=%#v err=%v", types, err)
	}
	job, err := client.CreateJob(context.Background(), CreateJobInput{ReportTypeID: testReportTypeID, Name: "Daily channel report"})
	if err != nil || job.ID != testJobID || len(job.Raw) == 0 {
		t.Fatalf("created job=%#v err=%v", job, err)
	}
	job, err = client.GetJob(context.Background(), testJobID)
	if err != nil || job.ID != testJobID {
		t.Fatalf("job=%#v err=%v", job, err)
	}
	jobs, err := client.ListJobs(context.Background(), ListRequest{PageSize: 10, PageToken: "jobs-next", IncludeSystemManaged: true})
	if err != nil || len(jobs.Items) != 1 || jobs.NextPageToken != "jobs-more" {
		t.Fatalf("jobs=%#v err=%v", jobs, err)
	}
	if err := client.DeleteJob(context.Background(), testJobID); err != nil {
		t.Fatal(err)
	}
	zone := time.FixedZone("UTC+8", 8*60*60)
	reports, err := client.ListReports(context.Background(), testJobID, ListReportsRequest{
		PageSize: 50, PageToken: "reports-next",
		CreatedAfter:       time.Date(2026, 8, 1, 9, 2, 3, 123456789, zone),
		StartTimeAtOrAfter: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		StartTimeBefore:    time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC),
	})
	if err != nil || len(reports.Items) != 1 || reports.NextPageToken != "reports-more" || len(reports.Items[0].Raw) == 0 {
		t.Fatalf("reports=%#v err=%v", reports, err)
	}
	report, err := client.GetReport(context.Background(), testJobID, testReportID)
	if err != nil || report.ID != testReportID || report.JobID != testJobID {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}

func TestWorkflowValidationRejectsBeforeRequest(t *testing.T) {
	requestSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requestSeen = true }))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	badTimes := ListReportsRequest{
		StartTimeAtOrAfter: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC),
		StartTimeBefore:    time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
	}
	tests := []func() error{
		func() error {
			_, err := client.ListReportTypes(context.Background(), ListRequest{PageSize: -1})
			return err
		},
		func() error { _, err := client.CreateJob(context.Background(), CreateJobInput{}); return err },
		func() error { _, err := client.GetJob(context.Background(), "bad/id"); return err },
		func() error {
			_, err := client.ListJobs(context.Background(), ListRequest{PageToken: " bad"})
			return err
		},
		func() error { return client.DeleteJob(context.Background(), "") },
		func() error { _, err := client.ListReports(context.Background(), testJobID, badTimes); return err },
		func() error { _, err := client.GetReport(context.Background(), testJobID, "bad/id"); return err },
	}
	for index, invoke := range tests {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("case %d error=%v", index, err)
		}
	}
	if requestSeen {
		t.Fatal("invalid workflow input reached server")
	}
}

func TestWorkflowRejectsInvalidPlatformContracts(t *testing.T) {
	responses := []any{
		map[string]any{"reportTypes": []any{reportTypeFixture(), reportTypeFixture()}},
		map[string]any{"jobs": []any{map[string]any{"id": "bad/id"}}},
		map[string]any{"reports": []any{reportFixture("https://example.com/v1/media/x?alt=media"), reportFixture("https://example.com/v1/media/x?alt=media")}},
	}
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, http.StatusOK, responses[index])
		index++
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	for _, invoke := range []func() error{
		func() error { _, err := client.ListReportTypes(context.Background(), ListRequest{}); return err },
		func() error { _, err := client.ListJobs(context.Background(), ListRequest{}); return err },
		func() error {
			_, err := client.ListReports(context.Background(), testJobID, ListReportsRequest{})
			return err
		},
	} {
		err := invoke()
		if requireHubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	}
}

func TestListRejectsMoreItemsThanRequested(t *testing.T) {
	second := reportTypeFixture()
	second["id"] = "channel_combined_a3"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, http.StatusOK, map[string]any{"reportTypes": []any{reportTypeFixture(), second}})
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	_, err := client.ListReportTypes(context.Background(), ListRequest{PageSize: 1})
	if requireHubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("error=%v", err)
	}
}

func TestHTTPErrorPreservesGoogleDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "2")
		writer.Header().Set("x-goog-request-id", "request-1")
		writeJSON(t, writer, http.StatusTooManyRequests, map[string]any{"error": map[string]any{
			"code": 429, "status": "RESOURCE_EXHAUSTED", "message": "quota exceeded",
			"errors": []any{map[string]any{"reason": "quotaExceeded", "domain": "youtube.reporting"}},
		}})
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	_, err := client.ListJobs(context.Background(), ListRequest{})
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.Hub.Code != socialhub.CodeRateLimited || !apiError.Retryable() ||
		apiError.Hub.RequestID != "request-1" || apiError.Hub.RetryAfter != 2*time.Second || apiError.Hub.Op != "job_list" {
		t.Fatalf("error=%#v", err)
	}
}
