package matrix

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

func TestMediaUploadContractAndExactSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/_matrix/media/v3/upload" || request.URL.Query().Get("filename") != "photo name.png" || request.Header.Get("Authorization") != "Bearer matrix-token" || request.Header.Get("Content-Type") != "image/png" || request.ContentLength != 3 {
			writeMatrixJSON(writer, http.StatusBadRequest, `{"errcode":"M_INVALID_PARAM"}`)
			return
		}
		body, err := io.ReadAll(request.Body)
		if err != nil || !bytes.Equal(body, []byte("PNG")) {
			writeMatrixJSON(writer, http.StatusBadRequest, `{"errcode":"M_BAD_JSON"}`)
			return
		}
		writeMatrixJSON(writer, http.StatusOK, `{"content_uri":"mxc://example.test/media-id"}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true)

	media, err := client.Upload(context.Background(), UploadRequest{Filename: "photo name.png", MIME: " IMAGE/PNG ", Size: 3}, strings.NewReader("PNG"))
	if err != nil || media.ID != "mxc://example.test/media-id" || media.URL != media.ID || media.Type != socialhub.MediaTypeImage || media.MIME != "image/png" || media.Size == nil || *media.Size != 3 || media.State != socialhub.MediaStateReady {
		t.Fatalf("media=%#v err=%v", media, err)
	}

	if _, err := client.Upload(context.Background(), UploadRequest{Filename: "photo name.png", MIME: "image/png", Size: 4}, strings.NewReader("PNG")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("short reader error=%v", err)
	}
	if _, err := client.Upload(context.Background(), UploadRequest{Filename: "photo name.png", MIME: "image/png", Size: 3}, strings.NewReader("PNGX")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long reader error=%v", err)
	}
}

func TestMediaUploadValidationAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writeMatrixJSON(writer, http.StatusOK, `{"content_uri":"https://example.test/not-mxc"}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true)
	tests := []struct {
		name    string
		input   UploadRequest
		content io.Reader
	}{
		{"reader", UploadRequest{Filename: "a.txt", MIME: "text/plain", Size: 1}, nil},
		{"filename", UploadRequest{Filename: "../a.txt", MIME: "text/plain", Size: 1}, strings.NewReader("x")},
		{"mime", UploadRequest{Filename: "a.txt", MIME: "text/*", Size: 1}, strings.NewReader("x")},
		{"size", UploadRequest{Filename: "a.txt", MIME: "text/plain"}, strings.NewReader("x")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := client.Upload(context.Background(), test.input, test.content); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	if _, err := client.Upload(context.Background(), UploadRequest{Filename: "a.txt", MIME: "text/plain", Size: 1}, strings.NewReader("x")); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid response error=%v", err)
	}
	for input, expected := range map[string]socialhub.MediaType{
		"image/png": socialhub.MediaTypeImage, "video/mp4": socialhub.MediaTypeVideo,
		"audio/ogg": socialhub.MediaTypeAudio, "application/pdf": socialhub.MediaTypeDocument,
	} {
		if actual := mediaTypeForMIME(input); actual != expected {
			t.Fatalf("media type %q=%q", input, actual)
		}
	}
}

func TestSyncContractAndRoomContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if request.Method != http.MethodGet || request.URL.Path != "/_matrix/client/v3/sync" || query.Get("since") != "s/1" || query.Get("timeout") != "1500" || query.Get("full_state") != "true" || request.Header.Get("Authorization") != "Bearer matrix-token" {
			writeMatrixJSON(writer, http.StatusBadRequest, `{"errcode":"M_INVALID_PARAM"}`)
			return
		}
		writeMatrixJSON(writer, http.StatusOK, `{"next_batch":"s/2","rooms":{"join":{"!room/alpha:example.test":{"timeline":{"events":[{"type":"m.room.message","event_id":"$one:example.test","sender":"@alice:example.test","origin_server_ts":1785657600000,"content":{"msgtype":"m.text","body":"one"}}],"limited":true,"prev_batch":"before"}}}}}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true)

	response, err := client.Sync(context.Background(), SyncRequest{Since: "s/1", Timeout: 1500 * time.Millisecond, FullState: true})
	room := response.Rooms.Join["!room/alpha:example.test"]
	if err != nil || response.NextBatch != "s/2" || len(room.Timeline.Events) != 1 || room.Timeline.Events[0].RoomID != "!room/alpha:example.test" || !room.Timeline.Limited || room.Timeline.PrevBatch != "before" {
		t.Fatalf("response=%#v room=%#v err=%v", response, room, err)
	}
	if _, err := client.Sync(context.Background(), SyncRequest{Timeout: -time.Second}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("negative timeout error=%v", err)
	}
	if _, err := client.Sync(context.Background(), SyncRequest{Since: "bad\nvalue"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid since error=%v", err)
	}
}

func TestSyncRejectsMalformedServerData(t *testing.T) {
	response := `{"rooms":{"join":{}}}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeMatrixJSON(writer, http.StatusOK, response)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true)

	if _, err := client.Sync(context.Background(), SyncRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("missing cursor error=%v", err)
	}
	response = `{"next_batch":"next","rooms":{"join":{"bad-room":{"timeline":{"events":[]}}}}}`
	if _, err := client.Sync(context.Background(), SyncRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad room error=%v", err)
	}
	response = `{"next_batch":"next","rooms":{"join":{"!room:example.test":{"timeline":{"events":[{"type":"m.room.message","room_id":"!other:example.test","event_id":"$one:example.test","content":{"msgtype":"m.text","body":"one"}}]}}}}}`
	if _, err := client.Sync(context.Background(), SyncRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("mismatched room error=%v", err)
	}
}
