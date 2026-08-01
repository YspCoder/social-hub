package stackexchange

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestFetchReactionAndBackoffContracts(t *testing.T) {
	var userHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if query.Get("key") != "app-key" || query.Get("access_token") != "access-token" || query.Get("site") != "stackoverflow" || request.UserAgent() != defaultUserAgent {
			writeJSON(writer, http.StatusUnauthorized, `{"error_id":401,"error_name":"invalid_access_token","error_message":"bad auth"}`)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/2.3/users/42":
			userHits.Add(1)
			writeJSON(writer, http.StatusOK, `{"items":[{"user_id":42,"account_id":7,"display_name":"&lt;Alice &amp; Bob&gt;","profile_image":"https://cdn.example/avatar.png","link":"https://stackoverflow.com/users/42/alice","user_type":"registered","reputation":123,"badge_counts":{"bronze":3,"silver":2,"gold":1}}],"has_more":false,"quota_max":10000,"quota_remaining":9999,"backoff":5}`)
		case request.Method == http.MethodGet && request.URL.Path == "/2.3/posts/100":
			if query.Get("filter") != "withbody" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"items":[{"post_id":100,"post_type":"answer","answer_id":100,"question_id":99,"owner":{"user_id":42},"body_markdown":"Use a typed adapter.","body":"<p>Use a typed adapter.</p>","link":"https://stackoverflow.com/a/100","creation_date":1754092800,"last_activity_date":1754092860,"score":4,"comment_count":1,"is_accepted":true}],"quota_max":10000,"quota_remaining":9998}`)
		case request.Method == http.MethodGet && request.URL.Path == "/2.3/users/42/questions":
			if query.Get("page") != "2" || query.Get("pagesize") != "100" || query.Get("fromdate") != "1754006400" || query.Get("todate") != "1754092800" || query.Get("sort") != "activity" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"items":[{"question_id":200,"post_type":"question","owner":{"user_id":42},"title":"How to map APIs?","body_markdown":"Question body","tags":["go","api"],"link":"https://stackoverflow.com/q/200","creation_date":1754092800,"score":2,"view_count":10,"answer_count":1}],"has_more":true,"quota_max":10000,"quota_remaining":9997}`)
		case request.Method == http.MethodGet && request.URL.Path == "/2.3/posts/100/comments":
			if query.Get("sort") != "creation" || query.Get("order") != "asc" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"items":[{"comment_id":300,"post_id":100,"owner":{"user_id":7},"reply_to_user":{"user_id":42},"body_markdown":"Useful comment here","creation_date":1754092900,"score":2}],"has_more":false}`)
		case request.Method == http.MethodPost && (request.URL.Path == "/2.3/posts/100/up" || request.URL.Path == "/2.3/posts/100/up/undo"):
			writeJSON(writer, http.StatusOK, `{"items":[],"has_more":false,"quota_max":10000,"quota_remaining":9996}`)
		case request.Method == http.MethodPost && request.URL.Path == "/2.3/posts/100/comments/add":
			if request.ParseForm() != nil || request.PostForm.Get("body") != "This is a useful comment." {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"items":[{"comment_id":301,"post_id":100,"owner":{"user_id":42},"body_markdown":"This is a useful comment.","creation_date":1754093000}]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/2.3/comments/301/delete":
			writeJSON(writer, http.StatusOK, `{"items":[]}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	clock := &mutableClock{now: time.Date(2025, 8, 2, 0, 0, 0, 0, time.UTC)}
	_, client := newTestClient(t, server, true, []string{"write_access"}, clock)

	user, err := client.GetUser(context.Background(), "42")
	if err != nil || user.ID != "42" || user.DisplayName == nil || *user.DisplayName != "<Alice & Bob>" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	quota := client.Quota()
	if quota.Maximum != 10000 || quota.Remaining != 9999 || quota.Backoff != 5*time.Second || quota.Method != "users" || !quota.ObservedAt.Equal(clock.Now()) {
		t.Fatalf("quota=%#v", quota)
	}
	if _, err := client.GetUser(context.Background(), "42"); !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("backoff error=%v", err)
	} else if typed := new(socialhub.Error); !errors.As(err, &typed) || typed.RetryAfter != 5*time.Second {
		t.Fatalf("typed backoff=%#v", typed)
	}
	if userHits.Load() != 1 {
		t.Fatalf("user requests during backoff=%d", userHits.Load())
	}

	post, err := client.GetPost(context.Background(), "100")
	if err != nil || post.ID != "100" || post.AuthorID == nil || *post.AuthorID != "42" || len(post.Relations) != 1 || post.Relations[0].PostID != "99" || post.Text == nil || *post.Text != "Use a typed adapter." {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	clock.Advance(5 * time.Second)
	if _, err := client.GetUser(context.Background(), "42"); err != nil || userHits.Load() != 2 {
		t.Fatalf("expired backoff err=%v hits=%d", err, userHits.Load())
	}

	start := time.Unix(1754006400, 0)
	end := time.Unix(1754092800, 0)
	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "2", MaxResults: 200, StartTime: &start, EndTime: &end})
	if err != nil || len(posts.Items) != 1 || posts.Items[0].ID != "200" || posts.NextCursor == nil || *posts.NextCursor != "3" || posts.PrevCursor == nil || *posts.PrevCursor != "1" {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "100", MaxResults: 10})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "300" || comments.Items[0].AuthorID == nil || *comments.Items[0].AuthorID != "7" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	reaction := socialhub.ReactionRequest{ActorID: "42", TargetID: "100", Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "100", Text: "This is a useful comment."})
	if err != nil || comment.ID != "301" || comment.PostID != "100" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), "301"); err != nil {
		t.Fatal(err)
	}
}

func TestValidationScopeErrorsAndRedirects(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/2.3/users/1":
			writeJSON(writer, http.StatusBadRequest, `{"error_id":400,"error_name":"invalid_parameter","error_message":"bad user"}`)
		case "/2.3/users/2":
			writeJSON(writer, http.StatusBadRequest, `{"error_id":502,"error_name":"throttle_violation","error_message":"slow down","backoff":3}`)
		case "/2.3/posts/500":
			writeJSON(writer, http.StatusOK, `{"items":[],"backoff":7,"error_id":502,"error_name":"throttle_violation","error_message":"slow down"}`)
		case "/2.3/posts/302":
			http.Redirect(writer, request, target.URL, http.StatusFound)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	clock := &mutableClock{now: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
	_, client := newTestClient(t, server, true, []string{"read_inbox"}, clock)
	if _, err := client.GetUser(context.Background(), "1"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("HTTP wrapper error=%v", err)
	}
	if _, err := client.GetUser(context.Background(), "2"); !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("HTTP backoff error=%v", err)
	}
	if _, err := client.GetUser(context.Background(), "3"); !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("HTTP method backoff error=%v", err)
	}
	clock.Advance(3 * time.Second)
	if _, err := client.GetPost(context.Background(), "500"); !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("2xx wrapper error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "302"); !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("method backoff error=%v", err)
	}
	clock.Advance(7 * time.Second)
	if _, err := client.GetPost(context.Background(), "302"); err == nil || redirected.Load() != 0 {
		t.Fatalf("redirect error=%v followed=%d", err, redirected.Load())
	}

	invalidCalls := []func() error{
		func() error { _, err := client.GetUser(context.Background(), "../1"); return err },
		func() error { _, err := client.GetPost(context.Background(), "0"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "x"})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "x"})
			return err
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "1", Kind: socialhub.ReactionRepost})
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{ActorID: "99", TargetID: "1", Kind: socialhub.ReactionLike})
		},
		func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "1", Text: "short"})
			return err
		},
		func() error {
			parent := "2"
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "1", ParentID: &parent, Text: "This is long enough."})
			return err
		},
		func() error { return client.DeleteComment(context.Background(), "bad") },
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
	if err := client.DeleteComment(context.Background(), "1"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}

	_, public := newTestClient(t, server, false, nil, clock)
	if _, err := public.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "1", Text: "This is long enough."}); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("public write error=%v", err)
	}
}

func TestAPIKeySecretReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("key") != "secret-api-key" || request.URL.Query().Get("access_token") != "" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(writer, http.StatusOK, `{"items":[{"user_id":42,"display_name":"Alice"}]}`)
	}))
	defer server.Close()
	clock := &mutableClock{now: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
	config := testConfig(server, false, nil)
	config.Accounts[0].AppID = ""
	config.Accounts[0].Settings["api_key_ref"] = "test://api-key"
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(clock),
		socialhub.WithSecretResolver(mapResolver{"test://api-key": "secret-api-key"}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "stackoverflow")
	if err != nil {
		t.Fatal(err)
	}
	user, err := common.(*Client).GetUser(context.Background(), "42")
	if err != nil || user.ID != "42" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
}
