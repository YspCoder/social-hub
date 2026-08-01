package soundcloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestTrackUploadWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "OAuth access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case "POST /api/tracks":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, header, err := request.FormFile("track[asset_data]")
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			defer file.Close()
			audio, _ := io.ReadAll(file)
			if request.FormValue("track[title]") != "Uploaded" || request.FormValue("track[artist]") != "Upload Artist" ||
				request.FormValue("track[sharing]") != "private" || request.FormValue("track[commentable]") != "false" ||
				header.Filename != "song.wav" || string(audio) != "audio" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusCreated)
			writeJSON(writer, trackJSON(testTrackURN))
		case "GET /api/tracks/soundcloud:tracks:456":
			writeJSON(writer, trackJSON(testTrackURN))
		case "PUT /api/tracks/soundcloud:tracks:456":
			var body struct {
				Track map[string]any `json:"track"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Track["title"] != "Renamed" || body.Track["metadata_artist"] != "Updated Artist" || body.Track["sharing"] != "public" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, trackJSON(testTrackURN))
		case "DELETE /api/tracks/soundcloud:tracks:456":
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	workflow := client.TrackUploadWorkflow()
	commentable := false
	post, err := workflow.Upload(context.Background(), TrackUploadRequest{
		Title: "Uploaded", Filename: `C:\music\song.wav`, Size: 5, Artist: "Upload Artist",
		Description: "Description", Sharing: "private", Genre: "Electronic", TagList: "demo", License: "cc-by", Commentable: &commentable,
	}, strings.NewReader("audio"))
	if err != nil || post.ID != testTrackURN || post.Status == nil || post.Status.State != socialhub.PublishStatePending || post.Media[0].State != socialhub.MediaStateProcessing {
		t.Fatalf("upload post=%#v err=%v", post, err)
	}
	status, err := workflow.Status(context.Background(), testTrackURN)
	if err != nil || status.ID != testTrackURN || status.Status != nil {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	title, artist, sharing := "Renamed", "Updated Artist", "public"
	updated, err := workflow.Update(context.Background(), testTrackURN, TrackUpdateRequest{Title: &title, Artist: &artist, Sharing: &sharing})
	if err != nil || updated.ID != testTrackURN {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	if err := workflow.Delete(context.Background(), testTrackURN); err != nil {
		t.Fatal(err)
	}
}

func TestTrackUploadValidationAndSizeEnforcement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writer.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	workflow := client.TrackUploadWorkflow()

	invalidUploads := []TrackUploadRequest{
		{},
		{Title: "track", Filename: "track.wav", Size: maxTrackSize + 1},
		{Title: "track", Filename: "track.wav", Size: 1, Sharing: "friends"},
		{Title: "track", Filename: "track.wav", Size: 1, License: "copyright"},
	}
	for index, input := range invalidUploads {
		if _, err := workflow.Upload(context.Background(), input, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid upload %d error=%v", index, err)
		}
	}
	if _, err := workflow.Upload(context.Background(), TrackUploadRequest{Title: "track", Filename: "track.wav", Size: 1}, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil reader error=%v", err)
	}
	if _, err := workflow.Upload(context.Background(), TrackUploadRequest{Title: "track", Filename: "track.wav", Size: 2}, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("short reader error=%v", err)
	}
	if _, err := workflow.Upload(context.Background(), TrackUploadRequest{Title: "track", Filename: "track.wav", Size: 1}, strings.NewReader("xx")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long reader error=%v", err)
	}
	if _, err := workflow.Upload(context.Background(), TrackUploadRequest{Title: "track", Filename: "track.wav", Size: 1}, failingReader{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("reader failure error=%v", err)
	}
	if _, err := workflow.Update(context.Background(), "bad", TrackUpdateRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad update ID error=%v", err)
	}
	if _, err := workflow.Update(context.Background(), testTrackURN, TrackUpdateRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty update error=%v", err)
	}
	empty, badSharing, badLicense := "", "friends", "copyright"
	for index, input := range []TrackUpdateRequest{{Title: &empty}, {Sharing: &badSharing}, {License: &badLicense}} {
		if _, err := workflow.Update(context.Background(), testTrackURN, input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid update %d error=%v", index, err)
		}
	}
	if err := workflow.Delete(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad delete error=%v", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("source failed") }
