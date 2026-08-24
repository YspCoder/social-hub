package panglemanagement

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const (
	CapabilityAppManagement       socialhub.Capability = "pangle_app_management"
	CapabilityPlacementManagement socialhub.Capability = "pangle_ad_placement_management"
	CapabilityExpectedCPM         socialhub.Capability = "pangle_expected_cpm_management"
)

type Client struct {
	accountID   socialhub.AccountID
	userID      ID
	roleID      ID
	securityKey string
	baseURL     *url.URL
	httpClient  *http.Client
	clock       socialhub.Clock
	sandbox     bool
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAppManagement: {
			Capability: CapabilityAppManagement, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "Pangle App create, update, status, and query workflows; an authorized role with App management permission is required",
			DocURL: documentationURL,
		},
		CapabilityPlacementManagement: {
			Capability: CapabilityPlacementManagement, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "typed Native, Banner, App Open, Rewarded Video, and Interstitial placement workflows; role permission and feature allowlists apply",
			DocURL: documentationURL,
		},
		CapabilityExpectedCPM: {
			Capability: CapabilityExpectedCPM, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "placement expected-CPM updates are permission-bound and subject to platform cooldowns",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publisher inventory management is not organic publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Pangle resources use typed Apps and Placements workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "this API configures inventory and does not upload creative media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publisher inventory has no organic engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publisher inventory has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "verification outcomes are delivered in-site rather than through this API"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Apps() AppsWorkflow             { return client }
func (client *Client) Placements() PlacementsWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
