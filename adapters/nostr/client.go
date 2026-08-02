package nostr

import (
	"context"
	"sync"
	"time"

	nostrgo "fiatjaf.com/nostr"

	"social-hub/pkg/socialhub"
)

const (
	CapabilityRelayQuery socialhub.Capability = "nostr_relay_query"
	CapabilityNIP19      socialhub.Capability = "nostr_nip19_identifiers"
	CapabilityNIP92      socialhub.Capability = "nostr_nip92_media_metadata"
)

// Client operates as one Nostr public key over a configured relay set.
type Client struct {
	accountID   socialhub.AccountID
	publicKey   nostrgo.PubKey
	secretKey   *nostrgo.SecretKey
	relays      []string
	writeQuorum int
	network     relayNetwork
	clock       socialhub.Clock

	closeOnce sync.Once
	closeErr  error
}

func (client *Client) Platform() socialhub.Platform { return "nostr" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	canWrite := client.secretKey != nil
	writeReason := "NIP-01 events signed by the configured private key"
	if !canWrite {
		writeReason = "read-only account: configure access_token_ref with nsec or a hex private key"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: {
			Capability: socialhub.CapPublish, Supported: canWrite, Approval: socialhub.ApprovalGranted,
			Reason: writeReason, DocURL: documentationURL,
		},
		socialhub.CapFetch: {
			Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "NIP-01 profile metadata and kind 1 text-note queries", DocURL: documentationURL,
		},
		socialhub.CapMedia: {
			Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Nostr core has no uniform binary upload protocol; remote NIP-92 metadata is mapped during fetch", DocURL: documentationURL,
		},
		socialhub.CapReact: {
			Capability: socialhub.CapReact, Supported: canWrite, Approval: socialhub.ApprovalGranted,
			Reason: writeReason + "; NIP-25 reactions and NIP-10 replies", DocURL: "https://github.com/nostr-protocol/nips/blob/master/25.md",
		},
		socialhub.CapMessage: {
			Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "NIP-17 private messaging requires gift wrapping and recipient relay discovery outside the NIP-01 adapter contract", DocURL: documentationURL,
		},
		socialhub.CapWebhook: {
			Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Nostr delivers events through WebSocket subscriptions rather than signed HTTP callbacks", DocURL: documentationURL,
		},
		CapabilityRelayQuery: {
			Capability: CapabilityRelayQuery, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "multi-relay NIP-01 REQ/EVENT/EOSE queries with event de-duplication", DocURL: documentationURL,
		},
		CapabilityNIP19: {
			Capability: CapabilityNIP19, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "hex, npub, nprofile, note, nevent, and nsec input normalization", DocURL: "https://github.com/nostr-protocol/nips/blob/master/19.md",
		},
		CapabilityNIP92: {
			Capability: CapabilityNIP92, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "NIP-92 imeta fields are mapped for remote media referenced by kind 1 notes", DocURL: "https://github.com/nostr-protocol/nips/blob/master/92.md",
		},
		CapabilityNIP25: {
			Capability: CapabilityNIP25, Supported: canWrite, Approval: socialhub.ApprovalGranted,
			Reason: writeReason + "; typed like, dislike, and emoji reaction publication", DocURL: "https://github.com/nostr-protocol/nips/blob/master/25.md",
		},
		CapabilityReposts: {
			Capability: CapabilityReposts, Supported: canWrite, Approval: socialhub.ApprovalGranted,
			Reason: writeReason + "; typed NIP-18 kind 6 repost publication", DocURL: "https://github.com/nostr-protocol/nips/blob/master/18.md",
		},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool) {
	if client.secretKey == nil {
		return nil, false
	}
	return client, true
}

func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return client, true }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }

func (client *Client) Reactor() (socialhub.Reactor, bool) {
	if client.secretKey == nil {
		return nil, false
	}
	return client, true
}

func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }

func (client *Client) Close() error {
	client.closeOnce.Do(func() { client.closeErr = client.network.Close() })
	return client.closeErr
}

func (client *Client) callContext(ctx context.Context, operation string, options ...socialhub.CallOption) (context.Context, context.CancelFunc, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return nil, nil, unsupported(operation, "NIP-01 has no relay-wide idempotency-key contract")
	}
	if resolved.Timeout > 0 {
		callCtx, cancel := context.WithTimeout(ctx, resolved.Timeout)
		return callCtx, cancel, nil
	}
	return ctx, func() {}, nil
}

func (client *Client) signEvent(event *nostrgo.Event) error {
	if client.secretKey == nil {
		return platformError("sign", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}
	if event.CreatedAt == 0 {
		event.CreatedAt = nostrgo.Timestamp(client.clock.Now().Unix())
	}
	if err := signNIP01Event(event, *client.secretKey); err != nil {
		return platformError("sign", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func eventTime(timestamp nostrgo.Timestamp) *time.Time {
	value := time.Unix(int64(timestamp), 0).UTC()
	return &value
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
