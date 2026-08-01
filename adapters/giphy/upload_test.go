package giphy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestStreamingUpload(t *testing.T) {
	payload := []byte("gif-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/upload/v1/gifs" || request.URL.Query().Get("api_key") != "api-key" || request.Header.Get("X-Request-ID") != "upload-request" {
			t.Errorf("request=%s %s query=%v request-id=%q", request.Method, request.URL.Path, request.URL.Query(), request.Header.Get("X-Request-ID"))
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			writeMetaError(writer, http.StatusBadRequest, "bad multipart")
			return
		}
		defer request.MultipartForm.RemoveAll()
		if request.FormValue("username") != "artist" || request.FormValue("tags") != "cat,funny" || request.FormValue("source_post_url") != "https://example.com/post" || request.FormValue("customer_id") != "customer" || request.FormValue("country_code") != "US" || request.FormValue("region") != "VA" || request.FormValue("api_key") != "api-key" {
			t.Errorf("form=%v", request.MultipartForm.Value)
		}
		file, header, err := request.FormFile("file")
		if err != nil {
			t.Errorf("FormFile: %v", err)
			writeMetaError(writer, http.StatusBadRequest, "missing file")
			return
		}
		defer file.Close()
		body, _ := io.ReadAll(file)
		if header.Filename != "animation.gif" || header.Header.Get("Content-Type") != "image/gif" || !bytes.Equal(body, payload) {
			t.Errorf("file=%q mime=%q body=%q", header.Filename, header.Header.Get("Content-Type"), body)
		}
		writeSingle(writer, `{"id":"uploaded","type":"gif","slug":"uploaded-slug"}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	result, err := client.Upload(context.Background(), UploadRequest{
		Filename: "animation.gif", MIME: "image/gif", Size: int64(len(payload)), Username: "artist", Tags: []string{"cat", "funny"},
		SourcePostURL: "https://example.com/post", CustomerID: "customer", CountryCode: "US", Region: "VA",
	}, bytes.NewReader(payload), socialhub.WithRequestID("upload-request"))
	if err != nil || result.ID != "uploaded" || result.Type != "gif" || result.Slug != "uploaded-slug" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestUploadExactSizeValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writeSingle(writer, `{"id":"uploaded"}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	for _, test := range []struct {
		name     string
		declared int64
		body     string
	}{
		{"short", 4, "abc"},
		{"long", 3, "abcd"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.Upload(context.Background(), UploadRequest{Filename: "x.gif", MIME: "image/gif", Size: test.declared}, strings.NewReader(test.body))
			if errorCode(err) != socialhub.CodeInvalidArgument {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
