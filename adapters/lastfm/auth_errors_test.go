package lastfm

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestAuthenticationFlowAndSignature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Query().Get("api_sig") != signature(request.URL.Query(), testAPISecret) {
			http.Error(writer, "bad signature", http.StatusBadRequest)
			return
		}
		switch request.URL.Query().Get("method") {
		case "auth.getToken":
			writeJSON(writer, http.StatusOK, `{"token":"request-token"}`)
		case "auth.getSession":
			if request.URL.Query().Get("token") != "request-token" {
				http.Error(writer, "bad token", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"session":{"name":"listener","key":"authorized-session","subscriber":"1"}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true)
	token, err := client.RequestToken(context.Background())
	if err != nil || token != "request-token" {
		t.Fatalf("token=%q err=%v", token, err)
	}
	authorizationURL, err := client.AuthorizationURL(token, "https://app.example/callback?state=one")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("api_key") != testAPIKey || parsed.Query().Get("token") != token || parsed.Query().Get("cb") == "" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	session, err := client.ExchangeSession(context.Background(), token)
	if err != nil || session.Name != "listener" || session.Key != "authorized-session" || !session.Subscriber {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	if _, err := client.AuthorizationURL("", ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty auth token=%v", err)
	}
	if _, err := client.AuthorizationURL(token, "ftp://app.example"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad callback=%v", err)
	}
	if _, err := client.ExchangeSession(context.Background(), " bad "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad exchange token=%v", err)
	}

	parameters := url.Values{
		"method": {"track.scrobble"}, "artist[1]": {"one"}, "artist[10]": {"ten"},
		"format": {"json"}, "callback": {"ignored"}, "api_sig": {"ignored"},
	}
	input := "artist[10]tenartist[1]onemethodtrack.scrobble" + testAPISecret
	expected := fmt.Sprintf("%x", md5.Sum([]byte(input)))
	if actual := signature(parameters, testAPISecret); actual != expected {
		t.Fatalf("signature=%s expected=%s", actual, expected)
	}
}

func TestAPIAndHTTPErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		retry     string
		want      error
		retryable bool
	}{
		{"API invalid argument", http.StatusOK, `{"error":6,"message":"Invalid parameters"}`, "", socialhub.ErrInvalidArgument, false},
		{"API session", http.StatusOK, `{"error":9,"message":"Invalid session key"}`, "", socialhub.ErrUnauthenticated, false},
		{"API unavailable", http.StatusOK, `{"error":11,"message":"Offline"}`, "", socialhub.ErrUnavailable, true},
		{"API rate", http.StatusOK, `{"error":29,"message":"Rate limit"}`, "12", socialhub.ErrRateLimited, true},
		{"HTTP rate", http.StatusTooManyRequests, `{}`, "7", socialhub.ErrRateLimited, true},
		{"HTTP server", http.StatusServiceUnavailable, `{}`, "", socialhub.ErrUnavailable, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Retry-After", test.retry)
				writeJSON(writer, test.status, test.body)
			}))
			defer server.Close()
			_, client := newTestClient(t, server, false)
			err := client.get(context.Background(), "test.error", nil, false, nil)
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
			var hubError *socialhub.Error
			if !errors.As(err, &hubError) || hubError.Retryable() != test.retryable {
				t.Fatalf("hub error=%#v", hubError)
			}
			if test.retry != "" && hubError.RetryAfter != time.Duration(mustInt(test.retry))*time.Second {
				t.Fatalf("retry after=%v", hubError.RetryAfter)
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, `{not-json`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	if err := client.get(context.Background(), "test.decode", nil, false, nil); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("decode error=%v", err)
	}
}

func mustInt(value string) int {
	parsed := 0
	_, _ = fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed)
	return parsed
}

func errorCode(err error) socialhub.ErrorCode {
	var hubError *socialhub.Error
	if errors.As(err, &hubError) {
		return hubError.Code
	}
	return ""
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}
