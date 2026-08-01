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

	"social-hub/pkg/socialhub"
)

func TestProjectWorkflowContracts(t *testing.T) {
	name, description := "Updated", ""
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Accept") != textMediaType {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v2/user/projects":
			if request.URL.Query().Get("page") != "2" || request.URL.Query().Get("per_page") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("Link", `<`+server.URL+`/v2/user/projects?page=1>; rel="prev", <`+server.URL+`/v2/user/projects?page=3>; rel="next", <https://evil.example/v2/user/projects?page=9>; rel="next"`)
			writeJSON(writer, http.StatusOK, `[{"id":7,"name":"SDK","description":"Typed workflows","shots_count":2}]`)
		case request.Method == http.MethodPost && request.URL.Path == "/v2/projects":
			var body map[string]json.RawMessage
			if request.Header.Get("Content-Type") != "application/json" || json.NewDecoder(request.Body).Decode(&body) != nil || string(body["name"]) != `"SDK"` || string(body["description"]) != `"Typed workflows"` {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusCreated, `{"id":7,"name":"SDK","description":"Typed workflows"}`)
		case request.Method == http.MethodPut && request.URL.Path == "/v2/projects/7":
			var body map[string]json.RawMessage
			if json.NewDecoder(request.Body).Decode(&body) != nil || string(body["name"]) != `"Updated"` || string(body["description"]) != `""` {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"id":7,"name":"Updated","description":""}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/v2/projects/7":
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, client := newTestClient(t, server, []string{"public", "upload"})
	page, err := client.ProjectWorkflow().ListProjects(context.Background(), "2", 150)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != 7 || page.NextCursor == nil || *page.NextCursor != "3" || page.PrevCursor == nil || *page.PrevCursor != "1" || !page.HasMore {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	created, err := client.ProjectWorkflow().CreateProject(context.Background(), CreateProjectRequest{Name: "SDK", Description: "Typed workflows"})
	if err != nil || created.ID != 7 {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	updated, err := client.ProjectWorkflow().UpdateProject(context.Background(), "7", UpdateProjectRequest{Name: &name, Description: &description})
	if err != nil || updated.ID != 7 || updated.Name != "Updated" {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	if err := client.ProjectWorkflow().DeleteProject(context.Background(), "7"); err != nil {
		t.Fatal(err)
	}
}

func TestProjectValidationAndResponseIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPost:
			writeJSON(writer, http.StatusOK, `{"id":0}`)
		case http.MethodPut:
			writeJSON(writer, http.StatusOK, `{"id":8}`)
		default:
			writer.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, []string{"public", "upload"})

	if _, err := client.CreateProject(context.Background(), CreateProjectRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("create validation=%v", err)
	}
	if _, err := client.CreateProject(context.Background(), CreateProjectRequest{Name: "SDK"}); err == nil {
		t.Fatal("expected invalid create response ID")
	}
	if _, err := client.UpdateProject(context.Background(), "bad", UpdateProjectRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("update validation=%v", err)
	}
	name := "Updated"
	if _, err := client.UpdateProject(context.Background(), "7", UpdateProjectRequest{Name: &name}); err == nil {
		t.Fatal("expected mismatched update response ID")
	}
	if err := client.DeleteProject(context.Background(), "0"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("delete validation=%v", err)
	}
	if _, err := client.ListProjects(context.Background(), "bad", 10); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("pagination validation=%v", err)
	}
}

func TestAttachmentWorkflowContracts(t *testing.T) {
	payload := []byte("attachment")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v2/shots/7/attachments":
			if request.ParseMultipartForm(1<<20) != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, header, err := request.FormFile("file")
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(file)
			_ = file.Close()
			if header.Filename != "source.zip" || header.Header.Get("Content-Type") != "application/zip" || !bytes.Equal(body, payload) {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusAccepted)
		case request.Method == http.MethodDelete && request.URL.Path == "/v2/shots/7/attachments/9":
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, []string{"upload"})

	upload, err := client.AttachmentWorkflow().UploadAttachment(context.Background(), AttachmentUploadRequest{
		ShotID: "7", Filename: "source.zip", MIME: "application/zip", Size: int64(len(payload)),
	}, bytes.NewReader(payload))
	if err != nil || upload.ShotID != "7" || upload.State != socialhub.PublishStatePending {
		t.Fatalf("upload=%#v err=%v", upload, err)
	}
	if err := client.AttachmentWorkflow().DeleteAttachment(context.Background(), "7", "9"); err != nil {
		t.Fatal(err)
	}
}

func TestAttachmentSizeValidationAndScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, []string{"upload"})
	base := AttachmentUploadRequest{ShotID: "7", Filename: "source.zip", MIME: "application/zip", Size: 4}
	for _, test := range []struct {
		name string
		body string
	}{{"short", "abc"}, {"long", "abcde"}} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := client.UploadAttachment(context.Background(), base, strings.NewReader(test.body)); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	invalid := []AttachmentUploadRequest{
		{ShotID: "bad", Filename: "source.zip", MIME: "application/zip", Size: 1},
		{ShotID: "7", Filename: "../source.zip", MIME: "application/zip", Size: 1},
		{ShotID: "7", Filename: "source.zip", MIME: "application/zip; charset=utf-8", Size: 1},
		{ShotID: "7", Filename: "source.zip", MIME: "application/zip", Size: maxAttachmentBytes + 1},
	}
	for _, input := range invalid {
		if _, err := client.UploadAttachment(context.Background(), input, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
	if err := client.DeleteAttachment(context.Background(), "7", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("delete validation=%v", err)
	}
	_, readOnly := newTestClient(t, server, []string{"public"})
	if _, err := readOnly.UploadAttachment(context.Background(), AttachmentUploadRequest{ShotID: "7", Filename: "x.zip", MIME: "application/zip", Size: 1}, strings.NewReader("x")); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
}
