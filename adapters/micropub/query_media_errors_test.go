package micropub

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestConfigSyndicationAndUnsupportedConfigCompatibility(t *testing.T) {
	var configCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("tenant") != "one" {
			t.Errorf("query=%v", request.URL.Query())
		}
		switch request.URL.Query().Get("q") {
		case "config":
			call := configCalls.Add(1)
			if call == 2 {
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte(`{"error":"invalid_request"}`))
				return
			}
			if call == 3 {
				_, _ = writer.Write([]byte(`not-json`))
				return
			}
			_, _ = writer.Write([]byte(`{"media-endpoint":"https://media.example/upload?site=one","syndicate-to":[{"uid":"target-1","name":"Archive","service":{"name":"Archive","url":"https://archive.example/","photo":"https://archive.example/icon.png"},"user":{"name":"Ada","url":"https://archive.example/ada"}}]}`))
		case "syndicate-to":
			_, _ = writer.Write([]byte(`{"syndicate-to":[{"uid":"target-2","name":"Network"}]}`))
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, nil, true, false, false)
	config, err := client.Config(context.Background())
	if err != nil || config.MediaEndpoint != "https://media.example/upload?site=one" || len(config.SyndicateTo) != 1 || len(config.Raw) == 0 {
		t.Fatalf("config=%#v err=%v", config, err)
	}
	config, err = client.Config(context.Background())
	if err != nil || config.MediaEndpoint != "" || len(config.Raw) != 0 {
		t.Fatalf("unsupported config=%#v err=%v", config, err)
	}
	config, err = client.Config(context.Background())
	if err != nil || config.MediaEndpoint != "" {
		t.Fatalf("unexpected config=%#v err=%v", config, err)
	}
	targets, err := client.SyndicationTargets(context.Background())
	if err != nil || len(targets) != 1 || targets[0].UID != "target-2" {
		t.Fatalf("targets=%#v err=%v", targets, err)
	}
}

func TestInvalidConfigIsEmptyAndInvalidSyndicationIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("q") == "config" {
			_, _ = writer.Write([]byte(`{"media-endpoint":"ftp://invalid","syndicate-to":[]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"syndicate-to":[{"uid":"same","name":"One"},{"uid":"same","name":"Two"}]}`))
	}))
	defer server.Close()
	_, client := newTestClient(t, server, nil, true, false, false)
	config, err := client.Config(context.Background())
	if err != nil || config.MediaEndpoint != "" {
		t.Fatalf("config=%#v err=%v", config, err)
	}
	if _, err := client.SyndicationTargets(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("error=%v", err)
	}
}

func TestMediaEndpointStreamsExactFileAndAllowsDifferentOrigin(t *testing.T) {
	content := []byte("media-body")
	mediaServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("site") != "one" || request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("X-Request-ID") != "media-request" {
			t.Errorf("request=%s headers=%v", request.URL, request.Header)
		}
		reader, err := request.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(part)
		if part.FormName() != "file" || part.FileName() != "photo.jpg" || part.Header.Get("Content-Type") != "image/jpeg" || !bytes.Equal(body, content) {
			t.Errorf("part name=%q filename=%q type=%q body=%q", part.FormName(), part.FileName(), part.Header.Get("Content-Type"), body)
		}
		if _, err := reader.NextPart(); !errors.Is(err, io.EOF) {
			t.Errorf("extra part error=%v", err)
		}
		writer.Header().Set("Location", "https://cdn.example/media/photo.jpg")
		writer.WriteHeader(http.StatusCreated)
	}))
	defer mediaServer.Close()
	primary := httptest.NewServer(http.NotFoundHandler())
	defer primary.Close()
	_, client := newTestClient(t, primary, []string{"create"}, true, false, false)
	result, err := client.UploadMedia(context.Background(), MediaUploadRequest{
		Endpoint: mediaServer.URL + "/upload?site=one", Filename: "photo.jpg", MIME: "image/jpeg", Size: int64(len(content)),
	}, bytes.NewReader(content), socialhub.WithRequestID("media-request"))
	if err != nil || result.URL != "https://cdn.example/media/photo.jpg" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestMediaValidationSizeAndResponseContracts(t *testing.T) {
	tests := []struct {
		name     string
		input    MediaUploadRequest
		body     []byte
		status   int
		location string
		code     socialhub.ErrorCode
	}{
		{"nil metadata", MediaUploadRequest{}, nil, 0, "", socialhub.CodeInvalidArgument},
		{"short body", MediaUploadRequest{Filename: "a.jpg", MIME: "image/jpeg", Size: 2}, []byte("a"), http.StatusCreated, "https://cdn.example/a", socialhub.CodeInvalidArgument},
		{"long body", MediaUploadRequest{Filename: "a.jpg", MIME: "image/jpeg", Size: 1}, []byte("ab"), http.StatusCreated, "https://cdn.example/a", socialhub.CodeInvalidArgument},
		{"status", MediaUploadRequest{Filename: "a.jpg", MIME: "image/jpeg", Size: 1}, []byte("a"), http.StatusOK, "https://cdn.example/a", socialhub.CodePlatformError},
		{"location", MediaUploadRequest{Filename: "a.jpg", MIME: "image/jpeg", Size: 1}, []byte("a"), http.StatusCreated, "", socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				_, _ = multipart.NewReader(request.Body, "unused").ReadForm(1)
				if test.location != "" {
					writer.Header().Set("Location", test.location)
				}
				writer.WriteHeader(test.status)
			}))
			defer server.Close()
			_, client := newTestClient(t, server, nil, true, false, false)
			input := test.input
			input.Endpoint = server.URL + "/media"
			var reader io.Reader
			if test.body != nil {
				reader = bytes.NewReader(test.body)
			}
			_, err := client.UploadMedia(context.Background(), input, reader)
			if errorCode(err) != test.code {
				t.Fatalf("error=%v code=%q", err, errorCode(err))
			}
		})
	}
}

func TestRedirectIsNotFollowedAndTokenIsNotForwarded(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { targetCalls.Add(1) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Location", target.URL+"/steal")
		writer.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, nil, true, false, false)
	if _, err := client.Config(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("error=%v", err)
	}
	if targetCalls.Load() != 0 {
		t.Fatalf("redirect target calls=%d", targetCalls.Load())
	}
}

func TestHTTPErrorMappingAndBoundedResponse(t *testing.T) {
	tests := []struct {
		name, body string
		status     int
		code       socialhub.ErrorCode
		class      socialhub.ErrorClass
	}{
		{"unauthorized", `{"error":"unauthorized"}`, 401, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"forbidden", `{"error":"forbidden"}`, 403, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"scope", `{"error":"insufficient_scope","scope":"create update"}`, 401, socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{"invalid", `{"error":"invalid_request","error_description":"bad"}`, 400, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"rate", `{}`, 429, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{"server", `{}`, 503, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"redirect", `{}`, 302, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{"Retry-After": {"1.5"}, "X-Request-Id": {"request-id"}}
			err := decodeHTTPError(test.status, header, []byte(test.body), "test")
			var hubError *socialhub.Error
			if !errors.As(err, &hubError) || hubError.Code != test.code || hubError.Class != test.class || hubError.RequestID != "request-id" || hubError.RetryAfter != 1500*time.Millisecond {
				t.Fatalf("error=%#v", hubError)
			}
			if test.name == "scope" && len(hubError.RequiredScopes) != 2 {
				t.Fatalf("scopes=%v", hubError.RequiredScopes)
			}
		})
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("999999999") != 0 {
		t.Fatal("invalid Retry-After accepted")
	}
	long := strings.Repeat("x", 600)
	if len([]rune(boundedMessage(long, 10))) != 10 || firstNonEmpty("", " value ") != " value " {
		t.Fatal("bounded helpers failed")
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(bytes.Repeat([]byte("x"), int(maxResponseBytes+1)))
	}))
	defer server.Close()
	_, client := newTestClient(t, server, nil, true, false, false)
	if _, err := client.Config(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("large response error=%v", err)
	}
}

func TestSourceAndQueryMalformedResponses(t *testing.T) {
	responses := []string{
		`{}`,
		`{"properties":null,"type":["h-entry"]}`,
		`{"properties":{"content":[]}}`,
		`{"properties":{"content":[null]}}`,
	}
	for _, body := range responses {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { _, _ = writer.Write([]byte(body)) }))
		_, client := newTestClient(t, server, nil, true, false, false)
		if _, err := client.Source(context.Background(), "https://site.example/post", nil); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("body=%s error=%v", body, err)
		}
		server.Close()
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { _, _ = writer.Write([]byte(`not-json`)) }))
	defer server.Close()
	_, client := newTestClient(t, server, nil, true, false, false)
	if _, err := client.SyndicationTargets(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("query error=%v", err)
	}
}
