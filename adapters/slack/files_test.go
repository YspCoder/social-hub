package slack

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestExternalFileUploadContract(t *testing.T) {
	uploadServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "" || request.Header.Get("X-Request-ID") != "upload-request" {
			http.Error(writer, "bad upload headers", http.StatusBadRequest)
			t.Errorf("upload headers=%v", request.Header)
			return
		}
		file, header, err := request.FormFile("file")
		if err != nil {
			http.Error(writer, "missing file", http.StatusBadRequest)
			t.Errorf("form file: %v", err)
			return
		}
		defer file.Close()
		body, _ := io.ReadAll(file)
		if string(body) != "hello" || header.Filename != "report 2026.txt" || header.Header.Get("Content-Type") != "text/plain" {
			http.Error(writer, "bad file", http.StatusBadRequest)
			t.Errorf("upload filename=%q type=%q body=%q", header.Filename, header.Header.Get("Content-Type"), body)
			return
		}
		_, _ = io.WriteString(writer, "OK")
	}))
	defer uploadServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/api/")
		body := requireSlackRequest(t, writer, request, method)
		if body == nil {
			return
		}
		switch method {
		case "files.getUploadURLExternal":
			if body["filename"] != "report 2026.txt" || body["length"] != float64(5) || body["alt_txt"] != "quarterly report" || body["snippet_type"] != "text" {
				t.Errorf("get upload URL body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{"ok": true, "upload_url": uploadServer.URL + "/upload", "file_id": testFileID})
		case "files.completeUploadExternal":
			files, _ := body["files"].([]any)
			file, _ := files[0].(map[string]any)
			if len(files) != 1 || file["id"] != testFileID || file["title"] != "Q2 report" || body["channel_id"] != testChannelID || body["thread_ts"] != testTimestamp || body["initial_comment"] != "attached" {
				t.Errorf("complete upload body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{"ok": true, "files": []any{map[string]any{"id": testFileID, "title": "Q2 report"}}})
		case "files.info":
			if body["file"] != "F999ABC" {
				t.Errorf("files.info body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{"ok": true, "file": map[string]any{
				"id": "F999ABC", "name": "clip.mp4", "title": "Clip", "mimetype": "video/mp4", "size": 99,
				"url_private": "https://files.test/clip.mp4", "original_w": 1920, "original_h": 1080, "duration_ms": 2500,
			}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer apiServer.Close()
	_, client := newTestAdapter(t, apiServer, testChannelID, false, allTestScopes())

	session, err := client.BeginFileUpload(context.Background(), FileUploadRequest{
		Filename: "report 2026.txt", Size: 5, MIME: "text/plain", Title: "Q2 report",
		AltText: "quarterly report", SnippetType: "text", ChannelID: testChannelID,
		ThreadPostID: testChannelID + ":" + testTimestamp, InitialComment: "attached",
	})
	if err != nil || !strings.HasPrefix(session.ID, "slack-file:") || session.MediaID != testFileID || session.PartSize != 5 {
		t.Fatalf("session=%#v error=%v", session, err)
	}
	part, err := client.UploadFilePart(context.Background(), session.ID, 0, strings.NewReader("hello"), socialhub.WithRequestID("upload-request"))
	if err != nil || part.Number != 0 || part.ETag != testFileID || part.Size != 5 {
		t.Fatalf("part=%#v error=%v", part, err)
	}
	media, err := client.CompleteFileUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != testFileID || media.MIME != "text/plain" || media.Type != socialhub.MediaTypeDocument || media.Size == nil || *media.Size != 5 || media.State != socialhub.MediaStateReady {
		t.Fatalf("media=%#v error=%v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), testFileID)
	if err != nil || status.ID != testFileID || status == media {
		t.Fatalf("local status=%#v error=%v", status, err)
	}
	remote, err := client.GetFile(context.Background(), "F999ABC")
	if err != nil || remote.Type != socialhub.MediaTypeVideo || remote.URL != "https://files.test/clip.mp4" || remote.Width == nil || *remote.Width != 1920 || remote.Duration == nil || *remote.Duration != 2500*time.Millisecond || len(remote.Extensions) != 1 {
		t.Fatalf("remote file=%#v error=%v", remote, err)
	}
	if _, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part}); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("completed session reuse=%v", err)
	}
}

func TestFileUploadValidationAndState(t *testing.T) {
	uploadServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		_, _ = io.WriteString(writer, "OK")
	}))
	defer uploadServer.Close()
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/api/")
		if requireSlackRequest(t, writer, request, method) == nil {
			return
		}
		switch method {
		case "files.getUploadURLExternal":
			writeTestJSON(t, writer, map[string]any{"ok": true, "upload_url": uploadServer.URL, "file_id": testFileID})
		case "files.completeUploadExternal":
			writeTestJSON(t, writer, map[string]any{"ok": true, "files": []any{map[string]any{"id": testFileID}}})
		}
	}))
	defer apiServer.Close()
	_, client := newTestAdapter(t, apiServer, testChannelID, false, allTestScopes())
	invalid := []FileUploadRequest{
		{},
		{Filename: "file.txt", Size: 0},
		{Filename: "../file.txt", Size: 1},
		{Filename: "file.txt", Size: 1, MIME: "text/plain\n"},
		{Filename: "file.txt", Size: 1, Title: strings.Repeat("x", 256)},
		{Filename: "file.txt", Size: 1, AltText: strings.Repeat("x", 2001)},
		{Filename: "file.txt", Size: 1, SnippetType: "bad type"},
		{Filename: "file.txt", Size: 1, InitialComment: "bad\x00text"},
		{Filename: "file.txt", Size: 1, ChannelID: "bad"},
		{Filename: "file.txt", Size: 1, ChannelID: testChannelID, ThreadPostID: testPrivateID + ":" + testTimestamp},
	}
	for index, input := range invalid {
		if _, err := client.BeginFileUpload(context.Background(), input); err == nil {
			t.Fatalf("invalid begin %d accepted", index)
		}
	}
	if _, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "file.txt", Size: 1, Category: "channel"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("common category=%v", err)
	}
	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "file.txt", Size: 4, MIME: "text/plain"})
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
		if _, err := client.UploadPart(context.Background(), input.id, input.number, input.reader); err == nil {
			t.Fatalf("invalid part %d accepted", index)
		}
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("too long")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("byte mismatch=%v", err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("data")); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("duplicate upload=%v", err)
	}
	for index, parts := range [][]socialhub.UploadedPart{
		nil,
		{{Number: 1, ETag: part.ETag, Size: part.Size}},
		{{Number: 0, ETag: "wrong", Size: part.Size}},
		{{Number: 0, ETag: part.ETag, Size: part.Size + 1}},
	} {
		if _, err := client.CompleteUpload(context.Background(), session.ID, parts); err == nil {
			t.Fatalf("invalid complete %d accepted", index)
		}
	}
	if _, err := client.CompleteUpload(context.Background(), "missing", []socialhub.UploadedPart{*part}); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing complete=%v", err)
	}
	if _, err := client.GetFile(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid file ID=%v", err)
	}
	_, restricted := newTestAdapter(t, apiServer, testChannelID, false, []string{"chat:write"})
	if _, err := restricted.BeginFileUpload(context.Background(), FileUploadRequest{Filename: "x", Size: 1}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("missing files scope=%v", err)
	}
}

func TestUploadURLPolicyTimeoutAndHTTPError(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusOK)
	uploadServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		currentStatus := int(status.Load())
		if currentStatus != http.StatusOK {
			writer.Header().Set("Retry-After", "3")
			writer.WriteHeader(currentStatus)
			return
		}
		select {
		case <-request.Context().Done():
			return
		case <-time.After(200 * time.Millisecond):
			_, _ = io.WriteString(writer, "OK")
		}
	}))
	defer uploadServer.Close()
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/api/")
		if requireSlackRequest(t, writer, request, method) == nil {
			return
		}
		writeTestJSON(t, writer, map[string]any{"ok": true, "upload_url": uploadServer.URL, "file_id": testFileID})
	}))
	defer apiServer.Close()
	_, client := newTestAdapter(t, apiServer, testChannelID, false, allTestScopes())
	input := FileUploadRequest{Filename: "x.bin", Size: 4}
	client.allowHTTPUploads = false
	if _, err := client.BeginFileUpload(context.Background(), input); err == nil {
		t.Fatal("HTTPS API origin accepted HTTP upload URL")
	}
	client.allowHTTPUploads = true
	session, err := client.BeginFileUpload(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err = client.UploadFilePart(context.Background(), session.ID, 0, bytes.NewReader([]byte("data")), socialhub.WithCallTimeout(30*time.Millisecond))
	platformErr := requireErrorCode(t, err, socialhub.CodeTemporarilyUnavailable)
	if platformErr.Class != socialhub.ClassRetryable || time.Since(started) > time.Second {
		t.Fatalf("timeout error=%#v elapsed=%s", platformErr, time.Since(started))
	}
	status.Store(http.StatusTooManyRequests)
	_, err = client.UploadFilePart(context.Background(), session.ID, 0, bytes.NewReader([]byte("data")))
	platformErr = requireErrorCode(t, err, socialhub.CodeRateLimited)
	if platformErr.RetryAfter != 3*time.Second || platformErr.HTTPStatus != http.StatusTooManyRequests {
		t.Fatalf("upload rate error=%#v", platformErr)
	}
	if validUploadURL("http://upload.test", false) || !validUploadURL("http://upload.test", true) || !validUploadURL("https://upload.test", false) || validUploadURL("https://user@upload.test", false) {
		t.Fatal("upload URL validation mismatch")
	}
}

func TestMalformedFileResponses(t *testing.T) {
	mode := "begin"
	uploadServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		_, _ = io.WriteString(writer, "OK")
	}))
	defer uploadServer.Close()
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/api/")
		if requireSlackRequest(t, writer, request, method) == nil {
			return
		}
		if method == "files.getUploadURLExternal" {
			fileID := testFileID
			if mode == "begin" {
				fileID = ""
			}
			writeTestJSON(t, writer, map[string]any{"ok": true, "upload_url": uploadServer.URL, "file_id": fileID})
			return
		}
		if method == "files.completeUploadExternal" {
			writeTestJSON(t, writer, map[string]any{"ok": true, "files": []any{}})
			return
		}
		writeTestJSON(t, writer, map[string]any{"ok": true, "file": map[string]any{"id": "FOTHER1"}})
	}))
	defer apiServer.Close()
	_, client := newTestAdapter(t, apiServer, testChannelID, false, allTestScopes())
	input := FileUploadRequest{Filename: "x", Size: 1}
	_, err := client.BeginFileUpload(context.Background(), input)
	requireErrorCode(t, err, socialhub.CodePlatformError)
	mode = "complete"
	session, err := client.BeginFileUpload(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	part, err := client.UploadFilePart(context.Background(), session.ID, 0, strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CompleteFileUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	requireErrorCode(t, err, socialhub.CodePlatformError)
	_, err = client.GetFile(context.Background(), "F999ABC")
	requireErrorCode(t, err, socialhub.CodeNotFound)
}
