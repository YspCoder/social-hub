package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testAccountID  = "AccountStable123"
	testArtistID   = "2takcwOaAZWiXQijPHIx7B"
	testAlbumID    = "4aawyAB9vmqN3uQ7FjRGTy"
	testTrackID    = "4iV5W9uYEdYUVa79Axb7Rh"
	testEpisodeID  = "512ojhOuo1ktJprKbVcKyQ"
	testPlaylistID = "3cEYpjA9oz9GiPac4AsH4n"
)

var allTestScopes = []string{
	ScopeUserReadPrivate, ScopeUserReadEmail, ScopeUserLibraryRead, ScopeUserLibraryModify,
	ScopeUserFollowRead, ScopeUserFollowModify, ScopePlaylistReadPrivate, ScopePlaylistReadCollaborative,
	ScopePlaylistModifyPrivate, ScopePlaylistModifyPublic, ScopeUserReadPlaybackState,
	ScopeUserReadCurrentlyPlaying, ScopeUserModifyPlaybackState,
}

type mapResolver map[string]string

func (r mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

var testNow = time.Date(2026, time.August, 2, 1, 2, 3, 0, time.UTC)

func newTestAdapter(t *testing.T, server *httptest.Server, accountType string, scopes []string) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": server.URL + "/v1", "auth_url": server.URL + "/authorize", "token_url": server.URL + "/api/token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "listener", ClientID: "client-id", SecretRef: "test://secret", AccessTokenRef: "test://token",
			Approval: socialhub.ApprovalConfig{AccountType: accountType, Scopes: scopes},
			Settings: map[string]any{"account_id": testAccountID},
		}},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://secret": "client-secret", "test://token": "access-token"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "listener")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationMetadataAndCapabilities(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, "premium", allTestScopes)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != docURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{
		CapabilityProfile, CapabilityCatalog, CapabilityLibraryRead, CapabilityLibraryModify,
		CapabilityPlaylistRead, CapabilityPlaylistModify, CapabilityPlaybackRead, CapabilityPlaybackControl,
	} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook,
	} {
		if capabilities.Has(capability) {
			t.Fatalf("common capability %q must be unsupported", capability)
		}
	}
	if client.Platform() != "spotify" || client.Account() != "listener" || client.ProfileWorkflow() == nil ||
		client.CatalogWorkflow() == nil || client.LibraryWorkflow() == nil || client.PlaylistWorkflow() == nil || client.PlaybackWorkflow() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must not be exposed")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher must not be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader must not be exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor must not be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must not be exposed")
	}
	if client.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), "listener"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "listener"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("OAuth after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), socialhub.AdapterConfig{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("init after close with invalid config error=%v", err)
	}
}

func TestAdapterValidationAndApprovalGates(t *testing.T) {
	validAccount := socialhub.AccountConfig{ID: "listener", AccessTokenRef: "test://token"}
	tests := []struct {
		name   string
		config socialhub.AdapterConfig
	}{
		{"wrong adapter", socialhub.AdapterConfig{Adapter: "other", Accounts: []socialhub.AccountConfig{validAccount}}},
		{"bad endpoint", socialhub.AdapterConfig{Adapter: adapterName, Settings: map[string]any{"base_url": "ftp://api.example"}, Accounts: []socialhub.AccountConfig{validAccount}}},
		{"missing token", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "listener"}}}},
		{"duplicate scope", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "listener", AccessTokenRef: "test://token", Approval: socialhub.ApprovalConfig{Scopes: []string{"scope", "scope"}}}}}},
		{"invalid scope", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "listener", AccessTokenRef: "test://token", Approval: socialhub.ApprovalConfig{Scopes: []string{"Bad Scope"}}}}}},
		{"bad account ID", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "listener", AccessTokenRef: "test://token", Settings: map[string]any{"account_id": "bad-id"}}}}},
		{"unknown setting", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "listener", AccessTokenRef: "test://token", Settings: map[string]any{"unknown": true}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := (&Adapter{}).Init(context.Background(), test.config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, "free", []string{ScopeUserLibraryRead})
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://secret": "client-secret"}
	if _, err := adapter.Client(context.Background(), "listener"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token error=%v", err)
	}
	adapter.config.Accounts[0].ClientID = ""
	if _, err := adapter.OAuth(context.Background(), "listener"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth client ID error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth account error=%v", err)
	}

	_, limited := newTestAdapter(t, server, "free", []string{ScopeUserLibraryRead})
	capabilities, _ := limited.Capabilities(context.Background())
	if capabilities.Has(CapabilityPlaybackControl) || capabilities[CapabilityPlaybackControl].Approval != socialhub.ApprovalRequired {
		t.Fatalf("playback control=%#v", capabilities[CapabilityPlaybackControl])
	}
	if err := limited.PausePlayback(context.Background(), ""); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope gate error=%v", err)
	}
	limited.scopes = []string{ScopeUserModifyPlaybackState}
	if err := limited.PausePlayback(context.Background(), ""); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("Premium gate error=%v", err)
	}
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(value))
}

func artistJSON() string {
	return fmt.Sprintf(`{"type":"artist","id":"%s","name":"Artist","uri":"spotify:artist:%s","external_urls":{"spotify":"https://open.spotify.com/artist/%s"}}`, testArtistID, testArtistID, testArtistID)
}

func albumJSON() string {
	return fmt.Sprintf(`{"type":"album","id":"%s","name":"Album","uri":"spotify:album:%s","album_type":"album","release_date":"2026-08-01","release_date_precision":"day","total_tracks":10,"images":[{"url":"https://cdn.example/album.jpg","height":640,"width":640}],"artists":[%s],"external_urls":{"spotify":"https://open.spotify.com/album/%s"}}`, testAlbumID, testAlbumID, artistJSON(), testAlbumID)
}

func trackJSON(id string) string {
	return fmt.Sprintf(`{"type":"track","id":"%s","name":"Track","uri":"spotify:track:%s","duration_ms":61000,"explicit":false,"disc_number":1,"track_number":2,"is_playable":true,"external_urls":{"spotify":"https://open.spotify.com/track/%s"},"external_ids":{"isrc":"TEST123"},"album":%s,"artists":[%s]}`, id, id, id, albumJSON(), artistJSON())
}

func episodeJSON() string {
	return fmt.Sprintf(`{"type":"episode","id":"%s","name":"Episode","uri":"spotify:episode:%s","description":"Podcast","duration_ms":120000,"explicit":false,"is_externally_hosted":false,"is_playable":true,"release_date":"2026-08-01","release_date_precision":"day","languages":["en"],"images":[{"url":"https://cdn.example/episode.jpg"}],"external_urls":{"spotify":"https://open.spotify.com/episode/%s"}}`, testEpisodeID, testEpisodeID, testEpisodeID)
}

func playlistJSON() string {
	return fmt.Sprintf(`{"type":"playlist","id":"%s","name":"Mix","uri":"spotify:playlist:%s","description":"Description","collaborative":false,"public":false,"snapshot_id":"snapshot-1","images":[{"url":"https://cdn.example/playlist.jpg"}],"external_urls":{"spotify":"https://open.spotify.com/playlist/%s"},"owner":{"account_id":"%s","id":"legacy-user"},"items":{"total":2,"items":[]}}`, testPlaylistID, testPlaylistID, testPlaylistID, testAccountID)
}

func errorCode(err error) socialhub.ErrorCode {
	var hubError *socialhub.Error
	if errors.As(err, &hubError) {
		return hubError.Code
	}
	return ""
}
