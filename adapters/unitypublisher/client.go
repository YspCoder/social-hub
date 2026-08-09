package unitypublisher

import (
	"context"
	"fmt"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityPublisherManagement socialhub.Capability = "unity_ads_publisher_management"

// Client exposes one Unity organization's monetization configuration workflows.
type Client struct {
	accountID      socialhub.AccountID
	organizationID string
	api            *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityPublisherManagement: {
			Capability: CapabilityPublisherManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "organization-bound Application, Placement, test-mode, and Test Device management",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Unity Ads monetization configuration is not organic publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publisher resources use typed Unity workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Publisher Manage API v2 has no media upload surface"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Unity Ads has no organic engagement workflow"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Publisher Manage API v2 has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Publisher Manage API v2 does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Applications() ApplicationsWorkflow { return client }
func (client *Client) Placements() PlacementsWorkflow     { return client }
func (client *Client) TestDevices() TestDevicesWorkflow   { return client }

func (client *Client) organizationPath() string {
	return "/public/v1/organizations/" + client.organizationID
}

func (client *Client) applicationPath(operation, applicationID string) (string, error) {
	if !validPathID(applicationID) {
		return "", invalidArgument(operation, "application ID is invalid")
	}
	return client.organizationPath() + "/applications/" + applicationID, nil
}

func (client *Client) placementPath(operation, applicationID, placementID string) (string, error) {
	applicationPath, err := client.applicationPath(operation, applicationID)
	if err != nil {
		return "", err
	}
	if !validUUID(placementID) {
		return "", invalidArgument(operation, "placement ID must be a UUID; the human-readable key is not valid for path operations")
	}
	return applicationPath + "/placements/" + placementID, nil
}

func (client *Client) testDevicePath(operation, testDeviceID string) (string, error) {
	if !validUUID(testDeviceID) {
		return "", invalidArgument(operation, "test device ID must be a UUID")
	}
	return client.organizationPath() + "/test-devices/" + testDeviceID, nil
}

func ownershipError(operation, resource string) error {
	return &socialhub.Error{
		Code: socialhub.CodePermissionDenied, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: fmt.Sprintf("Unity returned a %s outside the configured organization or requested parent", resource),
	}
}

var _ socialhub.Client = (*Client)(nil)
