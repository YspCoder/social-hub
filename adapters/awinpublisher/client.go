package awinpublisher

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityProgramDiscovery       socialhub.Capability = "awin_program_discovery"
	CapabilityEnhancedFeed           socialhub.Capability = "awin_enhanced_feed"
	CapabilityTrackingLink           socialhub.Capability = "awin_tracking_link"
	CapabilityTransactionAttribution socialhub.Capability = "awin_transaction_attribution"
	CapabilityPerformanceReporting   socialhub.Capability = "awin_performance_reporting"
)

// Client exposes Publisher API workflows for one Awin publisher account.
type Client struct {
	accountID   socialhub.AccountID
	publisherID int64
	api         *transport.Client
	httpClient  *http.Client
	approval    socialhub.ApprovalConfig
	clock       socialhub.Clock

	feedMu      sync.Mutex
	activeFeeds map[string]struct{}
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		CapabilityProgramDiscovery: {
			Capability: CapabilityProgramDiscovery, Supported: true, Approval: approval,
			Reason: "publisher program discovery and relationship filtering", DocURL: documentationURL,
		},
		CapabilityEnhancedFeed: {
			Capability: CapabilityEnhancedFeed, Supported: true, Approval: approval,
			Reason: "streaming retail Enhanced Feed download", DocURL: documentationURL,
		},
		CapabilityTrackingLink: {
			Capability: CapabilityTrackingLink, Supported: true, Approval: approval,
			Reason: "deep, click-reference, and optional short tracking links", DocURL: documentationURL,
		},
		CapabilityTransactionAttribution: {
			Capability: CapabilityTransactionAttribution, Supported: true, Approval: approval,
			Reason: "publisher transaction and basket attribution", DocURL: documentationURL,
		},
		CapabilityPerformanceReporting: {
			Capability: CapabilityPerformanceReporting, Supported: true, Approval: approval,
			Reason: "advertiser performance reporting by region", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Awin Publisher API is not an organic publishing product"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate reads use typed Awin Publisher workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Enhanced Feed media URLs are read-only"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Publisher API exposes no organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Publisher API is not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these Publisher API workflows are request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Awin() PublisherWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
