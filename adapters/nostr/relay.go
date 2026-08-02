package nostr

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"

	nostrgo "fiatjaf.com/nostr"

	"social-hub/pkg/socialhub"
)

type queryReport struct {
	Succeeded []string            `json:"succeeded"`
	Failed    map[string]string   `json:"failed,omitempty"`
	Sources   map[string][]string `json:"sources,omitempty"`
}

type publishReport struct {
	Quorum    int               `json:"quorum"`
	Succeeded []string          `json:"succeeded"`
	Failed    map[string]string `json:"failed,omitempty"`
}

type relayNetwork interface {
	Query(context.Context, nostrgo.Filter) ([]nostrgo.Event, queryReport, error)
	Publish(context.Context, nostrgo.Event, int) (publishReport, error)
	Close() error
}

type relaySet struct {
	urls       []string
	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc

	mu     sync.Mutex
	relays map[string]*nostrgo.Relay
	closed bool
}

func newRelayNetwork(urls []string, httpClient *http.Client) relayNetwork {
	ctx, cancel := context.WithCancel(context.Background())
	return &relaySet{
		urls: append([]string(nil), urls...), httpClient: httpClient,
		ctx: ctx, cancel: cancel, relays: make(map[string]*nostrgo.Relay),
	}
}

func (network *relaySet) Query(ctx context.Context, filter nostrgo.Filter) ([]nostrgo.Event, queryReport, error) {
	type result struct {
		url    string
		events []nostrgo.Event
		reason string
		err    error
	}
	results := make(chan result, len(network.urls))
	for _, relayURL := range network.urls {
		go func() {
			events, reason, err := network.queryRelay(ctx, relayURL, filter)
			results <- result{url: relayURL, events: events, reason: reason, err: err}
		}()
	}

	report := queryReport{Failed: make(map[string]string), Sources: make(map[string][]string)}
	byID := make(map[nostrgo.ID]nostrgo.Event)
	var failures []error
	var failureReasons []string
	for range network.urls {
		current := <-results
		if current.err != nil {
			report.Failed[current.url] = boundedMessage(current.reason, 512)
			failures = append(failures, current.err)
			failureReasons = append(failureReasons, current.reason)
			continue
		}
		report.Succeeded = append(report.Succeeded, current.url)
		for _, event := range current.events {
			if !validNIP01Event(event) {
				continue
			}
			byID[event.ID] = event
			hexID := event.ID.Hex()
			if !slices.Contains(report.Sources[hexID], current.url) {
				report.Sources[hexID] = append(report.Sources[hexID], current.url)
			}
		}
	}
	sort.Strings(report.Succeeded)
	for id := range report.Sources {
		sort.Strings(report.Sources[id])
	}
	if len(report.Succeeded) == 0 {
		return nil, report, aggregateRelayErrors("query", failureReasons, failures)
	}
	events := make([]nostrgo.Event, 0, len(byID))
	for _, event := range byID {
		events = append(events, event)
	}
	return events, report, nil
}

func (network *relaySet) queryRelay(ctx context.Context, relayURL string, filter nostrgo.Filter) ([]nostrgo.Event, string, error) {
	relay, err := network.ensureRelay(ctx, relayURL)
	if err != nil {
		reason := "error: " + boundedMessage(err.Error(), 384)
		return nil, reason, relayError("query", reason, err)
	}
	subscription, err := relay.Subscribe(ctx, filter, nostrgo.SubscriptionOptions{Label: "social-hub"})
	if err != nil {
		reason := "error: " + boundedMessage(err.Error(), 384)
		return nil, reason, relayError("query", reason, err)
	}
	defer subscription.Unsub()

	events := make([]nostrgo.Event, 0, max(filter.Limit, 1))
	for {
		select {
		case event, open := <-subscription.Events:
			if !open {
				reason := "error: subscription closed before EOSE"
				return nil, reason, relayError("query", reason, context.Cause(subscription.Context))
			}
			events = append(events, event)
		case <-subscription.EndOfStoredEvents:
			return events, "", nil
		case reason := <-subscription.ClosedReason:
			return nil, reason, relayError("query", reason, nil)
		case <-ctx.Done():
			reason := "error: " + boundedMessage(context.Cause(ctx).Error(), 384)
			return nil, reason, relayError("query", reason, context.Cause(ctx))
		case <-subscription.Context.Done():
			cause := context.Cause(subscription.Context)
			reason := "error: subscription ended"
			if cause != nil {
				reason = boundedMessage(cause.Error(), 384)
				if closedReason, found := strings.CutPrefix(reason, "CLOSED received: "); found {
					reason = closedReason
				} else {
					reason = "error: " + reason
				}
			}
			return nil, reason, relayError("query", reason, cause)
		}
	}
}

func (network *relaySet) Publish(ctx context.Context, event nostrgo.Event, quorum int) (publishReport, error) {
	type result struct {
		url    string
		reason string
		err    error
	}
	results := make(chan result, len(network.urls))
	for _, relayURL := range network.urls {
		go func() {
			relay, err := network.ensureRelay(ctx, relayURL)
			if err == nil {
				err = relay.Publish(ctx, event)
			}
			reason := ""
			if err != nil {
				reason = boundedMessage(err.Error(), 512)
			}
			results <- result{url: relayURL, reason: reason, err: err}
		}()
	}

	report := publishReport{Quorum: quorum, Failed: make(map[string]string)}
	var failures []error
	var failureReasons []string
	for range network.urls {
		current := <-results
		if current.err == nil {
			report.Succeeded = append(report.Succeeded, current.url)
			continue
		}
		report.Failed[current.url] = current.reason
		failures = append(failures, current.err)
		failureReasons = append(failureReasons, current.reason)
	}
	sort.Strings(report.Succeeded)
	if len(report.Succeeded) < quorum {
		return report, aggregateRelayErrors("publish", failureReasons, failures)
	}
	return report, nil
}

func (network *relaySet) ensureRelay(ctx context.Context, relayURL string) (*nostrgo.Relay, error) {
	network.mu.Lock()
	if network.closed {
		network.mu.Unlock()
		return nil, errors.New("relay network is closed")
	}
	if relay := network.relays[relayURL]; relay != nil && relay.IsConnected() {
		network.mu.Unlock()
		return relay, nil
	}
	network.mu.Unlock()

	// The upstream optimized verifier is not checkptr-safe under -race. Events
	// are verified with validNIP01Event before they leave this network layer.
	relay := nostrgo.NewRelay(network.ctx, relayURL, nostrgo.RelayOptions{AssumeValid: true})
	if err := relay.ConnectWithClient(ctx, network.httpClient); err != nil {
		_ = relay.Close()
		return nil, err
	}
	network.mu.Lock()
	defer network.mu.Unlock()
	if network.closed {
		_ = relay.Close()
		return nil, errors.New("relay network is closed")
	}
	if existing := network.relays[relayURL]; existing != nil && existing.IsConnected() {
		_ = relay.Close()
		return existing, nil
	}
	if existing := network.relays[relayURL]; existing != nil {
		_ = existing.Close()
	}
	network.relays[relayURL] = relay
	return relay, nil
}

func (network *relaySet) Close() error {
	network.mu.Lock()
	if network.closed {
		network.mu.Unlock()
		return nil
	}
	network.closed = true
	network.cancel()
	relays := make([]*nostrgo.Relay, 0, len(network.relays))
	for _, relay := range network.relays {
		relays = append(relays, relay)
	}
	network.relays = nil
	network.mu.Unlock()

	errorsFound := make([]error, 0, len(relays))
	for _, relay := range relays {
		if err := relay.Close(); err != nil {
			errorsFound = append(errorsFound, err)
		}
	}
	return errors.Join(errorsFound...)
}

func aggregateRelayErrors(operation string, reasons []string, failures []error) error {
	if len(failures) == 1 {
		var platform *socialhub.Error
		if errors.As(failures[0], &platform) {
			platform.Op = operation
			return platform
		}
	}
	code, class := socialhub.CodePlatformError, socialhub.ClassRetryable
	platformCode := "error"
	for _, reason := range reasons {
		candidate, _ := relayReason(reason)
		candidateCode, candidateClass := classifyRelayCode(candidate)
		if candidateCode == socialhub.CodeRateLimited || code == socialhub.CodePlatformError {
			platformCode, code, class = candidate, candidateCode, candidateClass
		}
		if candidateCode == socialhub.CodeRateLimited {
			break
		}
	}
	return &socialhub.Error{
		Code: code, Class: class, Op: operation, Platform: "nostr", Product: productName,
		PlatformCode: platformCode, PlatformMessage: "relay quorum was not satisfied", Cause: errors.Join(failures...),
	}
}

func relayHint(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func relayTag(name, identifier, hint, marker, author string) nostrgo.Tag {
	tag := nostrgo.Tag{name, identifier}
	if hint != "" || marker != "" || author != "" {
		tag = append(tag, hint)
	}
	if marker != "" || author != "" {
		tag = append(tag, marker)
	}
	if author != "" {
		tag = append(tag, author)
	}
	return tag
}
