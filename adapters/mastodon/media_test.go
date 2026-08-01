package mastodon

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestMastodonMediaUploadAndStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v2/media":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, header, err := request.FormFile("file")
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			defer file.Close()
			body, _ := io.ReadAll(file)
			if header.Filename != "clip.mp4" || header.Header.Get("Content-Type") != "video/mp4" || string(body) != "video-data" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"id":"media-444","type":"video","url":"","preview_url":"https://cdn.example/preview.jpg","meta":{"original":{"width":720,"height":1280,"duration":12.5}}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/media/media-444":
			writeJSON(writer, `{"id":"media-444","type":"video","url":"https://cdn.example/clip.mp4","preview_url":"https://cdn.example/preview.jpg","meta":{"original":{"width":720,"height":1280,"duration":12.5}}}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, allTestScopes())

	input := socialhub.BeginUploadRequest{Filename: "clip.mp4", Type: socialhub.MediaTypeVideo, MIME: "video/mp4", Size: int64(len("video-data"))}
	session, err := client.BeginUpload(context.Background(), input)
	if err != nil || session.ID == "" || session.PartSize != input.Size {
		t.Fatalf("session=%#v error=%v", session, err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader("video-data"))
	if err != nil || part.Number != 1 || part.Size != input.Size {
		t.Fatalf("part=%#v error=%v", part, err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader("video-data")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("repeat upload error=%v", err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "media-444" || media.State != socialhub.MediaStateProcessing || media.Width == nil || *media.Width != 720 {
		t.Fatalf("media=%#v error=%v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), media.ID)
	if err != nil || status.State != socialhub.MediaStateReady || status.URL != "https://cdn.example/clip.mp4" || status.Duration == nil || *status.Duration != 12500000000 {
		t.Fatalf("status=%#v error=%v", status, err)
	}
}

func TestMastodonMediaValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, allTestScopes())
	invalid := []socialhub.BeginUploadRequest{
		{},
		{Filename: "file.pdf", Type: socialhub.MediaTypeDocument, MIME: "application/pdf", Size: 1},
	}
	for _, input := range invalid {
		if _, err := client.BeginUpload(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
	if _, err := client.UploadPart(context.Background(), "missing", 2, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("part error=%v", err)
	}
	if _, err := client.CompleteUpload(context.Background(), "missing", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("complete error=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("status error=%v", err)
	}
}
