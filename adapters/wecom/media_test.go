package wecom

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

func TestTemporaryMediaUploadLifecycle(t *testing.T) {
	var uploads int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/cgi-bin/media/upload" || request.URL.Query().Get("access_token") != "access-token" || request.URL.Query().Get("type") != "image" {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		reader, err := request.MultipartReader()
		if err != nil {
			http.Error(writer, "not multipart", http.StatusBadRequest)
			return
		}
		part, err := reader.NextPart()
		if err != nil || part.FormName() != "media" || part.FileName() != "photo.jpg" || part.Header.Get("Content-Type") != "image/jpeg" {
			http.Error(writer, "bad media part", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(part)
		if err != nil || (string(body) != "123456" && string(body) != "1234567") {
			http.Error(writer, "bad media body", http.StatusBadRequest)
			return
		}
		uploads++
		writeTestJSON(t, writer, map[string]any{"errcode": 0, "type": "image", "media_id": "media-" + string(rune('0'+uploads)), "created_at": testNow.Unix()})
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	descriptor := socialhub.BeginUploadRequest{Filename: "photo.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 6}
	session, err := client.BeginUpload(context.Background(), descriptor)
	if err != nil || !strings.HasPrefix(session.ID, "direct:") || session.PartSize != 6 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, bytes.NewBufferString("123456"))
	if err != nil || part.Number != 0 || part.ETag != "media-1" || part.Size != 6 {
		t.Fatalf("part=%#v err=%v", part, err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 0, bytes.NewBufferString("123456")); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("duplicate upload=%v", err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "media-1" || media.State != socialhub.MediaStateReady || media.ExpiresAt == nil || !media.ExpiresAt.Equal(testNow.Add(72*time.Hour)) {
		t.Fatalf("media=%#v err=%v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), media.ID)
	if err != nil || status.ID != media.ID || status.Size == nil || *status.Size != 6 {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	status.ID = "changed"
	again, _ := client.MediaStatus(context.Background(), media.ID)
	if again.ID != media.ID {
		t.Fatal("media status did not return a copy")
	}

	mismatch, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "photo.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 7})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.UploadPart(context.Background(), mismatch.ID, 0, bytes.NewBufferString("123456")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("byte mismatch=%v", err)
	}
}

func TestMediaValidationAndStateErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false)
	invalid := []socialhub.BeginUploadRequest{
		{},
		{Filename: " photo.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 6},
		{Filename: "bad\nname", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 6},
		{Filename: "photo.jpg", Type: socialhub.MediaTypeImage, MIME: "image/gif", Size: 6},
		{Filename: "voice.amr", Type: socialhub.MediaTypeAudio, MIME: "audio/mpeg", Size: 6},
		{Filename: "video.mp4", Type: socialhub.MediaTypeVideo, MIME: "video/quicktime", Size: 6},
		{Filename: "photo.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 5},
		{Filename: "photo.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: maxImageBytes + 1},
		{Filename: "unknown", Type: socialhub.MediaType("unknown"), MIME: "application/octet-stream", Size: 6},
	}
	for index, descriptor := range invalid {
		if _, err := client.BeginUpload(context.Background(), descriptor); err == nil {
			t.Fatalf("descriptor %d accepted", index)
		}
	}
	valid := []socialhub.BeginUploadRequest{
		{Filename: "photo.png", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: 6},
		{Filename: "voice.amr", Type: socialhub.MediaTypeAudio, MIME: "audio/amr", Size: 6},
		{Filename: "voice.amr", Type: socialhub.MediaTypeAudio, MIME: "audio/x-amr", Size: 6},
		{Filename: "video.mp4", Type: socialhub.MediaTypeVideo, MIME: "video/mp4", Size: 6},
		{Filename: "file.bin", Type: socialhub.MediaTypeDocument, MIME: "application/octet-stream", Size: 6},
	}
	for index, descriptor := range valid {
		if _, err := client.BeginUpload(context.Background(), descriptor); err != nil {
			t.Fatalf("valid descriptor %d=%v", index, err)
		}
	}
	if _, err := client.UploadPart(context.Background(), "", 0, bytes.NewReader(nil)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid upload arguments=%v", err)
	}
	if _, err := client.UploadPart(context.Background(), "missing", 0, bytes.NewReader(nil)); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing upload session=%v", err)
	}
	if _, err := client.CompleteUpload(context.Background(), "missing", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid completion=%v", err)
	}
	if _, err := client.CompleteUpload(context.Background(), "missing", []socialhub.UploadedPart{{Number: 0}}); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing completion session=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing media=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty media ID=%v", err)
	}
	if got := safeFilename(`a\"b`); got != `a\\\"b` {
		t.Fatalf("safe filename=%q", got)
	}
}
