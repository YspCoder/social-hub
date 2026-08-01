package giphy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestDiscoveryAndAnalyticsWorkflows(t *testing.T) {
	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/v1/") {
			if request.URL.Query().Get("api_key") != "api-key" {
				t.Errorf("api_key=%q", request.URL.Query().Get("api_key"))
				writeMetaError(writer, http.StatusUnauthorized, "missing key")
				return
			}
		}
		gif := gifFixture(serverURL, "gif1", "gif")
		switch request.URL.Path {
		case "/v1/gifs/search":
			query := request.URL.Query()
			if query.Get("q") != "cats & dogs" || query.Get("limit") != "2" || query.Get("offset") != "3" || query.Get("lang") != "en" || query.Get("channel_ids") != "5,6" || query.Get("rating") != "pg" || query.Get("customer_id") != "customer" || query.Get("bundle") != "messaging_non_clips" || query.Get("country_code") != "US" || query.Get("region") != "VA" || query.Get("remove_low_contrast") != "true" {
				t.Errorf("search query=%v", query)
			}
			writeList(writer, `[`+gif+`]`, 3, 10, 1)
		case "/v1/stickers/trending":
			writeList(writer, `[`+gifFixture(serverURL, "sticker1", "sticker")+`]`, 0, 1, 1)
		case "/v1/gifs/translate":
			if request.URL.Query().Get("s") != "birthday" {
				t.Errorf("translate query=%v", request.URL.Query())
			}
			writeSingle(writer, gif)
		case "/v1/stickers/random":
			if request.URL.Query().Get("tag") != "party" {
				t.Errorf("random query=%v", request.URL.Query())
			}
			writeSingle(writer, gifFixture(serverURL, "random1", "sticker"))
		case "/v1/gifs/gif1":
			writeSingle(writer, gif)
		case "/v1/gifs":
			if request.URL.Query().Get("ids") != "gif1,gif2" {
				t.Errorf("ids=%q", request.URL.Query().Get("ids"))
			}
			writeList(writer, `[`+gif+`,`+gifFixture(serverURL, "gif2", "gif")+`]`, 0, 2, 2)
		case "/v1/randomid":
			writeSingle(writer, `{"random_id":"random-customer"}`)
		case "/v1/gifs/categories":
			writeList(writer, `[{"name":"reactions","name_encoded":"reactions","subcategories":[{"name":"happy","name_encoded":"happy"}]}]`, 0, 1, 1)
		case "/v1/gifs/search/tags":
			writeSingle(writer, `[{"name":"cat"},{"name":"cats"}]`)
		case "/v1/tags/related/haha":
			writeSingle(writer, `[{"name":"lol"}]`)
		case "/v1/trending/searches":
			writeSingle(writer, `["happy","wow"]`)
		case "/v2/pingback_simple":
			query := request.URL.Query()
			if query.Get("api_key") != "" || query.Get("analytics_response_payload") != "payload" || query.Get("action_type") != "SEEN" || query.Get("customer_id") != "customer" || query.Get("ts") != strconv.FormatInt(testNow.UnixMilli(), 10) {
				t.Errorf("analytics query=%v", query)
			}
			writer.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request %s", request.URL.String())
			http.NotFound(writer, request)
		}
	}))
	serverURL = server.URL
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()
	removeLowContrast := true
	search, err := client.Search(ctx, SearchRequest{
		Content: ContentGIF, Query: "cats & dogs", Limit: 2, Offset: 3, Language: "en", ChannelIDs: []int64{5, 6},
		CommonOptions: CommonOptions{CustomerID: "customer", Rating: RatingPG, CountryCode: "US", Region: "VA", Bundle: "messaging_non_clips", RemoveLowContrast: &removeLowContrast},
	}, socialhub.WithRequestID("search-request"))
	if err != nil || len(search.Items) != 1 || search.Items[0].ID != "gif1" || search.Pagination.Offset != 3 || search.ResponseID != "response-id" || search.Items[0].Images["original"].Width != 640 {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	if search.Items[0].TrackingURL(AnalyticsView) == "" || search.Items[0].TrackingURL(AnalyticsClick) == "" || search.Items[0].TrackingURL(AnalyticsSend) == "" || search.Items[0].TrackingURL("unknown") != "" {
		t.Fatal("analytics URL selection failed")
	}
	trending, err := client.Trending(ctx, TrendingRequest{Content: ContentSticker})
	if err != nil || len(trending.Items) != 1 || trending.Items[0].Type != "sticker" {
		t.Fatalf("trending=%#v err=%v", trending, err)
	}
	translated, err := client.Translate(ctx, TranslateRequest{Content: ContentGIF, Query: "birthday"})
	if err != nil || translated.ID != "gif1" {
		t.Fatalf("translated=%#v err=%v", translated, err)
	}
	random, err := client.Random(ctx, RandomRequest{Content: ContentSticker, Tag: "party"})
	if err != nil || random.ID != "random1" {
		t.Fatalf("random=%#v err=%v", random, err)
	}
	got, err := client.Get(ctx, GetRequest{ID: "gif1"})
	if err != nil || got.ID != "gif1" || got.User == nil || got.User.Username != "artist" {
		t.Fatalf("get=%#v err=%v", got, err)
	}
	many, err := client.GetMany(ctx, GetManyRequest{IDs: []string{"gif1", "gif2"}})
	if err != nil || len(many.Items) != 2 {
		t.Fatalf("many=%#v err=%v", many, err)
	}
	randomID, err := client.RandomID(ctx)
	if err != nil || randomID != "random-customer" {
		t.Fatalf("random ID=%q err=%v", randomID, err)
	}
	categories, err := client.Categories(ctx, "customer")
	if err != nil || len(categories.Items) != 1 || len(categories.Items[0].Subcategories) != 1 {
		t.Fatalf("categories=%#v err=%v", categories, err)
	}
	autocomplete, err := client.Autocomplete(ctx, TermRequest{Query: "ca", Limit: 2, CustomerID: "customer"})
	if err != nil || len(autocomplete) != 2 || autocomplete[0].Name != "cat" {
		t.Fatalf("autocomplete=%#v err=%v", autocomplete, err)
	}
	related, err := client.Related(ctx, "haha", "customer")
	if err != nil || len(related) != 1 || related[0].Name != "lol" {
		t.Fatalf("related=%#v err=%v", related, err)
	}
	searches, err := client.TrendingSearches(ctx, "customer")
	if err != nil || len(searches) != 2 || searches[1] != "wow" {
		t.Fatalf("searches=%#v err=%v", searches, err)
	}
	if err := client.Register(ctx, AnalyticsRequest{TrackingURL: got.TrackingURL(AnalyticsView), CustomerID: "customer", Timestamp: testNow}); err != nil {
		t.Fatal(err)
	}
}

func gifFixture(baseURL, identifier, contentType string) string {
	return fmt.Sprintf(`{"type":%q,"id":%q,"slug":"slug","url":%q,"username":"artist","rating":"g","title":"Title","alt_text":"Accessible title","user":{"username":"artist","display_name":"Artist","profile_url":"https://giphy.com/artist","avatar_url":"https://media.giphy.com/avatar.gif","is_verified":true},"images":{"original":{"url":"https://media.giphy.com/media/original.gif","width":"640","height":480,"size":"1234","mp4":"https://media.giphy.com/media/original.mp4","mp4_size":"1000"}},"analytics":{"onload":{"url":%q},"onclick":{"url":%q},"onsent":{"url":%q}}}`,
		contentType, identifier, "https://giphy.com/gifs/"+identifier,
		baseURL+"/v2/pingback_simple?analytics_response_payload=payload&action_type=SEEN",
		baseURL+"/v2/pingback_simple?analytics_response_payload=payload&action_type=CLICK",
		baseURL+"/v2/pingback_simple?analytics_response_payload=payload&action_type=SENT",
	)
}

func writeSingle(writer http.ResponseWriter, data string) {
	writeJSON(writer, http.StatusOK, `{"data":`+data+`,"meta":{"status":200,"msg":"OK","response_id":"response-id"}}`)
}

func writeList(writer http.ResponseWriter, data string, offset, total, count int) {
	writeJSON(writer, http.StatusOK, fmt.Sprintf(`{"data":%s,"pagination":{"offset":%d,"total_count":%d,"count":%d},"meta":{"status":200,"msg":"OK","response_id":"response-id"}}`, data, offset, total, count))
}

func writeMetaError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, fmt.Sprintf(`{"data":[],"meta":{"status":%d,"msg":%q,"response_id":"error-id"}}`, status, message))
}
