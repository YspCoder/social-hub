package x

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestChunkedMediaUploadWorkflow(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var commands []string
	var uploaded string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			mu.Lock()
			commands = append(commands, request.URL.Query().Get("command"))
			mu.Unlock()
			_, _ = writer.Write([]byte(`{"data":{"id":"99","size":5,"processing_info":{"state":"succeeded"}}}`))
			return
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		command := request.FormValue("command")
		mu.Lock()
		commands = append(commands, command)
		mu.Unlock()
		switch command {
		case "INIT":
			if request.FormValue("total_bytes") != "5" || request.FormValue("media_category") != "tweet_video" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"id":"99","expires_after_secs":3600}}`))
		case "APPEND":
			file, _, err := request.FormFile("media")
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(file)
			_ = file.Close()
			mu.Lock()
			uploaded = string(data)
			mu.Unlock()
			_, _ = writer.Write([]byte(`{"data":{"id":"99"}}`))
		case "FINALIZE":
			_, _ = writer.Write([]byte(`{"data":{"id":"99","size":5,"processing_info":{"state":"pending","check_after_secs":1}}}`))
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "clip.mp4", Type: socialhub.MediaTypeVideo, MIME: "video/mp4", Size: 5})
	if err != nil {
		t.Fatal(err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, bytes.NewBufferString("video"))
	if err != nil {
		t.Fatal(err)
	}
	if part.Size != 5 {
		t.Fatalf("uploaded part size = %d", part.Size)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil {
		t.Fatal(err)
	}
	if media.State != socialhub.MediaStateProcessing {
		t.Fatalf("finalized media state = %q", media.State)
	}
	media, err = client.MediaStatus(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if media.State != socialhub.MediaStateReady {
		t.Fatalf("status media state = %q", media.State)
	}

	mu.Lock()
	gotCommands, gotUpload := append([]string(nil), commands...), uploaded
	mu.Unlock()
	if !reflect.DeepEqual(gotCommands, []string{"INIT", "APPEND", "FINALIZE", "STATUS"}) {
		t.Fatalf("commands = %v", gotCommands)
	}
	if gotUpload != "video" {
		t.Fatalf("uploaded body = %q", gotUpload)
	}
}
