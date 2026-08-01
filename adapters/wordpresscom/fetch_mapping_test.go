package wordpresscom

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestFetchAndMapping(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/rest/v1.1/me":
			writeJSON(writer, http.StatusOK, `{"ID":7,"username":"alice","display_name":"Alice","avatar_URL":"https://cdn.example/avatar.png","profile_URL":"https://example.wordpress.com/about"}`)
		case "/rest/v1.1/sites/123":
			writeJSON(writer, http.StatusOK, `{"ID":123,"name":"Example","description":"A site","URL":"https://example.wordpress.com","jetpack":true,"post_count":4}`)
		case "/rest/v1.1/sites/123/posts/10":
			writeJSON(writer, http.StatusOK, postJSON(10, "publish"))
		case "/rest/v1.1/sites/123/posts":
			query := request.URL.Query()
			if query.Get("number") != "100" || query.Get("page_handle") != "opaque-token" || query.Get("author") != "7" || query.Get("after") != start.Format(time.RFC3339) || query.Get("before") != end.Format(time.RFC3339) {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"found":2,"posts":[`+postJSON(11, "draft")+`],"meta":{"next_page":"next-token"}}`)
		case "/rest/v1.1/sites/123/posts/10/replies":
			if request.URL.Query().Get("number") != "100" || request.URL.Query().Get("page") != "2" || request.URL.Query().Get("type") != "comment" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"found":250,"site_ID":123,"comments":[{"ID":21,"post":{"ID":10},"author":{"ID":7},"date":"2026-08-01T12:00:00Z","raw_content":"hello","content":"<p>hello</p>","parent":{"ID":20},"like_count":3}]}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, []string{"users", "posts", "comments"})

	user, err := client.GetUser(context.Background(), "me")
	if err != nil || user.ID != "7" || user.Username == nil || *user.Username != "alice" || user.DisplayName == nil || *user.DisplayName != "Alice" || len(user.Extensions["wordpress.user"]) == 0 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	site, err := client.GetSite(context.Background())
	if err != nil || site.ID != 123 || site.Name != "Example" || !site.Jetpack || len(site.Raw) == 0 {
		t.Fatalf("site=%#v err=%v", site, err)
	}
	post, err := client.GetPost(context.Background(), "10")
	if err != nil || post.ID != "10" || post.AuthorID == nil || *post.AuthorID != "7" || post.Text == nil || *post.Text != "<p>Body</p>" || post.Visibility == nil || *post.Visibility != "public" || post.Status.State != socialhub.PublishStatePublished || len(post.Media) != 2 || post.Media[0].ID != "31" || post.Media[1].ID != "32" || len(post.Metrics) != 2 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "7", Cursor: "opaque-token", MaxResults: 150, StartTime: &start, EndTime: &end})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "next-token" || !page.HasMore || page.Items[0].Status.State != socialhub.PublishStatePending {
		t.Fatalf("posts=%#v err=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "10", Cursor: "2", MaxResults: 150})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "21" || comments.Items[0].Text != "hello" || comments.Items[0].ParentID == nil || *comments.Items[0].ParentID != "20" || comments.NextCursor == nil || *comments.NextCursor != "3" || comments.PrevCursor == nil || *comments.PrevCursor != "1" || !comments.HasMore {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
}

func TestFetchValidationAndBadResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/rest/v1.1/me":
			writeJSON(writer, http.StatusOK, `{"ID":8}`)
		case "/rest/v1.1/sites/123":
			writeJSON(writer, http.StatusOK, `{"ID":0}`)
		case "/rest/v1.1/sites/123/posts/10":
			writeJSON(writer, http.StatusOK, `{"ID":9,"site_ID":123}`)
		case "/rest/v1.1/sites/123/posts":
			writeJSON(writer, http.StatusOK, `{"posts":[],"meta":{"next_page":"bad\nvalue"}}`)
		case "/rest/v1.1/sites/123/posts/10/replies":
			writeJSON(writer, http.StatusOK, `{"site_ID":999,"comments":[]}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, []string{"global"})
	invalid := []func() error{
		func() error { _, err := client.GetUser(context.Background(), "8"); return err },
		func() error { _, err := client.GetPost(context.Background(), "bad"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "bad"})
			return err
		},
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: -1})
			return err
		},
		func() error {
			end := time.Now()
			start := end.Add(time.Hour)
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start, EndTime: &end})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "x"})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "10", Cursor: "zero"})
			return err
		},
	}
	for index, call := range invalid {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
	bad := []func() error{
		func() error { _, err := client.GetUser(context.Background(), "me"); return err },
		func() error { _, err := client.GetSite(context.Background()); return err },
		func() error { _, err := client.GetPost(context.Background(), "10"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "10"})
			return err
		},
	}
	for index, call := range bad {
		err := call()
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("bad response %d error=%v", index, err)
		}
	}
}

func TestMappingHelpers(t *testing.T) {
	if referenceID([]byte(`false`)) != "" || referenceID([]byte(`9`)) != "9" || referenceID([]byte(`{"ID":8}`)) != "8" || referenceID([]byte(`"bad"`)) != "" {
		t.Fatal("reference mapping failed")
	}
	for _, test := range []struct {
		mime      string
		extension string
		expected  socialhub.MediaType
	}{
		{"image/gif", "gif", socialhub.MediaTypeAnimation},
		{"image/png", "png", socialhub.MediaTypeImage},
		{"video/mp4", "mp4", socialhub.MediaTypeVideo},
		{"audio/mpeg", "mp3", socialhub.MediaTypeAudio},
		{"", "gif", socialhub.MediaTypeAnimation},
		{"application/pdf", "pdf", socialhub.MediaTypeDocument},
	} {
		if actual := mediaType(test.mime, test.extension); actual != test.expected {
			t.Fatalf("media type=%s expected=%s", actual, test.expected)
		}
	}
	now := time.Now()
	if mapPublishStatus("1", "trash", nil, nil, now).State != socialhub.PublishStateFailed || mapPublishStatus("1", "future", nil, nil, now).State != socialhub.PublishStatePending {
		t.Fatal("publish state mapping failed")
	}
	media := mapMedia(wpMedia{ID: 1, MIME: "video/mp4", Size: 20, Width: 640, Height: 360, Length: 5})
	if media.Size == nil || *media.Size != 20 || media.Width == nil || media.Height == nil || media.Duration == nil || *media.Duration != 5*time.Second {
		t.Fatalf("media=%#v", media)
	}
}

func postJSON(id int, status string) string {
	return `{"ID":` + strconv.Itoa(id) + `,"site_ID":123,"author":{"ID":7},"date":"2026-08-01T10:00:00Z","modified":"2026-08-01T11:00:00Z","title":"Title","content":"<p>Body</p>","excerpt":"Excerpt","status":"` + status + `","URL":"https://example.wordpress.com/post","comment_count":4,"like_count":5,"attachments":{"32":{"ID":32,"URL":"https://cdn.example/video.mp4","mime_type":"video/mp4","width":640,"height":360,"length":4},"31":{"ID":31,"URL":"https://cdn.example/image.png","mime_type":"image/png","width":100,"height":80}}}`
}
