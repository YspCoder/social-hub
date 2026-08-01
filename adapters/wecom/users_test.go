package wecom

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestGetUserMappingAndUnsupportedFetches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/cgi-bin/user/get" || request.URL.Query().Get("access_token") != "access-token" || request.URL.Query().Get("userid") != "alice" {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		writeTestJSON(t, writer, map[string]any{
			"errcode": 0, "userid": "alice", "name": "Alice", "alias": "alice.alias",
			"avatar": "https://cdn.example/alice.jpg", "department": []int64{1, 2}, "status": 1,
		})
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	user, err := client.GetUser(context.Background(), "alice")
	if err != nil || user.ID != "alice" || user.DisplayName == nil || *user.DisplayName != "Alice" || user.Username == nil || *user.Username != "alice.alias" || user.Extensions["wecom.corp_api"] == nil {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	if _, err := client.GetUser(context.Background(), "a|b"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid user ID=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "post"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get post=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list posts=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list comments=%v", err)
	}
}

func TestBusinessTokenErrorInvalidatesCache(t *testing.T) {
	var tokenCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/cgi-bin/gettoken":
			call := tokenCalls.Add(1)
			writeTestJSON(t, writer, map[string]any{"errcode": 0, "access_token": map[bool]string{true: "expired", false: "fresh"}[call == 1], "expires_in": 7200})
		case "/cgi-bin/user/get":
			if request.URL.Query().Get("access_token") == "expired" {
				writeTestJSON(t, writer, map[string]any{"errcode": 40014, "errmsg": "invalid access_token"})
				return
			}
			writeTestJSON(t, writer, map[string]any{"errcode": 0, "userid": "alice", "name": "Alice"})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	config := testConfig(server.URL, false)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].SecretRef = "test://corp-secret"
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{"test://corp-secret": "corp-secret"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	if _, err := client.GetUser(context.Background(), "alice"); errorCode(err) != socialhub.CodeUnauthenticated {
		t.Fatalf("first user error=%v", err)
	}
	user, err := client.GetUser(context.Background(), "alice")
	if err != nil || user.ID != "alice" || tokenCalls.Load() != 2 {
		t.Fatalf("second user=%#v token calls=%d err=%v", user, tokenCalls.Load(), err)
	}
}
