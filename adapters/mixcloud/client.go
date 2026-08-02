package mixcloud

import (
	"context"
	"net/url"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityIdentity   socialhub.Capability = "mixcloud_identity"
	CapabilityCloudcasts socialhub.Capability = "mixcloud_cloudcasts"
	CapabilityDiscovery  socialhub.Capability = "mixcloud_discovery"
	CapabilityUpload     socialhub.Capability = "mixcloud_upload"
	CapabilityLibrary    socialhub.Capability = "mixcloud_library"
	CapabilityProUpload  socialhub.Capability = "mixcloud_pro_upload_options"
)

// Client exposes normalized reads/reactions and Mixcloud-specific workflows.
type Client struct {
	accountID   socialhub.AccountID
	username    string
	accountType string
	api         *transport.Client
	apiBaseURL  *url.URL
	clock       socialhub.Clock
	userAgent   string
}

func (client *Client) Platform() socialhub.Platform { return "mixcloud" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	proApproval := socialhub.ApprovalUnknown
	if client.accountType != "" {
		proApproval = socialhub.ApprovalRequired
		if strings.EqualFold(client.accountType, "pro") {
			proApproval = socialhub.ApprovalGranted
		}
	}
	return socialhub.Capabilities{
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "users, Cloudcasts, user uploads, and Cloudcast comments", DocURL: documentationURL},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "Favourite and Repost toggling; comment creation and deletion are not documented", DocURL: documentationURL},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "a Cloudcast is created together with its MP3 bytes; use UploadWorkflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Mixcloud has no independent or resumable media resource; use UploadWorkflow"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Mixcloud API does not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Mixcloud API does not expose signed webhooks"},
		CapabilityIdentity:   {Capability: CapabilityIdentity, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "authorized-user identity and public profiles", DocURL: documentationURL},
		CapabilityCloudcasts: {Capability: CapabilityCloudcasts, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "Cloudcast detail, user uploads, and comments", DocURL: documentationURL},
		CapabilityDiscovery:  {Capability: CapabilityDiscovery, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "search across Cloudcasts, users, and tags", DocURL: documentationURL},
		CapabilityUpload:     {Capability: CapabilityUpload, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "streaming single-request MP3 upload and metadata editing", DocURL: documentationURL},
		CapabilityLibrary:    {Capability: CapabilityLibrary, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "Favourite, Repost, Listen Later, and Follow toggles", DocURL: documentationURL},
		CapabilityProUpload:  {Capability: CapabilityProUpload, Supported: true, Approval: proApproval, Reason: "scheduling, comment/stat controls, and co-hosts require Mixcloud Pro", DocURL: documentationURL},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return client, true }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return client, true }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) UserWorkflow() UserWorkflow           { return client }
func (client *Client) CloudcastWorkflow() CloudcastWorkflow { return client }
func (client *Client) DiscoveryWorkflow() DiscoveryWorkflow { return client }
func (client *Client) UploadWorkflow() UploadWorkflow       { return client }
func (client *Client) LibraryWorkflow() LibraryWorkflow     { return client }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
