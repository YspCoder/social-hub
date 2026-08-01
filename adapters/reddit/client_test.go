package reddit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestRedditFetchAndInteractionContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("User-Agent") != testUserAgent {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("x-ratelimit-used", "2.5")
		writer.Header().Set("x-ratelimit-remaining", "97.5")
		writer.Header().Set("x-ratelimit-reset", "60")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/me":
			writeJSON(writer, `{"id":"user1","name":"testuser","icon_img":"https://cdn.example/avatar.png","total_karma":12,"link_karma":7,"comment_karma":5}`)
		case request.Method == http.MethodGet && request.URL.Path == "/api/info":
			if request.URL.Query().Get("id") != "t3_abc" || request.URL.Query().Get("raw_json") != "1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, listingJSON("", videoThingJSON("abc")))
		case request.Method == http.MethodGet && request.URL.Path == "/user/testuser/submitted":
			if request.URL.Query().Get("after") != "t3_cursor" || request.URL.Query().Get("limit") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, listingJSON("t3_next", imageThingJSON("def")))
		case request.Method == http.MethodGet && request.URL.Path == "/comments/abc":
			writeJSON(writer, `[`+listingJSON("", videoThingJSON("abc"))+`,`+listingJSON("", commentThingJSON())+`]`)
		case request.Method == http.MethodPost && request.URL.Path == "/api/vote":
			if request.ParseForm() != nil || request.Form.Get("id") != "t3_abc" || (request.Form.Get("dir") != "1" && request.Form.Get("dir") != "0") {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{}`)
		case request.Method == http.MethodPost && request.URL.Path == "/api/comment":
			if request.ParseForm() != nil || request.Form.Get("thing_id") != "t3_abc" || request.Form.Get("text") != "new comment" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"json":{"errors":[],"data":{"things":[{"kind":"t1","data":{"id":"new1","name":"t1_new1","author":"testuser","author_fullname":"t2_user1","link_id":"t3_abc","parent_id":"t3_abc","body":"new comment","created_utc":1785542400}}]}}}`)
		case request.Method == http.MethodPost && request.URL.Path == "/api/del":
			if request.ParseForm() != nil || !validThingFullname(request.Form.Get("id")) {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"identity", "read", "history", "submit", "edit", "vote"})

	user, err := client.GetUser(context.Background(), "t2_user1")
	if err != nil || user.ID != "t2_user1" || user.Username == nil || *user.Username != "testuser" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "abc")
	if err != nil || post.ID != "t3_abc" || len(post.Media) != 1 || post.Media[0].Duration == nil || *post.Media[0].Duration != 12*time.Second {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "t3_cursor", MaxResults: 200})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "t3_next" || page.Items[0].Media[0].Type != socialhub.MediaTypeImage {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "t3_abc", MaxResults: 100})
	if err != nil || len(comments.Items) != 2 || comments.Items[1].ParentID == nil || *comments.Items[1].ParentID != "t1_parent" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	reaction := socialhub.ReactionRequest{ActorID: "t2_user1", TargetID: "t3_abc", Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "t3_abc", Text: "new comment"})
	if err != nil || comment.ID != "t1_new1" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatal(err)
	}
	rate := client.RateLimit()
	if rate.Used != 2.5 || rate.Remaining != 97.5 || rate.Reset != time.Minute || rate.ObservedAt.IsZero() {
		t.Fatalf("rate=%#v", rate)
	}
}

func listingJSON(after, child string) string {
	afterJSON := "null"
	if after != "" {
		afterJSON = `"` + after + `"`
	}
	return `{"kind":"Listing","data":{"after":` + afterJSON + `,"before":null,"children":[` + child + `]}}`
}

func videoThingJSON(id string) string {
	return `{"kind":"t3","data":{"id":"` + id + `","name":"t3_` + id + `","author":"testuser","author_fullname":"t2_user1","title":"Video","selftext":"body","url":"https://v.redd.it/video","permalink":"/r/golang/comments/` + id + `/video/","subreddit":"golang","created_utc":1785542400,"score":5,"ups":6,"upvote_ratio":0.9,"num_comments":2,"media":{"reddit_video":{"fallback_url":"https://v.redd.it/video.mp4","width":720,"height":1280,"duration":12,"transcoding_status":"completed"}}}}`
}

func imageThingJSON(id string) string {
	return `{"kind":"t3","data":{"id":"` + id + `","name":"t3_` + id + `","author":"testuser","author_fullname":"t2_user1","title":"Image","url":"https://i.redd.it/image.jpg","permalink":"/r/golang/comments/` + id + `/image/","post_hint":"image","created_utc":1785542400}}`
}

func commentThingJSON() string {
	return `{"kind":"t1","data":{"id":"comment1","name":"t1_comment1","author":"other","author_fullname":"t2_other","link_id":"t3_abc","parent_id":"t3_abc","body":"top","score":2,"created_utc":1785542400,"replies":{"kind":"Listing","data":{"children":[{"kind":"t1","data":{"id":"reply1","name":"t1_reply1","author":"testuser","author_fullname":"t2_user1","link_id":"t3_abc","parent_id":"t1_parent","body":"reply","score":1,"created_utc":1785542401}}]}}}}`
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(writer, value)
}
