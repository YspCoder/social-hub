package vk

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

func TestWallPhotoUploadContract(t *testing.T) {
	uploadServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "" || request.Header.Get("X-Request-ID") != "upload-request" {
			http.Error(writer, "bad upload headers", http.StatusBadRequest)
			t.Errorf("upload headers=%v", request.Header)
			return
		}
		file, header, err := request.FormFile("photo")
		if err != nil {
			http.Error(writer, "missing photo", http.StatusBadRequest)
			t.Errorf("form file: %v", err)
			return
		}
		defer file.Close()
		body, _ := io.ReadAll(file)
		if string(body) != "image" || header.Filename != "photo.jpg" || header.Header.Get("Content-Type") != "image/jpeg" {
			http.Error(writer, "bad photo", http.StatusBadRequest)
			t.Errorf("upload filename=%q type=%q body=%q", header.Filename, header.Header.Get("Content-Type"), body)
			return
		}
		writeTestJSON(t, writer, map[string]any{"server": 9, "photo": "opaque-photo", "hash": "opaque-hash"})
	}))
	defer uploadServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/method/")
		form, ok := requireVKRequest(t, writer, request, method)
		if !ok {
			return
		}
		switch method {
		case "photos.getWallUploadServer":
			if form.Get("group_id") != "456" {
				t.Errorf("upload server form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"upload_url": uploadServer.URL + "/upload"}})
		case "photos.saveWallPhoto":
			if form.Get("server") != "9" || form.Get("photo") != "opaque-photo" || form.Get("hash") != "opaque-hash" || form.Get("group_id") != "456" || form.Get("user_id") != "" {
				t.Errorf("save photo form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": []any{map[string]any{
				"id": 7, "owner_id": -456, "width": 800, "height": 600,
				"sizes": []any{map[string]any{"url": "https://cdn.test/photo.jpg", "width": 800, "height": 600}},
			}}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer apiServer.Close()
	_, client := newTestAdapter(t, apiServer, TokenUser, -456, false)

	request := socialhub.BeginUploadRequest{Filename: "photo.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 5, Category: "wall"}
	session, err := client.BeginUpload(context.Background(), request)
	if err != nil || !strings.HasPrefix(session.ID, "wall-photo:") || session.PartSize != 5 {
		t.Fatalf("session=%#v error=%v", session, err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("image"), socialhub.WithRequestID("upload-request"))
	if err != nil || part.Number != 0 || part.ETag != "opaque-hash" || part.Size != 5 {
		t.Fatalf("part=%#v error=%v", part, err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "photo-456_7" || media.URL != "https://cdn.test/photo.jpg" || media.MIME != "image/jpeg" || media.Size == nil || *media.Size != 5 || media.State != socialhub.MediaStateReady {
		t.Fatalf("media=%#v error=%v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), media.ID)
	if err != nil || status.ID != media.ID || status == media {
		t.Fatalf("status=%#v error=%v", status, err)
	}
	if _, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part}); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("completed session reuse=%v", err)
	}
}

func TestWallPhotoUploadValidationAndState(t *testing.T) {
	uploadServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writeTestJSON(t, writer, map[string]any{"server": 1, "photo": "photo", "hash": "hash"})
	}))
	defer uploadServer.Close()
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/method/")
		if _, ok := requireVKRequest(t, writer, request, method); !ok {
			return
		}
		switch method {
		case "photos.getWallUploadServer":
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"upload_url": uploadServer.URL}})
		case "photos.saveWallPhoto":
			writeTestJSON(t, writer, map[string]any{"response": []any{map[string]any{"id": 1, "owner_id": 123}}})
		}
	}))
	defer apiServer.Close()
	_, user := newTestAdapter(t, apiServer, TokenUser, 123, false)
	_, community := newTestAdapter(t, apiServer, TokenCommunity, -456, false)

	invalidBegin := []socialhub.BeginUploadRequest{
		{},
		{Filename: "x", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: maxWallPhotoBytes + 1},
		{Filename: "x", Type: socialhub.MediaTypeVideo, MIME: "video/mp4", Size: 1},
		{Filename: "x", Type: socialhub.MediaTypeImage, MIME: "image/webp", Size: 1},
		{Filename: "x", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: 1, Category: "avatar"},
	}
	for index, input := range invalidBegin {
		if _, err := user.BeginUpload(context.Background(), input); err == nil {
			t.Fatalf("invalid begin %d accepted", index)
		}
	}
	valid := socialhub.BeginUploadRequest{Filename: "x.png", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: 4}
	if _, err := community.BeginUpload(context.Background(), valid); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("community begin=%v", err)
	}
	session, err := user.BeginUpload(context.Background(), valid)
	if err != nil {
		t.Fatal(err)
	}
	invalidParts := []struct {
		id     string
		number int
		reader io.Reader
	}{
		{"", 0, strings.NewReader("data")},
		{session.ID, 1, strings.NewReader("data")},
		{session.ID, 0, nil},
		{"missing", 0, strings.NewReader("data")},
	}
	for index, input := range invalidParts {
		if _, err := user.UploadPart(context.Background(), input.id, input.number, input.reader); err == nil {
			t.Fatalf("invalid part %d accepted", index)
		}
	}
	if _, err := user.UploadPart(context.Background(), session.ID, 0, strings.NewReader("too long")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("byte mismatch=%v", err)
	}
	part, err := user.UploadPart(context.Background(), session.ID, 0, strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := user.UploadPart(context.Background(), session.ID, 0, strings.NewReader("data")); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("duplicate part=%v", err)
	}
	invalidCompletes := [][]socialhub.UploadedPart{
		nil,
		{{Number: 1, ETag: part.ETag, Size: part.Size}},
		{{Number: 0, ETag: "wrong", Size: part.Size}},
		{{Number: 0, ETag: part.ETag, Size: part.Size + 1}},
	}
	for index, parts := range invalidCompletes {
		if _, err := user.CompleteUpload(context.Background(), session.ID, parts); err == nil {
			t.Fatalf("invalid complete %d accepted", index)
		}
	}
	if _, err := user.CompleteUpload(context.Background(), "missing", []socialhub.UploadedPart{*part}); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing complete=%v", err)
	}
	if _, err := user.MediaStatus(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("blank media status=%v", err)
	}
	if _, err := user.MediaStatus(context.Background(), "photo123_1"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing media status=%v", err)
	}
}

func TestUploadURLPolicyAndTimeout(t *testing.T) {
	uploadServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
			return
		case <-time.After(200 * time.Millisecond):
			writeTestJSON(t, writer, map[string]any{"server": 1, "photo": "photo", "hash": "hash"})
		}
	}))
	defer uploadServer.Close()
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/method/")
		if _, ok := requireVKRequest(t, writer, request, method); !ok {
			return
		}
		writeTestJSON(t, writer, map[string]any{"response": map[string]any{"upload_url": uploadServer.URL}})
	}))
	defer apiServer.Close()
	_, client := newTestAdapter(t, apiServer, TokenUser, 123, false)
	input := socialhub.BeginUploadRequest{Filename: "x.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 4}

	client.allowHTTPUploads = false
	if _, err := client.BeginUpload(context.Background(), input); err == nil {
		t.Fatal("HTTPS API origin accepted HTTP upload URL")
	}
	client.allowHTTPUploads = true
	session, err := client.BeginUpload(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err = client.UploadPart(context.Background(), session.ID, 0, bytes.NewReader([]byte("data")), socialhub.WithCallTimeout(30*time.Millisecond))
	platformErr := requireErrorCode(t, err, socialhub.CodeTemporarilyUnavailable)
	if platformErr.Class != socialhub.ClassRetryable || time.Since(started) > time.Second {
		t.Fatalf("timeout error=%#v elapsed=%s", platformErr, time.Since(started))
	}
	if validUploadURL("http://upload.test", false) || !validUploadURL("http://upload.test", true) || !validUploadURL("https://upload.test", false) || validUploadURL("https://user@upload.test", false) || validUploadURL("://bad", true) {
		t.Fatal("upload URL validation mismatch")
	}
	if got := safeFilename("a\r\n\\\"b.jpg"); strings.ContainsAny(got, "\r\n") || !strings.Contains(got, `\\`) || !strings.Contains(got, `\"`) {
		t.Fatalf("safe filename=%q", got)
	}
}

func TestMalformedUploadResponses(t *testing.T) {
	uploadReply := `{"server":0,"photo":"","hash":""}`
	uploadStatus := http.StatusOK
	uploadServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writer.WriteHeader(uploadStatus)
		_, _ = io.WriteString(writer, uploadReply)
	}))
	defer uploadServer.Close()
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/method/")
		if _, ok := requireVKRequest(t, writer, request, method); !ok {
			return
		}
		if method == "photos.getWallUploadServer" {
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"upload_url": uploadServer.URL}})
			return
		}
		writeTestJSON(t, writer, map[string]any{"response": []any{}})
	}))
	defer apiServer.Close()
	_, client := newTestAdapter(t, apiServer, TokenUser, 123, false)
	input := socialhub.BeginUploadRequest{Filename: "x.gif", Type: socialhub.MediaTypeImage, MIME: "image/gif", Size: 4}
	session, err := client.BeginUpload(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("data"))
	requireErrorCode(t, err, socialhub.CodePlatformError)
	uploadReply = `{"server":1,"photo":"photo","hash":"hash"}`
	part, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	requireErrorCode(t, err, socialhub.CodePlatformError)

	second, err := client.BeginUpload(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	uploadStatus = http.StatusBadGateway
	_, err = client.UploadPart(context.Background(), second.ID, 0, strings.NewReader("data"))
	requireErrorCode(t, err, socialhub.CodePlatformError)
}
