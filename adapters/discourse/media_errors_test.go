package discourse

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestSynchronousMediaUpload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/uploads.json" || request.Header.Get("Api-Key") != "api-key" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
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
		content, _ := io.ReadAll(file)
		if request.FormValue("upload_type") != "composer" || request.FormValue("synchronous") != "true" || header.Filename != "photo.png" || string(content) != "png" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, `{"id":50,"url":"/uploads/default/original/photo.png","original_filename":"photo.png","filesize":3,"width":640,"height":480,"extension":"png","short_url":"upload://abc.png","short_path":"/uploads/short/photo.png"}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false)
	input := socialhub.BeginUploadRequest{Filename: "photo.png", MIME: "image/png", Type: socialhub.MediaTypeImage, Size: 3}
	session, err := client.BeginUpload(context.Background(), input)
	if err != nil || session.ID == "" || len(session.ID) != 32 || session.PartSize != 3 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("png"))
	if err != nil || part.Number != 0 || part.Size != 3 {
		t.Fatalf("part=%#v err=%v", part, err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "50" || media.URL != server.URL+"/uploads/default/original/photo.png" || media.MIME != "image/png" || media.Type != socialhub.MediaTypeImage || media.Size == nil || *media.Size != 3 || media.Width == nil || *media.Width != 640 || media.Height == nil || *media.Height != 480 || len(media.Extensions["discourse.upload"]) == 0 {
		t.Fatalf("media=%#v err=%v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), "50")
	if err != nil || status.ID != "50" || status.URL != media.URL {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	status.URL = "changed"
	again, _ := client.MediaStatus(context.Background(), "50")
	if again.URL == "changed" {
		t.Fatal("MediaStatus must return a copy")
	}
}

func TestMediaValidationAndFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		file, header, err := request.FormFile("file")
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = file.Close()
		if header.Filename == "bad.txt" {
			writeJSON(writer, http.StatusOK, `{"id":0}`)
			return
		}
		writeJSON(writer, http.StatusOK, `{"id":51,"short_url":"upload://file"}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false)

	invalid := []socialhub.BeginUploadRequest{
		{},
		{Filename: "bad\nname", MIME: "image/png", Type: socialhub.MediaTypeImage, Size: 1},
		{Filename: "x", MIME: "bad", Type: socialhub.MediaTypeImage, Size: 1},
		{Filename: "x", MIME: "text/plain", Type: socialhub.MediaTypeImage, Size: 1},
		{Filename: "x", MIME: "image/png", Type: "other", Size: 1},
		{Filename: "x", MIME: "image/png", Type: socialhub.MediaTypeImage, Size: 1, Category: "avatar"},
	}
	for index, input := range invalid {
		if _, err := client.BeginUpload(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid upload %d error=%v", index, err)
		}
	}
	if !validUploadRequest(socialhub.BeginUploadRequest{Filename: "x.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: 1}) ||
		!validUploadRequest(socialhub.BeginUploadRequest{Filename: "x.mp3", MIME: "audio/mpeg", Type: socialhub.MediaTypeAudio, Size: 1}) ||
		!validUploadRequest(socialhub.BeginUploadRequest{Filename: "x.pdf", MIME: "application/pdf", Type: socialhub.MediaTypeDocument, Size: 1}) ||
		!validUploadRequest(socialhub.BeginUploadRequest{Filename: "x.gif", MIME: "image/gif", Type: socialhub.MediaTypeAnimation, Size: 1, Category: "custom_emoji"}) {
		t.Fatal("valid upload types rejected")
	}
	if uploadType("") != "composer" || uploadType("custom_emoji") != "custom_emoji" || !validUploadType("profile_background") || validUploadType("avatar") {
		t.Fatal("upload type validation failed")
	}

	input := socialhub.BeginUploadRequest{Filename: "x.txt", MIME: "text/plain", Type: socialhub.MediaTypeDocument, Size: 2}
	session, _ := client.BeginUpload(context.Background(), input)
	if _, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader("ab")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("part number=%v", err)
	}
	if _, err := client.UploadPart(context.Background(), "missing", 0, strings.NewReader("ab")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing session=%v", err)
	}
	if _, err := client.CompleteUpload(context.Background(), session.ID, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing parts=%v", err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("abc")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("size mismatch=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad media ID=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), "999"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unknown media=%v", err)
	}
	badSession, _ := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{
		Filename: "bad.txt", MIME: "text/plain", Type: socialhub.MediaTypeDocument, Size: 2,
	})
	var platformErr *socialhub.Error
	if _, err := client.UploadPart(context.Background(), badSession.ID, 0, strings.NewReader("ab")); !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
		t.Fatalf("bad upload response=%v", err)
	}

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	_ = pipeReader.Close()
	if err := writeUpload(pipeWriter, multipartWriter, input, bytes.NewReader([]byte("ab"))); err == nil {
		t.Fatal("closed pipe must fail")
	}
}

func TestHTTPErrorMapping(t *testing.T) {
	header := http.Header{
		"Retry-After":                     {"2.5"},
		"Discourse-Rate-Limit-Error-Code": {"admin_api_key_rate_limit"},
		"X-Request-Id":                    {"request-1"},
	}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"errors":["Slow down"],"error_type":"rate_limit","extras":{"wait_seconds":1}}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || !platformErr.Retryable() || platformErr.RetryAfter != 2500*time.Millisecond || platformErr.PlatformCode != "admin_api_key_rate_limit" || platformErr.PlatformMessage != "Slow down" || platformErr.RequestID != "request-1" {
		t.Fatalf("rate error=%#v", platformErr)
	}
	err = decodeHTTPError(http.StatusTooManyRequests, nil, []byte(`{"error":"Wait","extras":{"wait_seconds":1.25}}`))
	if !errors.As(err, &platformErr) || platformErr.RetryAfter != 1250*time.Millisecond || platformErr.PlatformMessage != "Wait" {
		t.Fatalf("body retry error=%#v", platformErr)
	}
	statuses := map[int]socialhub.ErrorCode{
		http.StatusBadRequest: socialhub.CodeInvalidArgument, http.StatusUnauthorized: socialhub.CodeUnauthenticated,
		http.StatusForbidden: socialhub.CodePermissionDenied, http.StatusNotFound: socialhub.CodeNotFound,
		http.StatusConflict: socialhub.CodeConflict, http.StatusInternalServerError: socialhub.CodeTemporarilyUnavailable,
		http.StatusTeapot: socialhub.CodePlatformError,
	}
	for status, want := range statuses {
		code, _ := classifyError(status)
		if code != want {
			t.Fatalf("status %d code=%s want=%s", status, code, want)
		}
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("90000") != 0 || parseRetryAfter("1.5") != 1500*time.Millisecond {
		t.Fatal("Retry-After parsing failed")
	}
}

func TestTransportAndResponseFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/posts/1.json":
			writeJSON(writer, http.StatusTooManyRequests, `{"errors":["slow"]}`)
		case "/posts/2.json":
			writeJSON(writer, http.StatusOK, `{`)
		case "/posts/3.json":
			writeJSON(writer, http.StatusOK, postFixture(4, 1, 1, "wrong"))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false)
	if _, err := client.GetPost(context.Background(), "1"); !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("rate limited=%v", err)
	}
	for _, id := range []string{"2", "3"} {
		var platformErr *socialhub.Error
		if _, err := client.GetPost(context.Background(), id); !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("post %s error=%v", id, err)
		}
	}
}
