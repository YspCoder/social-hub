package stackexchange

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testQuestionTitle = "How can I use typed Go adapters?"
	testQuestionBody  = "I need a typed adapter that preserves platform fields safely."
	testAnswerBody    = "Use the QnA workflow because it preserves question-specific fields."
)

func TestQnAWorkflowContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if query.Get("key") != "app-key" || query.Get("access_token") != "access-token" || query.Get("site") != "stackoverflow" || request.UserAgent() != defaultUserAgent {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/2.3/questions/add":
			if request.ParseForm() != nil || request.PostForm.Get("title") != testQuestionTitle || request.PostForm.Get("body") != testQuestionBody || request.PostForm.Get("tags") != "go;api" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"items":[{"question_id":600,"post_type":"question","owner":{"user_id":42},"title":"How can I use typed Go adapters?","body_markdown":"Created question","tags":["go","api"],"link":"https://stackoverflow.com/q/600","creation_date":1785628800}],"quota_max":10000,"quota_remaining":9000}`)
		case request.Method == http.MethodPost && request.URL.Path == "/2.3/questions/600/answers/add":
			if request.ParseForm() != nil || request.PostForm.Get("body") != testAnswerBody {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"items":[{"answer_id":601,"question_id":600,"post_type":"answer","owner":{"user_id":42},"body_markdown":"Created answer","link":"https://stackoverflow.com/a/601","creation_date":1785628810}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/2.3/search/advanced":
			if query.Get("q") != "typed adapters" || query.Get("tagged") != "go;api" || query.Get("page") != "3" || query.Get("pagesize") != "100" || query.Get("sort") != "votes" || query.Get("filter") != "withbody" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"items":[{"question_id":700,"post_type":"question","title":"Typed adapters","body_markdown":"Search result","link":"https://stackoverflow.com/q/700"}],"has_more":true}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	clock := &mutableClock{now: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
	_, client := newTestClient(t, server, true, []string{"write_access"}, clock)
	workflow := client.QnAWorkflow()
	question, err := workflow.CreateQuestion(context.Background(), CreateQuestionRequest{
		Title: testQuestionTitle, Body: testQuestionBody, Tags: []string{"go", "api", "go"},
	})
	if err != nil || question.ID != "600" || question.Status == nil || question.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("question=%#v err=%v", question, err)
	}
	answer, err := workflow.CreateAnswer(context.Background(), CreateAnswerRequest{QuestionID: "600", Body: testAnswerBody})
	if err != nil || answer.ID != "601" || len(answer.Relations) != 1 || answer.Relations[0].PostID != "600" {
		t.Fatalf("answer=%#v err=%v", answer, err)
	}
	page, err := workflow.SearchQuestions(context.Background(), SearchQuestionsRequest{
		Query: " typed adapters ", Tagged: []string{"go", "api", "go"}, Cursor: "3", MaxResults: 150, Sort: "votes",
	})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != "700" || page.NextCursor == nil || *page.NextCursor != "4" || page.PrevCursor == nil || *page.PrevCursor != "2" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
}

func TestQnAValidationAndPublicSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/2.3/search/advanced" || request.URL.Query().Get("key") != "app-key" || request.URL.Query().Get("access_token") != "" || request.URL.Query().Get("tagged") != "go" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, `{"items":[],"has_more":false,"quota_max":10000,"quota_remaining":9999}`)
	}))
	defer server.Close()
	clock := &mutableClock{now: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
	_, public := newTestClient(t, server, false, nil, clock)
	if _, err := public.SearchQuestions(context.Background(), SearchQuestionsRequest{Tagged: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := public.CreateQuestion(context.Background(), CreateQuestionRequest{Title: testQuestionTitle, Body: testQuestionBody, Tags: []string{"go"}}); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("public create error=%v", err)
	}

	invalidQuestions := []CreateQuestionRequest{
		{Title: "short", Body: testQuestionBody, Tags: []string{"go"}},
		{Title: testQuestionTitle, Body: "short", Tags: []string{"go"}},
		{Title: testQuestionTitle, Body: testQuestionBody},
		{Title: testQuestionTitle, Body: testQuestionBody, Tags: []string{"go", "api", "one", "two", "three", "four"}},
		{Title: testQuestionTitle, Body: testQuestionBody, Tags: []string{"go;api"}},
	}
	for _, input := range invalidQuestions {
		if _, err := public.CreateQuestion(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("question=%#v error=%v", input, err)
		}
	}
	invalidAnswers := []CreateAnswerRequest{{QuestionID: "x", Body: testAnswerBody}, {QuestionID: "1", Body: "short"}}
	for _, input := range invalidAnswers {
		if _, err := public.CreateAnswer(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("answer=%#v error=%v", input, err)
		}
	}
	invalidSearches := []SearchQuestionsRequest{{}, {Query: "go", Cursor: "0"}, {Query: "go", MaxResults: -1}, {Query: "go", Sort: "hot"}, {Tagged: []string{"bad;tag"}}}
	for _, input := range invalidSearches {
		if _, err := public.SearchQuestions(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("search=%#v error=%v", input, err)
		}
	}
}
