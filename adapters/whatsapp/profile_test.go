package whatsapp

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

func TestBusinessProfileReadAndUpdate(t *testing.T) {
	var getCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requireBearer(t, request)
		if request.URL.Path != "/123456789/whatsapp_business_profile" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		switch request.Method {
		case http.MethodGet:
			if request.URL.Query().Get("fields") != profileFields {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			getCalls++
			if getCalls == 1 {
				writeTestJSON(t, writer, map[string]any{"data": []map[string]any{{
					"messaging_product": "whatsapp", "about": "About", "address": "Address", "description": "Description",
					"email": "hello@example.test", "profile_picture_url": "https://example.test/profile.jpg",
					"websites": []string{"https://example.test"}, "vertical": "PROF_SERVICES",
				}}})
				return
			}
			writeTestJSON(t, writer, map[string]any{"data": []map[string]any{{
				"business_profile": map[string]any{"about": "Nested", "vertical": "OTHER"},
			}}})
		case http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode update: %v", err)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			websites, _ := body["websites"].([]any)
			if body["messaging_product"] != "whatsapp" || body["about"] != "Updated" || body["vertical"] != "RETAIL" || len(websites) != 1 || websites[0] != "https://shop.example.test" {
				t.Errorf("update body=%#v", body)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]bool{"success": true})
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)
	flat, err := client.GetBusinessProfile(context.Background())
	if err != nil || flat.About != "About" || flat.Email != "hello@example.test" || flat.Vertical != "PROF_SERVICES" || len(flat.Websites) != 1 {
		t.Fatalf("flat=%#v error=%v", flat, err)
	}
	nested, err := client.GetBusinessProfile(context.Background())
	if err != nil || nested.About != "Nested" || nested.Vertical != "OTHER" {
		t.Fatalf("nested=%#v error=%v", nested, err)
	}
	about := "Updated"
	vertical := "retail"
	websites := []string{"https://shop.example.test"}
	if err := client.UpdateBusinessProfile(context.Background(), BusinessProfileUpdate{About: &about, Vertical: &vertical, Websites: &websites}); err != nil {
		t.Fatal(err)
	}
}

func TestBusinessProfileValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)
	empty := ""
	long := strings.Repeat("x", 257)
	longEmail := strings.Repeat("x", 129)
	badVertical := "SPACE"
	tooManyWebsites := []string{"https://one.test", "https://two.test", "https://three.test"}
	badWebsites := []string{"ftp://example.test"}
	cases := []BusinessProfileUpdate{
		{},
		{About: &empty},
		{Address: &long},
		{Description: &long},
		{Email: &longEmail},
		{Websites: &tooManyWebsites},
		{Websites: &badWebsites},
		{Vertical: &badVertical},
	}
	for _, input := range cases {
		if err := client.UpdateBusinessProfile(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
	if !validWebURL("https://example.test") || validWebURL("https://user:secret@example.test") || !validVertical("finance") || validVertical("space") || !lengthOutside("", 0, 10) {
		t.Fatal("profile validation helpers mismatch")
	}
}

func TestBusinessProfileRejectsMalformedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			writeTestJSON(t, writer, map[string]any{"data": []any{}})
			return
		}
		writeTestJSON(t, writer, map[string]bool{"success": false})
	}))
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)
	if _, err := client.GetBusinessProfile(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("malformed get error=%v", err)
	}
	about := "Updated"
	if err := client.UpdateBusinessProfile(context.Background(), BusinessProfileUpdate{About: &about}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("false success error=%v", err)
	}
}
