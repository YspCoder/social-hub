package officialaccount

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/extensions/material"
	"social-hub/pkg/socialhub"
)

func TestTemporaryAndPermanentMaterialWorkflows(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/cgi-bin/token" {
			_, _ = writer.Write([]byte(`{"access_token":"token","expires_in":7200}`))
			return
		}
		switch request.URL.Path {
		case "/cgi-bin/media/upload", "/cgi-bin/material/add_material":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, _, err := request.FormFile("media")
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(file)
			_ = file.Close()
			if string(data) != "image" || request.URL.Query().Get("type") != "image" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if request.URL.Path == "/cgi-bin/media/upload" {
				_, _ = writer.Write([]byte(`{"type":"image","media_id":"temporary-1","created_at":1785542400}`))
			} else {
				_, _ = writer.Write([]byte(`{"media_id":"permanent-1","url":"https://mmbiz.example/material"}`))
			}
		case "/cgi-bin/material/batchget_material":
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			_, _ = writer.Write([]byte(`{"total_count":2,"item_count":1,"item":[{"media_id":"permanent-1","name":"image.jpg","update_time":1785542400,"url":"https://mmbiz.example/material"}]}`))
		case "/cgi-bin/material/del_material":
			_, _ = writer.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "image.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 5})
	if err != nil {
		t.Fatal(err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, bytes.NewBufferString("image"))
	if err != nil {
		t.Fatal(err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "temporary-1" || media.ExpiresAt == nil {
		t.Fatalf("media = %#v, err = %v", media, err)
	}

	manager := client.MaterialManager()
	asset, err := manager.Upload(context.Background(), material.Permanent, socialhub.MediaTypeImage, bytes.NewBufferString("image"), material.Metadata{Filename: "image.jpg", MIME: "image/jpeg"})
	if err != nil || asset.ID != "permanent-1" {
		t.Fatalf("asset = %#v, err = %v", asset, err)
	}
	page, err := manager.List(context.Background(), material.ListRequest{Kind: material.Permanent, Type: socialhub.MediaTypeImage, Limit: 1})
	if err != nil || len(page.Items) != 1 || !page.HasMore {
		t.Fatalf("material page = %#v, err = %v", page, err)
	}
	if _, err := manager.Get(context.Background(), asset.ID); err != nil {
		t.Fatal(err)
	}
	if err := manager.Delete(context.Background(), asset.ID); err != nil {
		t.Fatal(err)
	}
}
