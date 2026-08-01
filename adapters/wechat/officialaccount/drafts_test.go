package officialaccount

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDraftPublicationWorkflow(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/cgi-bin/token" {
			_, _ = writer.Write([]byte(`{"access_token":"token","expires_in":7200}`))
			return
		}
		switch request.URL.Path {
		case "/cgi-bin/draft/add":
			var body struct {
				Articles []Article `json:"articles"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if len(body.Articles) != 1 || body.Articles[0].ThumbMediaID != "thumb-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"media_id":"draft-1"}`))
		case "/cgi-bin/freepublish/submit":
			_, _ = writer.Write([]byte(`{"publish_id":"publish-1"}`))
		case "/cgi-bin/freepublish/get":
			_, _ = writer.Write([]byte(`{"publish_id":"publish-1","publish_status":0,"article_id":"article-1"}`))
		case "/cgi-bin/draft/delete":
			_, _ = writer.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	service := client.Drafts()
	mediaID, err := service.Add(context.Background(), []Article{{Title: "Title", Content: "<p>Body</p>", ThumbMediaID: "thumb-1"}})
	if err != nil || mediaID != "draft-1" {
		t.Fatalf("media ID = %q, err = %v", mediaID, err)
	}
	job, err := service.Publish(context.Background(), mediaID)
	if err != nil || job.PublishID != "publish-1" {
		t.Fatalf("publish job = %#v, err = %v", job, err)
	}
	job, err = service.Status(context.Background(), job.PublishID)
	if err != nil || job.ArticleID != "article-1" {
		t.Fatalf("publish status = %#v, err = %v", job, err)
	}
	if err := service.Delete(context.Background(), mediaID); err != nil {
		t.Fatal(err)
	}
}
