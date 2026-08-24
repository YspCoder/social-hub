package unityadvertising

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityAdvertisingManagement socialhub.Capability = "unity_advertising_management"

// Client exposes one Unity organization's paid user-acquisition workflows.
type Client struct {
	accountID      socialhub.AccountID
	organizationID string
	api            *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdvertisingManagement: {
			Capability: CapabilityAdvertisingManagement, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "organization-bound App, Campaign, Bid, Targeting, Budget, Creative, and Creative Pack management; Unity must grant API access",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid user acquisition is not organic publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Unity workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ad creative upload is exposed through Creatives(), not organic media upload"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Unity Ads has no organic engagement workflow"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Advertising Management API v1 has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Advertising Management API v1 does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Apps() AppsWorkflow                   { return client }
func (client *Client) Creatives() CreativesWorkflow         { return client }
func (client *Client) CreativePacks() CreativePacksWorkflow { return client }
func (client *Client) Campaigns() CampaignsWorkflow         { return client }
func (client *Client) Bids() BidsWorkflow                   { return client }

func (client *Client) organizationPath() string {
	return "/advertise/v1/organizations/" + client.organizationID
}

func (client *Client) appPath(operation, campaignSetID string) (string, error) {
	if !validMongoID(campaignSetID) {
		return "", invalidArgument(operation, "campaign set ID must be a 24-character hexadecimal ID")
	}
	return client.organizationPath() + "/apps/" + campaignSetID, nil
}

func (client *Client) campaignPath(operation, campaignSetID, campaignID string) (string, error) {
	appPath, err := client.appPath(operation, campaignSetID)
	if err != nil {
		return "", err
	}
	if !validMongoID(campaignID) {
		return "", invalidArgument(operation, "campaign ID must be a 24-character hexadecimal ID")
	}
	return appPath + "/campaigns/" + campaignID, nil
}

func (client *Client) creativePath(operation, campaignSetID, creativeID string) (string, error) {
	appPath, err := client.appPath(operation, campaignSetID)
	if err != nil {
		return "", err
	}
	if !validMongoID(creativeID) {
		return "", invalidArgument(operation, "creative ID must be a 24-character hexadecimal ID")
	}
	return appPath + "/creatives/" + creativeID, nil
}

func (client *Client) creativePackPath(operation, campaignSetID, creativePackID string) (string, error) {
	appPath, err := client.appPath(operation, campaignSetID)
	if err != nil {
		return "", err
	}
	if !validMongoID(creativePackID) {
		return "", invalidArgument(operation, "creative pack ID must be a 24-character hexadecimal ID")
	}
	return appPath + "/creative-packs/" + creativePackID, nil
}

var _ socialhub.Client = (*Client)(nil)
