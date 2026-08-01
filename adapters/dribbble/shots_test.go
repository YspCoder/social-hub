package dribbble

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestShotWorkflowContracts(t *testing.T) {
	payload := []byte("image-bytes")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v2/shots":
			if request.Header.Get("X-Request-ID") != "upload-request" || request.ParseMultipartForm(1<<20) != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, header, err := request.FormFile("image")
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(file)
			_ = file.Close()
			if header.Filename != "shot.png" || header.Header.Get("Content-Type") != "image/png" || !bytes.Equal(body, payload) ||
				request.FormValue("title") != "Typed Shot" || request.FormValue("description") != "Description" || request.FormValue("low_profile") != "true" ||
				request.FormValue("rebound_source_id") != "9" || request.FormValue("team_id") != "2" || request.FormValue("scheduled_for") != "2026-08-03T00:00:00Z" ||
				len(request.MultipartForm.Value["tags[]"]) != 2 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("Location", server.URL+"/v2/shots/10")
			writer.WriteHeader(http.StatusAccepted)
		case request.Method == http.MethodPut && request.URL.Path == "/v2/shots/10":
			var body struct {
				Title       string   `json:"title"`
				Description string   `json:"description"`
				Tags        []string `json:"tags"`
				TeamID      string   `json:"team_id"`
			}
			if request.Header.Get("Content-Type") != "application/json" || json.NewDecoder(request.Body).Decode(&body) != nil || body.Title != "Updated" || body.Description != "" || len(body.Tags) != 0 || body.TeamID != "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, shotJSON(10, "Updated description"))
		case request.Method == http.MethodDelete && request.URL.Path == "/v2/shots/10":
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, []string{"public", "upload"})
	scheduled := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	pending, err := client.ShotWorkflow().CreateShot(context.Background(), CreateShotRequest{
		Filename: "shot.png", MIME: "image/png", Size: int64(len(payload)), Title: "Typed Shot", Description: "Description",
		LowProfile: true, ReboundSourceID: "9", ScheduledFor: &scheduled, Tags: []string{"go", "api", "go"}, TeamID: "2",
	}, bytes.NewReader(payload), socialhub.WithRequestID("upload-request"))
	if err != nil || pending.ID != "10" || pending.Location != server.URL+"/v2/shots/10" || pending.State != socialhub.PublishStatePending {
		t.Fatalf("pending=%#v err=%v", pending, err)
	}
	title, description, team := "Updated", "", ""
	post, err := client.ShotWorkflow().UpdateShot(context.Background(), "10", UpdateShotRequest{Title: &title, Description: &description, Tags: []string{}, TeamID: &team})
	if err != nil || post.ID != "10" || post.Text == nil || *post.Text != "Updated description" {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	if err := client.ShotWorkflow().DeleteShot(context.Background(), "10"); err != nil {
		t.Fatal(err)
	}
}

func TestShotUploadSizeLocationAndValidation(t *testing.T) {
	t.Run("exact size", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = io.Copy(io.Discard, request.Body)
			writer.Header().Set("Location", serverURL(request)+"/v2/shots/1")
			writer.WriteHeader(http.StatusAccepted)
		}))
		defer server.Close()
		_, client := newTestClient(t, server, []string{"upload"})
		for _, test := range []struct {
			name string
			size int64
			body string
		}{{"short", 4, "abc"}, {"long", 3, "abcd"}} {
			t.Run(test.name, func(t *testing.T) {
				_, err := client.CreateShot(context.Background(), CreateShotRequest{Filename: "x.png", MIME: "image/png", Size: test.size, Title: "Shot"}, strings.NewReader(test.body))
				if !errors.Is(err, socialhub.ErrInvalidArgument) {
					t.Fatalf("error=%v", err)
				}
			})
		}
	})
	t.Run("location origin", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = io.Copy(io.Discard, request.Body)
			writer.Header().Set("Location", "https://evil.example/v2/shots/1")
			writer.WriteHeader(http.StatusAccepted)
		}))
		defer server.Close()
		_, client := newTestClient(t, server, []string{"upload"})
		if _, err := client.CreateShot(context.Background(), CreateShotRequest{Filename: "x.png", MIME: "image/png", Size: 1, Title: "Shot"}, strings.NewReader("x")); err == nil {
			t.Fatal("expected invalid Location error")
		}
	})

	client := &Client{}
	invalidCreates := []CreateShotRequest{
		{Filename: "../x.png", MIME: "image/png", Size: 1, Title: "Shot"},
		{Filename: "x.png", MIME: "text/plain", Size: 1, Title: "Shot"},
		{Filename: "x.png", MIME: "image/png", Size: maxShotBytes + 1, Title: "Shot"},
		{Filename: "x.png", MIME: "image/png", Size: 1},
		{Filename: "x.png", MIME: "image/png", Size: 1, Title: "Shot", TeamID: "bad"},
		{Filename: "x.png", MIME: "image/png", Size: 1, Title: "Shot", Tags: make([]string, 13)},
	}
	for _, input := range invalidCreates {
		if _, err := client.CreateShot(context.Background(), input, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
	if _, err := client.UpdateShot(context.Background(), "bad", UpdateShotRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("update ID error=%v", err)
	}
	if _, err := client.UpdateShot(context.Background(), "1", UpdateShotRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty update error=%v", err)
	}
	if err := client.DeleteShot(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("delete error=%v", err)
	}
}

func serverURL(request *http.Request) string { return "http://" + request.Host }
