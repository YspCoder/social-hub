package tiktok

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestDisplayAPIContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/v2/user/info/":
			if request.Method != http.MethodGet || request.URL.Query().Get("fields") == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"data":{"user":{"open_id":"open-id","display_name":"Creator","username":"creator","avatar_url":"https://cdn.example/avatar.jpg","profile_deep_link":"https://www.tiktok.com/@creator","follower_count":10}},"error":{"code":"ok","log_id":"log-1"}}`)
		case "/v2/video/query/":
			var body struct {
				Filters struct {
					VideoIDs []string `json:"video_ids"`
				} `json:"filters"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || len(body.Filters.VideoIDs) != 1 || body.Filters.VideoIDs[0] != "video-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"data":{"videos":[{"id":"video-1","create_time":1785542400,"share_url":"https://www.tiktok.com/video/1","video_description":"hello","duration":12,"width":1080,"height":1920,"like_count":2,"comment_count":1,"share_count":3,"view_count":10}]},"error":{"code":"ok"}}`)
		case "/v2/video/list/":
			var body struct {
				Cursor   int64 `json:"cursor"`
				MaxCount int   `json:"max_count"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Cursor != 1785542400000 || body.MaxCount != 20 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"data":{"videos":[{"id":"video-2","create_time":1785542300,"title":"next"}],"cursor":1785542300000,"has_more":true},"error":{"code":"ok"}}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"user.info.basic", "user.info.profile", "user.info.stats", "video.list"}, false)
	user, err := client.GetUser(context.Background(), "open-id")
	if err != nil || user.ID != "open-id" || user.Username == nil || *user.Username != "creator" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "video-1")
	if err != nil || post.ID != "video-1" || len(post.Media) != 1 || len(post.Metrics) != 4 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "1785542400000", MaxResults: 100})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "1785542300000" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "video-1"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comments error=%v", err)
	}
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(value))
}
