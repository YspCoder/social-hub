package dingtalk

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestContactAndRobotContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-acs-dingtalk-access-token") != "access-token" || request.Header.Get("Accept") != "application/json" {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/v1.0/contact/users/union-1":
			if request.Method != http.MethodGet {
				http.Error(writer, "bad method", http.StatusMethodNotAllowed)
				return
			}
			writeTestJSON(t, writer, http.StatusOK, map[string]any{
				"avatarUrl": "https://cdn.example/alice.jpg", "email": "alice@example.test",
				"loginEmail": "alice.login@example.test", "mobile": "13800000000", "nick": "Alice",
				"openId": "open-alice", "stateCode": "86", "unionId": "union-1", "visitor": false,
			})
		case "/v1.0/robot/groupMessages/send":
			var payload robotSendPayload
			if request.Method != http.MethodPost || json.NewDecoder(request.Body).Decode(&payload) != nil ||
				payload.MessageKey != "sampleText" || payload.RobotCode != "ding-robot" || payload.OpenConversationID != "cid-1" || payload.MessageParam != `{"content":"hello"}` {
				http.Error(writer, "bad group payload", http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, http.StatusOK, map[string]any{"processQueryKey": "group-job"})
		case "/v1.0/robot/oToMessages/batchSend":
			var payload robotSendPayload
			if request.Method != http.MethodPost || json.NewDecoder(request.Body).Decode(&payload) != nil ||
				payload.MessageKey != "sampleText" || payload.RobotCode != "ding-robot" || len(payload.UserIDs) != 2 || payload.OpenConversationID != "" {
				http.Error(writer, "bad OTO payload", http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, http.StatusOK, map[string]any{
				"processQueryKey": "oto-job", "filteredStaffIdList": []string{"filtered"},
				"flowControlledStaffIdList": []string{"limited"}, "invalidStaffIdList": []string{"invalid"},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, true)

	detail, err := client.GetUserByUnionID(context.Background(), "union-1")
	if err != nil || detail.UnionID != "union-1" || detail.OpenID != "open-alice" || detail.Nick != "Alice" || detail.StateCode != "86" {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}
	user, err := client.GetUser(context.Background(), "union-1", socialhub.WithRequestID("caller-1"))
	if err != nil || user.ID != "union-1" || user.Username == nil || *user.Username != "open-alice" ||
		user.DisplayName == nil || *user.DisplayName != "Alice" || user.Extensions["dingtalk.contact_user"] == nil {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	message := RobotMessage{Key: "sampleText", Param: json.RawMessage(`{"content":"hello"}`)}
	group, err := client.SendGroupMessage(context.Background(), GroupMessageRequest{OpenConversationID: "cid-1", Message: message})
	if err != nil || group.ProcessQueryKey != "group-job" {
		t.Fatalf("group=%#v err=%v", group, err)
	}
	oto, err := client.BatchSendOTO(context.Background(), BatchOTORequest{UserIDs: []string{"alice", "bob"}, Message: message})
	if err != nil || oto.ProcessQueryKey != "oto-job" || len(oto.FilteredStaffIDList) != 1 ||
		len(oto.FlowControlledStaffIDList) != 1 || len(oto.InvalidStaffIDList) != 1 {
		t.Fatalf("oto=%#v err=%v", oto, err)
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

func TestWorkflowValidationAndCallOptions(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, true)
	validMessage := RobotMessage{Key: "sampleText", Param: json.RawMessage(`{"content":"hello"}`)}
	if _, err := client.GetUserByUnionID(context.Background(), " bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid UnionID=%v", err)
	}
	invalidGroups := []GroupMessageRequest{
		{Message: validMessage},
		{OpenConversationID: "cid", Message: RobotMessage{Key: "", Param: json.RawMessage(`{}`)}},
		{OpenConversationID: "cid", Message: RobotMessage{Key: "sampleText", Param: json.RawMessage(`[]`)}},
		{OpenConversationID: "cid", Message: RobotMessage{Key: "sampleText", Param: json.RawMessage(`{`)}},
		{OpenConversationID: "cid", Message: RobotMessage{Key: "sampleText", Param: json.RawMessage(`null`)}},
	}
	for index, request := range invalidGroups {
		if _, err := client.SendGroupMessage(context.Background(), request); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid group %d=%v", index, err)
		}
	}
	invalidOTO := []BatchOTORequest{
		{Message: validMessage},
		{UserIDs: []string{"bad\n"}, Message: validMessage},
		{UserIDs: []string{"same", "same"}, Message: validMessage},
		{UserIDs: make([]string, 101), Message: validMessage},
	}
	for index, request := range invalidOTO {
		if _, err := client.BatchSendOTO(context.Background(), request); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid OTO %d=%v", index, err)
		}
	}
	if _, err := client.GetUserByUnionID(context.Background(), "union", socialhub.WithFields("nick")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("fields=%v", err)
	}
	if _, err := client.SendGroupMessage(context.Background(), GroupMessageRequest{OpenConversationID: "cid", Message: validMessage}, socialhub.WithIdempotencyKey("key")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("idempotency=%v", err)
	}
	_, noRobot := newTestClient(t, server, false, false)
	if _, err := noRobot.SendGroupMessage(context.Background(), GroupMessageRequest{OpenConversationID: "cid", Message: validMessage}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("missing robot=%v", err)
	}
	capabilities, _ := noRobot.Capabilities(context.Background())
	if capabilities.Has(CapabilityRobotMessages) || !strings.Contains(capabilities[CapabilityRobotMessages].Reason, "configure") {
		t.Fatalf("robot capability=%#v", capabilities[CapabilityRobotMessages])
	}
}

func TestPermissionErrorAndManagedTokenInvalidation(t *testing.T) {
	t.Run("permission details", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("x-acs-request-id", "header-request")
			writeTestJSON(t, writer, http.StatusForbidden, map[string]any{
				"code": "Forbidden.AccessDenied.AccessTokenPermissionDenied", "message": "scope required",
				"requestid": "body-request", "accessdenieddetail": map[string]any{"requiredScopes": []string{"Contact.User.Read"}},
			})
		}))
		defer server.Close()
		_, client := newTestClient(t, server, false, false)
		_, err := client.GetUserByUnionID(context.Background(), "union")
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || !errors.Is(err, socialhub.ErrApprovalRequired) ||
			platformErr.RequestID != "body-request" || len(platformErr.RequiredScopes) != 1 || platformErr.ApprovalURL == "" {
			t.Fatalf("permission error=%#v", platformErr)
		}
	})

	t.Run("401 invalidates token", func(t *testing.T) {
		var tokenCalls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch {
			case strings.HasPrefix(request.URL.Path, "/v1.0/oauth2/"):
				call := tokenCalls.Add(1)
				writeTestJSON(t, writer, http.StatusOK, map[string]any{"access_token": "token-" + string(rune('0'+call)), "expires_in": 7200})
			case request.URL.Path == "/v1.0/contact/users/union":
				if request.Header.Get("x-acs-dingtalk-access-token") == "token-1" {
					writeTestJSON(t, writer, http.StatusUnauthorized, map[string]any{"code": "InvalidAuthentication", "message": "expired"})
					return
				}
				writeTestJSON(t, writer, http.StatusOK, map[string]any{"unionId": "union", "nick": "Alice"})
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()
		_, client := newTestClient(t, server, true, false)
		capabilities, _ := client.Capabilities(context.Background())
		if !capabilities.Has(CapabilityAppToken) {
			t.Fatalf("token capability=%#v", capabilities[CapabilityAppToken])
		}
		if _, err := client.GetUserByUnionID(context.Background(), "union"); !errors.Is(err, socialhub.ErrUnauthenticated) {
			t.Fatalf("first read=%v", err)
		}
		detail, err := client.GetUserByUnionID(context.Background(), "union")
		if err != nil || detail.UnionID != "union" || tokenCalls.Load() != 2 {
			t.Fatalf("second read=%#v token calls=%d err=%v", detail, tokenCalls.Load(), err)
		}
		refreshed, err := client.RefreshAppToken(context.Background())
		if err != nil || refreshed.AccessToken != "token-3" || tokenCalls.Load() != 3 {
			t.Fatalf("explicit refresh=%#v calls=%d err=%v", refreshed, tokenCalls.Load(), err)
		}
	})
}

func TestErrorsAndRedirectRefusal(t *testing.T) {
	tests := []struct {
		status int
		code   string
		want   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusTooManyRequests, "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusBadGateway, "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusGatewayTimeout, "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusBadRequest, "InvalidParameter.Name", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusOK, "90018", socialhub.CodeRateLimited, socialhub.ClassRetryable},
	}
	for _, test := range tests {
		code, class := classifyError(test.status, test.code)
		if code != test.want || class != test.class {
			t.Fatalf("classify(%d,%q)=%s/%s", test.status, test.code, code, class)
		}
	}
	header := make(http.Header)
	header.Set("Retry-After", "9")
	header.Set("x-acs-request-id", "request-1")
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"code":"Throttling.TooFast","message":"slow"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.RetryAfter != 9*time.Second || platformErr.RequestID != "request-1" {
		t.Fatalf("decoded error=%#v", platformErr)
	}
	if err := decodeHTTPError(http.StatusInternalServerError, nil, []byte(`{"code":0}`)); !errors.Is(err, socialhub.ErrUnavailable) {
		t.Fatalf("zero-code HTTP error=%v", err)
	}

	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		targetCalls.Add(1)
		writeTestJSON(t, writer, http.StatusOK, map[string]any{"unionId": "union"})
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false)
	if _, err := client.GetUserByUnionID(context.Background(), "union"); err == nil {
		t.Fatal("expected redirect refusal")
	}
	if targetCalls.Load() != 0 {
		t.Fatalf("redirect target calls=%d", targetCalls.Load())
	}
}
