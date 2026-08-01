package pinterest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestPinterestOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		clientID, secret, ok := request.BasicAuth()
		if request.URL.Path != "/v5/oauth/token" || request.Method != http.MethodPost || !ok || clientID != "client-id" || secret != "client-secret" || request.ParseForm() != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			writeJSON(writer, `{"access_token":"pina_access","refresh_token":"pinr_refresh","token_type":"bearer","response_type":"authorization_code","expires_in":2592000,"refresh_token_expires_in":5184000,"scope":"boards:read pins:read"}`)
		case "refresh_token":
			writeJSON(writer, `{"access_token":"pina_renewed","refresh_token":"pinr_rotated","token_type":"bearer","response_type":"refresh_token","expires_in":2592000,"refresh_token_expires_at":1790000000,"scope":"pins:read"}`)
		case "client_credentials":
			writeJSON(writer, `{"access_token":"pinc_app","token_type":"bearer","response_type":"client_credentials","expires_in":2592000,"scope":"pins:read"}`)
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, nil)
	oauth, err := adapter.OAuth(context.Background(), "pinner")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state", []string{"boards:read", "pins:read"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("scope") != "boards:read,pins:read" || parsed.Query().Get("state") != "state" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	result, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
	if err != nil || result.Token.AccessToken != "pina_access" || result.Token.RefreshToken != "pinr_refresh" || time.Until(result.Token.ExpiresAt) < 29*24*time.Hour || result.RefreshExpiresAt.IsZero() {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), result.Token.RefreshToken, []string{"pins:read"})
	if err != nil || refreshed.Token.AccessToken != "pina_renewed" || refreshed.Token.RefreshToken != "pinr_rotated" || refreshed.RefreshExpiresAt.Unix() != 1790000000 {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
	app, err := oauth.ClientCredentials(context.Background(), []string{"pins:read"})
	if err != nil || app.Token.AccessToken != "pinc_app" || app.Token.RefreshToken != "" {
		t.Fatalf("app=%#v err=%v", app, err)
	}
}
