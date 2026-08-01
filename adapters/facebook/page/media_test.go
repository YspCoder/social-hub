package page

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestPagePhotoUpload(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v26.0/123/photos" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil || request.FormValue("published") != "false" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		file, header, err := request.FormFile("source")
		if err != nil || header.Filename != "photo.jpg" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		data, _ := io.ReadAll(file)
		_ = file.Close()
		if string(data) != "image" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte(`{"id":"photo-1"}`))
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "photo.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 5})
	if err != nil {
		t.Fatal(err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, bytes.NewBufferString("image"))
	if err != nil || part.ETag != "photo-1" {
		t.Fatalf("part = %#v, err = %v", part, err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "photo-1" || media.State != socialhub.MediaStateReady {
		t.Fatalf("media = %#v, err = %v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), media.ID)
	if err != nil || status.ID != media.ID {
		t.Fatalf("status = %#v, err = %v", status, err)
	}
}

func TestPageVideoUploadIsUnsupported(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	_, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "clip.mp4", Type: socialhub.MediaTypeVideo, MIME: "video/mp4", Size: 10})
	if !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("error = %v", err)
	}
}
