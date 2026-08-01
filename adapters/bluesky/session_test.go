package bluesky

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func testJWT(exp int64) string {
	payload, _ := json.Marshal(map[string]int64{"exp": exp})
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func TestLegacySessionLifecycle(t *testing.T) {
	expiresAt := testNow.Add(time.Hour).Unix()
	accessJWT := testJWT(expiresAt)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			var input map[string]string
			if request.Header.Get("Authorization") != "" || request.Header.Get("Content-Type") != "application/json" || json.NewDecoder(request.Body).Decode(&input) != nil || input["identifier"] != "alice.test" || input["password"] != "app-password" || input["authFactorToken"] != "123456" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"accessJwt": accessJWT, "refreshJwt": "refresh-one", "handle": "alice.test", "did": "did:plc:alice",
				"email": "alice@example.test", "emailConfirmed": true, "active": true, "status": "takendown",
			})
		case "/xrpc/com.atproto.server.refreshSession":
			if request.Header.Get("Authorization") != "Bearer refresh-one" || request.ContentLength > 0 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"accessJwt": testJWT(expiresAt + 3600), "refreshJwt": "refresh-two", "handle": "alice.test", "did": "did:plc:alice",
			})
		case "/xrpc/com.atproto.server.deleteSession":
			if request.Header.Get("Authorization") != "Bearer refresh-two" || request.ContentLength > 0 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := &SessionClient{ServiceURL: server.URL, Identifier: "alice.test", Password: "app-password", HTTPClient: server.Client()}

	session, err := client.Create(context.Background(), "123456")
	if err != nil || session.DID != "did:plc:alice" || session.Token.AccessToken != accessJWT || session.Token.RefreshToken != "refresh-one" || session.Token.TokenType != "Bearer" || !session.Token.ExpiresAt.Equal(time.Unix(expiresAt, 0)) || !session.EmailConfirmed || !session.Active {
		t.Fatalf("created session=%#v error=%v", session, err)
	}
	refreshed, err := client.Refresh(context.Background(), session.Token.RefreshToken)
	if err != nil || refreshed.Token.RefreshToken != "refresh-two" || !refreshed.Active {
		t.Fatalf("refreshed session=%#v error=%v", refreshed, err)
	}
	if err := client.Delete(context.Background(), refreshed.Token.RefreshToken); err != nil {
		t.Fatal(err)
	}
}

func TestLegacySessionErrorsAndValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Query().Get("case") {
		default:
			writer.Header().Set("X-Request-ID", "request-2")
			writer.WriteHeader(http.StatusBadRequest)
			writeTestJSON(t, writer, map[string]string{"error": "AuthFactorTokenRequired", "message": "second factor required"})
		}
	}))
	defer server.Close()
	client := &SessionClient{ServiceURL: server.URL, Identifier: "alice.test", Password: "password", HTTPClient: server.Client()}
	if _, err := client.Create(context.Background(), ""); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("MFA error=%v", err)
	}

	invalid := []func() error{
		func() error { _, err := (&SessionClient{}).Create(context.Background(), ""); return err },
		func() error { _, err := client.Refresh(context.Background(), ""); return err },
		func() error { return client.Delete(context.Background(), "") },
		func() error {
			_, err := (&SessionClient{ServiceURL: "bad", Identifier: "a", Password: "b", HTTPClient: server.Client()}).Create(context.Background(), "")
			return err
		},
	}
	for _, call := range invalid {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation error=%v", err)
		}
	}

	active := false
	if _, err := mapSession(sessionResponse{AccessJWT: "a.b.c", RefreshJWT: "refresh", Handle: "alice.test", DID: "did:plc:alice", Active: &active, Status: "suspended"}); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("inactive account error=%v", err)
	}
	if _, err := mapSession(sessionResponse{}); err == nil {
		t.Fatal("malformed session should fail")
	}
	for _, token := range []string{"", "not-a-jwt", "a.%%%.c", "a." + base64.RawURLEncoding.EncodeToString([]byte(`{"exp":"bad"}`)) + ".c"} {
		if expiry := jwtExpiry(token); !expiry.IsZero() {
			t.Fatalf("token=%q expiry=%v", token, expiry)
		}
	}
	if got := boundedMessage(strings.Repeat("界", 5), 3); got != "界界界" {
		t.Fatalf("bounded message=%q", got)
	}
}
