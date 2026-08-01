package reddit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestSubmissionWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("User-Agent") != testUserAgent || request.ParseForm() != nil {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/api/submit":
			if request.Form.Get("kind") != "self" || request.Form.Get("sr") != "golang" || request.Form.Get("title") != "Title" || request.Form.Get("text") != "Body" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"json":{"errors":[],"data":{"id":"created","name":"t3_created","url":"https://www.reddit.com/r/golang/comments/created/"}}}`)
		case "/api/del":
			if request.Form.Get("id") != "t3_created" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"submit", "edit"})
	workflow := client.SubmissionWorkflow()
	post, err := workflow.Submit(context.Background(), SubmissionRequest{Kind: SubmissionSelf, Subreddit: "golang", Title: "Title", Text: "Body", SendReplies: true})
	if err != nil || post.ID != "t3_created" || post.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	if err := workflow.Delete(context.Background(), post.ID); err != nil {
		t.Fatal(err)
	}
	invalid := []SubmissionRequest{
		{Kind: SubmissionSelf, Subreddit: "r/golang", Title: "Title"},
		{Kind: SubmissionLink, Subreddit: "golang", Title: "Title", URL: "file:///tmp/link"},
	}
	for _, input := range invalid {
		if err := validateSubmission(input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
}
