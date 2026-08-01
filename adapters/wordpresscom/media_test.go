package wordpresscom

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

func TestMediaUploadLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/rest/v1.1/sites/123/media/new":
			multipartReader, err := request.MultipartReader()
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			part, err := multipartReader.NextPart()
			if err != nil || part.FormName() != "media[]" || part.FileName() != "image.png" || part.Header.Get("Content-Type") != "image/png" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(part)
			if err != nil || string(body) != "data" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if _, err := multipartReader.NextPart(); !errors.Is(err, io.EOF) {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"media":[{"ID":31,"URL":"https://cdn.example/image.png","mime_type":"image/png","width":100,"height":80}],"errors":[]}`)
		case "/rest/v1.1/sites/123/media/31":
			writeJSON(writer, http.StatusOK, `{"ID":31,"URL":"https://cdn.example/image.png","mime_type":"image/png","size":4}`)
		case "/rest/v1.1/sites/123/media/31/delete":
			writeJSON(writer, http.StatusOK, `{"ID":31}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, []string{"media"})
	input := socialhub.BeginUploadRequest{Filename: "image.png", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: 4}
	session, err := client.BeginUpload(context.Background(), input)
	if err != nil || session.ID == "" || session.PartSize != 4 || session.MediaID != "" {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader("data")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("wrong part=%v", err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("data"))
	if err != nil || part.Number != 0 || part.Size != 4 {
		t.Fatalf("part=%#v err=%v", part, err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("data")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("reused part=%v", err)
	}
	if _, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{{Number: 0, Size: 3}}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("wrong complete=%v", err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "31" || media.Size == nil || *media.Size != 4 || media.State != socialhub.MediaStateReady || media.Width == nil || *media.Width != 100 {
		t.Fatalf("media=%#v err=%v", media, err)
	}
	if _, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("reused complete=%v", err)
	}
	status, err := client.MediaStatus(context.Background(), "31")
	if err != nil || status.ID != "31" || status.Size == nil || *status.Size != 4 {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	if err := client.DeleteMedia(context.Background(), "31"); err != nil {
		t.Fatal(err)
	}
}

func TestMediaValidationSizeAndPlatformErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/rest/v1.1/sites/123/media/new":
			multipartReader, err := request.MultipartReader()
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			part, err := multipartReader.NextPart()
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			filename := part.FileName()
			_, _ = io.Copy(io.Discard, part)
			switch filename {
			case "error.png":
				writeJSON(writer, http.StatusOK, `{"media":[],"errors":[{"error":"upload_error","message":"rejected"}]}`)
			case "multiple.png":
				writeJSON(writer, http.StatusOK, `{"media":[{"ID":1,"URL":"https://cdn.example/1"},{"ID":2,"URL":"https://cdn.example/2"}]}`)
			default:
				writeJSON(writer, http.StatusOK, `{"media":[{"ID":1,"URL":"https://cdn.example/1","mime_type":"image/png"}]}`)
			}
		case "/rest/v1.1/sites/123/media/1", "/rest/v1.1/sites/123/media/1/delete":
			writeJSON(writer, http.StatusOK, `{}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, []string{"media"})
	invalid := []socialhub.BeginUploadRequest{
		{},
		{Filename: "bad\nname", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: 1},
		{Filename: "x", Type: socialhub.MediaTypeImage, MIME: "bad", Size: 1},
		{Filename: "x", Type: socialhub.MediaTypeVideo, MIME: "image/png", Size: 1},
		{Filename: "x", Type: socialhub.MediaTypeDocument, MIME: "image/png", Size: 1},
		{Filename: "x", Type: "other", MIME: "application/octet-stream", Size: 1},
	}
	for index, input := range invalid {
		if _, err := client.BeginUpload(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid input %d error=%v", index, err)
		}
	}
	if _, err := client.UploadPart(context.Background(), "missing", 0, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing session=%v", err)
	}
	if _, err := client.UploadPart(context.Background(), "missing", 0, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil reader=%v", err)
	}
	if _, err := client.CompleteUpload(context.Background(), "missing", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing complete=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad status ID=%v", err)
	}
	if err := client.DeleteMedia(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad delete ID=%v", err)
	}

	for _, test := range []struct {
		filename string
		size     int64
		body     string
		code     socialhub.ErrorCode
	}{
		{"short.png", 2, "x", socialhub.CodeInvalidArgument},
		{"long.png", 1, "xx", socialhub.CodeInvalidArgument},
		{"error.png", 1, "x", socialhub.CodePlatformError},
		{"multiple.png", 1, "x", socialhub.CodePlatformError},
	} {
		session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: test.filename, Type: socialhub.MediaTypeImage, MIME: "image/png", Size: test.size})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.UploadPart(context.Background(), session.ID, 0, strings.NewReader(test.body))
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.code {
			t.Fatalf("upload %s error=%#v", test.filename, err)
		}
	}
	if _, err := client.MediaStatus(context.Background(), "1"); !isPlatformError(err) {
		t.Fatalf("bad status response=%v", err)
	}
	if err := client.DeleteMedia(context.Background(), "1"); !isPlatformError(err) {
		t.Fatalf("bad delete response=%v", err)
	}
}

func TestMediaScopeAndHelpers(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, []string{"posts"})
	input := socialhub.BeginUploadRequest{Filename: "x.pdf", Type: socialhub.MediaTypeDocument, MIME: "application/pdf", Size: 1}
	if _, err := client.BeginUpload(context.Background(), input); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
	first, err := newUploadID()
	if err != nil || len(first) != 32 {
		t.Fatalf("upload ID=%q err=%v", first, err)
	}
	second, err := newUploadID()
	if err != nil || first == second {
		t.Fatalf("second upload ID=%q err=%v", second, err)
	}
	if !validUploadRequest(input) {
		t.Fatal("document upload should be valid")
	}
}

func isPlatformError(err error) bool {
	var platformErr *socialhub.Error
	return errors.As(err, &platformErr) && platformErr.Code == socialhub.CodePlatformError
}
