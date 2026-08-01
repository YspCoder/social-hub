package page

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestOAuthExchangeAndListPages(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v26.0/oauth/access_token":
			_ = request.ParseForm()
			if request.Form.Get("client_secret") != "app-secret" {
				writer.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = writer.Write([]byte(`{"access_token":"user-token","token_type":"bearer","expires_in":3600}`))
		case "/v26.0/me/accounts":
			if request.Header.Get("Authorization") != "Bearer user-token" {
				writer.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = writer.Write([]byte(`{"data":[{"id":"123","name":"Example Page","access_token":"page-token","tasks":["CREATE_CONTENT"]}]}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := OAuthClient{ClientID: "app-id", ClientSecret: "app-secret", AuthURL: server.URL + "/v26.0/dialog/oauth", TokenURL: server.URL + "/v26.0/oauth/access_token", APIURL: server.URL + "/v26.0", HTTPClient: server.Client()}
	authorizationURL, err := client.AuthorizationURL("https://app.example/callback", "state", []string{"pages_manage_posts"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("state") != "state" || parsed.Query().Get("scope") != "pages_manage_posts" {
		t.Fatalf("authorization query = %v", parsed.Query())
	}
	token, err := client.Exchange(context.Background(), "code", "https://app.example/callback")
	if err != nil || token.AccessToken != "user-token" {
		t.Fatalf("token = %#v, err = %v", token, err)
	}
	pages, err := client.ListPages(context.Background(), token.AccessToken)
	if err != nil || len(pages) != 1 || pages[0].AccessToken != "page-token" {
		t.Fatalf("pages = %#v, err = %v", pages, err)
	}
}
