package zhihu

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestSearchAndHotListContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-secret" || request.Header.Get("X-OAuth-Token") != "" || request.Header.Get("Content-Type") != "application/json" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/api/v1/content/zhihu_search":
			if request.URL.Query().Get("Query") != "RAG" || request.URL.Query().Get("Count") != "10" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"Code":0,"Message":"success","Data":{"HasMore":false,"SearchHashId":"hash-1","Items":[{"Title":"RAG review","ContentType":"Article","ContentID":"123","ContentText":"summary","Url":"https://zhuanlan.zhihu.com/p/123","CommentCount":15,"VoteUpCount":128,"AuthorName":"Creator","AuthorAvatar":"https://pic.example/a.jpg","AuthorBadge":"","AuthorBadgeText":"","EditTime":1710000000,"CommentInfoList":[{"Content":"selected"}],"AuthorityLevel":"2","RankingScore":0.98}]}}`))
		case "/api/v1/content/hot_list":
			if request.URL.Query().Get("Limit") != "30" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"Code":0,"Message":"success","Data":{"Total":1,"Items":[{"Title":"Hot question","Url":"https://www.zhihu.com/question/1","ThumbnailUrl":"https://pic.example/hot.jpg","Summary":"hot summary"}]}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, true, true, false)
	result, err := client.SearchWorkflow().Search(context.Background(), SearchRequest{Query: " RAG ", Count: 99})
	if err != nil {
		t.Fatal(err)
	}
	if result.SearchHashID != "hash-1" || len(result.Items) != 1 || result.Items[0].ContentID != "123" || result.Items[0].EditedAt == nil || result.Items[0].EditedAt.Unix() != 1710000000 || len(result.Items[0].Comments) != 1 {
		t.Fatalf("search result=%#v", result)
	}
	hot, err := client.SearchWorkflow().HotList(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if hot.Total != 1 || len(hot.Items) != 1 || hot.Items[0].Title != "Hot question" {
		t.Fatalf("hot list=%#v", hot)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true, false, false)
	if _, err := client.SearchWorkflow().Search(context.Background(), SearchRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("error=%v", err)
	}
}
