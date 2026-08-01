package bluesky

import (
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

func TestBlobUploadLifecycle(t *testing.T) {
	const body = "image-bytes"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/xrpc/com.atproto.repo.uploadBlob" || request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Content-Type") != "image/png" || request.ContentLength != int64(len(body)) {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		payload, err := io.ReadAll(request.Body)
		if err != nil || string(payload) != body {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writeTestJSON(t, writer, map[string]any{"blob": map[string]any{
			"$type": "blob", "ref": map[string]string{"$link": "bafk-image"}, "mimeType": "image/png", "size": len(body),
		}})
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	input := socialhub.BeginUploadRequest{Filename: "photo.png", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: int64(len(body))}
	session, err := client.BeginUpload(context.Background(), input)
	if err != nil || session.ID == "" || session.PartSize != input.Size {
		t.Fatalf("session=%#v error=%v", session, err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader(body), socialhub.WithRequestID("upload-1"))
	if err != nil || part.Number != 1 || part.Size != input.Size {
		t.Fatalf("part=%#v error=%v", part, err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader(body)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("repeated upload error=%v", err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "bafk-image" || media.MIME != "image/png" || media.Type != socialhub.MediaTypeImage || media.State != socialhub.MediaStateReady || media.Size == nil || *media.Size != input.Size {
		t.Fatalf("media=%#v error=%v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), media.ID)
	if err != nil || status.ID != media.ID || status.State != socialhub.MediaStateReady {
		t.Fatalf("status=%#v error=%v", status, err)
	}
	if _, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("repeated completion error=%v", err)
	}
}

func TestBlobUploadReaderBoundsAndRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writeTestJSON(t, writer, map[string]any{"blob": map[string]any{
			"$type": "blob", "ref": map[string]string{"$link": "bafk-bounded"}, "mimeType": "image/jpeg", "size": 5,
		}})
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	input := socialhub.BeginUploadRequest{Filename: "photo.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 5}

	for name, source := range map[string]io.Reader{
		"short": strings.NewReader("123"),
		"long":  strings.NewReader("123456"),
	} {
		t.Run(name, func(t *testing.T) {
			session, err := client.BeginUpload(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.UploadPart(context.Background(), session.ID, 1, source); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("size mismatch error=%v", err)
			}
			part, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader("12345"))
			if err != nil || part.Size != 5 {
				t.Fatalf("retry part=%#v error=%v", part, err)
			}
		})
	}

	session, err := client.BeginUpload(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader("12345"), socialhub.WithCallTimeout(-time.Second)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("request construction error=%v", err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader("12345")); err != nil {
		t.Fatalf("retry after request error=%v", err)
	}
}

func TestBlobUploadEarlyServerRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusRequestEntityTooLarge)
		writeTestJSON(t, writer, map[string]string{"error": "BlobTooLarge", "message": "provider limit exceeded"})
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{
		Filename: "video.mp4", Type: socialhub.MediaTypeVideo, MIME: "video/mp4", Size: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.UploadPart(ctx, session.ID, 1, strings.NewReader(strings.Repeat("x", 1024))); err == nil {
		t.Fatal("server rejection should fail")
	}
}

func TestBlobUploadValidationAndClientIsolation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server)
	invalid := []socialhub.BeginUploadRequest{
		{},
		{Filename: "photo.png", Type: socialhub.MediaTypeImage, MIME: "text/plain", Size: 1},
		{Filename: "photo.png", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: maxImageBlobSize + 1},
		{Filename: "clip.mov", Type: socialhub.MediaTypeVideo, MIME: "video/quicktime", Size: 1},
		{Filename: "clip.mp4", Type: socialhub.MediaTypeVideo, MIME: "video/mp4", Size: maxVideoBlobSize + 1},
		{Filename: "document.pdf", Type: socialhub.MediaTypeDocument, MIME: "application/pdf", Size: 1},
	}
	for _, input := range invalid {
		if _, err := client.BeginUpload(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
	if _, err := client.UploadPart(context.Background(), "missing", 2, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("part validation error=%v", err)
	}
	if _, err := client.CompleteUpload(context.Background(), "missing", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("completion validation error=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty media ID error=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), "bafk-other-client"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("foreign blob error=%v", err)
	}
	if _, _, err := client.buildMediaEmbed([]PostMedia{{MediaID: "bafk-other-client"}}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("foreign publish media error=%v", err)
	}
}

type terminalErrorReader struct {
	data string
	done bool
}

func (r *terminalErrorReader) Read(buffer []byte) (int, error) {
	if r.data != "" {
		n := copy(buffer, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	if !r.done {
		r.done = true
		return 0, errors.New("source failed")
	}
	return 0, io.EOF
}

func TestWriteExactPropagatesSourceFailure(t *testing.T) {
	var output strings.Builder
	err := writeExact(&output, &terminalErrorReader{data: "abc"}, 3)
	if err == nil || !strings.Contains(err.Error(), "source failed") || output.String() != "abc" {
		t.Fatalf("output=%q error=%v", output.String(), err)
	}
}
