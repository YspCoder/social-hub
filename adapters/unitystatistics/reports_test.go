package unitystatistics

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestReportWorkflowRoutesQueriesAndStreaming(t *testing.T) {
	const acquisitionCSV = "timestamp,campaign name,clicks\n2026-08-09,\"Launch\nCampaign\",10\n2026-08-10,Other,5\n#__EOF__,rows=2,\n"
	const skanJSON = `{"data":[{"timestamp":"2026-08-09","installs":4}]}`
	seen := make(map[string]bool)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testBearerToken {
			t.Errorf("Authorization=%q", request.Header.Get("Authorization"))
		}
		key := request.Method + " " + request.URL.Path
		if seen[key] {
			t.Errorf("duplicate route=%s", key)
		}
		seen[key] = true
		switch request.URL.Path {
		case "/advertise/stats/v2/organizations/" + testOrganizationID + "/reports/acquisitions":
			query := request.URL.Query()
			if request.Header.Get("Accept") != "text/csv" || request.Header.Get("Accept-Encoding") != "identity" ||
				query.Get("start") != "2026-08-09T00:00:00Z" || query.Get("end") != "2026-08-11T00:00:00Z" ||
				query.Get("scale") != "day" || query.Get("metrics") != "clicks,d0Payer,d28CostPerPayer" ||
				query.Get("breakdowns") != "campaign,country" || query.Get("appIds") != "legacy-app,unity-app" ||
				query.Get("campaignIds") != "campaign-1" || query.Get("gameIds") != "game-1" ||
				query.Get("creativePackIds") != "pack-1" || query.Get("creativePackTypes") != "video,playable" ||
				query.Get("countries") != "US,CN" || query.Get("platforms") != "ios,android" ||
				query.Get("eventTypes") != "purchase" || query.Get("eventNames") != "buy_10_diamonds" ||
				query.Get("format") != "csv" || query.Get("eofMarker") != "true" {
				t.Errorf("headers=%v query=%v", request.Header, query)
			}
			writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
			writer.Header().Set("RateLimit-Policy", "1;w=1, 30;w=1800")
			writer.Header().Set("RateLimit", "limit=1, remaining=0, reset=1")
			writer.Header().Set("Unity-RateLimit", "limit=1, remaining=0, reset=1; limit=30, remaining=29, reset=42")
			_, _ = io.WriteString(writer, acquisitionCSV)
		case "/advertise/stats/v2/organizations/" + testOrganizationID + "/reports/skan":
			query := request.URL.Query()
			if request.Header.Get("Accept") != "application/json" || request.Header.Get("Accept-Encoding") != "gzip" ||
				query.Get("metrics") != "installs,spend" || query.Get("breakdowns") != "app" || query.Get("appIds") != "app-1" ||
				query.Get("campaignIds") != "campaign-1" || query.Get("gameIds") != "game-1" || query.Get("format") != "json" || query.Has("eofMarker") {
				t.Errorf("headers=%v query=%v", request.Header, query)
			}
			writer.Header().Set("Content-Type", "application/json")
			writer.Header().Set("Content-Encoding", "gzip")
			compressed := gzip.NewWriter(writer)
			_, _ = io.WriteString(compressed, skanJSON)
			_ = compressed.Close()
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	start := time.Date(2026, 8, 9, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	end := start.Add(48 * time.Hour)
	var acquisition bytes.Buffer
	result, err := client.Reports().DownloadAcquisitionsReport(context.Background(), AcquisitionsReportRequest{
		Start: start, End: end, Scale: ScaleDay,
		Metrics:    []AcquisitionMetric{MetricClicks, MetricD0Payer, MetricD28CostPerPayer},
		Breakdowns: []AcquisitionBreakdown{BreakdownCampaign, BreakdownCountry},
		AppIDs:     []string{"legacy-app", "unity-app"}, CampaignIDs: []string{"campaign-1"}, GameIDs: []string{"game-1"},
		CreativePackIDs: []string{"pack-1"}, CreativePackTypes: []CreativePackType{CreativePackVideo, CreativePackPlayable},
		Countries: []CountryCode{"US", "CN"}, Platforms: []Platform{PlatformIOS, PlatformAndroid},
		EventTypes: []string{"purchase"}, EventNames: []string{"buy_10_diamonds"}, Format: FormatCSV, EOFMarker: true,
	}, &acquisition, DownloadOptions{})
	if err != nil || acquisition.String() != acquisitionCSV || !result.EOFVerified || result.DataRows != 2 || result.BytesWritten != int64(len(acquisitionCSV)) ||
		result.Format != FormatCSV || result.ContentType == "" || result.RateLimitPolicy == "" || result.RateLimit == "" || result.UnityRateLimit == "" {
		t.Fatalf("acquisition=%q result=%#v err=%v", acquisition.String(), result, err)
	}
	var skan bytes.Buffer
	result, err = client.Reports().DownloadSKANReport(context.Background(), SKANReportRequest{
		Start: start, End: end, Scale: ScaleDay, Metrics: []SKANMetric{SKANMetricInstalls, SKANMetricSpend},
		Breakdowns: []SKANBreakdown{SKANBreakdownApp}, AppIDs: []string{"app-1"}, CampaignIDs: []string{"campaign-1"},
		GameIDs: []string{"game-1"}, Format: FormatJSON,
	}, &skan, DownloadOptions{Compression: CompressionGzip})
	if err != nil || skan.String() != skanJSON || result.ContentEncoding != "gzip" || result.Format != FormatJSON || result.EOFVerified {
		t.Fatalf("skan=%q result=%#v err=%v", skan.String(), result, err)
	}
	if len(seen) != 2 {
		t.Fatalf("covered routes=%v", seen)
	}
}

func TestDeflateAndNoContentReports(t *testing.T) {
	const report = "timestamp,clicks\n2026-08-09,3\n"
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if calls.Add(1) == 1 {
			if request.Header.Get("Accept-Encoding") != "deflate" {
				t.Errorf("Accept-Encoding=%q", request.Header.Get("Accept-Encoding"))
			}
			writer.Header().Set("Content-Type", "text/csv")
			writer.Header().Set("Content-Encoding", "deflate")
			compressed := zlib.NewWriter(writer)
			_, _ = io.WriteString(compressed, report)
			_ = compressed.Close()
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	var output bytes.Buffer
	result, err := client.DownloadAcquisitionsReport(context.Background(), acquisitionRequest(false), &output, DownloadOptions{Compression: CompressionDeflate})
	if err != nil || output.String() != report || result.ContentEncoding != "deflate" {
		t.Fatalf("output=%q result=%#v err=%v", output.String(), result, err)
	}
	output.Reset()
	result, err = client.DownloadAcquisitionsReport(context.Background(), acquisitionRequest(false), &output, DownloadOptions{})
	if err != nil || !result.NoData || result.StatusCode != http.StatusNoContent || output.Len() != 0 {
		t.Fatalf("result=%#v output=%q err=%v", result, output.String(), err)
	}
}

func TestReportResponseFailures(t *testing.T) {
	mode := "missing-eof"
	redirected := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch mode {
		case "missing-eof":
			writer.Header().Set("Content-Type", "text/csv")
			_, _ = io.WriteString(writer, "a,b\n1,2\n")
		case "bad-count":
			writer.Header().Set("Content-Type", "text/csv")
			_, _ = io.WriteString(writer, "a,b\n1,2\n#__EOF__,rows=2\n")
		case "trailing":
			writer.Header().Set("Content-Type", "text/csv")
			_, _ = io.WriteString(writer, "a,b\n1,2\n#__EOF__,rows=1\n3,4\n")
		case "bad-csv":
			writer.Header().Set("Content-Type", "text/csv")
			_, _ = io.WriteString(writer, "a,b\n\"unterminated\n")
		case "bad-columns":
			writer.Header().Set("Content-Type", "text/csv")
			_, _ = io.WriteString(writer, "a,b\n1,2,3\n#__EOF__,rows=1\n")
		case "wrong-type":
			writer.Header().Set("Content-Type", "application/octet-stream")
			_, _ = io.WriteString(writer, "12345")
		case "too-large":
			writer.Header().Set("Content-Type", "text/csv")
			_, _ = io.WriteString(writer, "12345")
		case "unexpected-encoding":
			writer.Header().Set("Content-Type", "text/csv")
			writer.Header().Set("Content-Encoding", "br")
			_, _ = io.WriteString(writer, "a,b\n")
		case "invalid-gzip":
			writer.Header().Set("Content-Type", "text/csv")
			writer.Header().Set("Content-Encoding", "gzip")
			_, _ = io.WriteString(writer, "not gzip")
		case "writer":
			writer.Header().Set("Content-Type", "text/csv")
			_, _ = io.WriteString(writer, "a,b\n1,2\n")
		case "created":
			writer.Header().Set("Content-Type", "text/csv")
			writer.WriteHeader(http.StatusCreated)
		case "redirect":
			http.Redirect(writer, request, "/credential-target", http.StatusFound)
		case "credential-target":
			redirected = true
		default:
			http.Error(writer, "bad mode", http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	for _, current := range []string{"missing-eof", "bad-count", "trailing", "bad-csv", "bad-columns"} {
		mode = current
		_, err := client.DownloadAcquisitionsReport(context.Background(), acquisitionRequest(true), io.Discard, DownloadOptions{})
		if requireHubError(t, err).Code != socialhub.CodePlatformError {
			t.Errorf("mode=%s err=%v", mode, err)
		}
	}
	for _, current := range []string{"wrong-type", "unexpected-encoding", "created"} {
		mode = current
		_, err := client.DownloadAcquisitionsReport(context.Background(), acquisitionRequest(false), io.Discard, DownloadOptions{})
		if requireHubError(t, err).Code != socialhub.CodePlatformError {
			t.Errorf("mode=%s err=%v", mode, err)
		}
	}
	mode = "too-large"
	var bounded bytes.Buffer
	_, err := client.DownloadAcquisitionsReport(context.Background(), acquisitionRequest(false), &bounded, DownloadOptions{MaxBytes: 4})
	if requireHubError(t, err).Code != socialhub.CodePlatformError || bounded.String() != "1234" {
		t.Fatalf("bounded=%q err=%v", bounded.String(), err)
	}
	mode = "invalid-gzip"
	_, err = client.DownloadAcquisitionsReport(context.Background(), acquisitionRequest(false), io.Discard, DownloadOptions{Compression: CompressionGzip})
	if requireHubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("invalid gzip err=%v", err)
	}
	mode = "writer"
	_, err = client.DownloadAcquisitionsReport(context.Background(), acquisitionRequest(false), failingWriter{}, DownloadOptions{})
	if hub := requireHubError(t, err); hub.Code != socialhub.CodePlatformError || hub.Class != socialhub.ClassPermanent {
		t.Fatalf("writer err=%v", err)
	}
	mode = "redirect"
	_, err = client.DownloadAcquisitionsReport(context.Background(), acquisitionRequest(false), io.Discard, DownloadOptions{})
	if err == nil || redirected {
		t.Fatalf("redirect err=%v followed=%v", err, redirected)
	}
}

func TestRateLimitErrorAndCompressedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/problem+json")
		writer.Header().Set("Content-Encoding", "gzip")
		writer.Header().Set("Retry-After", "2.5")
		writer.Header().Set("RateLimit-Policy", "1;w=1, 30;w=1800")
		writer.Header().Set("RateLimit", "limit=1, remaining=0, reset=1")
		writer.Header().Set("Unity-RateLimit", "limit=1, remaining=0, reset=1; limit=30, remaining=0, reset=42")
		writer.WriteHeader(http.StatusTooManyRequests)
		compressed := gzip.NewWriter(writer)
		_, _ = io.WriteString(compressed, `{"title":"Too Many Requests","detail":"token=secret-value exhausted","code":50,"status":429,"requestId":"request-1"}`)
		_ = compressed.Close()
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.DownloadAcquisitionsReport(context.Background(), acquisitionRequest(false), io.Discard, DownloadOptions{Compression: CompressionGzip})
	var api *APIError
	if !errors.As(err, &api) || !api.Retryable() || api.Hub.Code != socialhub.CodeRateLimited || api.Hub.RetryAfter != 2500*time.Millisecond ||
		api.Hub.RequestID != "request-1" || api.RateLimitPolicy == "" || api.RateLimit == "" || api.UnityRateLimit == "" ||
		api.Hub.PlatformMessage == "" || bytes.Contains([]byte(api.Hub.PlatformMessage), []byte("secret-value")) {
		t.Fatalf("error=%#v", api)
	}
}

func acquisitionRequest(eof bool) AcquisitionsReportRequest {
	start := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	return AcquisitionsReportRequest{
		Start: start, End: start.Add(24 * time.Hour), Scale: ScaleDay,
		Metrics: []AcquisitionMetric{MetricClicks}, Format: FormatCSV, EOFMarker: eof,
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
