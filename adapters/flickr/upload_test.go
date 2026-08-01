package flickr

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestUploadStreamsMultipartAndSignsMetadata(t *testing.T) {
	payload := []byte("flickr-binary-payload")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/upload" || request.Method != http.MethodPost || request.Header.Get("Accept") != "application/xml" || request.Header.Get("X-Request-ID") != "upload-request" {
			t.Errorf("upload request=%s %s headers=%v", request.Method, request.URL.Path, request.Header)
		}
		verifyOAuthSignature(t, request, "consumer-secret", "token-secret", true)
		defer request.MultipartForm.RemoveAll()
		if request.FormValue("api_key") != "api-key" || request.FormValue("title") != "Demo" || request.FormValue("description") != "Description" || request.FormValue("tags") != `one "two words" "quoted\"tag"` || request.FormValue("is_public") != "1" || request.FormValue("is_friend") != "0" || request.FormValue("safety_level") != "2" || request.FormValue("content_type") != "3" || request.FormValue("hidden") != "1" {
			t.Errorf("multipart values=%v", request.MultipartForm.Value)
		}
		if _, signed := request.MultipartForm.Value["photo"]; signed {
			t.Error("binary photo appeared among signed form values")
		}
		file, header, err := request.FormFile("photo")
		if err != nil {
			t.Errorf("FormFile: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()
		actual, err := io.ReadAll(file)
		if err != nil || !bytes.Equal(actual, payload) || header.Filename != "demo.jpg" || header.Header.Get("Content-Type") != "image/jpeg" {
			t.Errorf("file=%q filename=%q MIME=%q err=%v", actual, header.Filename, header.Header.Get("Content-Type"), err)
		}
		writer.Header().Set("Content-Type", "application/xml")
		_, _ = writer.Write([]byte(`<rsp stat="ok"><photoid> photo-123 </photoid></rsp>`))
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	result, err := client.PhotoUploadWorkflow().Upload(context.Background(), UploadPhotoRequest{
		Filename: "demo.jpg", MIME: "image/jpeg", Size: int64(len(payload)), Title: "Demo", Description: "Description",
		Tags: []string{"one", "two words", `quoted"tag`}, IsPublic: boolPointer(true), IsFriend: boolPointer(false),
		SafetyLevel: 2, ContentType: 3, Hidden: 1,
	}, bytes.NewReader(payload), socialhub.WithRequestID("upload-request"))
	if err != nil || result.PhotoID != "photo-123" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestUploadRejectsReaderSizeMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writer.Header().Set("Content-Type", "application/xml")
		_, _ = writer.Write([]byte(`<rsp stat="ok"><photoid>photo-1</photoid></rsp>`))
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	tests := []struct {
		name string
		size int64
		body string
	}{
		{"short", 5, "1234"},
		{"long", 3, "1234"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.PhotoUploadWorkflow().Upload(context.Background(), UploadPhotoRequest{Filename: "demo.jpg", MIME: "image/jpeg", Size: test.size}, strings.NewReader(test.body))
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeInvalidArgument || !strings.Contains(platformErr.PlatformMessage, "expected exactly") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestUploadErrorMappingAndRedirectPolicy(t *testing.T) {
	t.Run("XML API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = io.Copy(io.Discard, request.Body)
			writer.Header().Set("Content-Type", "application/xml")
			writer.Header().Set("X-Correlation-ID", "upload-correlation")
			_, _ = writer.Write([]byte(`<rsp stat="fail"><err code="6" msg="daily limit reached"/></rsp>`))
		}))
		defer server.Close()
		_, client := newTestClient(t, server)
		_, err := client.PhotoUploadWorkflow().Upload(context.Background(), UploadPhotoRequest{Filename: "demo.jpg", MIME: "image/jpeg", Size: 1}, strings.NewReader("x"))
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || platformErr.PlatformCode != "6" || platformErr.RequestID != "upload-correlation" {
			t.Fatalf("error=%#v", err)
		}
	})

	t.Run("redirect", func(t *testing.T) {
		var followed bool
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/other" {
				followed = true
			}
			if request.URL.Path == "/upload" {
				_, _ = io.Copy(io.Discard, request.Body)
				http.Redirect(writer, request, "/other", http.StatusTemporaryRedirect)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		_, client := newTestClient(t, server)
		_, err := client.PhotoUploadWorkflow().Upload(context.Background(), UploadPhotoRequest{Filename: "demo.jpg", MIME: "image/jpeg", Size: 1}, strings.NewReader("x"))
		if err == nil || followed {
			t.Fatalf("error=%v followed=%v", err, followed)
		}
	})
}

func TestUploadTransportFailureReleasesWriter(t *testing.T) {
	client := &Client{
		accountID: "main", apiKey: "api-key", consumerSecret: "consumer-secret", accessToken: "access-token", tokenSecret: "token-secret",
		permission: PermissionWrite, uploadURL: "https://upload.invalid/services/upload/", public: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("transport failed before reading body")
		})}, signed: &http.Client{}, clock: fixedClock{testNow},
	}
	service := &PhotoUploadService{client: client}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := service.Upload(ctx, UploadPhotoRequest{Filename: "demo.jpg", MIME: "image/jpeg", Size: 1 << 20}, io.LimitReader(strings.NewReader(strings.Repeat("x", 1<<20)), 1<<20))
	if errorCode(err) != socialhub.CodeTemporarilyUnavailable {
		t.Fatalf("error=%v", err)
	}
}

func TestUploadValidation(t *testing.T) {
	client := &Client{permission: PermissionWrite, signed: &http.Client{}}
	service := &PhotoUploadService{client: client}
	tests := []struct {
		name  string
		input UploadPhotoRequest
		body  io.Reader
	}{
		{"reader", UploadPhotoRequest{Filename: "demo.jpg", MIME: "image/jpeg", Size: 1}, nil},
		{"filename", UploadPhotoRequest{Filename: "../demo.jpg", MIME: "image/jpeg", Size: 1}, strings.NewReader("x")},
		{"mime", UploadPhotoRequest{Filename: "demo.jpg", MIME: "text/plain", Size: 1}, strings.NewReader("x")},
		{"size", UploadPhotoRequest{Filename: "demo.jpg", MIME: "image/jpeg", Size: 0}, strings.NewReader("x")},
		{"tag", UploadPhotoRequest{Filename: "demo.jpg", MIME: "image/jpeg", Size: 1, Tags: []string{"\n"}}, strings.NewReader("x")},
		{"enum", UploadPhotoRequest{Filename: "demo.jpg", MIME: "image/jpeg", Size: 1, SafetyLevel: 4}, strings.NewReader("x")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Upload(context.Background(), test.input, test.body)
			if errorCode(err) != socialhub.CodeInvalidArgument {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestUploadErrorClassification(t *testing.T) {
	tests := []struct {
		platformCode int
		wantCode     socialhub.ErrorCode
		wantClass    socialhub.ErrorClass
	}{
		{2, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{3, socialhub.CodePlatformError, socialhub.ClassPermanent},
		{6, socialhub.CodeRateLimited, socialhub.ClassUserAction},
		{9, socialhub.CodeConflict, socialhub.ClassPermanent},
		{11, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{13, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{14, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{99, socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
	}
	for _, test := range tests {
		code, class := classifyUploadError(http.StatusOK, test.platformCode)
		if code != test.wantCode || class != test.wantClass {
			t.Fatalf("platform code=%d got=(%q,%q) want=(%q,%q)", test.platformCode, code, class, test.wantCode, test.wantClass)
		}
	}
}
