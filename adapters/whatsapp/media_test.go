package whatsapp

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

func TestMediaUploadAndLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requireBearer(t, request)
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/123456789/media":
			reader, err := request.MultipartReader()
			if err != nil {
				t.Errorf("multipart reader: %v", err)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			fields := map[string]string{}
			for {
				part, err := reader.NextPart()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Errorf("next part: %v", err)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				data, err := io.ReadAll(part)
				if err != nil {
					t.Errorf("read part: %v", err)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				if part.FormName() == "file" {
					if part.FileName() != "photo.jpg" || part.Header.Get("Content-Type") != "image/jpeg" || string(data) != "image-bytes" {
						t.Errorf("file name=%q type=%q data=%q", part.FileName(), part.Header.Get("Content-Type"), data)
						writer.WriteHeader(http.StatusBadRequest)
						return
					}
				} else {
					fields[part.FormName()] = string(data)
				}
			}
			if fields["messaging_product"] != "whatsapp" || fields["type"] != "image/jpeg" {
				t.Errorf("fields=%v", fields)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]string{"id": "media-1"})
		case request.Method == http.MethodGet && request.URL.Path == "/media-1":
			if request.URL.Query().Get("phone_number_id") != "123456789" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"id": "media-1", "url": "https://lookaside.example.test/media", "mime_type": "image/jpeg",
				"sha256": "digest", "file_size": "11", "messaging_product": "whatsapp",
			})
		case request.Method == http.MethodGet && request.URL.Path == "/media-2":
			writeTestJSON(t, writer, map[string]any{
				"id": "media-2", "url": "https://lookaside.example.test/media2", "mime_type": "image/jpeg",
				"sha256": "digest2", "file_size": 12, "messaging_product": "whatsapp",
			})
		case request.Method == http.MethodDelete && request.URL.Path == "/media-1":
			if request.URL.Query().Get("phone_number_id") != "123456789" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]bool{"success": true})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)
	data := []byte("image-bytes")
	uploaded, err := client.UploadMedia(context.Background(), MediaUploadRequest{
		Filename: "photo.jpg", MIME: " IMAGE/JPEG ", Size: int64(len(data)), Reader: bytes.NewReader(data),
	})
	if err != nil || uploaded.ID != "media-1" || uploaded.MIME != "image/jpeg" || uploaded.Size != int64(len(data)) || uploaded.MessagingProduct != "whatsapp" {
		t.Fatalf("uploaded=%#v error=%v", uploaded, err)
	}
	info, err := client.GetMedia(context.Background(), "media-1")
	if err != nil || info.Size != 11 || info.URL == "" || info.SHA256 != "digest" {
		t.Fatalf("info=%#v error=%v", info, err)
	}
	numeric, err := client.GetMedia(context.Background(), "media-2")
	if err != nil || numeric.Size != 12 {
		t.Fatalf("numeric=%#v error=%v", numeric, err)
	}
	if err := client.DeleteMedia(context.Background(), "media-1"); err != nil {
		t.Fatal(err)
	}
}

func TestMediaValidationAndDeclaredSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writeTestJSON(t, writer, map[string]string{"id": "media"})
	}))
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)
	invalid := []MediaUploadRequest{
		{},
		{Filename: "file.gif", MIME: "image/gif", Size: 1, Reader: strings.NewReader("x")},
		{Filename: "large.jpg", MIME: "image/jpeg", Size: 5*megabyte + 1, Reader: strings.NewReader("x")},
	}
	for _, input := range invalid {
		if _, err := client.UploadMedia(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
	if _, err := client.UploadMedia(context.Background(), MediaUploadRequest{Filename: "short.txt", MIME: "text/plain", Size: 4, Reader: strings.NewReader("abc")}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("short read error=%v", err)
	}
	if _, err := client.UploadMedia(context.Background(), MediaUploadRequest{Filename: "long.txt", MIME: "text/plain", Size: 3, Reader: strings.NewReader("abcd")}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long read error=%v", err)
	}
	if _, err := client.GetMedia(context.Background(), " "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("get empty error=%v", err)
	}
	if err := client.DeleteMedia(context.Background(), " "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("delete empty error=%v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestMediaUploadEarlyHTTPResponseDoesNotBlock(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"id":"media-early"}`)), Request: request,
		}, nil
	})}
	client := newTestClientWithHTTP(t, "https://graph.example.test/v25.0", httpClient, allScopes(), true)
	done := make(chan error, 1)
	go func() {
		_, err := client.UploadMedia(context.Background(), MediaUploadRequest{
			Filename: "photo.jpg", MIME: "image/jpeg", Size: 1 << 20, Reader: io.LimitReader(zeroReader{}, 1<<20),
		})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("early response error=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("upload blocked after HTTP response stopped reading the request body")
	}
}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 0
	}
	return len(buffer), nil
}
