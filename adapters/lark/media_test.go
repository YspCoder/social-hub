package lark

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestMediaUploadContract(t *testing.T) {
	imageBytes := []byte("image-bytes")
	fileBytes := []byte("pdf-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer tenant-access-token" || request.Header.Get("Accept") != "application/json" || request.Header.Get("Idempotency-Key") != "" {
			t.Errorf("unexpected headers: %v", request.Header)
			http.Error(writer, "bad headers", http.StatusBadRequest)
			return
		}
		if err := request.ParseMultipartForm(maxFileBytes); err != nil {
			t.Errorf("parse multipart: %v", err)
			http.Error(writer, "bad multipart", http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/open-apis/im/v1/images":
			if request.FormValue("image_type") != "message" {
				t.Errorf("image_type=%q", request.FormValue("image_type"))
			}
			file, header, err := request.FormFile("image")
			if err != nil {
				t.Errorf("image form file: %v", err)
				return
			}
			defer file.Close()
			content, _ := io.ReadAll(file)
			if header.Filename != "picture.png" || header.Header.Get("Content-Type") != "image/png" || !bytes.Equal(content, imageBytes) {
				t.Errorf("image header=%v content=%q", header.Header, content)
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]string{"image_key": "img_testresource"}})
		case "/open-apis/im/v1/files":
			if request.FormValue("file_type") != "pdf" || request.FormValue("file_name") != "report.pdf" {
				t.Errorf("file form=%v", request.MultipartForm.Value)
			}
			file, header, err := request.FormFile("file")
			if err != nil {
				t.Errorf("file form file: %v", err)
				return
			}
			defer file.Close()
			content, _ := io.ReadAll(file)
			if header.Filename != "report.pdf" || header.Header.Get("Content-Type") != "application/pdf" || !bytes.Equal(content, fileBytes) {
				t.Errorf("file header=%v content=%q", header.Header, content)
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]string{"file_key": testResourceKey}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)

	imageSession, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{
		Filename: "picture.png", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: int64(len(imageBytes)),
	})
	if err != nil || !strings.HasPrefix(imageSession.ID, "lark-resource:") || imageSession.PartSize != int64(len(imageBytes)) {
		t.Fatalf("image session=%#v err=%v", imageSession, err)
	}
	imagePart, err := client.UploadPart(context.Background(), imageSession.ID, 0, bytes.NewReader(imageBytes))
	if err != nil || imagePart.ETag != "img_testresource" || imagePart.Size != int64(len(imageBytes)) {
		t.Fatalf("image part=%#v err=%v", imagePart, err)
	}
	if _, err := client.UploadPart(context.Background(), imageSession.ID, 0, bytes.NewReader(imageBytes)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("duplicate upload=%v", err)
	}
	image, err := client.CompleteUpload(context.Background(), imageSession.ID, []socialhub.UploadedPart{*imagePart})
	if err != nil || image.ID != "img_testresource" || image.Type != socialhub.MediaTypeImage || image.State != socialhub.MediaStateReady || image.Size == nil || *image.Size != int64(len(imageBytes)) {
		t.Fatalf("image=%#v err=%v", image, err)
	}
	status, err := client.MediaStatus(context.Background(), image.ID)
	if err != nil || status.ID != image.ID || status.Extensions["lark.resource"] == nil {
		t.Fatalf("status=%#v err=%v", status, err)
	}

	fileSession, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{
		Filename: "report.pdf", Type: socialhub.MediaTypeDocument, MIME: "application/pdf", Size: int64(len(fileBytes)),
	})
	if err != nil {
		t.Fatal(err)
	}
	filePart, err := client.UploadPart(context.Background(), fileSession.ID, 0, bytes.NewReader(fileBytes))
	if err != nil || filePart.ETag != testResourceKey {
		t.Fatalf("file part=%#v err=%v", filePart, err)
	}
	file, err := client.CompleteUpload(context.Background(), fileSession.ID, []socialhub.UploadedPart{*filePart})
	if err != nil || file.ID != testResourceKey || file.Type != socialhub.MediaTypeDocument {
		t.Fatalf("file=%#v err=%v", file, err)
	}
}

func TestMediaUploadValidationAndState(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)
	ctx := context.Background()

	invalidBegins := []socialhub.BeginUploadRequest{
		{},
		{Filename: "bad/name", Type: socialhub.MediaTypeImage, Size: 1},
		{Filename: "image.png", Type: socialhub.MediaTypeImage, MIME: " image/png", Size: 1},
		{Filename: "image.png", Type: socialhub.MediaTypeImage, Size: 1, Category: "avatar"},
		{Filename: "image.png", Type: socialhub.MediaTypeImage, Size: maxImageBytes + 1},
		{Filename: "file.bin", Type: socialhub.MediaTypeDocument, Size: maxFileBytes + 1},
		{Filename: "file.bin", Type: socialhub.MediaTypeDocument, Size: 1, Category: "archive"},
	}
	for index, input := range invalidBegins {
		if _, err := client.BeginUpload(ctx, input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid begin %d=%v", index, err)
		}
	}
	if _, err := client.BeginUpload(ctx, socialhub.BeginUploadRequest{Filename: "x", Type: socialhub.MediaTypeDocument, Size: 1}, socialhub.WithIdempotencyKey("key")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("begin options=%v", err)
	}

	session, err := client.BeginUpload(ctx, socialhub.BeginUploadRequest{Filename: "x.bin", Type: socialhub.MediaTypeDocument, Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CompleteUpload(ctx, session.ID, []socialhub.UploadedPart{{Number: 0, ETag: "key", Size: 2}}); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("complete before upload=%v", err)
	}
	invalidUploads := []struct {
		session string
		part    int
		reader  io.Reader
	}{
		{"", 0, strings.NewReader("xx")},
		{session.ID, 1, strings.NewReader("xx")},
		{session.ID, 0, nil},
		{"lark-resource:missing", 0, strings.NewReader("xx")},
	}
	for index, input := range invalidUploads {
		_, err := client.UploadPart(ctx, input.session, input.part, input.reader)
		if index == len(invalidUploads)-1 {
			if !errors.Is(err, socialhub.ErrNotFound) {
				t.Fatalf("unknown upload=%v", err)
			}
		} else if !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid upload %d=%v", index, err)
		}
	}
	if _, err := client.UploadPart(ctx, session.ID, 0, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("short upload=%v", err)
	}
	if _, err := client.UploadPart(ctx, session.ID, 0, strings.NewReader("xx"), socialhub.WithFields("id")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("upload options=%v", err)
	}
	if _, err := client.CompleteUpload(ctx, "", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid complete=%v", err)
	}
	if _, err := client.CompleteUpload(ctx, "lark-resource:missing", []socialhub.UploadedPart{{Number: 0}}); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("unknown complete=%v", err)
	}
	if _, err := client.CompleteUpload(ctx, session.ID, []socialhub.UploadedPart{{Number: 0}}, socialhub.WithFields("id")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("complete options=%v", err)
	}
	if _, err := client.MediaStatus(ctx, "bad/key"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid status=%v", err)
	}
	if _, err := client.MediaStatus(ctx, "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("unknown status=%v", err)
	}
	if _, err := client.MediaStatus(ctx, "missing", socialhub.WithFields("id")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("status options=%v", err)
	}
}

func TestMediaHelpers(t *testing.T) {
	tests := []struct {
		input socialhub.BeginUploadRequest
		want  string
	}{
		{socialhub.BeginUploadRequest{Category: "mp4"}, "mp4"},
		{socialhub.BeginUploadRequest{MIME: "audio/opus"}, "opus"},
		{socialhub.BeginUploadRequest{MIME: "video/mp4"}, "mp4"},
		{socialhub.BeginUploadRequest{MIME: "application/pdf"}, "pdf"},
		{socialhub.BeginUploadRequest{MIME: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"}, "doc"},
		{socialhub.BeginUploadRequest{MIME: "application/vnd.ms-excel"}, "xls"},
		{socialhub.BeginUploadRequest{MIME: "application/vnd.ms-powerpoint"}, "ppt"},
		{socialhub.BeginUploadRequest{MIME: "application/octet-stream"}, "stream"},
	}
	for _, test := range tests {
		got, err := larkFileType(test.input)
		if err != nil || got != test.want {
			t.Fatalf("file type input=%#v got=%q err=%v", test.input, got, err)
		}
	}
	if messageTypeForMedia(socialhub.MediaTypeImage) != "image" || messageTypeForMedia(socialhub.MediaTypeAudio) != "audio" || messageTypeForMedia(socialhub.MediaTypeVideo) != "media" || messageTypeForMedia(socialhub.MediaTypeDocument) != "file" {
		t.Fatal("message media type mapping mismatch")
	}
	if !validFilename("report.pdf") || validFilename("bad/name") || validFilename(strings.Repeat("x", 256)) || !validMIME("image/png") || validMIME("image /png") || safeFilename(`a"b`) != `a\"b` {
		t.Fatal("media validation helper mismatch")
	}
}
