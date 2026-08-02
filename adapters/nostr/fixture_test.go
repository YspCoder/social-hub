package nostr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	nostrgo "fiatjaf.com/nostr"
	"github.com/coder/websocket"

	"social-hub/pkg/socialhub"
)

type relayFixture struct {
	t      *testing.T
	server *httptest.Server
	url    string

	mu                sync.Mutex
	events            map[string]nostrgo.Event
	published         []nostrgo.Event
	commands          []string
	rejectPublication string
	closeQuery        string
}

func newRelayFixture(t *testing.T, events ...nostrgo.Event) *relayFixture {
	t.Helper()
	fixture := &relayFixture{t: t, events: make(map[string]nostrgo.Event)}
	for _, event := range events {
		fixture.events[event.ID.Hex()] = event
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	fixture.url = "ws" + strings.TrimPrefix(fixture.server.URL, "http")
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (fixture *relayFixture) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	connection, err := websocket.Accept(writer, request, nil)
	if err != nil {
		fixture.t.Errorf("accept websocket: %v", err)
		return
	}
	defer connection.Close(websocket.StatusNormalClosure, "")
	for {
		_, payload, readErr := connection.Read(request.Context())
		if readErr != nil {
			return
		}
		var envelope []json.RawMessage
		if err := json.Unmarshal(payload, &envelope); err != nil || len(envelope) == 0 {
			fixture.t.Errorf("decode NIP-01 envelope %s: %v", payload, err)
			return
		}
		var command string
		if err := json.Unmarshal(envelope[0], &command); err != nil {
			fixture.t.Errorf("decode NIP-01 command: %v", err)
			return
		}
		fixture.mu.Lock()
		fixture.commands = append(fixture.commands, command)
		fixture.mu.Unlock()
		switch command {
		case "REQ":
			fixture.handleREQ(request.Context(), connection, envelope)
		case "EVENT":
			fixture.handleEVENT(request.Context(), connection, envelope)
		case "CLOSE":
			continue
		default:
			fixture.t.Errorf("unexpected command %q", command)
			return
		}
	}
}

func (fixture *relayFixture) handleREQ(ctx context.Context, connection *websocket.Conn, envelope []json.RawMessage) {
	if len(envelope) < 3 {
		fixture.t.Errorf("REQ has %d fields", len(envelope))
		return
	}
	var subscriptionID string
	var filter nostrgo.Filter
	if err := json.Unmarshal(envelope[1], &subscriptionID); err != nil {
		fixture.t.Errorf("decode subscription ID: %v", err)
		return
	}
	if err := json.Unmarshal(envelope[2], &filter); err != nil {
		fixture.t.Errorf("decode filter: %v", err)
		return
	}
	fixture.mu.Lock()
	closeReason := fixture.closeQuery
	events := make([]nostrgo.Event, 0, len(fixture.events))
	for _, event := range fixture.events {
		if filter.Matches(event) {
			events = append(events, event)
		}
	}
	fixture.mu.Unlock()
	if closeReason != "" {
		fixture.write(ctx, connection, []any{"CLOSED", subscriptionID, closeReason})
		return
	}
	sortEvents(events)
	if filter.Limit > 0 && len(events) > filter.Limit {
		events = events[:filter.Limit]
	}
	for _, event := range events {
		fixture.write(ctx, connection, []any{"EVENT", subscriptionID, event})
	}
	fixture.write(ctx, connection, []any{"EOSE", subscriptionID})
}

func (fixture *relayFixture) handleEVENT(ctx context.Context, connection *websocket.Conn, envelope []json.RawMessage) {
	if len(envelope) != 2 {
		fixture.t.Errorf("EVENT has %d fields", len(envelope))
		return
	}
	var event nostrgo.Event
	if err := json.Unmarshal(envelope[1], &event); err != nil {
		fixture.t.Errorf("decode event: %v", err)
		return
	}
	fixture.mu.Lock()
	fixture.published = append(fixture.published, event)
	reason := fixture.rejectPublication
	if reason == "" {
		fixture.events[event.ID.Hex()] = event
	}
	fixture.mu.Unlock()
	fixture.write(ctx, connection, []any{"OK", event.ID.Hex(), reason == "", reason})
}

func (fixture *relayFixture) write(ctx context.Context, connection *websocket.Conn, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		fixture.t.Errorf("marshal relay response: %v", err)
		return
	}
	if err := connection.Write(ctx, websocket.MessageText, payload); err != nil {
		fixture.t.Errorf("write relay response: %v", err)
	}
}

func (fixture *relayFixture) publications() []nostrgo.Event {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return append([]nostrgo.Event(nil), fixture.published...)
}

func (fixture *relayFixture) sawCommand(command string) bool {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	for _, current := range fixture.commands {
		if current == command {
			return true
		}
	}
	return false
}

func signedEvent(t *testing.T, secret nostrgo.SecretKey, timestamp int64, kind nostrgo.Kind, tags nostrgo.Tags, content string) nostrgo.Event {
	t.Helper()
	event := nostrgo.Event{CreatedAt: nostrgo.Timestamp(timestamp), Kind: kind, Tags: tags, Content: content}
	if err := signNIP01Event(&event, secret); err != nil {
		t.Fatalf("sign fixture event: %v", err)
	}
	if !validNIP01Event(event) {
		t.Fatal("signed fixture event did not verify")
	}
	return event
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

type secretMap map[string]string

func (secrets secretMap) Resolve(_ context.Context, reference string) (string, error) {
	value, found := secrets[reference]
	if !found {
		return "", socialhub.ErrNotFound
	}
	return value, nil
}

func openTestClient(t *testing.T, relayURLs []string, publicKey, credential string, quorum int) (*Adapter, *Client) {
	t.Helper()
	account := socialhub.AccountConfig{
		ID: "primary", Settings: map[string]any{
			"relay_urls": relayURLs, "write_quorum": quorum,
		},
	}
	if publicKey != "" {
		account.Settings["public_key"] = publicKey
	}
	options := []socialhub.Option{socialhub.WithClock(fixedClock{now: time.Unix(1_800_000_000, 0).UTC()})}
	if credential != "" {
		account.AccessTokenRef = "secret://nostr"
		options = append(options, socialhub.WithSecretResolver(secretMap{"secret://nostr": credential}))
	}
	adapter := &Adapter{}
	config := socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{account}}
	if err := adapter.Init(context.Background(), config, options...); err != nil {
		t.Fatalf("init adapter: %v", err)
	}
	common, err := adapter.Client(context.Background(), "primary")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client := common.(*Client)
	t.Cleanup(func() { _ = client.Close(); _ = adapter.Close() })
	return adapter, client
}
