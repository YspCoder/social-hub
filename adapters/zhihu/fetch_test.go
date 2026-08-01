package zhihu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestListPostsUsesSeparateOAuthHeaderAndMapsContents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/v1/user/contents" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		query := request.URL.Query()
		if request.Header.Get("Authorization") != "Bearer access-secret" || request.Header.Get("X-OAuth-Token") != "oauth-user-token" || request.Header.Get("X-Request-Timestamp") != "1785542400" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if query.Get("Offset") != "20" || query.Get("Limit") != "2" || query.Get("ContentType") != "all" || query.Get("SortField") != "ts" || query.Get("SortOrder") != "desc" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte(`{"Code":0,"Message":"success","Data":{"Items":[{"ContentType":"answer","Url":"https://www.zhihu.com/answer/123","CreatedAt":1745486539,"LikeCount":128,"CommentCount":12,"FavoriteCount":20,"Title":"How?","Summary":"A summary"}],"Paging":{"IsEnd":false,"NextOffset":"22","Totals":100}}}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, true, true, false)
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "20", MaxResults: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || !page.HasMore || page.NextCursor == nil || *page.NextCursor != "22" {
		t.Fatalf("page=%#v", page)
	}
	post := page.Items[0]
	if post.ID != "https://www.zhihu.com/answer/123" || post.Text == nil || *post.Text != "A summary" || post.CreatedAt == nil || post.CreatedAt.Unix() != 1745486539 || len(post.Metrics) != 3 {
		t.Fatalf("post=%#v", post)
	}
	var extension map[string]any
	if err := json.Unmarshal(post.Extensions["zhihu.content"], &extension); err != nil || extension["content_type"] != "answer" || extension["favorite_count"] != float64(20) {
		t.Fatalf("extension=%#v err=%v", extension, err)
	}
}

func TestFetcherRejectsUnsupportedSelections(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true, false, false)
	if _, err := client.GetUser(context.Background(), "user-1"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get user error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "post-1"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get post error=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "post-1"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list comments error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "arbitrary-user"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("user selector error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "bad"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("cursor error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 51}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("limit error=%v", err)
	}
	start := time.Now()
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time filter error=%v", err)
	}
}
