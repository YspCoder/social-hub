package mailchimp

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAudienceMetadata socialhub.Capability = "mailchimp_audience_metadata"
	CapabilityListMetadata     socialhub.Capability = "mailchimp_list_metadata"
	CapabilityCampaignRead     socialhub.Capability = "mailchimp_campaign_read"
	CapabilityReportRead       socialhub.Capability = "mailchimp_campaign_report_read"
)

// Client exposes privacy-bounded Marketing API reads for one API-key account.
type Client struct {
	accountID     socialhub.AccountID
	api           *transport.Client
	apiKey        string
	authorization string
	approval      socialhub.ApprovalConfig
	clock         socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if client.approval.AccountType == "api_key" {
		approval = socialhub.ApprovalGranted
	}
	readState := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityAudienceMetadata: readState(CapabilityAudienceMetadata, "non-PII audience metadata and total contact counts", documentationURL),
		CapabilityListMetadata:     readState(CapabilityListMetadata, "non-PII legacy Lists metadata and aggregate statistics", documentationURL),
		CapabilityCampaignRead:     readState(CapabilityCampaignRead, "campaign metadata without content, member, or reply-to data", documentationURL),
		CapabilityReportRead:       readState(CapabilityReportRead, "aggregate campaign reports without member activity or share credentials", documentationURL),
		socialhub.CapPublish:       {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "campaign creation, scheduling, sending, and actions are outside this read-only adapter"},
		socialhub.CapFetch:         {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Mailchimp resources retain provider semantics through the typed Marketing workflow"},
		socialhub.CapMedia:         {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "file-manager and campaign-content APIs are outside this metadata surface"},
		socialhub.CapReact:         {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Mailchimp Marketing API is not an organic reaction product"},
		socialhub.CapMessage:       {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "transactional and campaign sending are deliberately excluded"},
		socialhub.CapWebhook:       {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "audience webhooks require a separate write and verification contract"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Marketing returns the bounded provider-native read workflow.
func (client *Client) Marketing() ReadWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
