package imgur

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
	payload := []byte("image-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/3/image" || request.Header.Get("Authorization") != "Client-ID client-id" || request.Header.Get("X-Request-ID") != "upload-request" {
			t.Errorf("request=%s %s auth=%q request-id=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"), request.Header.Get("X-Request-ID"))
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			writeEnvelope(writer, http.StatusBadRequest, `false`)
			return
		}
		if request.FormValue("album") != "album-delete" || request.FormValue("name") != "source" || request.FormValue("title") != "title" || request.FormValue("description") != "description" || request.FormValue("disable_audio") != "true" {
			t.Errorf("form=%v", request.MultipartForm.Value)
		}
		file, header, err := request.FormFile("image")
		if err != nil {
			t.Errorf("FormFile: %v", err)
			writeEnvelope(writer, http.StatusBadRequest, `false`)
			return
		}
		defer file.Close()
		body, _ := io.ReadAll(file)
		if header.Filename != "photo.png" || header.Header.Get("Content-Type") != "image/png" || !bytes.Equal(body, payload) {
			t.Errorf("file=%q mime=%q body=%q", header.Filename, header.Header.Get("Content-Type"), body)
		}
		writeEnvelope(writer, http.StatusOK, `{"id":"uploaded","deletehash":"deletehash","type":"image/png","size":11,"link":"https://i.imgur.com/uploaded.png"}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	disableAudio := true
	image, err := client.Upload(context.Background(), UploadRequest{
		Filename: "photo.png", MIME: "image/png", Size: int64(len(payload)), Album: "album-delete",
		Name: "source", Title: "title", Description: "description", DisableAudio: &disableAudio,
	}, bytes.NewReader(payload), socialhub.WithRequestID("upload-request"))
	if err != nil || image.ID != "uploaded" || image.DeleteHash != "deletehash" || image.Size != int64(len(payload)) {
		t.Fatalf("image=%#v err=%v", image, err)
	}
}

func TestUploadExactSizeValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writeEnvelope(writer, http.StatusOK, `{"id":"uploaded","link":"https://i.imgur.com/uploaded.png"}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	for _, test := range []struct {
		name     string
		declared int64
		body     string
	}{
		{"short", 4, "abc"},
		{"long", 3, "abcd"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.Upload(context.Background(), UploadRequest{Filename: "photo.png", MIME: "image/png", Size: test.declared}, strings.NewReader(test.body))
			if errorCode(err) != socialhub.CodeInvalidArgument {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestUploadValidationAndResponse(t *testing.T) {
	client := &Client{}
	tests := []struct {
		name   string
		input  UploadRequest
		reader io.Reader
	}{
		{"nil reader", UploadRequest{Filename: "x.png", MIME: "image/png", Size: 1}, nil},
		{"filename", UploadRequest{Filename: "../x.png", MIME: "image/png", Size: 1}, strings.NewReader("x")},
		{"mime", UploadRequest{Filename: "x.png", MIME: "text/plain", Size: 1}, strings.NewReader("x")},
		{"size", UploadRequest{Filename: "x.png", MIME: "image/png"}, strings.NewReader("x")},
		{"album", UploadRequest{Filename: "x.png", MIME: "image/png", Size: 1, Album: "bad/id"}, strings.NewReader("x")},
		{"text", UploadRequest{Filename: "x.png", MIME: "image/png", Size: 1, Title: string([]byte{0})}, strings.NewReader("x")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := client.Upload(context.Background(), test.input, test.reader); errorCode(err) != socialhub.CodeInvalidArgument {
				t.Fatalf("error=%v", err)
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writeEnvelope(writer, http.StatusOK, `{"id":"uploaded"}`)
	}))
	defer server.Close()
	_, configured := newTestClient(t, server, false)
	if _, err := configured.Upload(context.Background(), UploadRequest{Filename: "x.png", MIME: "image/png", Size: 1}, strings.NewReader("x")); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid response=%v", err)
	}
}
