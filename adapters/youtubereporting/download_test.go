package youtubereporting

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestDownloadReportIdentityAndGzip(t *testing.T) {
	const csv = "day,views\n2026-08-08,42\n"
	var metadataCalls atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request, http.MethodGet, request.URL.Path)
		switch request.URL.Path {
		case "/v1/jobs/" + testJobID + "/reports/" + testReportID:
			metadataCalls.Add(1)
			writeJSON(t, writer, http.StatusOK, reportFixture(server.URL+"/v1/media/jobs/"+testJobID+"/reports/"+testReportID+"?alt=media"))
		case "/v1/media/jobs/" + testJobID + "/reports/" + testReportID:
			if request.URL.Query().Get("alt") != "media" {
				t.Errorf("download query=%v", request.URL.Query())
			}
			writer.Header().Set("Content-Type", "text/csv")
			writer.Header().Set("ETag", `"etag-1"`)
			writer.Header().Set("Last-Modified", "Sun, 10 Aug 2026 12:00:00 GMT")
			switch request.Header.Get("Accept-Encoding") {
			case "identity":
				_, _ = io.WriteString(writer, csv)
			case "gzip":
				writer.Header().Set("Content-Encoding", "gzip")
				compressed := gzip.NewWriter(writer)
				_, _ = io.WriteString(compressed, csv)
				_ = compressed.Close()
			default:
				t.Errorf("Accept-Encoding=%q", request.Header.Get("Accept-Encoding"))
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))

	for _, gzipTransfer := range []bool{false, true} {
		var output bytes.Buffer
		result, err := client.DownloadReport(context.Background(), testJobID, testReportID, &output, DownloadOptions{MaxBytes: 1024, Gzip: gzipTransfer})
		if err != nil || output.String() != csv || result.BytesWritten != int64(len(csv)) || result.ContentType != "text/csv" ||
			result.ETag != `"etag-1"` || result.LastModified == "" || result.Report.ID != testReportID {
			t.Fatalf("gzip=%v result=%#v output=%q err=%v", gzipTransfer, result, output.String(), err)
		}
		if gzipTransfer && result.ContentEncoding != "gzip" || !gzipTransfer && result.ContentEncoding != "" {
			t.Errorf("gzip=%v encoding=%q", gzipTransfer, result.ContentEncoding)
		}
	}
	if metadataCalls.Load() != 2 {
		t.Fatalf("metadata calls=%d", metadataCalls.Load())
	}
}

func TestDownloadReportBoundsOutput(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/jobs/"+testJobID+"/reports/"+testReportID {
			writeJSON(t, writer, http.StatusOK, reportFixture(server.URL+"/v1/media/report?alt=media"))
			return
		}
		_, _ = io.WriteString(writer, "12345")
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	var output bytes.Buffer
	_, err := client.DownloadReport(context.Background(), testJobID, testReportID, &output, DownloadOptions{MaxBytes: 4})
	if requireHubError(t, err).Code != socialhub.CodePlatformError || output.String() != "1234" {
		t.Fatalf("output=%q err=%v", output.String(), err)
	}
}

func TestDownloadRejectsUnsafeMetadataURLs(t *testing.T) {
	var downloadURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/jobs/"+testJobID+"/reports/"+testReportID {
			t.Fatal("unsafe URL triggered download request")
		}
		writeJSON(t, writer, http.StatusOK, reportFixture(downloadURL))
	}))
	defer server.Close()
	tests := []string{
		"https://evil.example/v1/media/report?alt=media",
		server.URL + "/v1/media/report?alt=json",
		server.URL + "/v1/media/report?alt=media&extra=1",
		server.URL + "/v1/jobs/report?alt=media",
		server.URL + "/v1/media/../jobs/report?alt=media",
		server.URL + "/v1/media/report?alt=media#fragment",
	}
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	for _, value := range tests {
		downloadURL = value
		_, err := client.DownloadReport(context.Background(), testJobID, testReportID, io.Discard, DownloadOptions{})
		if requireHubError(t, err).Code != socialhub.CodePlatformError {
			t.Errorf("url=%q err=%v", value, err)
		}
	}
}

func TestDownloadDoesNotFollowRedirect(t *testing.T) {
	redirected := false
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/jobs/" + testJobID + "/reports/" + testReportID:
			writeJSON(t, writer, http.StatusOK, reportFixture(server.URL+"/v1/media/report?alt=media"))
		case "/v1/media/report":
			http.Redirect(writer, request, "/credential-target", http.StatusFound)
		case "/credential-target":
			redirected = true
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	if _, err := client.DownloadReport(context.Background(), testJobID, testReportID, io.Discard, DownloadOptions{}); err == nil || redirected {
		t.Fatalf("redirect error=%v redirected=%v", err, redirected)
	}
}

func TestDownloadEncodingHTTPAndWriterErrors(t *testing.T) {
	mode := "encoding"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/jobs/"+testJobID+"/reports/"+testReportID {
			writeJSON(t, writer, http.StatusOK, reportFixture(server.URL+"/v1/media/report?alt=media"))
			return
		}
		switch mode {
		case "encoding":
			writer.Header().Set("Content-Encoding", "br")
			_, _ = io.WriteString(writer, "data")
		case "invalid-gzip":
			writer.Header().Set("Content-Encoding", "gzip")
			_, _ = io.WriteString(writer, "not-gzip")
		case "http":
			writeJSON(t, writer, http.StatusTooManyRequests, map[string]any{"error": map[string]any{"status": "RESOURCE_EXHAUSTED"}})
		case "writer":
			_, _ = io.WriteString(writer, "data")
		case "no-content":
			writer.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))

	if _, err := client.DownloadReport(context.Background(), testJobID, testReportID, io.Discard, DownloadOptions{}); requireHubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("encoding error=%v", err)
	}
	mode = "invalid-gzip"
	if _, err := client.DownloadReport(context.Background(), testJobID, testReportID, io.Discard, DownloadOptions{Gzip: true}); requireHubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("gzip error=%v", err)
	}
	mode = "http"
	if _, err := client.DownloadReport(context.Background(), testJobID, testReportID, io.Discard, DownloadOptions{}); requireHubError(t, err).Code != socialhub.CodeRateLimited {
		t.Fatalf("http error=%v", err)
	}
	mode = "writer"
	if _, err := client.DownloadReport(context.Background(), testJobID, testReportID, errorWriter{}, DownloadOptions{}); requireHubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("writer error=%v", err)
	}
	mode = "no-content"
	if _, err := client.DownloadReport(context.Background(), testJobID, testReportID, io.Discard, DownloadOptions{}); requireHubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("status error=%v", err)
	}
	if _, err := client.DownloadReport(context.Background(), "", testReportID, io.Discard, DownloadOptions{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("input error=%v", err)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("disk unavailable") }
