package threads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestTypedInsightsDiscoveryModerationRepostAndQuota(t *testing.T) {
	post := map[string]any{
		"id": "public-post", "media_product_type": "THREADS", "media_type": "TEXT_POST",
		"permalink": "https://www.threads.com/@public/post/one", "username": "public", "text": "public text", "timestamp": "2026-08-01T10:00:00Z",
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("access_token") != "access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/post-1/insights":
			if request.URL.Query().Get("metric") != "views,likes" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"data": []any{
				map[string]any{"id": "post-1/insights/views/lifetime", "name": "views", "period": "lifetime", "title": "Views", "description": "views", "values": []any{map[string]any{"value": 42}}},
			}})
		case request.Method == http.MethodGet && request.URL.Path == "/me/threads_insights":
			writeTestJSON(t, writer, map[string]any{"data": []any{
				map[string]any{"id": "user-1/insights/followers_count/day", "name": "followers_count", "period": "day", "total_value": map[string]any{"value": 100}},
			}})
		case request.Method == http.MethodGet && request.URL.Path == "/profile_lookup":
			if request.URL.Query().Get("username") != "public" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"id": "public-user", "username": "public", "name": "Public User"})
		case request.Method == http.MethodGet && request.URL.Path == "/profile_posts":
			if request.URL.Query().Get("username") != "public" || request.URL.Query().Get("after") != "profile-cursor" || request.URL.Query().Get("limit") != "10" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"data": []any{post}})
		case request.Method == http.MethodGet && request.URL.Path == "/keyword_search":
			if request.URL.Query().Get("q") != "Go SDK" || request.URL.Query().Get("search_type") != "RECENT" || request.URL.Query().Get("after") != "search-cursor" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"data": []any{post}, "paging": map[string]any{"cursors": map[string]string{"after": "search-next"}}})
		case request.Method == http.MethodGet && request.URL.Path == "/me/mentions":
			if request.URL.Query().Get("limit") != "5" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"data": []any{post}})
		case request.Method == http.MethodPost && request.URL.Path == "/reply-1/manage_reply":
			if request.ParseForm() != nil || request.PostForm.Get("hide") != "true" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]bool{"success": true})
		case request.Method == http.MethodPost && request.URL.Path == "/reply-2/manage_pending_reply":
			if request.ParseForm() != nil || request.PostForm.Get("approve") != "false" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]bool{"success": true})
		case request.Method == http.MethodPost && request.URL.Path == "/post-1/repost":
			writeTestJSON(t, writer, map[string]string{"id": "repost-1"})
		case request.Method == http.MethodGet && request.URL.Path == "/me/threads_publishing_limit":
			if !strings.Contains(request.URL.Query().Get("fields"), "delete_quota_usage") {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"data": []any{map[string]any{
				"quota_usage": 3, "config": map[string]int64{"quota_total": 250, "quota_duration": 86400},
				"reply_quota_usage": 4, "reply_config": map[string]int64{"quota_total": 1000, "quota_duration": 86400},
				"delete_quota_usage": 1, "delete_config": map[string]int64{"quota_total": 100, "quota_duration": 86400},
			}}})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, allScopes())

	postInsights, err := client.PostInsights(context.Background(), "post-1", []string{"views", "likes"})
	if err != nil || len(postInsights) != 1 || postInsights[0].Name != "views" || len(postInsights[0].Values) != 1 || string(postInsights[0].Values[0].Value) != "42" {
		t.Fatalf("post insights=%#v error=%v", postInsights, err)
	}
	accountInsights, err := client.AccountInsights(context.Background(), []string{"followers_count"})
	if err != nil || len(accountInsights) != 1 || accountInsights[0].TotalValue == nil || string(accountInsights[0].TotalValue.Value) != "100" {
		t.Fatalf("account insights=%#v error=%v", accountInsights, err)
	}
	profile, err := client.LookupProfile(context.Background(), "public")
	if err != nil || profile.ID != "public-user" || profile.Username == nil || *profile.Username != "public" {
		t.Fatalf("profile=%#v error=%v", profile, err)
	}
	profilePosts, err := client.ProfilePosts(context.Background(), "public", PageRequest{Cursor: "profile-cursor", MaxResults: 10})
	if err != nil || len(profilePosts.Items) != 1 {
		t.Fatalf("profile posts=%#v error=%v", profilePosts, err)
	}
	search, err := client.KeywordSearch(context.Background(), KeywordSearchRequest{Query: "Go SDK", Type: KeywordSearchRecent, Cursor: "search-cursor"})
	if err != nil || len(search.Items) != 1 || search.NextCursor == nil || *search.NextCursor != "search-next" {
		t.Fatalf("search=%#v error=%v", search, err)
	}
	mentions, err := client.Mentions(context.Background(), PageRequest{MaxResults: 5})
	if err != nil || len(mentions.Items) != 1 {
		t.Fatalf("mentions=%#v error=%v", mentions, err)
	}
	if err := client.SetReplyHidden(context.Background(), "reply-1", true); err != nil {
		t.Fatal(err)
	}
	if err := client.ReviewPendingReply(context.Background(), "reply-2", false); err != nil {
		t.Fatal(err)
	}
	repost, err := client.Repost(context.Background(), "post-1")
	if err != nil || repost.ID != "repost-1" || !hasRelation(*repost, socialhub.RelationRepost, "post-1") {
		t.Fatalf("repost=%#v error=%v", repost, err)
	}
	quota, err := client.PublishingQuota(context.Background())
	if err != nil || quota.PostUsage != 3 || quota.PostConfig.Total != 250 || quota.ReplyUsage != 4 || quota.DeleteUsage != 1 {
		t.Fatalf("quota=%#v error=%v", quota, err)
	}
}

func TestTypedWorkflowValidationAndScopeGating(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, allScopes())
	invalidCalls := []func() error{
		func() error { _, err := client.PostInsights(context.Background(), "", []string{"views"}); return err },
		func() error {
			_, err := client.PostInsights(context.Background(), "post", []string{"bad-name"})
			return err
		},
		func() error { _, err := client.AccountInsights(context.Background(), nil); return err },
		func() error { _, err := client.LookupProfile(context.Background(), ""); return err },
		func() error { _, err := client.ProfilePosts(context.Background(), "", PageRequest{}); return err },
		func() error { _, err := client.KeywordSearch(context.Background(), KeywordSearchRequest{}); return err },
		func() error {
			_, err := client.KeywordSearch(context.Background(), KeywordSearchRequest{Query: "q", Type: "POPULAR"})
			return err
		},
		func() error { return client.SetReplyHidden(context.Background(), "", true) },
		func() error { return client.ReviewPendingReply(context.Background(), "", true) },
		func() error { _, err := client.Repost(context.Background(), ""); return err },
	}
	for _, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation error=%v", err)
		}
	}

	_, limited := newTestAdapter(t, server, []string{"threads_basic"})
	if _, err := limited.PostInsights(context.Background(), "post", []string{"views"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("insights scope error=%v", err)
	}
	if _, err := limited.LookupProfile(context.Background(), "public"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("discovery scope error=%v", err)
	}
	if _, err := limited.CreateContainer(context.Background(), ContainerRequest{Type: ContainerImage, ImageURL: "https://cdn.test/a.jpg"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("container scope error=%v", err)
	}
	if _, err := limited.PublishingQuota(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("quota scope error=%v", err)
	}
}

func TestInsightEndTimeMapping(t *testing.T) {
	response := graphInsightPage{Data: []graphInsight{{
		Name: "views", Values: []graphInsightValue{{Value: json.RawMessage(`1`), EndTime: graphTime{Time: testNow}}},
	}}}
	mapped := mapInsights(response)
	if len(mapped) != 1 || mapped[0].Values[0].EndTime == nil || !mapped[0].Values[0].EndTime.Equal(testNow) {
		t.Fatalf("mapped=%#v", mapped)
	}
}
