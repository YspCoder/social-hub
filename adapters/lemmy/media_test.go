package lemmy

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

	"social-hub/pkg/socialhub"
)

func TestPictrsImageUploadLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/pictrs/image" || request.Header.Get("Authorization") != "Bearer jwt-token" ||
			request.Header.Get("User-Agent") != userAgent {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		file, header, err := request.FormFile("images[]")
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()
		body, _ := io.ReadAll(file)
		if header.Filename != "image.png" || header.Header.Get("Content-Type") != "image/png" || string(body) != "png" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, `{"msg":"ok","files":[{"file":"hash/image.png","delete_token":"delete-secret"}]}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	input := socialhub.BeginUploadRequest{Filename: "image.png", MIME: "image/png", Type: socialhub.MediaTypeImage, Size: 3}
	session, err := client.BeginUpload(context.Background(), input)
	if err != nil || len(session.ID) != 32 || session.PartSize != 3 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("png"))
	if err != nil || part.Number != 0 || part.Size != 3 || part.ETag != "hash/image.png" {
		t.Fatalf("part=%#v err=%v", part, err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("png")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("repeated part=%v", err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "hash/image.png" || media.URL != server.URL+"/pictrs/image/hash%2Fimage.png" ||
		media.MIME != "image/png" || media.Type != socialhub.MediaTypeImage || media.Size == nil || *media.Size != 3 ||
		media.State != socialhub.MediaStateReady || len(media.Extensions["lemmy.pictrs_file"]) == 0 {
		t.Fatalf("media=%#v err=%v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), media.ID)
	if err != nil || status.URL != media.URL {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	status.URL = "changed"
	again, _ := client.MediaStatus(context.Background(), media.ID)
	if again.URL == "changed" {
		t.Fatal("MediaStatus must return a value copy")
	}
}

func TestPictrsValidationAndFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			writeJSON(writer, http.StatusBadRequest, `{"error":"invalid_image"}`)
			return
		}
		file, header, err := request.FormFile("images[]")
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.Copy(io.Discard, file)
		_ = file.Close()
		switch header.Filename {
		case "large.png":
			writeJSON(writer, http.StatusOK, `{"msg":"too_large"}`)
		case "bad.png":
			writeJSON(writer, http.StatusOK, `{"msg":"error"}`)
		case "http.png":
			writeJSON(writer, http.StatusRequestEntityTooLarge, `{"error":"too_large"}`)
		default:
			writeJSON(writer, http.StatusOK, `{"msg":"ok","files":[{"file":"good.png","delete_token":"token"}]}`)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	invalid := []socialhub.BeginUploadRequest{
		{},
		{Filename: "bad\nname", MIME: "image/png", Type: socialhub.MediaTypeImage, Size: 1},
		{Filename: "x", MIME: "bad", Type: socialhub.MediaTypeImage, Size: 1},
		{Filename: "x", MIME: "video/mp4", Type: socialhub.MediaTypeImage, Size: 1},
		{Filename: "x", MIME: "image/png", Type: socialhub.MediaTypeVideo, Size: 1},
		{Filename: "x", MIME: "image/png", Type: socialhub.MediaTypeImage, Size: 1, Category: "avatar"},
	}
	for index, input := range invalid {
		if _, err := client.BeginUpload(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid upload %d error=%v", index, err)
		}
	}
	if !validImageUpload(socialhub.BeginUploadRequest{Filename: "x.gif", MIME: "image/gif", Type: socialhub.MediaTypeAnimation, Size: 1}) {
		t.Fatal("valid animation rejected")
	}
	if _, err := client.UploadPart(context.Background(), "missing", 1, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad part number=%v", err)
	}
	if _, err := client.UploadPart(context.Background(), "missing", 0, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil part=%v", err)
	}
	if _, err := client.UploadPart(context.Background(), "missing", 0, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing session=%v", err)
	}
	if _, err := client.CompleteUpload(context.Background(), "missing", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing complete=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), "bad\nid"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad status ID=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), "unknown.png"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unknown status=%v", err)
	}

	short, _ := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "short.png", MIME: "image/png", Type: socialhub.MediaTypeImage, Size: 3})
	if _, err := client.UploadPart(context.Background(), short.ID, 0, strings.NewReader("xy")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("size mismatch=%v", err)
	}
	for _, filename := range []string{"large.png", "bad.png", "http.png"} {
		session, _ := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: filename, MIME: "image/png", Type: socialhub.MediaTypeImage, Size: 1})
		_, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("x"))
		if filename == "bad.png" {
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
				t.Fatalf("bad response=%v", err)
			}
		} else if !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("%s error=%v", filename, err)
		}
	}

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	_ = pipeReader.Close()
	input := socialhub.BeginUploadRequest{Filename: "x.png", MIME: "image/png", Type: socialhub.MediaTypeImage, Size: 1}
	if err := writeImagePart(pipeWriter, multipartWriter, input, bytes.NewReader([]byte("x"))); err == nil {
		t.Fatal("closed pipe must fail")
	}
	if !validMediaID("file.png") || validMediaID("") || validMediaID("bad\nid") {
		t.Fatal("media ID validation failed")
	}
}
